package main

import (
	"github.com/miekg/dns"
	"net"
	"strings"
)

// Proxy a request to an external server
func Proxy(w dns.ResponseWriter, req *dns.Msg, addr string) (err error) {
	transport := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		transport = "tcp"
	}

	// Default to port 53
	if !strings.Contains(addr, ":") {
		addr = addr + ":53"
	}

	c := &dns.Client{Net: transport}
	resp, _, err := c.Exchange(req, addr)
	if err == nil {
		w.WriteMsg(resp)
		return nil
	} else {
		return err
	}
}
