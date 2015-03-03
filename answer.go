package main

import (
	"encoding/json"
	"io/ioutil"
	"net"

	"github.com/miekg/dns"
)

const DEFAULT_KEY = "default"

type Zone []string
type Zones map[string]Zone
type Answers map[string]Zones

func ReadAnswersFile(path string) (out Answers, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	out = make(Answers)
	err = json.Unmarshal(data, &out)
	return
}

// Look for answers for the client's IP
func LocalAnswer(question *dns.Question, rrType string, clientIp string) (Zone, bool) {
	if rrType == "A" {
		zones, ok := answers[clientIp]
		if ok {
			zone, ok := zones[question.Name]
			if ok && len(zone) > 0 {
				return zone, true
			}
		}
	}

	return nil, false
}

// Look for answers for the default entry
func DefaultAnswer(question *dns.Question, rrType string, clientIp string) (Zone, bool) {
	if rrType == "A" {
		zones, ok := answers[DEFAULT_KEY]
		if ok {
			zone, ok := zones[question.Name]

			if ok && len(zone) > 0 {
				return zone, true
			}
		}
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

/*
type Answer struct {
	Type     string `json:"type"`
	Ttl      uint32 `json:"ttl"`
	Value    string `json:"value"`
	Priority uint16 `json:"priority"`
	Weight   uint16 `json:"weight"`
	Port     uint16 `json:"port"`
}

func (all *Zone) OfType(rrType string) Zone {
	var out Zone
	rrType = strings.ToUpper(rrType)

	for _, v := range *all {
		if strings.ToUpper(v.Type) == rrType {
			out = append(out, v)
		}
	}

	return out
}
*/
