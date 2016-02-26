package main

import (
	"math/rand"
	"net"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
)

// The top-level key in the JSON for the default (not client-specific answers)
const DEFAULT_KEY = "default"

// The 2nd-level key in the JSON for the recursive resolver addresses
const RECURSE_KEY = "recurse"

// Maximum recursion when resolving CNAMEs
const MAX_DEPTH = 10

// Recursive servers
func (answers *Answers) Recursers(clientIp string) []string {
	var hosts []string
	more := answers.recursersFor(clientIp)
	if len(more) > 0 {
		hosts = append(hosts, more...)
	}
	more = answers.recursersFor(DEFAULT_KEY)
	if len(more) > 0 {
		hosts = append(hosts, more...)
	}

	return hosts
}

func (answers *Answers) recursersFor(clientIp string) []string {
	var hosts []string
	client, ok := (*answers)[clientIp]
	if ok {
		if ok && len(client.Recurse) > 0 {
			hosts = client.Recurse
		}
	}

	return hosts
}

// Search suffixes
func (answers *Answers) SearchSuffixes(clientIp string) []string {
	var hosts []string
	client, ok := (*answers)[clientIp]
	if ok {
		if ok && len(client.Search) > 0 {
			hosts = client.Search
		}
	}

	return hosts
}

func (answers *Answers) Addresses(clientIp string, fqdn string, cnameParents []dns.RR, depth int, searches []string) (records []dns.RR, ok bool) {
	fqdn = dns.Fqdn(fqdn)

	log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp, "depth": depth}).Debug("Trying to resolve addresses")

	// Limit recursing for non-obvious loops
	if len(cnameParents) >= MAX_DEPTH {
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Warn("Followed CNAME too many times ", cnameParents)
		return nil, false
	}

	// Look for a CNAME entry
	result, ok := answers.MatchingAny(dns.TypeCNAME, clientIp, fqdn, searches)
	if ok && len(result) > 0 {
		cname := result[0].(*dns.CNAME)
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Matched CNAME ", cname.Target)

		// Stop obvious loops
		if dns.Fqdn(cname.Target) == fqdn {
			log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Warn("CNAME is a loop ", cname.Target)
			return nil, false
		}

		// Recurse to find the eventual A for this CNAME
		children, ok := answers.Addresses(clientIp, dns.Fqdn(cname.Target), append(cnameParents, cname), depth+1, searches)
		if ok && len(children) > 0 {
			log.WithFields(log.Fields{"fqdn": fqdn, "target": cname.Target, "client": clientIp}).Debug("Resolved CNAME ", children)
			records = append(records, cname)
			records = append(records, children...)
			return records, true
		}
	}

	// Look for an A entry
	result, ok = answers.MatchingAny(dns.TypeA, clientIp, fqdn, searches)
	if ok && len(result) > 0 {
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Matched A ", result)
		shuffle(&result)
		return result, true
	}

	// Try the default section of the config
	if clientIp != DEFAULT_KEY {
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Trying defaults")
		return answers.Addresses(DEFAULT_KEY, fqdn, cnameParents, depth+1, searches)
	}

	// When resolving CNAMES, check recursive server
	if len(cnameParents) > 0 {
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Trying recursive servers")
		r := new(dns.Msg)
		r.SetQuestion(fqdn, dns.TypeA)
		msg, err := ResolveTryAll(r, answers.Recursers(clientIp))
		if err == nil {
			return msg.Answer, true
		}
	}

	log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Did not match anything")
	return nil, false
}

func (answers *Answers) MatchingAny(qtype uint16, clientIp string, label string, searches []string) (records []dns.RR, ok bool) {
	records, ok = answers.Matching(qtype, clientIp, label)
	if ok {
		log.WithFields(log.Fields{"fqdn": label, "client": clientIp}).Debug("Matched exact FQDN")
		return
	}

	if searches != nil && len(searches) > 0 {
		for _, suffix := range searches {
			newFqdn := strings.TrimRight(label, ".") + "." + strings.TrimRight(suffix, ".") + "."

			records, ok = answers.Matching(qtype, clientIp, newFqdn)
			if ok {
				log.WithFields(log.Fields{"fqdn": newFqdn, "client": clientIp}).Debug("Matched alternate suffix")
				return
			}
		}
	}

	return nil, false
}

func (answers *Answers) Matching(qtype uint16, clientIp string, fqdn string) (records []dns.RR, ok bool) {
	client, ok := (*answers)[clientIp]
	if ok {
		switch qtype {
		case dns.TypeA:
			log.WithFields(log.Fields{"qtype": "A", "client": clientIp, "fqdn": fqdn}).Debug("Searching for A")
			res, ok := client.A[fqdn]
			if ok && len(res.Answer) > 0 {
				ttl := uint32(*defaultTtl)
				if res.Ttl != nil {
					ttl = *res.Ttl
				}

				for i := 0; i < len(res.Answer); i++ {
					hdr := dns.RR_Header{Name: fqdn, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl}
					ip := net.ParseIP(res.Answer[i])
					record := &dns.A{Hdr: hdr, A: ip}
					records = append(records, record)
				}

				shuffle(&records)
			}

		case dns.TypeCNAME:
			log.WithFields(log.Fields{"qtype": "CNAME", "client": clientIp, "fqdn": fqdn}).Debug("Searching for CNAME")
			res, ok := client.Cname[fqdn]
			ttl := uint32(*defaultTtl)
			if res.Ttl != nil {
				ttl = *res.Ttl
			}

			if ok {
				hdr := dns.RR_Header{Name: fqdn, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl}
				record := &dns.CNAME{Hdr: hdr, Target: res.Answer}
				records = append(records, record)
			}

		case dns.TypePTR:
			log.WithFields(log.Fields{"qtype": "PTR", "client": clientIp, "fqdn": fqdn}).Debug("Searching for PTR")
			res, ok := client.Ptr[fqdn]
			ttl := uint32(*defaultTtl)
			if res.Ttl != nil {
				ttl = *res.Ttl
			}

			if ok {
				hdr := dns.RR_Header{Name: fqdn, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: ttl}
				record := &dns.PTR{Hdr: hdr, Ptr: res.Answer}
				records = append(records, record)
			}

		case dns.TypeTXT:
			log.WithFields(log.Fields{"qtype": "TXT", "client": clientIp, "fqdn": fqdn}).Debug("Searching for TXT")
			res, ok := client.Txt[fqdn]
			ttl := uint32(*defaultTtl)
			if res.Ttl != nil {
				ttl = *res.Ttl
			}

			if ok {
				for i := 0; i < len(res.Answer); i++ {
					hdr := dns.RR_Header{Name: fqdn, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl}
					str := res.Answer[i]
					if len(str) > 255 {
						log.WithFields(log.Fields{"qtype": "TXT", "client": clientIp, "fqdn": fqdn}).Warn("TXT record too long: ", str)
						return nil, false
					}
					record := &dns.TXT{Hdr: hdr, Txt: []string{str}}
					records = append(records, record)
				}
			}
		}
	}

	if len(records) > 0 {
		return records, true
	} else {
		return nil, false
	}
}

func shuffle(items *[]dns.RR) {

	for i := range *items {
		j := rand.Intn(i + 1)
		(*items)[i], (*items)[j] = (*items)[j], (*items)[i]
	}
}
