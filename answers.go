package main

import (
	"math/rand"
	"net"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
)

// The top-level key in the config for the default (not client-specific answers)
const DEFAULT_KEY = "default"

// The 2nd-level key in the config for the recursive resolver addresses
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
	var suffixes []string
	client, ok := (*answers)[clientIp]
	if ok {
		if ok && len(client.Search) > 0 {
			suffixes = client.Search
		}
	}

	return suffixes
}

// Authoritative suffixes
func (answers *Answers) AuthoritativeSuffixes() []string {
	var suffixes []string
	client, ok := (*answers)[DEFAULT_KEY]
	if ok && len(client.Authoritative) > 0 {
		for _, suffix := range client.Authoritative {
			withDots := "." + strings.Trim(suffix, ".") + "."
			suffixes = append(suffixes, withDots)
		}
	}

	return suffixes
}

// Checks whether the specified FQDN is under a domain that
// we are authoritative for and if so returns that domain
func (answers *Answers) IsAuthoritativeDomain(fqdn string) (string, bool) {
	authoritativeFor := answers.AuthoritativeSuffixes()
	for _, suffix := range authoritativeFor {
		if strings.HasSuffix(fqdn, suffix) {
			domain := strings.TrimLeft(suffix, ".")
			return domain, true
		}
	}

	return "", false
}

func (answers *Answers) Addresses(qtype uint16, clientIp string, fqdn string, cnameParents []dns.RR, depth int) (records []dns.RR, ok bool) {
	fqdn = dns.Fqdn(fqdn)
	log.WithFields(log.Fields{"fqdn": fqdn, "qtype": dns.Type(qtype).String(), "client": clientIp, "depth": depth}).Debug("Trying to resolve addresses")

	// Limit recursing for non-obvious loops
	if len(cnameParents) >= MAX_DEPTH {
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp, "depth": depth}).Warn("Followed CNAME too many times ", cnameParents)
		return nil, false
	}

	// Look for a CNAME entry
	log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp, "depth": depth}).Debug("Trying CNAME Records")
	result, ok := answers.Matching(dns.TypeCNAME, clientIp, fqdn)
	if ok && len(result) > 0 {
		cname := result[0].(*dns.CNAME)
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp, "depth": depth}).Debug("Matched CNAME ", cname.Target)

		// Stop obvious loops
		if dns.Fqdn(cname.Target) == fqdn {
			log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp, "depth": depth}).Warn("CNAME is a loop ", cname.Target)
			return nil, false
		}

		// Recurse to resolve the target for this CNAME
		children, ok := answers.Addresses(qtype, clientIp, dns.Fqdn(cname.Target), append(cnameParents, cname), depth+1)
		if ok {
			log.WithFields(log.Fields{"fqdn": fqdn, "qtype": dns.Type(qtype).String(), "target": cname.Target, "client": clientIp, "depth": depth}).
				Debug("Resolved CNAME ", children)
			records = append(records, cname)
			records = append(records, children...)
			return records, true
		}
	}

	// Look for a matching record entry
	log.WithFields(log.Fields{"fqdn": fqdn, "qtype": dns.Type(qtype).String(), "client": clientIp, "depth": depth}).Debug("Trying Qtype records")
	result, ok = answers.Matching(qtype, clientIp, fqdn)
	if ok {
		if len(result) > 0 {
			shuffle(&result)
			log.WithFields(log.Fields{"fqdn": fqdn, "qtype": dns.Type(qtype).String(), "client": clientIp, "depth": depth}).Debug("Matched record ", result)
		} else {
			log.WithFields(log.Fields{"fqdn": fqdn, "qtype": dns.Type(qtype).String(), "client": clientIp, "depth": depth}).Debug("Matched name but not type")
		}

		return result, true
	}

	// When resolving CNAMES, check recursive server
	if len(cnameParents) > 0 {
		log.WithFields(log.Fields{"fqdn": fqdn, "qtype": dns.Type(qtype).String(), "client": clientIp, "depth": depth}).Debug("Trying recursive servers")
		r := new(dns.Msg)
		r.SetQuestion(fqdn, qtype)
		msg, err := ResolveTryAll(r, answers.Recursers(clientIp))
		if err == nil {
			return msg.Answer, true
		}
	}

	log.WithFields(log.Fields{"fqdn": fqdn, "qtype": dns.Type(qtype).String(), "client": clientIp, "depth": depth}).Debug("Did not match anything")
	return nil, false
}

func (answers *Answers) Matching(qtype uint16, clientIp string, label string) (records []dns.RR, ok bool) {
	authoritativeFor := answers.AuthoritativeSuffixes()
	authoritative := false
	for _, suffix := range authoritativeFor {
		if strings.HasSuffix(label, suffix) {
			authoritative = true
			break
		}
	}

	// Only do a search if the name does not have a suffix for which we are authoritative
	var clientSearches, defaultSearches []string
	if !authoritative {
		clientSearches = answers.SearchSuffixes(clientIp)
		defaultSearches = answers.SearchSuffixes(DEFAULT_KEY)
	}

	// Client answers, client search
	log.WithFields(log.Fields{"label": label, "client": clientIp}).Debug("Trying client answers, client search")
	records, ok = answers.MatchingSearch(qtype, clientIp, label, clientSearches)
	if ok {
		return
	}

	// Default answers, client search
	log.WithFields(log.Fields{"label": label, "client": clientIp}).Debug("Trying default answers, client search")
	records, ok = answers.MatchingSearch(qtype, DEFAULT_KEY, label, clientSearches)
	if ok {
		return
	}

	// Default answers, default search
	log.WithFields(log.Fields{"label": label, "client": clientIp}).Debug("Trying default answers, default search")
	records, ok = answers.MatchingSearch(qtype, DEFAULT_KEY, label, defaultSearches)
	if ok {
		return
	}

	return nil, false
}

func (answers *Answers) MatchingSearch(qtype uint16, clientIp string, label string, searches []string) (records []dns.RR, ok bool) {
	records, ok = answers.MatchingExact(qtype, clientIp, label, label)
	if ok {
		log.WithFields(log.Fields{"fqdn": label, "client": clientIp}).Debug("Matched exact FQDN")
		return
	}

	base := strings.TrimRight(label, ".")
	limit := int(*ndots)
	if limit == 0 || strings.Count(base, ".") < limit {
		if searches != nil && len(searches) > 0 {
			for _, suffix := range searches {
				newFqdn := base + "." + strings.TrimRight(suffix, ".") + "."
				log.WithFields(log.Fields{"fqdn": newFqdn, "client": clientIp}).Debug("Trying alternate suffix")

				records, ok = answers.MatchingExact(qtype, clientIp, newFqdn, label)
				if ok {
					log.WithFields(log.Fields{"fqdn": newFqdn, "client": clientIp}).Debug("Matched alternate suffix")
					return
				}
			}
		}
	}

	return nil, false
}

func (answers *Answers) MatchingExact(qtype uint16, clientIp string, fqdn string, answerFqdn string) (records []dns.RR, ok bool) {
	client, ok := (*answers)[clientIp]
	if ok {
		switch qtype {
		case dns.TypeA:
			//log.WithFields(log.Fields{"qtype": "A", "client": clientIp, "fqdn": fqdn}).Debug("Searching for A")
			res, ok := client.A[fqdn]
			if ok && len(res.Answer) > 0 {
				ttl := uint32(*defaultTtl)
				if res.Ttl != nil {
					ttl = *res.Ttl
				}

				for i := 0; i < len(res.Answer); i++ {
					hdr := dns.RR_Header{Name: answerFqdn, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl}
					ip := net.ParseIP(res.Answer[i])
					record := &dns.A{Hdr: hdr, A: ip}
					records = append(records, record)
				}

				shuffle(&records)
			}

		case dns.TypeCNAME:
			//log.WithFields(log.Fields{"qtype": "CNAME", "client": clientIp, "fqdn": fqdn}).Debug("Searching for CNAME")
			res, ok := client.Cname[fqdn]
			ttl := uint32(*defaultTtl)
			if res.Ttl != nil {
				ttl = *res.Ttl
			}

			if ok {
				hdr := dns.RR_Header{Name: answerFqdn, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl}
				record := &dns.CNAME{Hdr: hdr, Target: res.Answer}
				records = append(records, record)
			}

		case dns.TypePTR:
			//log.WithFields(log.Fields{"qtype": "PTR", "client": clientIp, "fqdn": fqdn}).Debug("Searching for PTR")
			res, ok := client.Ptr[fqdn]
			ttl := uint32(*defaultTtl)
			if res.Ttl != nil {
				ttl = *res.Ttl
			}

			if ok {
				hdr := dns.RR_Header{Name: answerFqdn, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: ttl}
				record := &dns.PTR{Hdr: hdr, Ptr: res.Answer}
				records = append(records, record)
			}

		case dns.TypeTXT:
			//log.WithFields(log.Fields{"qtype": "TXT", "client": clientIp, "fqdn": fqdn}).Debug("Searching for TXT")
			res, ok := client.Txt[fqdn]
			ttl := uint32(*defaultTtl)
			if res.Ttl != nil {
				ttl = *res.Ttl
			}

			if ok {
				for i := 0; i < len(res.Answer); i++ {
					hdr := dns.RR_Header{Name: answerFqdn, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl}
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

		if len(records) > 0 {
			return records, true
		}

		// Check whether there are any records for that name at all
		// and if so return the empty records slice and the bool set
		// to True. This allows the caller to determine if the name
		// is non-existent or if there is just no data for the type.
		if _, ok := client.A[fqdn]; ok {
			return records, true
		}

		if _, ok := client.Cname[fqdn]; ok {
			return records, true
		}

		if _, ok := client.Ptr[fqdn]; ok {
			return records, true
		}

		if _, ok := client.Txt[fqdn]; ok {
			return records, true
		}
	}

	return nil, false
}

func shuffle(items *[]dns.RR) {

	for i := range *items {
		j := rand.Intn(i + 1)
		(*items)[i], (*items)[j] = (*items)[j], (*items)[i]
	}
}
