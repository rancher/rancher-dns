package main

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
)

// The top-level key in the JSON for the default (not client-specific answers)
const DEFAULT_KEY = "default"

// The 2nd-level key in the JSON for the recursive resolver addresses
const RECURSE_KEY = "recurse"

type RecordA struct {
	Ttl    *uint32
	Answer []string
}

type RecordCname struct {
	Ttl    *uint32
	Answer string
}

type RecordTxt struct {
	Ttl    *uint32
	Answer []string
}

type ClientAnswers struct {
	Recurse []string
	A       map[string]RecordA
	Cname   map[string]RecordCname
	Txt     map[string]RecordTxt
}

type Answers map[string]ClientAnswers

func ReadAnswersFile(path string) (out Answers, err error) {
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		log.Warnf("Failed to find %s", path)
		return Answers{}, nil
	} else if err != nil {
		return
	}

	out = make(Answers)
	err = json.Unmarshal(data, &out)
	return
}

func (answers *Answers) RecurseHosts(clientIp string) []string {
	var hosts []string
	more := answers.recurseHostsFor(clientIp)
	if len(more) > 0 {
		hosts = append(hosts, more...)
	}
	more = answers.recurseHostsFor(DEFAULT_KEY)
	if len(more) > 0 {
		hosts = append(hosts, more...)
	}

	return hosts
}

func (answers *Answers) recurseHostsFor(clientIp string) []string {
	var hosts []string
	client, ok := (*answers)[clientIp]
	if ok {
		if ok && len(client.Recurse) > 0 {
			hosts = client.Recurse
		}
	}

	return hosts
}

func (answers *Answers) Addresses(clientIp string, fqdn string, cnameParents []dns.RR, depth int) (records []dns.RR, ok bool) {
	fqdn = dns.Fqdn(fqdn)
	log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp, "depth": depth}).Debug("Trying to resolve addresses")

	// Limit recursing for non-obvious loops
	if len(cnameParents) >= 10 {
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Warn("Followed CNAME too many times ", cnameParents)
		return nil, false
	}

	// Look for a CNAME entry
	result, ok := answers.Matching(dns.TypeCNAME, clientIp, fqdn)
	if ok && len(result) > 0 {
		cname := result[0].(*dns.CNAME)
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Matched CNAME ", cname.Target)

		// Stop obvious loops
		if dns.Fqdn(cname.Target) == fqdn {
			log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Warn("CNAME is a loop ", cname.Target)
			return nil, false
		}

		// Recurse to find the eventual A for this CNAME
		children, ok := answers.Addresses(clientIp, dns.Fqdn(cname.Target), append(cnameParents, cname), depth+1)
		if ok && len(children) > 0 {
			log.WithFields(log.Fields{"fqdn": fqdn, "target": cname.Target, "client": clientIp}).Debug("Resolved CNAME ", children)
			records = append(records, cname)
			records = append(records, children...)
			return records, true
		}
	}

	// Look for an A entry
	result, ok = answers.Matching(dns.TypeA, clientIp, fqdn)
	if ok && len(result) > 0 {
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Matched A ", result)
		shuffle(&result)
		return result, true
	}

	// Try the default section of the config
	if clientIp != DEFAULT_KEY {
		return answers.Addresses(DEFAULT_KEY, fqdn, cnameParents, depth+1)
	}

	// When resolving CNAMES, check recursive server
	if len(cnameParents) > 0 {
		log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Trying recursive servers")
		msg, err := ResolveTryAll(fqdn, dns.TypeA, answers.RecurseHosts(clientIp))
		if err == nil {
			return msg.Answer, true
		}
	}

	log.WithFields(log.Fields{"fqdn": fqdn, "client": clientIp}).Debug("Did not match anything")
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

func Respond(w dns.ResponseWriter, req *dns.Msg, records []dns.RR) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true
	m.RecursionAvailable = true
	m.Compress = true
	m.Answer = records

	// Figure out the max response size
	bufsize := uint16(512)
	tcp := isTcp(w)

	if o := req.IsEdns0(); o != nil {
		bufsize = o.UDPSize()
	}

	if tcp {
		bufsize = dns.MaxMsgSize - 1
	} else if bufsize < 512 {
		bufsize = 512
	}

	if m.Len() > dns.MaxMsgSize {
		fqdn := dns.Fqdn(req.Question[0].Name)
		log.WithFields(log.Fields{"fqdn": fqdn}).Debug("Response too big, dropping Extra")
		m.Extra = nil
		if m.Len() > dns.MaxMsgSize {
			log.WithFields(log.Fields{"fqdn": fqdn}).Debug("Response still too big")
			m := new(dns.Msg)
			m.SetRcode(m, dns.RcodeServerFailure)
		}
	}

	if m.Len() > int(bufsize) && !tcp {
		log.Debug("Too big 1")
		m.Extra = nil
		if m.Len() > int(bufsize) {
			log.Debug("Too big 2")
			m.Answer = nil
			m.Truncated = true
		}
	}

	err := w.WriteMsg(m)
	if err != nil {
		log.Warn("Failed to return reply: ", err, m.Len())
	}

}
