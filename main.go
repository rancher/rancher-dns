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
	"github.com/skynetservices/skydns/cache"
)

var (
	showVersion    = flag.Bool("version", false, "Show version")
	debug          = flag.Bool("debug", false, "Debug")
	listen         = flag.String("listen", ":53", "Address to listen to (TCP and UDP)")
	listenReload   = flag.String("listenReload", "127.0.0.1:8113", "Address to listen to for reload requests (TCP)")
	answersFile    = flag.String("answers", "./answers.yaml", "File containing the answers to respond with")
	defaultTtl     = flag.Uint("ttl", 600, "TTL for answers")
	ndots          = flag.Uint("ndots", 0, "Queries with more than this number of dots will not use search paths")
	cacheCapacity  = flag.Uint("cache-capacity", 1000, "Cache capacity")
	logFile        = flag.String("log", "", "Log file")
	pidFile        = flag.String("pid-file", "", "PID to write to")
	metadataServer = flag.String("metadata-server", "", "Metadata server url")

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
		m.RecursionAvailable = false
		m.Rcode = dns.RcodeNotImplemented
		w.WriteMsg(m)
		log.WithFields(log.Fields{"question": fqdn, "type": rrString, "client": clientIp}).Warn("Rejected non-inet query")
		return
	}

	// ANY queries are bad, mmmkay...
	if question.Qtype == dns.TypeANY {
		m.Authoritative = false
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

	if msg := clientSpecificCacheHit(clientIp, req); msg != nil {
		Respond(w, req, msg)
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debug("Sent client-specific cached response")
		return
	}

	if msg := globalCacheHit(req); msg != nil {
		Respond(w, req, msg)
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Debug("Sent globally cached response")
		return
	}

	// Try to find a matching record in the config
	found, ok := answers.Addresses(question.Qtype, clientIp, fqdn, nil, 1)
	if ok {
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn, "answers": len(found)}).Debug("Answered locally")
		if len(found) > 0 {
			m.Answer = found
		} else {
			// If the name is valid but there are no records of the
			// requested type, send back a NODATA response (RFC2308)
			domain, authoritative := answers.IsAuthoritativeDomain(fqdn)
			if !authoritative {
				domain = fqdn
			}
			m.Ns = []dns.RR{soa(domain)}
			m.Ns[0].Header().Ttl = uint32(*defaultTtl)
		}
		addToClientSpecificCache(clientIp, req, m)
		Respond(w, req, m)
		return
	}

	log.Debug("No match found in config")

	// If we are authoritative for the suffix but the queried name
	// does not exist, send back a NXDOMAIN response (RFC1035 4.1.1)
	domain, authoritative := answers.IsAuthoritativeDomain(fqdn)
	if authoritative {
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).
			Debugf("Name does not exist, but I am authoritative for %s", domain)
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeNameError)
		m.Authoritative = true
		m.RecursionAvailable = true
		m.Ns = []dns.RR{soa(domain)}
		m.Ns[0].Header().Ttl = uint32(*defaultTtl)
		Respond(w, req, m)
		return
	}

	// Phone a friend - Forward original query
	msg, err := ResolveTryAll(req, answers.Recursers(clientIp))
	if err == nil {
		msg.Compress = true
		msg.Id = req.Id
		addToGlobalCache(req, msg)
		Respond(w, req, msg)
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn, "rcode": dns.RcodeToString[msg.Rcode]}).
			Debug("Sent recursive response")
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

// soa returns a suitable SOA resource record for the specified domain
func soa(domain string) dns.RR {
	serial++
	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    uint32(*defaultTtl),
		},
		Ns:      "ns.dns." + domain,
		Mbox:    "hostmaster." + domain,
		Serial:  serial,
		Refresh: 28800,
		Retry:   7200,
		Expire:  604800,
		Minttl:  300,
	}
}
