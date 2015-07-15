package main

import (
	"flag"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
)

var (
	debug       = flag.Bool("debug", false, "Debug")
	listen      = flag.String("listen", ":53", "Address to listen to (TCP and UDP)")
	answersFile = flag.String("answers", "./answers.json", "File containing the answers to respond with")
	defaultTtl  = flag.Uint("ttl", 600, "TTL for answers")
	logFile     = flag.String("log", "", "Log file")
	pidFile     = flag.String("pid-file", "", "PID to write to")

	answers Answers
)

func main() {
	log.Info("Starting rancher-dns")
	parseFlags()
	err := loadAnswers()
	if err != nil {
		log.Fatal("Cannot startup without a valid Answers file")
	}
	watchSignals()

	seed := time.Now().UTC().UnixNano()
	log.Debug("Set random seed to ", seed)
	rand.Seed(seed)

	udpServer := &dns.Server{Addr: *listen, Net: "udp"}
	tcpServer := &dns.Server{Addr: *listen, Net: "tcp"}

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

func loadAnswers() (err error) {
	temp, err := ParseAnswers(*answersFile)
	if err == nil {
		answers = temp
		log.Info("Loaded answers for ", len(answers), " IPs")
	} else {
		log.Errorf("Failed to load answers: %v", err)
	}

	return err
}

func watchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	go func() {
		for _ = range c {
			log.Info("Received HUP signal, reloading answers")
			loadAnswers()
		}
	}()
}

func route(w dns.ResponseWriter, req *dns.Msg) {
	clientIp, _, _ := net.SplitHostPort(w.RemoteAddr().String())

	// One question at a time please
	if len(req.Question) != 1 {
		dns.HandleFailed(w, req)
		log.WithFields(log.Fields{"client": clientIp}).Warn("Rejected multi-question query")
		return
	}

	question := req.Question[0]
	rrString := dns.Type(question.Qtype).String()

	// We are assuming the JSON config has all names as lower case
	fqdn := strings.ToLower(question.Name)

	// Internets only
	if question.Qclass != dns.ClassINET {
		m := new(dns.Msg)
		m.SetReply(req)
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
		m := new(dns.Msg)
		m.SetReply(req)
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

	// A records may return CNAME answer(s) plus A answer(s)
	if question.Qtype == dns.TypeA {
		found, ok := answers.Addresses(clientIp, fqdn, nil, 1)
		if ok && len(found) > 0 {
			log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn, "answers": len(found)}).Info("Answered locally")
			Respond(w, req, found)
			return
		}
	} else {
		// Specific request for another kind of record
		keys := []string{clientIp, DEFAULT_KEY}
		for _, key := range keys {
			// Client-specific answers
			found, ok := answers.Matching(question.Qtype, key, fqdn)
			if ok {
				log.WithFields(log.Fields{"client": key, "type": rrString, "question": fqdn, "answers": len(found)}).Info("Answered from config for ", key)
				Respond(w, req, found)
				return
			}
		}

		log.Debug("No match found in config")
	}

	// Phone a friend
	msg, err := ResolveTryAll(fqdn, question.Qtype, answers.Recursers(clientIp))
	if err == nil {
		msg.SetReply(req)
		w.WriteMsg(msg)
		log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Info("Sent recursive response")
		return
	}

	// I give up
	log.WithFields(log.Fields{"client": clientIp, "type": rrString, "question": fqdn}).Warn("No answer found")
	dns.HandleFailed(w, req)
}

func isTcp(w dns.ResponseWriter) bool {
	_, ok := w.RemoteAddr().(*net.TCPAddr)
	return ok
}
