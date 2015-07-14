package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
	"strings"
)

func ResolveTryAll(fqdn string, qtype uint16, resolvers []string) (resp *dns.Msg, err error) {
	for _, resolver := range resolvers {
		log.WithFields(log.Fields{"fqdn": fqdn, "resolver": resolver}).Debug("Recursing")
		resp, err = Resolve(fqdn, qtype, resolver)
		if err == nil {
			break
		}
	}

	return
}

// Proxy a request to an external server
func Resolve(fqdn string, qtype uint16, resolver string) (resp *dns.Msg, err error) {
	resp, err = resolveTransport("udp", fqdn, qtype, resolver)
	if err != nil {
		if resp != nil && resp.Truncated {
			log.Debug("Response truncated, retrying with TCP")
			resp, err = resolveTransport("tcp", fqdn, qtype, resolver)
		} else {
			log.WithFields(log.Fields{"fqdn": fqdn, "resolver": resolver}).Warn("Recurser error: ", err)
		}
	}

	return
}

func resolveTransport(transport string, fqdn string, qtype uint16, resolver string) (resp *dns.Msg, err error) {
	// Default to port 53
	if !strings.Contains(resolver, ":") {
		resolver = resolver + ":53"
	}

	c := &dns.Client{Net: transport}
	m := new(dns.Msg)
	m.SetQuestion(fqdn, qtype)

	resp, _, err = c.Exchange(m, resolver)
	return
}
