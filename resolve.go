package main

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
	"strings"
)

func ResolveTryAll(req *dns.Msg, resolvers []string) (resp *dns.Msg, err error) {
	for _, resolver := range resolvers {
		log.WithFields(log.Fields{"fqdn": req.Question[0].Name, "resolver": resolver}).Debug("Recursing")
		resp, err = Resolve(req, resolver)
		if err == nil && resp != nil {
			break
		}
	}

	return
}

// Proxy a request to an external server
func Resolve(req *dns.Msg, resolver string) (resp *dns.Msg, err error) {
	resp, err = resolveTransport(req, "udp", resolver)
	if err != nil {
		if resp != nil && resp.Truncated {
			log.Debug("Response truncated, retrying with TCP")
			resp, err = resolveTransport(req, "tcp", resolver)
		} else {
			log.WithFields(log.Fields{"fqdn": req.Question[0].Name, "resolver": resolver}).Warn("Recurser error: ", err)
		}
	}

	return
}

func resolveTransport(req *dns.Msg, transport, resolver string) (resp *dns.Msg, err error) {
	// Default to port 53
	if !strings.Contains(resolver, ":") {
		resolver = resolver + ":53"
	}

	t := time.Duration(*recurserTimeout) * time.Second
	c := &dns.Client{
		Net:          transport,
		DialTimeout:  t,
		ReadTimeout:  t,
		WriteTimeout: t,
	}

	resp, _, err = c.Exchange(req, resolver)
	return
}
