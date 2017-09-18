package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/miekg/dns"
	"github.com/rancher/rancher-dns/cache"
)

var (
	showVersion     = flag.Bool("version", false, "Show version")
	debug           = flag.Bool("debug", false, "Debug")
	listen          = flag.String("listen", ":53", "Address to listen to (TCP and UDP)")
	listenReload    = flag.String("listenReload", "127.0.0.1:8113", "Address to listen to for reload requests (TCP)")
	answersFile     = flag.String("answers", "./answers.yaml", "File containing the answers to respond with")
	defaultTtl      = flag.Uint("ttl", 600, "TTL for answers")
	recurserTimeout = flag.Uint("recurser-timeout", 2, "timeout (in seconds) for recurser")
	ndots           = flag.Uint("ndots", 0, "Queries with more than this number of dots will not use search paths")
	cacheCapacity   = flag.Uint("cache-capacity", 1000, "Cache capacity")
	logFile         = flag.String("log", "", "Log file")
	pidFile         = flag.String("pid-file", "", "PID to write to")
	metadataServer  = flag.String("metadata-server", "", "Metadata server url")
	metadataAnswer  = flag.String("rancher-metadata-answer", "169.254.169.250", "Metadata IP address(es), comma-delimited (adds static A records)")
	neverRecurseTo  = flag.String("never-recurse-to", "169.254.169.250", "Never recurse to IP address(es), comma-delimited")

	answers                   Answers
	globalCache               *cache.Cache
	clientSpecificCaches      map[string]*cache.Cache
	clientSpecificCachesMutex sync.RWMutex
	VERSION                   string
	reloadChan                = make(chan chan error)
	serial                    = uint32(1)
	configGenerator           *ConfigGenerator
)

func metadataDriven() bool {
	return *metadataServer != ""
}

func main() {
	parseFlags()

	log.Infof("Starting rancher-dns %s", VERSION)
	err := loadAnswers()
	if err != nil {
		log.Fatal("Cannot startup without a valid Answers file")
	}

	if *showVersion {
		fmt.Printf("%s\n", VERSION)
		os.Exit(0)
	}

	if metadataDriven() {
		configGenerator = &ConfigGenerator{}
		err = configGenerator.Init(metadataServer)
		if err != nil {
			log.Fatalf("Cannot startup: failed to init config generator: %v", err)
		}
	}

	watchSignals()
	watchHttp()

	seed := time.Now().UTC().UnixNano()
	log.Debug("Set random seed to ", seed)
	rand.Seed(seed)

	udpServer := &dns.Server{Addr: *listen, Net: "udp"}
	tcpServer := &dns.Server{Addr: *listen, Net: "tcp"}

	globalCache = cache.New(int(*cacheCapacity), int(*defaultTtl))
	clientSpecificCaches = make(map[string]*cache.Cache)

	dns.HandleFunc(".", route)

	go func() {
		log.Fatal(udpServer.ListenAndServe())
	}()
	log.Info("Listening on ", *listen)
	log.Fatal(tcpServer.ListenAndServe())
}

func parseFlags() {
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	if *logFile != "" {
		if output, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
			log.Fatalf("Failed to log to file %s: %v", *logFile, err)
		} else {
			log.SetOutput(output)
		}
	}

	if *pidFile != "" {
		log.Infof("Writing pid %d to %s", os.Getpid(), *pidFile)
		if err := ioutil.WriteFile(*pidFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
			log.Fatalf("Failed to write pid file %s: %v", *pidFile, err)
		}
	}
}

func loadAnswersFromMeta(name string) {
	newAnswers, err := configGenerator.GenerateAnswers()
	if err != nil {
		log.Errorf("Failed to generate answers: %v", err)
		return
	}
	ConvertPtrIps(&newAnswers)

	if reflect.DeepEqual(newAnswers, answers) {
		log.Debug("No changes in dns data")
		return
	}

	log.Infof("Reloading answers")
	clearClientSpecificCaches()
	answers = newAnswers
	// write to file (debugging purposes)
	b, err := json.Marshal(answers)
	if err != nil {
		log.Errorf("Failed to marshall answers: %v", err)
	}
	err = ioutil.WriteFile(*answersFile, b, 0644)
	if err != nil {
		log.Errorf("Failed to write answers to file: %v", err)
	}
	log.Infof("Reloaded answers")
}

func loadAnswers() (err error) {
	log.Debug("Loading answers")
	temp, err := ParseAnswers(*answersFile)
	if err == nil {
		clearClientSpecificCaches()
		answers = temp
		log.Infof("Loaded answers")
	} else {
		log.Errorf("Failed to load answers: %v", err)
	}

	return err
}

func watchSignals() {
	if metadataDriven() {
		go configGenerator.metaFetcher.OnChange(5, loadAnswersFromMeta)
	} else {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGHUP)

		go func() {
			for _ = range c {
				log.Info("Received HUP signal")
				reloadChan <- nil
			}
		}()

		go func() {
			for resp := range reloadChan {
				err := loadAnswers()
				if resp != nil {
					resp <- err
				}
			}
		}()
	}

}

func watchHttp() {
	reloadRouter := mux.NewRouter()
	reloadRouter.HandleFunc("/v1/reload", httpReload).Methods("POST")
	log.Info("Listening for Reload on ", *listenReload)
	go http.ListenAndServe(*listenReload, reloadRouter)
}

func httpReload(w http.ResponseWriter, req *http.Request) {
	log.Debugf("Received HTTP reload request")
	respChan := make(chan error)
	reloadChan <- respChan
	err := <-respChan

	if err == nil {
		io.WriteString(w, "OK")
	} else {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
	}
}

func route(w dns.ResponseWriter, req *dns.Msg) {
	// Setup reply
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true
	m.RecursionAvailable = true
	m.Compress = true

	clientIp, _, _ := net.SplitHostPort(w.RemoteAddr().String())

	// One question at a time please
	if len(req.Question) != 1 {
		dns.HandleFailed(w, req)
		log.WithFields(log.Fields{"client": clientIp}).Warn("Rejected multi-question query")
		return
	}

	question := req.Question[0]
	rrString := dns.Type(question.Qtype).String()

	// We are assuming the config has all names as lower case
	fqdn := strings.ToLower(question.Name)

	// Internets only
	if question.Qclass != dns.ClassINET {
		m.Authoritative = false
		m.RecursionDesired = false
		m.RecursionAvailable = false
		m.Rcode = dns.RcodeNotImplemented
		w.WriteMsg(m)
		log.WithFields(log.Fields{"question": fqdn, "type": rrString, "client": clientIp}).Warn("Rejected non-inet query")
		return
	}

	// ANY queries are bad, mmmkay...
	if question.Qtype == dns.TypeANY {
		m.Authoritative = false
		m.RecursionDesired = false
		m.RecursionAvailable = false
		m.Rcode = dns.RcodeNotImplemented
		w.WriteMsg(m)
		log.WithFields(log.Fields{"question": fqdn, "type": rrString, "client": clientIp}).Warn("Rejected ANY query")
		return
	}

	proto := "UDP"
	if isTcp(w) {
		proto = "TCP"
	}

	log.WithFields(log.Fields{"question": fqdn, "type": rrString, "client": clientIp, "proto": proto}).Debug("Request")

	if msg, exp := clientSpecificCacheHit(clientIp, req); msg != nil {
		update(msg, exp)
		Respond(w, req, msg)
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debug("Sent client-specific cached response")
		return
	}

	// A records may return CNAME answer(s) plus A answer(s)
	if question.Qtype == dns.TypeA {
		found, ok := answers.Addresses(clientIp, fqdn, nil, 1)
		if ok && len(found) > 0 {
			log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn, "answers": len(found)}).Debug("Answered locally")
			m.Answer = found
			addToClientSpecificCache(clientIp, req, m)
			Respond(w, req, m)
			return
		}
	} else if question.Qtype == dns.TypeAAAA {
		_, ok := answers.Addresses(clientIp, fqdn, nil, 1)
		if ok {
			log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debug("Answered locally, no error and empty answer")
			m.Authoritative = true
			m.Rcode = dns.RcodeSuccess
			addToClientSpecificCache(clientIp, req, m)
			Respond(w, req, m)
			return
		}
	} else {
		// Specific request for another kind of record
		keys := []string{clientIp, DEFAULT_KEY}
		for _, key := range keys {
			// Client-specific answers
			found, ok := answers.Matching(question.Qtype, key, fqdn)
			if ok {
				log.WithFields(log.Fields{"client": key, "type": rrString, "question": fqdn, "answers": len(found)}).Debug("Answered from config for ", key)
				m.Answer = found
				addToClientSpecificCache(clientIp, req, m)
				Respond(w, req, m)
				return
			}
		}

		log.Debug("No match found in config")
	}

	if msg, exp := globalCacheHit(req); msg != nil {
		update(msg, exp)
		Respond(w, req, msg)
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debug("Sent globally cached response")
		return
	}

	// If we are authoritative for a suffix the label has, there's no point trying the recursive DNS
	authoritativeFor := answers.AuthoritativeSuffixes()
	for _, suffix := range authoritativeFor {
		if strings.HasSuffix(fqdn, suffix) {
			log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debugf("Not answered locally, but I am authoritative for %s", suffix)
			m.Authoritative = true
			m.RecursionAvailable = false
			m.Rcode = dns.RcodeNameError
			me := strings.TrimLeft(suffix, ".")
			hdr := dns.RR_Header{Name: me, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: uint32(*defaultTtl)}
			serial++
			record := &dns.SOA{Hdr: hdr, Ns: me, Mbox: me, Serial: serial, Refresh: 60, Retry: 10, Expire: 86400, Minttl: 1}
			m.Ns = append(m.Ns, record)
			Respond(w, req, m)
			return
		}
	}

	// Phone a friend - Forward original query
	msg, err := ResolveTryAll(req, answers.Recursers(clientIp))
	if err == nil && msg != nil {
		msg.Compress = true
		msg.Id = req.Id

		// We don't support AAAA, but an NXDOMAIN from the recursive resolver
		// doesn't necessarily mean there are never any records for that domain,
		// so rewrite the response code to NOERROR.
		if (question.Qtype == dns.TypeAAAA) && (msg.Rcode == dns.RcodeNameError) {
			log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debug("Rewrote AAAA NXDOMAIN to NOERROR")
			msg.Rcode = dns.RcodeSuccess
		}

		addToGlobalCache(req, msg)
		if msg, exp := globalCacheHit(req); msg != nil {
			update(msg, exp)
			Respond(w, req, msg)
			log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debug("Sent recursive response")
			return
		}
		// For very small TTLs, globalCacheHit above could fail despite adding - respond with the original msg.
		Respond(w, req, msg)
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debug("Sent recursive response")
		return
	}

	// I give up
	log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Info("No answer found")
	dns.HandleFailed(w, req)
}

func isTcp(w dns.ResponseWriter) bool {
	_, ok := w.RemoteAddr().(*net.TCPAddr)
	return ok
}

func update(msg *dns.Msg, exp time.Time) {
	if len(msg.Answer) > 1 {
		shuffle(&msg.Answer)
	}
	var ttl = uint32(time.Until(exp).Seconds())
	for i := 0; i < len(msg.Answer); i++ {
		msg.Answer[i].Header().Ttl = ttl
	}
}
