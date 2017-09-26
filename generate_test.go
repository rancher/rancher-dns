package main

import (
	//"github.com/Sirupsen/logrus"
	"strings"
	"testing"

	"github.com/rancher/go-rancher-metadata/metadata"
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

	a := getRecordAFromDefault(answers, "vipsvc.foo.default.discover.internal.")
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

	a := getRecordAFromDefault(answers, "regularsvc.foo.default.discover.internal.")
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

	a := getRecordAFromDefault(answers, "stoppedsvc.foo.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for service with only one stopped container [%v]", a.Answer)
	}
}

func TestOneStoppedContainer(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "stoppedonesvc.foo.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for service with 2 containers, 1 stopped [%v]", a.Answer)
	}
}

func TestOneUnhealthyContainer(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "unhealthysvc.foo.default.discover.internal.")
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

	a := getRecordAFromDefault(answers, "healthempty.foo.default.discover.internal.")

	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for service with health check empty, expected count 1: [%v]", a.Answer)
	}

	if a.Answer[0] != "192.168.0.34" {
		t.Fatalf("Incorrect ip, expected 192.168.0.34, actual: [%v]", a.Answer)
	}
}

func TestExternalgService(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "externalCnameSvc.foo.default.discover.internal.")
	if len(a.Answer) != 0 {
		t.Fatalf("Incorrect number of A records, should be 0: [%v]", len(a.Answer))
	}

	c := getRecordCnameFromDefault(answers, "externalCnameSvc.foo.default.discover.internal.")

	if c.Answer != "google.com." {
		t.Fatalf("Incorrect answer for cname [%v]", c.Answer)
	}
}

func TestExternalIpsService(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "externalIpsSvc.foo.default.discover.internal.")
	if len(a.Answer) != 2 {
		t.Fatalf("Incorrect number of A records, should be 2: [%v]", len(a.Answer))
	}
}

func TestAliasService(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "aliasSvc.foo.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
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
	a := getRecordAFromDefault(answers, "clientIp1.default.discover.internal.")
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
	a := getRecordAFromDefault(answers, "clientStandalone.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
}

func TestClientKubernetes(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}
	ip := "clientKubern"
	c := getClientAnswers(answers, ip)
	if c == nil {
		t.Fatalf("Can't find client answers for: [%v]", ip)
	}
	if len(c.A) != 0 {
		t.Fatalf("Incorrect number of A records: [%v]", len(c.A))
	}
	a := getRecordAFromDefault(answers, "clientKubernetes.foo.default.svc.cluster.local.")
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
	uuid := "clientKubern"
	c := getClientAnswers(answers, uuid)
	if c == nil {
		t.Fatalf("Can't find client answers for: [%v]", uuid)
	}
	if len(c.A) != 0 {
		t.Fatalf("Incorrect number of A records: [%v]", len(c.A))
	}

	//container name is clientKubernetesVip too
	a := getRecordAFromDefault(answers, "clientKubernetesVip.foo.default.svc.cluster.local.")
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
	a := getRecordAFromDefault(answers, "rancher-metadata.discover.internal.")
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

	ips := []string{"container101", "container201"}
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
	a := getRecordAFromDefault(answers, "networkFromMaster.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
	if a.Answer[0] != "192.168.0.34" {
		t.Fatal("Incorrect ip for master, should be: [192.168.0.34]")
	}

	a = getRecordAFromDefault(answers, "networkFromChild.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}
	if a.Answer[0] != "192.168.0.34" {
		t.Fatalf("Incorrect ip for child, should be: [192.168.0.34], actual value is [%v]", a.Answer[0])
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
		if len(c.A) != 1 {
			t.Fatalf("Incorrect number of A records, should be 2: [%v]", len(c.A))
		}

		fqdns := []string{"myalias."}
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

func TestContainerLink(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	ip := "172.17.0.777"
	c := getClientAnswers(answers, ip)
	if c == nil {
		t.Fatalf("Can't find client answers for: [%v]", ip)
	}
	if len(c.A) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(c.A))
	}
	fqdn := "containerLink."
	for key, val := range c.A {
		ok := strings.EqualFold(fqdn, key)
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

		fqdns := []string{"myaliascname."}
		for _, fqdn := range fqdns {
			val, ok := c.Cname[fqdn]
			if !ok {
				t.Fatalf("Can't find the fqnd cname link %s for client %s", fqdn, ip)
			}

			if val.Answer != "google.com." {
				t.Fatalf("Record [%v] doesn't match expected value google.com.", val.Answer[0])
			}
		}
	}
}

func TestSidekicks(t *testing.T) {
	answers, err := c.GenerateAnswers()
	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "primary.foo.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for primary service [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.1" {
		t.Fatalf("Incorrect answer for primary service ip [%v]", a.Answer[0])
	}

	a = getRecordAFromDefault(answers, "sidekick.primary.foo.default.discover.internal.")
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

	a := getRecordAFromDefault(answers, "primaryn.foo.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for primary service [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.11" {
		t.Fatalf("Incorrect answer for primary service ip [%v]", a.Answer[0])
	}

	a = getRecordAFromDefault(answers, "primaryn.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for primary service container [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.11" {
		t.Fatalf("Incorrect answer for primary service ip [%v]", a.Answer[0])
	}

	a = getRecordAFromDefault(answers, "sidekickn.primaryn.foo.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for sidekick service [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.11" {
		t.Fatalf("Incorrect answer for sidekick service [%v]", a.Answer[0])
	}

	a = getRecordAFromDefault(answers, "sidekickn.default.discover.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of answers for sidekick service container [%v]", a.Answer)
	}
	if a.Answer[0] != "192.168.0.11" {
		t.Fatalf("Incorrect answer for sidekick service container; expected ip 192.168.0.11; actual value is [%v]", a.Answer[0])
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

func (mf tMetaFetcher) GetServices() ([]metadata.Service, error) {
	var services []metadata.Service

	var containers []metadata.Container
	c := metadata.Container{
		Name:            "clientip1",
		UUID:            "clientIp1016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "clientip1Svc",
		ServiceUUID:     "clientip1Svc",
		PrimaryIp:       "172.17.0.2",
		State:           "running",
		EnvironmentName: "Default",
		DnsSearch:       []string{"regularSvc.discover.internal", "foo.discover.internal", "discover.internal"},
	}
	containers = []metadata.Container{c}
	clientip1Svc := metadata.Service{
		Name:            "clientip1Svc",
		UUID:            "clientip1Svc",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}
	c = metadata.Container{
		Name:            "vip_container",
		UUID:            "vip_container016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "vipSvc",
		ServiceUUID:     "vipSvc",
		PrimaryIp:       "192.168.1.1",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	vip := metadata.Service{
		Name:            "vipSvc",
		UUID:            "vipSvc",
		Vip:             "10.1.1.1",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}

	c = metadata.Container{
		Name:            "regular_container",
		UUID:            "regular_container016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "regularSvc",
		ServiceUUID:     "regularSvc",
		PrimaryIp:       "192.168.1.1",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	regular := metadata.Service{
		Name:            "regularSvc",
		UUID:            "regularSvc",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}

	c = metadata.Container{
		Name:            "regular_container",
		UUID:            "regular_container016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "regularSvc",
		ServiceUUID:     "regularSvc",
		PrimaryIp:       "192.168.1.1",
		State:           "stopped",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	stopped := metadata.Service{
		Name:            "stoppedSvc",
		UUID:            "stoppedSvc",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}
	c1 := metadata.Container{
		Name:            "c_stopped",
		UUID:            "c_stopped016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "stoppedoneSvc",
		ServiceUUID:     "stoppedoneSvc",
		PrimaryIp:       "192.168.1.1",
		State:           "stopped",
		EnvironmentName: "Default",
	}
	c2 := metadata.Container{
		Name:            "c_running",
		UUID:            "c_running016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "stoppedoneSvc",
		ServiceUUID:     "stoppedoneSvc",
		PrimaryIp:       "192.168.1.2",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c1, c2}
	stoppedone := metadata.Service{
		Name:            "stoppedoneSvc",
		UUID:            "stoppedoneSvc",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}

	c1 = metadata.Container{
		Name:            "c_stopped",
		UUID:            "c_stopped016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "stoppedoneSvc",
		ServiceUUID:     "stoppedoneSvc",
		PrimaryIp:       "192.168.1.1",
		State:           "running",
		HealthState:     "unheatlhy",
		EnvironmentName: "Default",
	}
	c2 = metadata.Container{
		Name:            "c_running",
		UUID:            "c_running016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "stoppedoneSvc",
		ServiceUUID:     "stoppedoneSvc",
		PrimaryIp:       "192.168.0.3",
		State:           "running",
		HealthState:     "healthy",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c1, c2}
	unhealthy := metadata.Service{
		Name:            "unhealthySvc",
		UUID:            "unhealthySvc",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}
	externalCname := metadata.Service{
		Name:            "externalCnameSvc",
		UUID:            "externalCnameSvc",
		Kind:            "externalService",
		StackName:       "foo",
		Hostname:        "google.com",
		EnvironmentName: "Default",
	}

	externalIPs := metadata.Service{
		Name:            "externalIpsSvc",
		UUID:            "externalIpsSvc",
		Kind:            "externalService",
		StackName:       "foo",
		ExternalIps:     []string{"10.1.1.1", "10.1.1.2"},
		EnvironmentName: "Default",
	}

	links := make(map[string]string)
	links["foo/regularSvc"] = "regularSvc"
	alias := metadata.Service{
		Name:            "aliasSvc",
		UUID:            "aliasSvc",
		Kind:            "dnsService",
		StackName:       "foo",
		Links:           links,
		EnvironmentName: "Default",
	}

	c = metadata.Container{
		Name:            "client_container",
		UUID:            "client_container016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "vipSvc",
		ServiceUUID:     "vipSvc",
		PrimaryIp:       "172.17.0.2",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	client := metadata.Service{
		Name:            "clientSvc",
		UUID:            "clientSvc",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}

	dnsLabels := make(map[string]string)
	dnsLabels["io.rancher.container.dns"] = "true"
	links = make(map[string]string)
	links["foo/regularSvc"] = "regularSvc"
	c1 = metadata.Container{
		Name:            "client_container1",
		UUID:            "container1016",
		StackName:       "foo",
		ServiceName:     "vipSvc",
		ServiceUUID:     "vipSvc",
		PrimaryIp:       "172.17.0.3",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}
	c2 = metadata.Container{
		Name:            "client_container2",
		UUID:            "container2016",
		StackName:       "foo",
		ServiceName:     "vipSvc",
		ServiceUUID:     "vipSvc",
		PrimaryIp:       "172.17.0.4",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c1, c2}
	svcWithLinks := metadata.Service{
		Name:            "svcWithLinks",
		UUID:            "svcWithLinks",
		Kind:            "service",
		StackName:       "foo",
		Links:           links,
		Containers:      containers,
		EnvironmentName: "Default",
	}

	links = make(map[string]string)
	links["myalias"] = "regularSvc"
	c1 = metadata.Container{
		Name:            "client_container6",
		UUID:            "client_container6016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "vipSvc",
		ServiceUUID:     "vipSvc",
		PrimaryIp:       "172.17.0.6",
		State:           "running",
		EnvironmentName: "Default",
	}
	c2 = metadata.Container{
		Name:            "client_container7",
		UUID:            "client_container7016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "vipSvc",
		ServiceUUID:     "vipSvc",
		PrimaryIp:       "172.17.0.7",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c1, c2}
	svcWithLinksAlias := metadata.Service{
		Name:            "svcWithLinksAlias",
		UUID:            "svcWithLinksAlias",
		Kind:            "service",
		StackName:       "foo",
		Links:           links,
		Containers:      containers,
		EnvironmentName: "Default",
	}

	links = make(map[string]string)
	links["myaliascname"] = "externalCnameSvc"
	c1 = metadata.Container{
		Name:            "client_container66",
		UUID:            "client_container66016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "svcWithLinksAliasCname",
		ServiceUUID:     "svcWithLinksAliasCname",
		PrimaryIp:       "172.17.0.66",
		State:           "running",
		EnvironmentName: "Default",
	}
	c2 = metadata.Container{
		Name:            "client_container77",
		UUID:            "client_container77016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "svcWithLinksAliasCname",
		ServiceUUID:     "svcWithLinksAliasCname",
		PrimaryIp:       "172.17.0.77",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c1, c2}
	svcWithLinksAliasCname := metadata.Service{
		Name:            "svcWithLinksAliasCname",
		UUID:            "svcWithLinksAliasCname",
		Kind:            "service",
		StackName:       "foo",
		Links:           links,
		Containers:      containers,
		EnvironmentName: "Default",
	}

	c = metadata.Container{
		Name:            "primary",
		UUID:            "primary016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "primary",
		ServiceUUID:     "primary",
		PrimaryIp:       "192.168.0.1",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	primary := metadata.Service{
		Name:            "primary",
		UUID:            "primary",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}

	c = metadata.Container{
		Name:            "sidekick",
		UUID:            "sidekick016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "sidekick",
		ServiceUUID:     "sidekick",
		PrimaryIp:       "192.168.0.2",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	sidekick := metadata.Service{
		Name:               "sidekick",
		UUID:               "sidekick",
		Kind:               "service",
		StackName:          "foo",
		PrimaryServiceName: "primary",
		Containers:         containers,
		EnvironmentName:    "Default",
	}

	c = metadata.Container{
		Name:            "clientKubernetes",
		UUID:            "clientKubernetes016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "clientKubernetes",
		ServiceUUID:     "clientKubernetes",
		PrimaryIp:       "172.17.0.11",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	kubernetes := metadata.Service{
		Name:            "clientKubernetes",
		UUID:            "clientKubernetes",
		Kind:            "kubernetesService",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}

	c = metadata.Container{
		Name:            "clientKubernetesVip",
		UUID:            "clientKubernetesVip016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "clientKubernetesVip",
		ServiceUUID:     "clientKubernetesVip",
		PrimaryIp:       "172.17.0.12",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	kubernetesVip := metadata.Service{
		Name:            "clientKubernetesVip",
		UUID:            "clientKubernetesVip",
		Vip:             "10.1.1.1",
		Kind:            "kubernetesService",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}

	c1 = metadata.Container{
		Name:            "healthEmpty1",
		UUID:            "healthEmpty1016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "healthEmpty",
		ServiceUUID:     "healthEmpty",
		PrimaryIp:       "192.168.0.33",
		State:           "running",
		HealthState:     "initializing",
		EnvironmentName: "Default",
	}
	c2 = metadata.Container{
		Name:            "healthEmpty2",
		UUID:            "healthEmpty2016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "healthEmpty",
		ServiceUUID:     "healthEmpty",
		PrimaryIp:       "192.168.0.34",
		State:           "running",
		HealthState:     "healthy",
		EnvironmentName: "Default",
	}
	c3 := metadata.Container{
		Name:            "healthEmpty3",
		UUID:            "healthEmpty3016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "healthEmpty",
		ServiceUUID:     "healthEmpty",
		PrimaryIp:       "192.168.0.35",
		State:           "running",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c1, c2, c3}
	healthCheck := metadata.HealthCheck{
		Port: 100,
	}
	healthEmpty := metadata.Service{
		Name:            "healthEmpty",
		UUID:            "healthEmpty016d5f89-f44b",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		HealthCheck:     healthCheck,
		EnvironmentName: "Default",
	}

	c = metadata.Container{
		Name:            "primaryn",
		StackName:       "foo",
		ServiceName:     "primaryn",
		ServiceUUID:     "primaryn",
		PrimaryIp:       "192.168.0.11",
		State:           "running",
		UUID:            "primaryn016d5f89-f44b",
		EnvironmentName: "Default",
	}
	containers = []metadata.Container{c}
	primaryn := metadata.Service{
		Name:            "primaryn",
		UUID:            "primaryn",
		Kind:            "service",
		StackName:       "foo",
		Containers:      containers,
		EnvironmentName: "Default",
	}

	c = metadata.Container{
		Name:        "sidekickn",
		UUID:        "sidekickn016d5f89-f44b",
		StackName:   "foo",
		ServiceName: "sidekickn",
		ServiceUUID: "sidekickn",
		State:       "running",
		NetworkFromContainerUUID: "primaryn016d5f89-f44b",
		EnvironmentName:          "Default",
	}
	containers = []metadata.Container{c}
	sidekickn := metadata.Service{
		Name:               "sidekickn",
		UUID:               "sidekickn",
		Kind:               "service",
		StackName:          "foo",
		PrimaryServiceName: "primaryn",
		Containers:         containers,
		EnvironmentName:    "Default",
	}

	services = append(services, kubernetes, healthEmpty, primaryn,
		sidekickn, kubernetesVip, clientip1Svc, vip, primary,
		sidekick, regular, stopped, stoppedone, unhealthy,
		externalCname, svcWithLinksAliasCname, svcWithLinksAlias,
		externalIPs, alias, client, svcWithLinks)
	return services, nil
}

func (mf tMetaFetcher) GetContainers() ([]metadata.Container, error) {
	dnsLabels := make(map[string]string)
	dnsLabels["io.rancher.container.dns"] = "true"
	c1 := metadata.Container{
		Name:            "clientip1",
		UUID:            "clientip1016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "clientip1Svc",
		ServiceUUID:     "clientip1Svc",
		PrimaryIp:       "172.17.0.2",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
		DnsSearch:       []string{"regularSvc.discover.internal", "foo.discover.internal", "discover.internal"},
	}

	c2 := metadata.Container{
		Name:            "clientip3",
		UUID:            "clientip3016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "svcWithLinks",
		ServiceUUID:     "svcWithLinks",
		PrimaryIp:       "172.17.0.3",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
		DnsSearch:       []string{"svcWithLinks.discover.internal", "foo.discover.internal", "discover.internal"},
	}

	c3 := metadata.Container{
		Name:            "clientip4",
		UUID:            "clientip4016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "svcWithLinks",
		ServiceUUID:     "svcWithLinks",
		PrimaryIp:       "172.17.0.4",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
		DnsSearch:       []string{"svcWithLinks.discover.internal", "foo.discover.internal", "discover.internal"},
	}
	c4 := metadata.Container{
		Name:            "client_container6",
		UUID:            "client_container6016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "svcWithLinksAlias",
		ServiceUUID:     "svcWithLinksAlias",
		PrimaryIp:       "172.17.0.6",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
		DnsSearch:       []string{"svcWithLinksAlias.discover.internal", "foo.discover.internal", "discover.internal"},
	}
	c5 := metadata.Container{
		Name:            "client_container7",
		UUID:            "client_container7016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "svcWithLinksAlias",
		ServiceUUID:     "svcWithLinksAlias",
		PrimaryIp:       "172.17.0.7",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
		DnsSearch:       []string{"svcWithLinksAlias.discover.internal", "foo.discover.internal", "discover.internal"},
	}
	c6 := metadata.Container{
		Name:            "clientStandalone",
		UUID:            "clientStandalone016d5f89-f44b",
		PrimaryIp:       "172.17.0.10",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
		DnsSearch:       []string{"regularSvc.discover.internal", "foo.discover.internal", "discover.internal"},
	}
	c7 := metadata.Container{
		Name:            "clientKubernetes",
		UUID:            "clientKubernetes016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "clientKubernetes",
		ServiceUUID:     "clientKubernetes",
		PrimaryIp:       "172.17.0.11",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}

	c8 := metadata.Container{
		Name:            "clientKubernetesVip",
		UUID:            "clientKubernetesVip016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "clientKubernetesVip",
		ServiceUUID:     "clientKubernetesVip",
		PrimaryIp:       "172.17.0.12",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}

	c9 := metadata.Container{
		Name:            "clientKubernetesVip",
		UUID:            "clientKubernetesVip016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "clientKubernetesVip",
		ServiceUUID:     "clientKubernetesVip",
		PrimaryIp:       "192.168.0.33",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}
	c10 := metadata.Container{
		Name:            "healthEmpty1",
		UUID:            "healthEmpty1016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "healthEmpty",
		ServiceUUID:     "healthEmpty",
		PrimaryIp:       "192.168.0.33",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}

	c11 := metadata.Container{
		Name:            "networkFromMaster",
		UUID:            "networkFromMaster016d5f89-f44b",
		PrimaryIp:       "192.168.0.34",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}
	c12 := metadata.Container{
		Name:  "networkFromChild",
		UUID:  "networkFromChild016d5f89-f44b",
		State: "running",
		NetworkFromContainerUUID: "networkFromMaster016d5f89-f44b",
		EnvironmentName:          "Default",
		Labels:                   dnsLabels,
	}

	c13 := metadata.Container{
		Name:        "sidekickn",
		UUID:        "sidekickn016d5f89-f44b",
		StackName:   "foo",
		ServiceName: "sidekickn",
		ServiceUUID: "sidekickn",
		State:       "running",
		NetworkFromContainerUUID: "primaryn016d5f89-f44b",
		EnvironmentName:          "Default",
		Labels:                   dnsLabels,
	}

	c14 := metadata.Container{
		Name:            "primaryn",
		UUID:            "primaryn016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "primaryn",
		ServiceUUID:     "primaryn",
		PrimaryIp:       "192.168.0.11",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
	}

	c15 := metadata.Container{
		Name:            "client_container66",
		UUID:            "client_container66016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "svcWithLinksAliasCname",
		ServiceUUID:     "svcWithLinksAliasCname",
		PrimaryIp:       "172.17.0.66",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
		DnsSearch:       []string{"svcWithLinksAliasCname.discover.internal", "foo.discover.internal", "discover.internal"},
	}
	c16 := metadata.Container{
		Name:            "client_container77",
		UUID:            "client_container77016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "svcWithLinksAliasCname",
		ServiceUUID:     "svcWithLinksAliasCname",
		PrimaryIp:       "172.17.0.77",
		State:           "running",
		EnvironmentName: "Default",
		Labels:          dnsLabels,
		DnsSearch:       []string{"svcWithLinksAliasCname.discover.internal", "foo.discover.internal", "discover.internal"},
	}
	links := make(map[string]string)
	links["containerLink"] = "regular_container016d5f89-f44b"
	c17 := metadata.Container{
		Name:            "client_container777",
		UUID:            "client_container777016d5f89-f44b",
		PrimaryIp:       "172.17.0.777",
		State:           "running",
		Links:           links,
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}

	c18 := metadata.Container{
		Name:            "regular_container",
		UUID:            "regular_container016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "regularSvc",
		ServiceUUID:     "regularSvc",
		PrimaryIp:       "192.168.1.1",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}

	c19 := metadata.Container{
		Name:            "healthEmpty2",
		UUID:            "healthEmpty2016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "healthEmpty",
		PrimaryIp:       "192.168.0.34",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}

	c20 := metadata.Container{
		Name:            "healthEmpty3",
		UUID:            "healthEmpty3016d5f89-f44b",
		StackName:       "foo",
		ServiceName:     "healthEmpty",
		PrimaryIp:       "192.168.0.35",
		State:           "running",
		Labels:          dnsLabels,
		EnvironmentName: "Default",
	}

	containers := []metadata.Container{c1, c2, c3, c4, c5, c6,
		c7, c8, c9, c10, c11, c12, c13, c14,
		c15, c16, c17, c18, c19, c20}
	return containers, nil
}

func (mf tMetaFetcher) OnChange(intervalSeconds int, do func(string)) {
	return
}

func (mf tMetaFetcher) GetSelfHost() (metadata.Host, error) {
	var host metadata.Host
	return host, nil
}
