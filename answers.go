package main

import (
	"math/rand"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/rancher/log"
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

func (answers *Answers) Addresses(clientIp string, fqdn string, cnameParents []dns.RR, depth int) (records []dns.RR, ok bool) {
	fqdn = dns.Fqdn(fqdn)

	log.Debugf("Trying to resolve addresses fqdn=%v client=%v depth=%v", fqdn, clientIp, depth)

	// Limit recursing for non-obvious loops
	if len(cnameParents) >= MAX_DEPTH {
		log.Warnf("Followed CNAME too many times %v fqdn=%v client=%v depth=%v", cnameParents, fqdn, clientIp, depth)
		return nil, false
	}

	// Look for a CNAME entry
	log.Debugf("Trying CNAME Records fqdn=%v client=%v depth=%v", fqdn, clientIp, depth)
	result, ok := answers.Matching(dns.TypeCNAME, clientIp, fqdn)
	if ok && len(result) > 0 {
		cname := result[0].(*dns.CNAME)
		log.Debugf("Matched CNAME %v fqdn=%v client=%v depth=%v", cname.Target, fqdn, clientIp, depth)

		// Stop obvious loops
		if dns.Fqdn(cname.Target) == fqdn {
			log.Warnf("CNAME is a loop %v fqdn=%v client=%v depth=%v", cname.Target, fqdn, clientIp, depth)
			return nil, false
		}

		// Recurse to find the eventual A for this CNAME
		children, ok := answers.Addresses(clientIp, dns.Fqdn(cname.Target), append(cnameParents, cname), depth+1)
		if ok && len(children) > 0 {
			log.Debugf("Resolved CNAME %v fqdn=%v target=%v client=%v depth=%v", children, fqdn, cname.Target, clientIp, depth)
			records = append(records, cname)
			records = append(records, children...)
			return records, true
		}
	}

	// Look for an A entry
	log.Debugf("Trying A Records fqdn=%v client=%v depth=%v", fqdn, clientIp, depth)
	result, ok = answers.Matching(dns.TypeA, clientIp, fqdn)
	if ok && len(result) > 0 {
		log.Debugf("Matched A %v fqdn=%v client=%v depth=%v", result, fqdn, clientIp, depth)
		shuffle(&result)
		return result, true
	}

	// When resolving CNAMES, check recursive server
	if len(cnameParents) > 0 {
		log.Debugf("Trying recursive servers fqdn=%v client=%v depth=%v", fqdn, clientIp, depth)
		r := new(dns.Msg)
		r.SetQuestion(fqdn, dns.TypeA)
		msg, err := ResolveTryAll(r, answers.Recursers(clientIp))
		if err == nil {
			return msg.Answer, true
		}
	}

	log.Debugf("Did not match anything fqdn=%v client=%v depth=%v", fqdn, clientIp, depth)
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

	// If we are authoritative for a suffix the label has, there's no point trying alternate search suffixes
	var clientSearches []string
	if authoritative {
		clientSearches = []string{}
	} else {
		clientSearches = answers.SearchSuffixes(clientIp)
	}

	// Client answers, client search
	log.Debugf("Trying client answers, client search label=%v client=%v", label, clientIp)
	records, ok = answers.MatchingSearch(qtype, clientIp, label, clientSearches)
	if ok {
		return
	}

	// Default answers, client search
	log.Debugf("Trying default answers, client search label=%v client=%v", label, clientIp)
	records, ok = answers.MatchingSearch(qtype, DEFAULT_KEY, label, clientSearches)
	if ok {
		return
	}

	// Default answers, default search
	log.Debugf("Trying default answers, default search label=%v client=%v", label, clientIp)
	defaultSearches := answers.SearchSuffixes(DEFAULT_KEY)
	records, ok = answers.MatchingSearch(qtype, DEFAULT_KEY, label, defaultSearches)
	if ok {
		return
	}

	return nil, false
}

func (answers *Answers) MatchingSearch(qtype uint16, clientIp string, label string, searches []string) (records []dns.RR, ok bool) {
	records, ok = answers.MatchingExact(qtype, clientIp, label, label)
	if ok {
		log.Debugf("Matched exact FQDN fqdn=%v client=%v", label, clientIp)
		return
	}

	base := strings.TrimRight(label, ".")
	limit := int(*ndots)
	if limit == 0 || strings.Count(base, ".") < limit {
		if searches != nil && len(searches) > 0 {
			for _, suffix := range searches {
				newFqdn := base + "." + strings.TrimRight(suffix, ".") + "."
				log.Debugf("Trying alternate suffix fqdn=%v client=%v", newFqdn, clientIp)

				records, ok = answers.MatchingExact(qtype, clientIp, newFqdn, label)
				if ok {
					log.Debugf("Matched alternate suffix fqdn=%v client=%v", newFqdn, clientIp)
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
			//log.Debugf("Searching for A qtype=A client=%v fqdn=%v", clientIp, fqdn)
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
			//log.Debugf("Searching for CNAME qtype=CNAME client=%v fqdn=%v", clientIp, fqdn)
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
			//log.Debugf("Searching for PTR qtype=PTR client=%v fqdn=%v", clientIp, fqdn)
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
			//log.Debugf("Searching for TXT qtype=TXT client=%v fqdn=%v", clientIp, fqdn)
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
						log.Warnf("TXT record too long: %v qtype=TXT client=%v fqdn=%v", str, clientIp, fqdn)
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

// Shuffles the sub-section of the supplied slice starting from the first A or AAAA record and going
// until the end. In other words, doesn't shuffle CNAME records at the start of the slice whose order
// should be maintained.
func shuffle(items *[]dns.RR) {
	max := len(*items)
	foundA := false

	for i := 0; i < max; i++ {
		record := (*items)[i].Header()
		if !foundA && record.Rrtype != dns.TypeA && record.Rrtype != dns.TypeAAAA {
			continue
		}
		foundA = true
		j := i + rand.Intn(max-i)
		(*items)[i], (*items)[j] = (*items)[j], (*items)[i]
	}
}
