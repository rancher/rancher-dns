package main

import (
	"bufio"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher-metadata/metadata"
	"os"
	"strings"
)

type MetadataFetcher interface {
	GetService(svcName string, stackName string) (*metadata.Service, error)
	GetServices() ([]metadata.Service, error)
	GetContainers() ([]metadata.Container, error)
	OnChange(intervalSeconds int, do func(string))
	GetSelfHost() (metadata.Host, error)
}

type rMetaFetcher struct {
	metadataClient metadata.Client
}

type ConfigGenerator struct {
	metaFetcher MetadataFetcher
}

func (mf rMetaFetcher) GetSelfHost() (metadata.Host, error) {
	return mf.metadataClient.GetSelfHost()
}

func (c *ConfigGenerator) Init(metadataServer *string) error {
	metadataClient, err := metadata.NewClientAndWait(fmt.Sprintf("http://%s/2015-12-19", *metadataServer))
	if err != nil {
		logrus.Errorf("Error initiating metadata client: %v", err)
		return err
	}

	c.metaFetcher = rMetaFetcher{
		metadataClient: metadataClient,
	}
	return nil
}

func (c *ConfigGenerator) GenerateAnswers() (Answers, error) {
	answers := make(Answers)
	aRecs, cRecs, clientIpsToLinks, clientIpToContainer, svcNameToSvc, err := c.GetRecords()
	if err != nil {
		return nil, err
	}

	//generate client record
	for clientIp, container := range clientIpToContainer {
		cARecs := make(map[string]RecordA)
		cCnameRecs := make(map[string]RecordCname)
		for key, linkAlias := range clientIpsToLinks[clientIp] {
			if strings.EqualFold(key, linkAlias) {
				// skip non-aliased service links
				// they are present in defaults
				continue
			}
			linkedService := svcNameToSvc[key]
			linkServiceFqdn := getServiceFqdn(&linkedService)
			if _, ok := aRecs[linkServiceFqdn]; ok {
				//we store 2 A records for link:
				// a) linkName.namespace
				// b) linkName.stackName.namespace
				cARecs[getLinkStackFqdn(linkAlias, &linkedService)] = aRecs[linkServiceFqdn]
				cARecs[getLinkGlobalFqdn(linkAlias, &linkedService)] = aRecs[linkServiceFqdn]
			} else if _, ok := cRecs[linkServiceFqdn]; ok {
				cCnameRecs[getLinkStackFqdn(linkAlias, &linkedService)] = cRecs[linkServiceFqdn]
				cCnameRecs[getLinkGlobalFqdn(linkAlias, &linkedService)] = cRecs[linkServiceFqdn]
			}
		}
		search := []string{}
		recurse := []string{}
		if container.DnsSearch != nil {
			search = container.DnsSearch
		}
		if container.Dns != nil {
			//exclude 169.254.169.250
			for _, dns := range container.Dns {
				if strings.EqualFold(dns, "169.254.169.250") {
					continue
				}
				recurse = append(recurse, dns)
			}
		}

		a := ClientAnswers{
			A:             cARecs,
			Cname:         cCnameRecs,
			Search:        search,
			Recurse:       recurse,
			Authoritative: []string{},
		}
		answers[clientIp] = a
	}

	globalRecurse, err := getGlobalRecurse()
	if err != nil {
		return nil, err
	}

	//generate default record
	a := ClientAnswers{
		A:             aRecs,
		Cname:         cRecs,
		Search:        []string{getDefaultRancherNamespace()},
		Recurse:       globalRecurse,
		Authoritative: []string{getDefaultRancherNamespace()},
	}
	answers["default"] = a

	return answers, nil
}

func (c *ConfigGenerator) GetRecords() (map[string]RecordA, map[string]RecordCname, map[string]map[string]string, map[string]metadata.Container, map[string]metadata.Service, error) {
	aRecs := make(map[string]RecordA)
	cRecs := make(map[string]RecordCname)
	clientIpsToLinks := make(map[string]map[string]string)
	clientIpToContainer := make(map[string]metadata.Container)
	svcNameToSvc := make(map[string]metadata.Service)

	services, err := c.metaFetcher.GetServices()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	containers, err := c.metaFetcher.GetContainers()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	host, err := c.metaFetcher.GetSelfHost()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	uuidToPrimaryIp := make(map[string]string)
	for _, c := range containers {
		if c.PrimaryIp == "" {
			continue
		}
		uuidToPrimaryIp[c.UUID] = c.PrimaryIp
	}

	// get service records
	for _, svc := range services {
		svcNameToSvc[fmt.Sprintf("%s/%s", svc.StackName, svc.Name)] = svc
		records, err := c.getServiceEndpoints(&svc, uuidToPrimaryIp)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		for i, rec := range records {
			if rec.IsCname {
				cnameRec := RecordCname{
					Answer: rec.IP,
				}
				cRecs[getServiceFqdn(&svc)] = cnameRec
				continue
			}
			add := false
			if rec.IsHealthy {
				if rec.Container != nil {
				}
				add = true
			} else {
				if rec.Container != nil {
				}
				if i == len(records)-1 {
					add = true
				}
			}
			if add {
				aRec := RecordA{
					Answer: []string{rec.IP},
				}
				if existing, ok := aRecs[getServiceFqdn(&svc)]; ok {
					aRec.Answer = append(aRec.Answer, existing.Answer...)
				}
				//add to the service record
				aRecs[getServiceFqdn(&svc)] = aRec
			}

			if rec.Container != nil {
				aRec := RecordA{
					Answer: []string{rec.Container.PrimaryIp},
				}
				//add to container record
				aRecs[getContainerFqdn(rec.Container, &svc)] = aRec
				//client section only for the containers running on the same host
				if rec.Container.HostUUID == host.UUID {
					clientIpToContainer[rec.Container.PrimaryIp] = (*rec.Container)
				}

			}
		}
	}

	for _, c := range containers {
		primaryIP := c.PrimaryIp
		if primaryIP == "" && c.NetworkFromContainerUUID != "" {
			primaryIP = uuidToPrimaryIp[c.NetworkFromContainerUUID]
		}

		if primaryIP == "" {
			continue
		}

		aRec := RecordA{
			Answer: []string{primaryIP},
		}
		var svc metadata.Service
		if c.ServiceName != "" && c.StackName != "" {
			svc = svcNameToSvc[fmt.Sprintf("%s/%s", c.StackName, c.ServiceName)]
		}
		aRecs[getContainerFqdn(&c, &svc)] = aRec

		//client section only for the containers running on the same host
		if c.HostUUID == host.UUID && c.PrimaryIp != "" {
			if svc.Name != "" {
				clientIpsToLinks[c.PrimaryIp] = svc.Links
			}
			clientIpToContainer[c.PrimaryIp] = c
		}
	}

	// add metadata record
	aRec := RecordA{
		Answer: []string{"169.254.169.250"},
	}
	//add to the service record
	aRecs[fmt.Sprintf("rancher-metadata.%s.", getDefaultRancherNamespace())] = aRec

	return aRecs, cRecs, clientIpsToLinks, clientIpToContainer, svcNameToSvc, nil
}

func getGlobalRecurse() ([]string, error) {
	var recurse []string
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return recurse, err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "nameserver") {
			recurse = append(recurse, strings.TrimSpace(l[11:len(l)]))
		}
	}
	return recurse, nil
}

func getDefaultRancherNamespace() string {
	return "rancher.internal"
}

func getDefaultKubernetesNamespace() string {
	return "cluster.local"
}

func getServiceGlobalNamespace(s *metadata.Service) string {
	if strings.EqualFold(s.Kind, "kubernetesService") {
		return fmt.Sprintf("svc.%s", getDefaultKubernetesNamespace())
	}
	return getDefaultRancherNamespace()
}

func getServiceFqdn(s *metadata.Service) string {
	namespace := getServiceGlobalNamespace(s)
	if s.PrimaryServiceName == "" || strings.EqualFold(s.Name, s.PrimaryServiceName) {
		return strings.ToLower(fmt.Sprintf("%s.%s.%s.", s.Name, s.StackName, namespace))
	} else {
		return strings.ToLower(fmt.Sprintf("%s.%s.%s.%s.", s.Name, s.PrimaryServiceName, s.StackName, namespace))
	}
}

func getLinkGlobalFqdn(linkName string, s *metadata.Service) string {
	return strings.ToLower(fmt.Sprintf("%s.%s.", linkName, getServiceGlobalNamespace(s)))
}

func getLinkStackFqdn(linkName string, s *metadata.Service) string {
	return strings.ToLower(fmt.Sprintf("%s.%s.%s.", linkName, s.StackName, getServiceGlobalNamespace(s)))
}

func getContainerFqdn(c *metadata.Container, s *metadata.Service) string {
	if s != nil && strings.EqualFold(s.Kind, "kubernetesService") {
		return strings.ToLower(fmt.Sprintf("%s.%s.%s.%s.", c.Name, s.Name, s.StackName, getServiceGlobalNamespace(s)))

	}
	return strings.ToLower(fmt.Sprintf("%s.%s.", c.Name, getDefaultRancherNamespace()))
}

func (c *ConfigGenerator) getServiceEndpoints(svc *metadata.Service, uuidToPrimaryIp map[string]string) ([]*Record, error) {
	var records []*Record
	var err error
	if strings.EqualFold(svc.Kind, "externalService") {
		records = c.getExternalServiceEndpoints(svc)
	} else if strings.EqualFold(svc.Kind, "dnsService") {
		records, err = c.getAliasServiceEndpoints(svc, uuidToPrimaryIp)
		if err != nil {
			return nil, err
		}
	} else {
		records = c.getRegularServiceEndpoints(svc, uuidToPrimaryIp)
	}
	return records, nil
}

func (c *ConfigGenerator) getRegularServiceEndpoints(svc *metadata.Service, uuidToPrimaryIp map[string]string) []*Record {
	var recs []*Record
	//get vip
	if svc.Vip != "" {
		rec := &Record{
			IP:        svc.Vip,
			IsHealthy: true,
			IsCname:   false,
		}
		recs = append(recs, rec)
		return recs
	}

	//get containers if not vip
	for i, c := range svc.Containers {
		isRunning := strings.EqualFold(c.State, "running") || strings.EqualFold(c.State, "starting")
		isHealthy := (strings.EqualFold(c.HealthState, "") && svc.HealthCheck.Port == 0) || strings.EqualFold(c.HealthState, "healthy") || strings.EqualFold(c.HealthState, "updating-healthy")
		primaryIP := c.PrimaryIp
		if primaryIP == "" && c.NetworkFromContainerUUID != "" {
			primaryIP = uuidToPrimaryIp[c.NetworkFromContainerUUID]
		}
		rec := &Record{
			IP:        primaryIP,
			IsHealthy: isHealthy && isRunning,
			IsCname:   false,
			Container: &svc.Containers[i],
		}
		recs = append(recs, rec)
	}
	return recs
}

func (c *ConfigGenerator) getExternalServiceEndpoints(svc *metadata.Service) []*Record {
	var recs []*Record
	for _, e := range svc.ExternalIps {
		rec := &Record{
			IP:        e,
			IsHealthy: true,
			IsCname:   false,
		}
		recs = append(recs, rec)
	}
	if svc.Hostname != "" {
		rec := &Record{
			IP:        svc.Hostname,
			IsHealthy: true,
			IsCname:   true,
		}
		recs = append(recs, rec)
	}
	return recs
}

func (c *ConfigGenerator) getAliasServiceEndpoints(svc *metadata.Service, uuidToPrimaryIp map[string]string) ([]*Record, error) {
	var recs []*Record
	for link := range svc.Links {
		svcName := strings.SplitN(link, "/", 2)
		service, err := c.metaFetcher.GetService(svcName[1], svcName[0])
		if err != nil {
			return nil, err
		}
		if service == nil {
			continue
		}
		newRecs, err := c.getServiceEndpoints(service, uuidToPrimaryIp)
		if err != nil {
			return nil, err
		}
		recs = append(recs, newRecs...)
	}
	return recs, nil
}

type Record struct {
	IP        string
	IsHealthy bool
	IsCname   bool
	Container *metadata.Container
}

func (mf rMetaFetcher) GetService(svcName string, stackName string) (*metadata.Service, error) {
	svcs, err := mf.metadataClient.GetServices()
	if err != nil {
		return nil, err
	}
	var service metadata.Service
	for _, svc := range svcs {
		if strings.EqualFold(svc.Name, svcName) && strings.EqualFold(svc.StackName, stackName) {
			service = svc
			break
		}
	}
	return &service, nil
}

func (mf rMetaFetcher) GetServices() ([]metadata.Service, error) {
	return mf.metadataClient.GetServices()
}

func (mf rMetaFetcher) GetContainers() ([]metadata.Container, error) {
	return mf.metadataClient.GetContainers()
}

func (mf rMetaFetcher) OnChange(intervalSeconds int, do func(string)) {
	mf.metadataClient.OnChange(intervalSeconds, do)
}
