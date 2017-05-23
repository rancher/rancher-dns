package main

import (
	"testing"

	"github.com/miekg/dns"
	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type Tests struct{}

var _ = check.Suite(&Tests{})

func (t *Tests) TestNone(c *check.C) {
	// ensuring no panics
	records := []dns.RR{}
	shuffle(&records)
}

func (t *Tests) TestOne(c *check.C) {
	arecord := &dns.RR_Header{
		Name:   "arecord3",
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    100,
	}
	records := []dns.RR{arecord}
	shuffle(&records)
	c.Check(records, check.DeepEquals, []dns.RR{arecord})
}

func (t *Tests) TestNoARecords(c *check.C) {
	cname1 := &dns.RR_Header{
		Name:   "cname1",
		Rrtype: dns.TypeCNAME,
		Class:  dns.ClassINET,
		Ttl:    100,
	}
	dname1 := &dns.RR_Header{
		Name:   "dName1",
		Rrtype: dns.TypeDNAME,
		Class:  dns.ClassINET,
		Ttl:    100,
	}
	any1 := &dns.RR_Header{
		Name:   "any1",
		Rrtype: dns.TypeANY,
		Class:  dns.ClassINET,
		Ttl:    100,
	}
	for i := 0; i < 100; i++ {
		records := []dns.RR{cname1, dname1, any1}
		shuffle(&records)
		c.Check(records, check.DeepEquals, []dns.RR{cname1, dname1, any1})
	}
}

func (t *Tests) TestShuffle(c *check.C) {
	t.testShuffle(dns.TypeA, c)
	t.testShuffle(dns.TypeAAAA, c)
}

func (t *Tests) testShuffle(aType uint16, c *check.C) {
	cname1 := &dns.RR_Header{
		Name:   "cname1",
		Rrtype: dns.TypeCNAME,
		Class:  dns.ClassINET,
		Ttl:    100,
	}
	cname2 := &dns.RR_Header{
		Name:   "cname2",
		Rrtype: dns.TypeCNAME,
		Class:  dns.ClassINET,
		Ttl:    100,
	}
	arecord1 := &dns.RR_Header{
		Name:   "arecord1",
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    100,
	}
	arecord2 := &dns.RR_Header{
		Name:   "arecord2",
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    100,
	}
	arecord3 := &dns.RR_Header{
		Name:   "arecord3",
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    100,
	}

	records := []dns.RR{
		cname1,
		cname2,
		arecord1,
		arecord2,
		arecord3,
	}
	expected := []dns.RR{cname1, cname2}
	aRecord1First := false
	aRecord2First := false
	aRecord3First := false
	for i := 0; i < 100; i++ {
		shuffle(&records)
		c.Check(records[:2], check.DeepEquals, expected)
		if records[2].Header().Name == "arecord1" {
			aRecord1First = true
		}
		if records[2].Header().Name == "arecord2" {
			aRecord2First = true
		}
		if records[2].Header().Name == "arecord3" {
			aRecord3First = true
		}
	}

	// shuffle...just checking that each A record was first at least once
	c.Check(aRecord1First, check.Equals, true)
	c.Check(aRecord2First, check.Equals, true)
	c.Check(aRecord3First, check.Equals, true)
}
