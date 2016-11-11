package main

import (
	//"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher-metadata/metadata"
	"strings"
	"testing"
)

var c *ConfigGenerator

func init() {
	c = &ConfigGenerator{
		metaFetcher: &tMetaFetcher{},
	}
}

type tMetaFetcher struct {
}

func TestVIP(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "vipsvc.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for vip [%v]", a.Answer)
	}
	if a.Answer[0] != "10.1.1.1" {
		t.Fatalf("Incorrect answer for vip [%v]", a.Answer[0])
	}
}

func TestRegular(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "regularsvc.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for regular service [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.1.1" {
		t.Fatalf("Incorrect answer for regular service ip [%v]", a.Answer[0])
	}
}

func TestStoppedContainer(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "stoppedsvc.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for service with only one stopped container [%v]", a.Answer)
	}
}

func TestOneStoppedContainer(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "stoppedonesvc.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for service with 2 containers, 1 stopped [%v]", a.Answer)
	}
}

func TestOneUnhealthyContainer(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "unhealthysvc.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for service with 2 containers, 1 stopped [%v]", a.Answer)
	}

	if a.Answer[0] != "192.168.0.3" {
		t.Fatalf("Incorrect answer for ip [%v]", a.Answer[0])
	}
}

func TestNoHealthStateWithHealthcheck(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "healthempty.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for service with health check empty, expected count 1: [%v]", a.Answer)
	}
}

func TestExternalCnameService(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "externalCnameSvc.foo.rancher.internal.")
	if len(a.Answer) != 0 {
		t.Fatalf("Incorrect number of A records, should be 0: [%v]", len(a.Answer))
	}

	c := getRecordCnameFromDefault(answers, "externalCnameSvc.foo.rancher.internal.")

	if c.Answer != "google.com" {
		t.Fatalf("Incorrect answer for cname [%v]", c.Answer)
	}
}

func TestExternalIpsService(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "externalIpsSvc.foo.rancher.internal.")
	if len(a.Answer) != 2 {
		t.Fatalf("Incorrect number of A records, should be 2: [%v]", len(a.Answer))
	}
}

func TestAliasService(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "aliasSvc.foo.rancher.internal.")
	if len(a.Answer) != 2 {
		t.Fatalf("Incorrect number of A records, should be 2: [%v]", len(a.Answer))
	}
}

func TestClientNoLinks(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}
	ip := "172.17.0.2"
	c := getClientAnswers(answers, ip)
	if c == nil {
		t.Fatalf("Can't find client answers for: [%v]", ip)
	}
	if len(c.A) != 0 {
		t.Fatalf("Incorrect number of A records: [%v]", len(c.A))
	}
	a := getRecordAFromDefault(answers, "clientIp1.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
}

func TestClientStandalone(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}
	ip := "172.17.0.10"
	c := getClientAnswers(answers, ip)
	if c == nil {
		t.Fatalf("Can't find client answers for: [%v]", ip)
	}
	if len(c.A) != 0 {
		t.Fatalf("Incorrect number of A records: [%v]", len(c.A))
	}
	a := getRecordAFromDefault(answers, "clientStandalone.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
}

func TestClientKubernetes(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}
	ip := "172.17.0.11"
	c := getClientAnswers(answers, ip)
	if c == nil {
		t.Fatalf("Can't find client answers for: [%v]", ip)
	}
	if len(c.A) != 0 {
		t.Fatalf("Incorrect number of A records: [%v]", len(c.A))
	}
	a := getRecordAFromDefault(answers, "clientKubernetes.foo.svc.cluster.local.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}

	//container name is clientKubernetes too
	a = getRecordAFromDefault(answers, "clientKubernetes.clientKubernetes.foo.svc.cluster.local.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
}

func TestVipClientKubernetes(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}
	ip := "172.17.0.12"
	c := getClientAnswers(answers, ip)
	if c == nil {
		t.Fatalf("Can't find client answers for: [%v]", ip)
	}
	if len(c.A) != 0 {
		t.Fatalf("Incorrect number of A records: [%v]", len(c.A))
	}

	//container name is clientKubernetesVip too
	a := getRecordAFromDefault(answers, "clientKubernetesVip.foo.svc.cluster.local.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}

	a = getRecordAFromDefault(answers, "clientKubernetesVip.clientKubernetesVip.foo.svc.cluster.local.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
}

func TestRancherMetadata(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}
	a := getRecordAFromDefault(answers, "rancher-metadata.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}

	if a.Answer[0] != "169.254.169.250" {
		t.Fatalf("Incorrect answer for ip [%v]", a.Answer[0])
	}
}

func TestClientWithLinksNoAlias(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	ips := []string{"172.17.0.3", "172.17.0.4"}
	for _, ip := range ips {
		c := getClientAnswers(answers, ip)
		if c == nil {
			t.Fatalf("Can't find client answers for: [%v]", ip)
		}
		if len(c.A) != 0 {
			//should be 0 as it is present in defaults section
			t.Fatalf("Incorrect number of A records, should be 0: [%v]", len(c.A))
		}
	}
}

func TestNetworkFrom(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}
	ip := "192.168.0.34"
	c := getClientAnswers(answers, ip)
	if c == nil {
		t.Fatalf("Can't find client answers for: [%v]", ip)
	}
	if len(c.A) != 0 {
		t.Fatalf("Incorrect number of A records: [%v]", len(c.A))
	}
	a := getRecordAFromDefault(answers, "networkFromMaster.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
	if a.Answer[0] != "192.168.0.34" {
		t.Fatalf("Incorrect ip for master, should be: [%v]", ip)
	}

	a = getRecordAFromDefault(answers, "networkFromChild.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
	if a.Answer[0] != "192.168.0.34" {
		t.Fatalf("Incorrect ip for child, should be: [%v]", ip)
	}
}

func TestClientWithLinksAlias(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	ips := []string{"172.17.0.6", "172.17.0.7"}
	for _, ip := range ips {
		c := getClientAnswers(answers, ip)
		if c == nil {
			t.Fatalf("Can't find client answers for: [%v]", ip)
		}
		if len(c.A) != 2 {
			t.Fatalf("Incorrect number of A records, should be 2: [%v]", len(c.A))
		}

		fqdns := []string{"myalias.foo.rancher.internal.", "myalias.rancher.internal."}
		for _, fqdn := range fqdns {
			val, ok := c.A[fqdn]
			if !ok {
				t.Fatalf("Can't find the fqnd link %s", fqdn)
			}
			if len(val.Answer) != 1 {
				t.Fatalf("Incorrect number of answers [%v], expected 1", len(val.Answer))
			}
			if val.Answer[0] != "192.168.1.1" {
				t.Fatalf("Record [%s] doesn't match expected value 192.168.1.1", val.Answer[0])
			}
		}
	}
}

func TestClientWithAliasCnameLinks(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	ips := []string{"172.17.0.66", "172.17.0.77"}
	for _, ip := range ips {
		c := getClientAnswers(answers, ip)
		if c == nil {
			t.Fatalf("Can't find client answers for: [%v]", ip)
		}

		fqdns := []string{"myaliascname.foo.rancher.internal.", "myaliascname.rancher.internal."}
		for _, fqdn := range fqdns {
			val, ok := c.Cname[fqdn]
			if !ok {
				t.Fatalf("Can't find the fqnd cname link %s", fqdn)
			}

			if val.Answer != "google.com" {
				t.Fatalf("Record [%v] doesn't match expected value google.com", val.Answer[0])
			}
		}
	}
}

func TestSidekicks(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "primary.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for primary service [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.1" {
		t.Fatalf("Incorrect answer for primary service ip [%v]", a.Answer[0])
	}

	a = getRecordAFromDefault(answers, "sidekick.primary.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for sidekick service [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.2" {
		t.Fatalf("Incorrect answer for regular service ip [%v]", a.Answer[0])
	}
}

func TestNSidekicks(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "primaryn.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for primary service [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.11" {
		t.Fatalf("Incorrect answer for primary service ip [%v]", a.Answer[0])
	}

	a = getRecordAFromDefault(answers, "primaryn.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for primary service container [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.11" {
		t.Fatalf("Incorrect answer for primary service ip [%v]", a.Answer[0])
	}

	a = getRecordAFromDefault(answers, "sidekickn.primaryn.foo.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for sidekick service [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.11" {
		t.Fatalf("Incorrect answer for sidekick service [%v]", a.Answer[0])
	}

	a = getRecordAFromDefault(answers, "sidekickn.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for sidekick service container [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.11" {
		t.Fatalf("Incorrect answer for regular service ip [%v]", a.Answer[0])
	}
}

func getClientAnswers(answers Answers, ip string) *ClientAnswers {
	for key, value := range answers {
		if strings.EqualFold(key, ip) {
			return &value
		}
	}

	return nil
}

func getRecordAFromDefault(answers Answers, fqdn string) RecordA {
	var def ClientAnswers
	for key, value := range answers {
		if strings.EqualFold(key, "default") {
			def = value
			break
		}
	}

	var a RecordA
	for key, value := range def.A {
		if strings.EqualFold(key, fqdn) {
			a = value
			break
		}
	}
	return a
}

func getRecordCnameFromDefault(answers Answers, fqdn string) RecordCname {
	var def ClientAnswers
	for key, value := range answers {
		if strings.EqualFold(key, "default") {
			def = value
			break
		}
	}

	var c RecordCname
	for key, value := range def.Cname {
		if strings.EqualFold(key, fqdn) {
			c = value
			break
		}
	}
	return c
}

func (mf tMetaFetcher) GetService(svcName string, stackName string) (*metadata.Service, error) {
	if svcName == "regularSvc" && stackName == "foo" {
		c1 := metadata.Container{
			Name:        "regular_container",
			StackName:   "foo",
			ServiceName: "regularSvc",
			PrimaryIp:   "192.168.1.1",
			State:       "running",
		}
		c2 := metadata.Container{
			Name:        "regular_container",
			StackName:   "foo",
			ServiceName: "regularSvc",
			PrimaryIp:   "192.168.1.2",
			State:       "running",
		}
		containers := []metadata.Container{c1, c2}
		return &metadata.Service{
			Name:       "regularSvc",
			Kind:       "service",
			StackName:  "foo",
			Containers: containers,
		}, nil
	}
	return nil, nil
}

func (mf tMetaFetcher) GetServices() ([]metadata.Service, error) {
	var services []metadata.Service

	var containers []metadata.Container
	c := metadata.Container{
		Name:        "clientip1",
		StackName:   "foo",
		ServiceName: "clientip1Svc",
		PrimaryIp:   "172.17.0.2",
		State:       "running",
		DnsSearch:   []string{"regularSvc.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}
	containers = []metadata.Container{c}
	clientip1Svc := metadata.Service{
		Name:       "clientip1Svc",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}
	c = metadata.Container{
		Name:        "vip_container",
		StackName:   "foo",
		ServiceName: "vipSvc",
		PrimaryIp:   "192.168.1.1",
		State:       "running",
	}
	containers = []metadata.Container{c}
	vip := metadata.Service{
		Name:       "vipSvc",
		Vip:        "10.1.1.1",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}

	c = metadata.Container{
		Name:        "regular_container",
		StackName:   "foo",
		ServiceName: "regularSvc",
		PrimaryIp:   "192.168.1.1",
		State:       "running",
	}
	containers = []metadata.Container{c}
	regular := metadata.Service{
		Name:       "regularSvc",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}

	c = metadata.Container{
		Name:        "regular_container",
		StackName:   "foo",
		ServiceName: "regularSvc",
		PrimaryIp:   "192.168.1.1",
		State:       "stopped",
	}
	containers = []metadata.Container{c}
	stopped := metadata.Service{
		Name:       "stoppedSvc",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}
	c1 := metadata.Container{
		Name:        "c_stopped",
		StackName:   "foo",
		ServiceName: "stoppedoneSvc",
		PrimaryIp:   "192.168.1.1",
		State:       "stopped",
	}
	c2 := metadata.Container{
		Name:        "c_running",
		StackName:   "foo",
		ServiceName: "stoppedoneSvc",
		PrimaryIp:   "192.168.1.2",
		State:       "running",
	}
	containers = []metadata.Container{c1, c2}
	stoppedone := metadata.Service{
		Name:       "stoppedoneSvc",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}

	c1 = metadata.Container{
		Name:        "c_stopped",
		StackName:   "foo",
		ServiceName: "stoppedoneSvc",
		PrimaryIp:   "192.168.1.1",
		State:       "running",
		HealthState: "unheatlhy",
	}
	c2 = metadata.Container{
		Name:        "c_running",
		StackName:   "foo",
		ServiceName: "stoppedoneSvc",
		PrimaryIp:   "192.168.0.3",
		State:       "running",
		HealthState: "healthy",
	}
	containers = []metadata.Container{c1, c2}
	unhealthy := metadata.Service{
		Name:       "unhealthySvc",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}
	externalCname := metadata.Service{
		Name:      "externalCnameSvc",
		Kind:      "externalService",
		StackName: "foo",
		Hostname:  "google.com",
	}

	externalIPs := metadata.Service{
		Name:        "externalIpsSvc",
		Kind:        "externalService",
		StackName:   "foo",
		ExternalIps: []string{"10.1.1.1", "10.1.1.2"},
	}

	links := make(map[string]string)
	links["foo/regularSvc"] = "foo/regularSvc"
	alias := metadata.Service{
		Name:      "aliasSvc",
		Kind:      "dnsService",
		StackName: "foo",
		Links:     links,
	}

	c = metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "vipSvc",
		PrimaryIp:   "172.17.0.2",
		State:       "running",
	}
	containers = []metadata.Container{c}
	client := metadata.Service{
		Name:       "clientSvc",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}

	links = make(map[string]string)
	links["foo/regularSvc"] = "foo/regularSvc"
	c1 = metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "vipSvc",
		PrimaryIp:   "172.17.0.3",
		State:       "running",
	}
	c2 = metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "vipSvc",
		PrimaryIp:   "172.17.0.4",
		State:       "running",
	}
	containers = []metadata.Container{c1, c2}
	svcWithLinks := metadata.Service{
		Name:       "svcWithLinks",
		Kind:       "service",
		StackName:  "foo",
		Links:      links,
		Containers: containers,
	}

	links = make(map[string]string)
	links["foo/regularSvc"] = "myalias"
	c1 = metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "vipSvc",
		PrimaryIp:   "172.17.0.6",
		State:       "running",
	}
	c2 = metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "vipSvc",
		PrimaryIp:   "172.17.0.7",
		State:       "running",
	}
	containers = []metadata.Container{c1, c2}
	svcWithLinksAlias := metadata.Service{
		Name:       "svcWithLinksAlias",
		Kind:       "service",
		StackName:  "foo",
		Links:      links,
		Containers: containers,
	}

	links = make(map[string]string)
	links["foo/externalCnameSvc"] = "myaliascname"
	c1 = metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "svcWithLinksAliasCname",
		PrimaryIp:   "172.17.0.66",
		State:       "running",
	}
	c2 = metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "svcWithLinksAliasCname",
		PrimaryIp:   "172.17.0.77",
		State:       "running",
	}
	containers = []metadata.Container{c1, c2}
	svcWithLinksAliasCname := metadata.Service{
		Name:       "svcWithLinksAliasCname",
		Kind:       "service",
		StackName:  "foo",
		Links:      links,
		Containers: containers,
	}

	c = metadata.Container{
		Name:        "primary",
		StackName:   "foo",
		ServiceName: "primary",
		PrimaryIp:   "192.168.0.1",
		State:       "running",
	}
	containers = []metadata.Container{c}
	primary := metadata.Service{
		Name:       "primary",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}

	c = metadata.Container{
		Name:        "sidekick",
		StackName:   "foo",
		ServiceName: "sidekick",
		PrimaryIp:   "192.168.0.2",
		State:       "running",
	}
	containers = []metadata.Container{c}
	sidekick := metadata.Service{
		Name:               "sidekick",
		Kind:               "service",
		StackName:          "foo",
		PrimaryServiceName: "primary",
		Containers:         containers,
	}

	c = metadata.Container{
		Name:        "clientKubernetes",
		StackName:   "foo",
		ServiceName: "clientKubernetes",
		PrimaryIp:   "172.17.0.11",
		State:       "running",
	}
	containers = []metadata.Container{c}
	kubernetes := metadata.Service{
		Name:       "clientKubernetes",
		Kind:       "kubernetesService",
		StackName:  "foo",
		Containers: containers,
	}

	c = metadata.Container{
		Name:        "clientKubernetesVip",
		StackName:   "foo",
		ServiceName: "clientKubernetesVip",
		PrimaryIp:   "172.17.0.12",
		State:       "running",
	}
	containers = []metadata.Container{c}
	kubernetesVip := metadata.Service{
		Name:       "clientKubernetesVip",
		Vip:        "10.1.1.1",
		Kind:       "kubernetesService",
		StackName:  "foo",
		Containers: containers,
	}

	c1 = metadata.Container{
		Name:        "clientKubernetesVip",
		StackName:   "foo",
		ServiceName: "clientKubernetesVip",
		PrimaryIp:   "192.168.0.33",
		State:       "running",
	}
	c2 = metadata.Container{
		Name:        "clientKubernetesVip",
		StackName:   "foo",
		ServiceName: "healthEmpty",
		PrimaryIp:   "192.168.0.33",
		State:       "running",
	}
	containers = []metadata.Container{c1, c2}
	healthCheck := metadata.HealthCheck{
		Port: 100,
	}
	healthEmpty := metadata.Service{
		Name:        "healthEmpty",
		Kind:        "service",
		StackName:   "foo",
		Containers:  containers,
		HealthCheck: healthCheck,
	}

	c = metadata.Container{
		Name:        "primaryn",
		StackName:   "foo",
		ServiceName: "primaryn",
		PrimaryIp:   "192.168.0.11",
		State:       "running",
		UUID:        "networkFromPrimary",
	}
	containers = []metadata.Container{c}
	primaryn := metadata.Service{
		Name:       "primaryn",
		Kind:       "service",
		StackName:  "foo",
		Containers: containers,
	}

	c = metadata.Container{
		Name:        "sidekickn",
		StackName:   "foo",
		ServiceName: "sidekickn",
		State:       "running",
		NetworkFromContainerUUID: "networkFromPrimary",
	}
	containers = []metadata.Container{c}
	sidekickn := metadata.Service{
		Name:               "sidekickn",
		Kind:               "service",
		StackName:          "foo",
		PrimaryServiceName: "primaryn",
		Containers:         containers,
	}

	services = append(services, kubernetes, healthEmpty, primaryn, sidekickn, kubernetesVip, clientip1Svc, vip, primary, sidekick, regular, stopped, stoppedone, unhealthy, externalCname, svcWithLinksAliasCname, svcWithLinksAlias, externalIPs, alias, client, svcWithLinks)
	return services, nil
}

func (mf tMetaFetcher) GetContainers() ([]metadata.Container, error) {
	c1 := metadata.Container{
		Name:        "clientip1",
		StackName:   "foo",
		ServiceName: "clientip1Svc",
		PrimaryIp:   "172.17.0.2",
		State:       "running",
		DnsSearch:   []string{"regularSvc.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}

	c2 := metadata.Container{
		Name:        "clientip1",
		StackName:   "foo",
		ServiceName: "svcWithLinks",
		PrimaryIp:   "172.17.0.3",
		State:       "running",
		DnsSearch:   []string{"svcWithLinks.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}

	c3 := metadata.Container{
		Name:        "clientip1",
		StackName:   "foo",
		ServiceName: "svcWithLinks",
		PrimaryIp:   "172.17.0.4",
		State:       "running",
		DnsSearch:   []string{"svcWithLinks.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}
	c4 := metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "svcWithLinksAlias",
		PrimaryIp:   "172.17.0.6",
		State:       "running",
		DnsSearch:   []string{"svcWithLinksAlias.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}
	c5 := metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "svcWithLinksAlias",
		PrimaryIp:   "172.17.0.7",
		State:       "running",
		DnsSearch:   []string{"svcWithLinksAlias.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}
	c6 := metadata.Container{
		Name:      "clientStandalone",
		PrimaryIp: "172.17.0.10",
		State:     "running",
		DnsSearch: []string{"regularSvc.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}
	c7 := metadata.Container{
		Name:        "clientKubernetes",
		StackName:   "foo",
		ServiceName: "clientKubernetes",
		PrimaryIp:   "172.17.0.11",
		State:       "running",
	}

	c8 := metadata.Container{
		Name:        "clientKubernetesVip",
		StackName:   "foo",
		ServiceName: "clientKubernetesVip",
		PrimaryIp:   "172.17.0.12",
		State:       "running",
	}

	c9 := metadata.Container{
		Name:        "clientKubernetesVip",
		StackName:   "foo",
		ServiceName: "clientKubernetesVip",
		PrimaryIp:   "192.168.0.33",
		State:       "running",
	}
	c10 := metadata.Container{
		Name:        "clientKubernetesVip",
		StackName:   "foo",
		ServiceName: "healthEmpty",
		PrimaryIp:   "192.168.0.33",
		State:       "running",
	}

	c11 := metadata.Container{
		Name:      "networkFromMaster",
		PrimaryIp: "192.168.0.34",
		State:     "running",
		UUID:      "networkFromMaster",
	}
	c12 := metadata.Container{
		Name:  "networkFromChild",
		State: "running",
		NetworkFromContainerUUID: "networkFromMaster",
	}

	c13 := metadata.Container{
		Name:        "sidekickn",
		StackName:   "foo",
		ServiceName: "sidekickn",
		State:       "running",
		NetworkFromContainerUUID: "networkFromPrimary",
	}

	c14 := metadata.Container{
		Name:        "primaryn",
		StackName:   "foo",
		ServiceName: "primaryn",
		PrimaryIp:   "192.168.0.11",
		State:       "running",
		UUID:        "networkFromPrimary",
	}

	c15 := metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "svcWithLinksAliasCname",
		PrimaryIp:   "172.17.0.66",
		State:       "running",
		DnsSearch:   []string{"svcWithLinksAliasCname.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}
	c16 := metadata.Container{
		Name:        "client_container",
		StackName:   "foo",
		ServiceName: "svcWithLinksAliasCname",
		PrimaryIp:   "172.17.0.77",
		State:       "running",
		DnsSearch:   []string{"svcWithLinksAliasCname.rancher.internal", "foo.rancher.internal", "rancher.internal"},
	}

	containers := []metadata.Container{c1, c2, c3, c4, c5, c6, c7, c8, c9, c10, c11, c12, c13, c14, c15, c16}
	return containers, nil
}

func (mf tMetaFetcher) OnChange(intervalSeconds int, do func(string)) {
	return
}

func (mf tMetaFetcher) GetSelfHost() (metadata.Host, error) {
	var host metadata.Host
	return host, nil
}
