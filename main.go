package main

import (
	"flag"
	"net"
	//	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
)

var (
	debug       = flag.Bool("debug", false, "Debug")
	listen      = flag.String("listen", ":53", "Address to listen to (TCP and UDP)")
	recurseAddr = flag.String("recurse", "", "Default DNS server where to send queries if no answers matched (IP[:port])")
	answersFile = flag.String("answers", "./answers.json", "File containing the answers to respond with")
	ttl         = flag.Uint("ttl", 600, "TTL for answers")

	answers Answers
)

func main() {
	log.Info("Starting rancher-dns.")
	parseFlags()
	loadAnswers()

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

	if *recurseAddr == "" {
		log.Fatal("--recurse is required")
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	// Default to port 53
	host, port, err := net.SplitHostPort(*recurseAddr)
	if err != nil {
		log.Fatal("Error parsing recurseAddr")
		if port == "" {
			port = "53"
		}

		*recurseAddr = net.JoinHostPort(host, port)
	}

}

func loadAnswers() {
	var err error

	log.Info("Loading answers")
	answers, err = ReadAnswersFile(*answersFile)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Loaded answers for ", len(answers), " IPs")
}

func route(w dns.ResponseWriter, req *dns.Msg) {
	if len(req.Question) == 0 {
		dns.HandleFailed(w, req)
		return
	}

	clientIp, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	question := req.Question[0]
	rrType := dns.Type(req.Question[0].Qtype).String()

	log.WithFields(log.Fields{
		"question": question.Name,
		"type":     rrType,
		"client":   clientIp,
	}).Debug("Request")

	// Client-specific answers
	found, ok := LocalAnswer(&question, rrType, clientIp)
	if ok {
		log.WithFields(log.Fields{
			"question": question.Name,
			"type":     rrType,
			"client":   clientIp,
			"from":     clientIp,
			"found":    len(found),
		}).Info("Found match for client")

		Respond(w, req, found)
		return
	}

	// Not-client-specific answers
	found, ok = DefaultAnswer(&question, rrType, clientIp)
	if ok {
		log.WithFields(log.Fields{
			"question": question.Name,
			"type":     rrType,
			"client":   clientIp,
			"from":     DEFAULT_KEY,
			"found":    len(found),
		}).Info("Found match in", DEFAULT_KEY)

		Respond(w, req, found)
		return
	}

	// Phone a friend
	Proxy(w, req, *recurseAddr)
}
