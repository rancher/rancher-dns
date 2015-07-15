package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
)

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
