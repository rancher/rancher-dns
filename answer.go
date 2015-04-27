package main

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/miekg/dns"
)

// The top-level key in the JSON for the default (not client-specific answers)
const DEFAULT_KEY = "default"

// The 2nd-level key in the JSON for the recursive resolver addresses
const RECURSE_KEY = "recurse"

type Zone []string
type Zones map[string]Zone
type Answers map[string]Zones

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

func (answers *Answers) Matching(clientIp string, fqdn string) (Zone, bool) {
	zones, ok := (*answers)[clientIp]
	if ok {
		zone, ok := zones[fqdn]
		if ok && len(zone) > 0 {
			return zone, true
		}
	}

	return nil, false
}

// Look for answers for the client's IP
func (answers *Answers) LocalAnswer(fqdn string, rrType string, clientIp string) (Zone, bool) {
	if rrType == "A" {
		return answers.Matching(clientIp, fqdn)
	}

	return nil, false
}

// Look for answers for the default entry
func (answers *Answers) DefaultAnswer(fqdn string, rrType string, clientIp string) (Zone, bool) {
	if rrType == "A" {
		return answers.Matching(DEFAULT_KEY, fqdn)
	}

	return nil, false
}

func Respond(w dns.ResponseWriter, req *dns.Msg, answers Zone) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Answer = make([]dns.RR, len(answers))

	for i := 0; i < len(answers); i++ {
		hdr := dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(*ttl)}
		ip := net.ParseIP(answers[i])
		m.Answer[i] = &dns.A{Hdr: hdr, A: ip}
	}

	w.WriteMsg(m)
}
