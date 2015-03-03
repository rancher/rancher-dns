package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
	"net"
)

// Proxy a request to an external server
func Proxy(w dns.ResponseWriter, req *dns.Msg, addr string) {
	transport := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		transport = "tcp"
	}

	ip, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	question := req.Question[0]
	rrType := dns.Type(req.Question[0].Qtype).String()

	c := &dns.Client{Net: transport}
	resp, _, err := c.Exchange(req, addr)
	if err != nil {
		dns.HandleFailed(w, req)
		log.WithFields(log.Fields{
			"question": question.Name,
			"type":     rrType,
			"client":   ip,
			"host":     addr,
			"error":    err,
		}).Error("Error making recursive request")
		return
	}

	log.WithFields(log.Fields{
		"question": question.Name,
		"type":     rrType,
		"client":   ip,
		"source":   "recurse",
	}).Info("Sent recursive response")

	w.WriteMsg(resp)
}
