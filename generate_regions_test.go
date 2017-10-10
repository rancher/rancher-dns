package main

import (
	"strings"
	"testing"

	"github.com/rancher/go-rancher-metadata/metadata"
)

var c1 *ConfigGenerator

func init() {
	// region1
	c1 = &ConfigGenerator{
		metaFetcher: &sMetaFetcher{},
	}
}

type sMetaFetcher struct {
}

// service link to another region/environment/stack/service
func TestClientWithLinksAliasRegionEnvironment(t *testing.T) {
	answers, err := c1.GenerateAnswers()

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

		fqdns := []string{"mylink.", "mylink.rancher.internal."}
		for _, fqdn := range fqdns {
			val, ok := c.A[fqdn]
			if !ok {
				t.Fatalf("Can't find the fqnd link %s", fqdn)
			}
			if len(val.Answer) != 2 {
				t.Fatalf("Incorrect number of answers [%v], expected 2", len(val.Answer))
			}
			ans := make(map[string]bool)
			for _, ival := range val.Answer {
				ans[ival] = true
			}
			expectedIps := []string{"173.17.0.19", "173.17.0.18"}
			for _, ival := range expectedIps {
				if _, ok := ans[ival]; !(ok) {
					t.Fatalf("Expected record [%s] is not present : %v", ival, val.Answer)
				}
			}
		}
	}
}

// service link to another environment/stack/service (local region)
func TestClientWithLinksAliasEnvironment(t *testing.T) {
	answers, err := c1.GenerateAnswers()

	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	ips := []string{"172.17.0.9", "172.17.0.10"}
	for _, ip := range ips {
		c := getClientAnswers(answers, ip)
		if c == nil {
			t.Fatalf("Can't find client answers for: [%v]", ip)
		}
		if len(c.A) != 2 {
			t.Fatalf("Incorrect number of A records, should be 2: [%v]", len(c.A))
		}

		fqdns := []string{"yourlink.", "yourlink.rancher.internal."}
		for _, fqdn := range fqdns {
			val, ok := c.A[fqdn]
			if !ok {
				t.Fatalf("Can't find the fqnd link %s", fqdn)
			}
			if len(val.Answer) != 1 {
				t.Fatalf("Incorrect number of answers [%v], expected 1", len(val.Answer))
			}
			if val.Answer[0] != "172.17.0.8" {
				t.Fatalf("Record [%s] doesn't match expected value 172.17.0.8", val.Answer[0])
			}
		}
	}
}

// service alias to region/environment/stack/service
func TestAliasServiceRegion(t *testing.T) {
	answers, err := c1.GenerateAnswers()

	if err != nil {
		t.Fatalf("Error generating answers %v", err)
	}

	a := getRecordAFromDefault(answers, "svcAlias.stackB.rancher.internal.")
	if len(a.Answer) != 1 {
		t.Fatalf("Incorrect number of A records, should be 1: [%v]", len(a.Answer))
	}

	if a.Answer[0] != "173.17.0.20" {
		t.Fatalf("Record [%s] doesn't match expected value 173.17.0.20", a.Answer[0])
	}
}

func (mf sMetaFetcher) GetServiceInLocalEnvironment(stackName string, svcName string) (metadata.Service, error) {
	var service metadata.Service
	return service, nil
}

func (mf sMetaFetcher) GetService(link string) (*metadata.Service, error) {
	splitSvcName := strings.Split(link, "/")
	var linkedService metadata.Service
	var err error
	if len(splitSvcName) == 4 {
		linkedService, err = mf.GetServiceFromRegionEnvironment(splitSvcName[0], splitSvcName[1], splitSvcName[2], splitSvcName[3])
	} else if len(splitSvcName) == 3 {
		linkedService, err = mf.GetServiceInLocalRegion(splitSvcName[0], splitSvcName[1], splitSvcName[2])
	} else {
		linkedService, err = mf.GetServiceInLocalEnvironment(splitSvcName[0], splitSvcName[1])
	}
	return &linkedService, err
}

func (mf sMetaFetcher) GetServices() ([]metadata.Service, error) {
	var services []metadata.Service
	var containers []metadata.Container
	links := make(map[string]string)
	links["region2/alpha/stackX/svcX"] = "mylink"
	c1 := metadata.Container{
		Name:            "client_container",
		StackName:       "stackA",
		ServiceName:     "kinara",
		EnvironmentUUID: "foo",
		PrimaryIp:       "172.17.0.6",
		State:           "running",
	}

	c2 := metadata.Container{
		Name:            "client_container",
		StackName:       "stackA",
		ServiceName:     "kinara",
		EnvironmentUUID: "foo",
		PrimaryIp:       "172.17.0.7",
		State:           "running",
	}
	containers = []metadata.Container{c1, c2}
	svcWithRegionAlias := metadata.Service{
		Name:            "kinara",
		Kind:            "service",
		StackName:       "stackA",
		EnvironmentUUID: "foo",
		Links:           links,
		Containers:      containers,
	}

	links1 := make(map[string]string)
	links1["bar/stackC/drone"] = "yourlink"
	c1 = metadata.Container{
		Name:            "client_container",
		StackName:       "stackB",
		ServiceName:     "svcB",
		EnvironmentUUID: "foo",
		PrimaryIp:       "172.17.0.9",
		State:           "running",
	}
	c2 = metadata.Container{
		Name:            "client_container",
		StackName:       "stackB",
		ServiceName:     "svcB",
		EnvironmentUUID: "foo",
		PrimaryIp:       "172.17.0.10",
		State:           "running",
	}
	containers = []metadata.Container{c1, c2}
	svcWithEnvironmentAlias := metadata.Service{
		Name:            "svcB",
		Kind:            "service",
		StackName:       "stackB",
		EnvironmentUUID: "foo",
		Links:           links1,
		Containers:      containers,
	}

	links2 := make(map[string]string)
	links2["region2/alpha/stackY/svcY"] = "region2/alpha/stackY/svcY"
	svcalias := metadata.Service{
		Name:            "svcAlias",
		Kind:            "dnsService",
		StackName:       "stackB",
		EnvironmentUUID: "foo",
		Links:           links2,
	}

	services = append(services, svcWithRegionAlias, svcWithEnvironmentAlias, svcalias)
	return services, nil
}

func (mf sMetaFetcher) GetContainers() ([]metadata.Container, error) {
	var containers []metadata.Container
	c1 := metadata.Container{
		Name:            "client_container",
		StackName:       "stackA",
		ServiceName:     "kinara",
		EnvironmentUUID: "foo",
		PrimaryIp:       "172.17.0.6",
		State:           "running",
	}
	c2 := metadata.Container{
		Name:            "client_container",
		StackName:       "stackA",
		ServiceName:     "kinara",
		EnvironmentUUID: "foo",
		PrimaryIp:       "172.17.0.7",
		State:           "running",
	}
	c3 := metadata.Container{
		Name:            "client_container",
		StackName:       "stackB",
		ServiceName:     "svcB",
		EnvironmentUUID: "foo",
		PrimaryIp:       "172.17.0.9",
		State:           "running",
	}
	c4 := metadata.Container{
		Name:            "client_container",
		StackName:       "stackB",
		ServiceName:     "svcB",
		EnvironmentUUID: "foo",
		PrimaryIp:       "172.17.0.10",
		State:           "running",
	}
	containers = append(containers, c1, c2, c3, c4)
	return containers, nil
}

func (mf sMetaFetcher) OnChange(intervalSeconds int, do func(string)) {
	return
}

func (mf sMetaFetcher) GetSelfHost() (metadata.Host, error) {
	host := metadata.Host{
		Name:            "host",
		EnvironmentUUID: "foo",
	}
	return host, nil
}

func (mf sMetaFetcher) GetRegionName() (string, error) {
	return "region1", nil
}

func (mf sMetaFetcher) GetServiceFromRegionEnvironment(regionName string, envName string, stackName string, svcName string) (metadata.Service, error) {
	var service metadata.Service
	var containers []metadata.Container
	if regionName == "region2" && envName == "alpha" && stackName == "stackX" && svcName == "svcX" {
		c1 := metadata.Container{
			Name:            "client_container",
			StackName:       "stackX",
			ServiceName:     "svcx",
			EnvironmentUUID: "alpha",
			PrimaryIp:       "173.17.0.18",
			State:           "running",
		}
		c2 := metadata.Container{
			Name:            "client_container",
			StackName:       "stackX",
			ServiceName:     "svcx",
			EnvironmentUUID: "alpha",
			PrimaryIp:       "173.17.0.19",
			State:           "running",
		}
		containers = []metadata.Container{c1, c2}
		service = metadata.Service{
			Name:            "svcX",
			Kind:            "service",
			StackName:       "stackX",
			EnvironmentUUID: "alpha",
			Containers:      containers,
		}
	} else if regionName == "region2" && envName == "alpha" && stackName == "stackY" && svcName == "svcY" {
		c1 := metadata.Container{
			Name:            "client_container",
			StackName:       "stackY",
			ServiceName:     "svcY",
			EnvironmentUUID: "alpha",
			PrimaryIp:       "173.17.0.20",
			State:           "running",
		}
		containers = []metadata.Container{c1}
		service = metadata.Service{
			Name:            "svcY",
			Kind:            "service",
			StackName:       "stackY",
			EnvironmentUUID: "alpha",
			Containers:      containers,
		}
	} else if regionName == "region1" && envName == "bar" && stackName == "stackC" && svcName == "drone" {
		c3 := metadata.Container{
			Name:            "client_container",
			StackName:       "stackC",
			ServiceName:     "drone",
			EnvironmentUUID: "bar",
			PrimaryIp:       "172.17.0.8",
			State:           "running",
		}
		containers = []metadata.Container{c3}
		service = metadata.Service{
			Name:            "drone",
			Kind:            "service",
			StackName:       "stackC",
			EnvironmentUUID: "bar",
			Containers:      containers,
		}
	}
	return service, nil
}

func (mf sMetaFetcher) GetServiceInLocalRegion(envName string, stackName string, svcName string) (metadata.Service, error) {
	var service metadata.Service
	regionName, err := mf.GetRegionName()
	if err != nil {
		return service, err
	}
	return mf.GetServiceFromRegionEnvironment(regionName, envName, stackName, svcName)
}
