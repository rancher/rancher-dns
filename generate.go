package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher-metadata/metadata"
)

var (
	fallbackRecurse = []string{"8.8.8.8", "8.8.4.4"}
)

type MetadataFetcher interface {
	GetService(link string) (*metadata.Service, error)
	GetServices() ([]metadata.Service, error)
	GetContainers() ([]metadata.Container, error)
	OnChange(intervalSeconds int, do func(string))
	GetSelfHost() (metadata.Host, error)
	GetRegionName() (string, error)
	GetServiceFromRegionEnvironment(regionName string, envName string, stackName string, svcName string) (metadata.Service, error)
	GetServiceInLocalRegion(envName string, stackName string, svcName string) (metadata.Service, error)
	GetServiceInLocalEnvironment(stackName string, svcName string) (metadata.Service, error)
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

func (mf rMetaFetcher) GetRegionName() (string, error) {
	return mf.metadataClient.GetRegionName()
}

func (c *ConfigGenerator) Init(metadataServer *string) error {
	metadataClient, err := metadata.NewClientAndWait(fmt.Sprintf("http://%s/2016-07-29", *metadataServer))
	if err != nil {
		logrus.Errorf("Error initiating metadata client: %v", err)
		return err
	}

	c.metaFetcher = rMetaFetcher{
		metadataClient: metadataClient,
	}
	return nil
}

func (c *ConfigGenerator) SetLinksForRegions(key string, linkAlias string, cARecs map[string]RecordA, cCnameRecs map[string]RecordCname, aRegionRecs map[string]RecordA, cRegionRecs map[string]RecordCname) {
	_, inaRegionRecs := aRegionRecs[key]
	_, incRegionRecs := cRegionRecs[key]
	if !inaRegionRecs && !incRegionRecs {
		linkedService, err := c.metaFetcher.GetService(key)
		if err != nil {
			logrus.Infof("Couldn't find linked service %v ", err)
			return
		}
		uuidToPrimaryIp := make(map[string]string)
		for _, c := range linkedService.Containers {
			if c.PrimaryIp == "" {
				continue
			}
			uuidToPrimaryIp[c.UUID] = c.PrimaryIp
		}
		records, err := c.getServiceEndpoints(linkedService, uuidToPrimaryIp)
		if err != nil {
			logrus.Warn(err)
			return
		}
		for _, record := range records {
			if record.IsCname {
				cnameRec := RecordCname{
					Answer: fmt.Sprintf("%s.", record.IP),
				}
				cRegionRecs[key] = cnameRec
				continue
			}
			aRec := RecordA{
				Answer: []string{record.IP},
			}
			if existing, ok := aRegionRecs[key]; ok {
				aRec.Answer = append(aRec.Answer, existing.Answer...)
			}
			aRegionRecs[key] = aRec
		}
	}
	if _, ok := aRegionRecs[key]; ok {
		cARecs[fmt.Sprintf("%s.", linkAlias)] = aRegionRecs[key]
		cARecs[fmt.Sprintf("%s.%s.", linkAlias, getDefaultRancherNamespace())] = aRegionRecs[key]
	} else if _, ok := cRegionRecs[key]; ok {
		cCnameRecs[fmt.Sprintf("%s.", linkAlias)] = cRegionRecs[key]
		cCnameRecs[fmt.Sprintf("%s.%s.", linkAlias, getDefaultRancherNamespace())] = cRegionRecs[key]
	}
}

func (c *ConfigGenerator) GenerateAnswers() (Answers, error) {
	answers := make(Answers)
	aRecs, cRecs, clientIpsToServiceLinks, clientIpsToContainerLinks, clientIpToContainer, svcNameToSvc, err := c.GetRecords()
	if err != nil {
		return nil, err
	}
	aRegionRecs := make(map[string]RecordA)
	cRegionRecs := make(map[string]RecordCname)

	//generate client record
	for clientIp, container := range clientIpToContainer {
		cARecs := make(map[string]RecordA)
		cCnameRecs := make(map[string]RecordCname)

		// 1. set container links
		for linkName, targetIp := range clientIpsToContainerLinks[clientIp] {
			aRec := RecordA{
				Answer: []string{targetIp},
			}
			cARecs[getLinkGlobalFqdn(linkName, nil)] = aRec
		}

		// 2. set service links
		// note that service link overrides the container link (if the names collide)
		for key, linkAlias := range clientIpsToServiceLinks[clientIp] {
			splitSvcName := strings.Split(key, "/")
			if len(splitSvcName) > 2 {
				c.SetLinksForRegions(key, linkAlias, cARecs, cCnameRecs, aRegionRecs, cRegionRecs)
			} else {
				if strings.EqualFold(key, linkAlias) {
					// skip non-aliased service links
					// they are present in defaults
					continue
				}
				linkedService := svcNameToSvc[key]
				linkServiceFqdn := getServiceFqdn(&linkedService)
				globalAliasName := getLinkGlobalFqdn(linkAlias, &linkedService)
				stackAliasName := getLinkStackFqdn(linkAlias, &linkedService)
				if _, ok := aRecs[linkServiceFqdn]; ok {
					//we store 2 A records for link:
					// a) linkName.namespace
					// b) linkName.stackName.namespace
					cARecs[stackAliasName] = aRecs[linkServiceFqdn]
					cARecs[globalAliasName] = aRecs[linkServiceFqdn]
				} else if _, ok := cRecs[linkServiceFqdn]; ok {
					cCnameRecs[stackAliasName] = cRecs[linkServiceFqdn]
					cCnameRecs[globalAliasName] = cRecs[linkServiceFqdn]
				}
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
	clientIpsToServiceLinks := make(map[string]map[string]string)
	clientIpToContainer := make(map[string]metadata.Container)
	svcNameToSvc := make(map[string]metadata.Service)
	clientIpsToContainerLinks := make(map[string]map[string]string)

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

	selfEnvironmentUUID := host.EnvironmentUUID

	uuidToPrimaryIp := make(map[string]string)
	for _, c := range containers {
		if c.PrimaryIp == "" {
			continue
		}
		uuidToPrimaryIp[c.UUID] = c.PrimaryIp
	}

	// get service records
	for _, svc := range services {
		// only fetch services from the same environment
		if svc.EnvironmentUUID != selfEnvironmentUUID {
			continue
		}
		svcNameToSvc[fmt.Sprintf("%s/%s", svc.StackName, svc.Name)] = svc
		records, err := c.getServiceEndpoints(&svc, uuidToPrimaryIp)
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

		if c.EnvironmentUUID != selfEnvironmentUUID {
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
		if c.ServiceName != "" && c.StackName != "" {
			svc = svcNameToSvc[fmt.Sprintf("%s/%s", c.StackName, c.ServiceName)]
		}
		aRecs[getContainerFqdn(&c, &svc)] = aRec

		//client section only for the containers running on the same host
		if c.HostUUID == host.UUID && c.PrimaryIp != "" {
			if svc.Name != "" {
				clientIpsToServiceLinks[c.PrimaryIp] = svc.Links
			}
			clientIpToContainer[c.PrimaryIp] = c

			//get container links
			containerLinks := make(map[string]string)
			for linkName, linkedContainerUUID := range c.Links {
				targetIP := containerUUIDToContainerIP[linkedContainerUUID]
				if targetIP == "" {
					continue
				}
				containerLinks[linkName] = targetIP
			}
			clientIpsToContainerLinks[c.PrimaryIp] = containerLinks
		}
	}

	// add metadata record
	aRec := RecordA{
		Answer: splitTrim(*metadataAnswer, ","),
	}
	//add to the service record
	aRecs[fmt.Sprintf("rancher-metadata.%s.", getDefaultRancherNamespace())] = aRec

	return aRecs, cRecs, clientIpsToServiceLinks, clientIpsToContainerLinks, clientIpToContainer, svcNameToSvc, nil
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
	if s != nil {
		return strings.ToLower(fmt.Sprintf("%s.%s.", linkName, getServiceGlobalNamespace(s)))
	}
	return strings.ToLower(fmt.Sprintf("%s.%s.", linkName, getDefaultRancherNamespace()))
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
		service, err := c.metaFetcher.GetService(link)
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

func (mf rMetaFetcher) GetService(link string) (*metadata.Service, error) {
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

func (mf rMetaFetcher) GetServiceFromRegionEnvironment(regionName string, envName string, stackName string, svcName string) (metadata.Service, error) {
	return mf.metadataClient.GetServiceFromRegionEnvironment(regionName, envName, stackName, svcName)
}

func (mf rMetaFetcher) GetServiceInLocalRegion(envName string, stackName string, svcName string) (metadata.Service, error) {
	return mf.metadataClient.GetServiceInLocalRegion(envName, stackName, svcName)
}

func (mf rMetaFetcher) GetServiceInLocalEnvironment(stackName string, svcName string) (metadata.Service, error) {
	return mf.metadataClient.GetServiceInLocalEnvironment(stackName, svcName)
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
