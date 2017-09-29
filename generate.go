package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	set "github.com/deckarep/golang-set"
	"github.com/rancher/go-rancher-metadata/metadata"
)

const (
	RANCHER_DOMAIN     = "discover.internal"
	OLD_RANCHER_DOMAIN = "rancher.internal"
)

var (
	fallbackRecurse = []string{"8.8.8.8", "8.8.4.4"}
)

type MetadataFetcher interface {
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
	metadataClient, err := metadata.NewClientAndWait(fmt.Sprintf("http://%s/2017-04-22", *metadataServer))
	if err != nil {
		logrus.Errorf("Error initiating metadata client: %v", err)
		return err
	}

	c.metaFetcher = rMetaFetcher{
		metadataClient: metadataClient,
	}
	return nil
}

func getSearchDomains(c *metadata.Container) []string {
	globalNamespace := getDefaultRancherNamespace()
	stackNamespace := fmt.Sprintf("%s.%s.%s", c.StackName, c.EnvironmentName, globalNamespace)
	envrironmentNamespace := fmt.Sprintf("%s.%s", c.EnvironmentName, globalNamespace)
	rancherSearch := []string{strings.ToLower(stackNamespace), strings.ToLower(envrironmentNamespace)}
	existing := set.NewSet()
	for _, value := range rancherSearch {
		existing.Add(value)
	}

	for _, value := range c.DnsSearch {
		if existing.Contains(value) {
			continue
		}
		if c.UUID != "" && len(c.UUID) > 12 && strings.EqualFold(value, fmt.Sprintf("%s.%s", c.UUID[:12], globalNamespace)) {
			continue
		}
		rancherSearch = append(rancherSearch, value)
	}

	return rancherSearch
}

func getAliasFqdn(alias string) string {
	return fmt.Sprintf("%s.", alias)
}

func (c *ConfigGenerator) GenerateAnswers() (Answers, error) {
	answers := make(Answers)
	aRecs, cRecs, clientUuidToServiceLinks, clientUuidToContainerLinks, clientUuidToContainer, svcUUIDToSvc, err := c.GetRecords()
	if err != nil {
		return nil, err
	}

	//generate client record
	for uuid, container := range clientUuidToContainer {
		cARecs := make(map[string]RecordA)
		cCnameRecs := make(map[string]RecordCname)

		// 1. set container links
		for linkName, targetIp := range clientUuidToContainerLinks[uuid] {
			aRec := RecordA{
				Answer: []string{targetIp},
			}
			cARecs[getAliasFqdn(linkName)] = aRec
		}

		// 2. set service links
		// note that service link overrides the container link (if the names collide)
		for linkAlias, linkName := range clientUuidToServiceLinks[uuid] {
			if _, ok := svcUUIDToSvc[linkName]; !ok {
				continue
			}
			linkedService := svcUUIDToSvc[linkName]

			linkServiceFqdn := getServiceFqdn(&linkedService)
			if _, ok := aRecs[linkServiceFqdn]; ok {
				cARecs[getAliasFqdn(linkAlias)] = aRecs[linkServiceFqdn]
			} else if _, ok := cRecs[linkServiceFqdn]; ok {
				cCnameRecs[getAliasFqdn(linkAlias)] = cRecs[linkServiceFqdn]
			}
		}

		search := getSearchDomains(&container)
		recurse := []string{}
		if container.Dns != nil {
			//exclude 169.254.169.250
			for _, dns := range container.Dns {
				if invalidRecurse(dns) {
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
		answers[uuid[:12]] = a
		if container.PrimaryIp != "" {
			answers[container.PrimaryIp] = a
		}
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

func invalidRecurse(dns string) bool {
	result := false
	for _, neverRecurseTo := range splitTrim(*neverRecurseTo, ",") {
		result = result || dns == neverRecurseTo
	}
	return result || strings.HasPrefix(dns, "127.")
}

func (c *ConfigGenerator) GetRecords() (map[string]RecordA, map[string]RecordCname, map[string]map[string]string, map[string]map[string]string, map[string]metadata.Container, map[string]metadata.Service, error) {
	aRecs := make(map[string]RecordA)
	cRecs := make(map[string]RecordCname)
	clientUuidToServiceLinks := make(map[string]map[string]string)
	clientUuidToContainer := make(map[string]metadata.Container)
	svcUUIDToSvc := make(map[string]metadata.Service)
	clientUuidToContainerLinks := make(map[string]map[string]string)

	services, err := c.metaFetcher.GetServices()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	containers, err := c.metaFetcher.GetContainers()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	host, err := c.metaFetcher.GetSelfHost()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
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
		svcUUIDToSvc[svc.UUID] = svc
	}

	for _, svc := range services {
		records, err := c.getServiceEndpoints(&svc, uuidToPrimaryIp, svcUUIDToSvc)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}
		for i, rec := range records {
			if rec.IsCname {
				cnameRec := RecordCname{
					Answer: fmt.Sprintf("%s.", rec.IP),
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
					if existing, ok := aRecs[getServiceFqdn(&svc)]; ok {
						if len(existing.Answer) == 0 {
							add = true
						}
					} else {
						add = true
					}
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

			if rec.Container != nil && rec.Container.PrimaryIp != "" {
				aRec := RecordA{
					Answer: []string{rec.Container.PrimaryIp},
				}
				//add to container record
				aRecs[getContainerFqdn(rec.Container, &svc)] = aRec
				//client section only for the containers running on the same host
				if rec.Container.HostUUID == host.UUID {
					clientUuidToContainer[rec.Container.UUID] = (*rec.Container)
				}

			}
		}
	}

	containerUUIDToContainerIP := make(map[string]string)
	var cWithIps []metadata.Container
	for _, c := range containers {
		primaryIP := c.PrimaryIp
		if primaryIP == "" && c.NetworkFromContainerUUID != "" {
			primaryIP = uuidToPrimaryIp[c.NetworkFromContainerUUID]
		}

		if primaryIP == "" {
			continue
		}

		containerUUIDToContainerIP[c.UUID] = primaryIP
		cWithIps = append(cWithIps, c)
	}

	for _, c := range cWithIps {
		primaryIP := containerUUIDToContainerIP[c.UUID]
		aRec := RecordA{
			Answer: []string{primaryIP},
		}
		var svc metadata.Service
		if c.ServiceUUID != "" {
			svc = svcUUIDToSvc[c.ServiceUUID]
		}
		aRecs[getContainerFqdn(&c, &svc)] = aRec

		//client section only for the containers running on the same host
		if c.HostUUID == host.UUID && c.PrimaryIp != "" {
			if svc.Name != "" {
				clientUuidToServiceLinks[c.UUID] = svc.Links
			}
			clientUuidToContainer[c.UUID] = c

			//get container links
			containerLinks := make(map[string]string)
			for linkName, linkedContainerUUID := range c.Links {
				targetIP := containerUUIDToContainerIP[linkedContainerUUID]
				if targetIP == "" {
					continue
				}
				containerLinks[linkName] = targetIP
			}
			clientUuidToContainerLinks[c.UUID] = containerLinks
		}
	}

	// add metadata record
	aRec := RecordA{
		Answer: splitTrim(*metadataAnswer, ","),
	}
	//add to the service record
	aRecs[fmt.Sprintf("rancher-metadata.%s.", getDefaultRancherNamespace())] = aRec

	return aRecs, cRecs, clientUuidToServiceLinks, clientUuidToContainerLinks, clientUuidToContainer, svcUUIDToSvc, nil
}

func splitTrim(s string, sep string) []string {
	t := strings.Split(s, ",")
	for i, _ := range t {
		t[i] = strings.TrimSpace(t[i])
	}
	return t
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
			dns := strings.TrimSpace(l[11:len(l)])
			if invalidRecurse(dns) {
				continue
			}
			recurse = append(recurse, dns)
		}
	}

	if len(recurse) == 0 {
		return fallbackRecurse, nil
	}

	return recurse, nil
}

func getDefaultRancherNamespace() string {
	return RANCHER_DOMAIN
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
		return strings.ToLower(fmt.Sprintf("%s.%s.%s.%s.", s.Name, s.StackName, s.EnvironmentName, namespace))
	} else {
		return strings.ToLower(fmt.Sprintf("%s.%s.%s.%s.%s.", s.Name, s.PrimaryServiceName, s.StackName, s.EnvironmentName, namespace))
	}
}

func getContainerFqdn(c *metadata.Container, s *metadata.Service) string {
	if s != nil && strings.EqualFold(s.Kind, "kubernetesService") {
		return strings.ToLower(fmt.Sprintf("%s.%s.%s.%s.", c.Name, s.Name, s.StackName, getServiceGlobalNamespace(s)))

	}
	return strings.ToLower(fmt.Sprintf("%s.%s.%s.%s.", c.Name, c.StackName, c.EnvironmentName, getDefaultRancherNamespace()))
}

func (c *ConfigGenerator) getServiceEndpoints(svc *metadata.Service, uuidToPrimaryIp map[string]string, svcUUIDToSvc map[string]metadata.Service) ([]*Record, error) {
	var records []*Record
	var err error
	if strings.EqualFold(svc.Kind, "externalService") {
		records = c.getExternalServiceEndpoints(svc)
	} else if strings.EqualFold(svc.Kind, "dnsService") {
		records, err = c.getAliasServiceEndpoints(svc, uuidToPrimaryIp, svcUUIDToSvc)
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
		if primaryIP != "" {
			rec := &Record{
				IP:        primaryIP,
				IsHealthy: isHealthy && isRunning,
				IsCname:   false,
				Container: &svc.Containers[i],
			}
			recs = append(recs, rec)
		}
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

func (c *ConfigGenerator) getAliasServiceEndpoints(svc *metadata.Service, uuidToPrimaryIp map[string]string, svcUUIDToSvc map[string]metadata.Service) ([]*Record, error) {
	var recs []*Record
	for _, svcUUID := range svc.Links {
		if svcUUID == "" {
			continue
		}
		if _, ok := svcUUIDToSvc[svcUUID]; !ok {
			continue
		}
		service := svcUUIDToSvc[svcUUID]
		newRecs, err := c.getServiceEndpoints(&service, uuidToPrimaryIp, svcUUIDToSvc)
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

func (mf rMetaFetcher) GetServices() ([]metadata.Service, error) {
	return mf.metadataClient.GetServices()
}

func (mf rMetaFetcher) GetContainers() ([]metadata.Container, error) {
	return mf.metadataClient.GetContainers()
}

func (mf rMetaFetcher) OnChange(intervalSeconds int, do func(string)) {
	mf.metadataClient.OnChange(intervalSeconds, do)
}
