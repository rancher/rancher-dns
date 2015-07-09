package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
	"strings"
)

func ResolveTryAll(fqdn string, resolvers []string) (resp *dns.Msg, err error) {
	for _, resolver := range resolvers {
		resp, err = Resolve(fqdn, resolver)
		if err == nil {
			break
		}
	}

	return
}

// Proxy a request to an external server
func Resolve(fqdn string, resolver string) (resp *dns.Msg, err error) {
	resp, err = resolveTransport("udp", fqdn, resolver)
	if err != nil {
		if resp.Truncated {
			log.Debug("Response truncated, retrying with TCP")
			resp, err = resolveTransport("tcp", fqdn, resolver)
		}
	}

	return
}

func resolveTransport(transport string, fqdn string, resolver string) (resp *dns.Msg, err error) {
	// Default to port 53
	if !strings.Contains(resolver, ":") {
		resolver = resolver + ":53"
	}

	c := &dns.Client{Net: transport}
	m := new(dns.Msg)
	m.SetQuestion(fqdn, dns.TypeA)

	resp, _, err = c.Exchange(m, resolver)
	return
}
