package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Axway/agent-sdk/pkg/apic"
	management "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/management/v1alpha1"
	"github.com/sirupsen/logrus"
)

type ServiceRegistry interface {
	ReadServices()
	WriteServices()
	GetAPIService(env, name string) *management.APIService
	FindService(logger *logrus.Entry, scope, name string) *management.APIService
	IsUsingMapping() bool
}

type serviceRegistry struct {
	logger         *logrus.Logger
	apicClient     apic.Client
	APIServices    map[string]APIServiceInfo
	ServiceMapping map[string][]string
	mappingFile    string
	dryRun         bool
}

func NewServiceRegistry(logger *logrus.Logger, apicClient apic.Client, mappingFile string, dryRun bool) ServiceRegistry {
	return &serviceRegistry{
		logger:         logger,
		apicClient:     apicClient,
		APIServices:    make(map[string]APIServiceInfo),
		ServiceMapping: make(map[string][]string),
		mappingFile:    mappingFile,
		dryRun:         dryRun,
	}
}

func saveToFile(logger *logrus.Logger, objType string, obj interface{}) {
	fileName := fmt.Sprintf("%s.json", objType)
	buf, err := json.Marshal(obj)
	if err != nil {
		logger.WithError(err).Errorf("unable to serialize %s to file %s", objType, fileName)
		return
	}
	os.WriteFile(fileName, buf, 0777)
}

func (t *serviceRegistry) WriteServices() {
	saveToFile(t.logger, "service-registry", t.APIServices)
}

func readFromFile(logger *logrus.Logger, fileName string, obj interface{}) {
	if fileName == "" {
		return
	}

	buf, err := os.ReadFile(fileName)
	if err != nil {
		logger.WithError(err).Errorf("unable to read mapping file %s", fileName)
		return
	}
	err = json.Unmarshal(buf, obj)
	if err != nil {
		logger.WithError(err).Errorf("unable to read mapping file %s", fileName)
	}
}

func (t *serviceRegistry) loadServiceMapping() {
	readFromFile(t.logger, t.mappingFile, &t.ServiceMapping)
}

func (t *serviceRegistry) ReadServices() {
	e := management.NewEnvironment("")
	t.loadServiceMapping()
	// envs, _ := t.apicClient.GetResources(e)
	envs, err := t.apicClient.GetAPIV1ResourceInstances(nil, e.GetKindLink())
	if err != nil {
		t.logger.WithError(err).Error("unable to read environments")
	}
	t.logger.Info("Reading API Service...")
	for _, env := range envs {
		envName := env.GetName()
		logger := t.logger.WithField("env", envName)
		t.readAPIServices(logger, envName)
	}
}

func (t *serviceRegistry) readAPIServices(logger *logrus.Entry, envName string) {
	s := management.NewAPIService("", envName)
	services, err := t.apicClient.GetAPIV1ResourceInstances(nil, s.GetKindLink())
	if err != nil {
		t.logger.WithError(err).Error("unable to read assets")
	}
	for _, service := range services {
		logger = logger.WithField("apiService", service.GetName())
		svc := management.NewAPIService("", "")
		svc.FromInstance(service)
		logger.Info("Reading APIService ok")

		serviceInfo := APIServiceInfo{
			APIService: svc,
		}
		t.APIServices[serviceKey(svc.Metadata.Scope.Name, svc.Name)] = serviceInfo
	}
}

func serviceKey(env, name string) string {
	return fmt.Sprintf("%s/%s", env, name)
}

func (t *serviceRegistry) findMappedService(logger *logrus.Entry, scope, name string) *management.APIService {
	key := serviceKey(scope, name)
	mappedSvc, ok := t.ServiceMapping[key]
	if ok && len(mappedSvc) != 0 {
		elem := strings.Split(mappedSvc[0], "/")
		if len(elem) == 2 {
			svc := t.GetAPIService(elem[0], elem[1])
			if svc != nil {
				logger.
					WithField("mappedServiceScope", svc.Metadata.Scope.Name).
					WithField("mappedServiceName", svc.Name).
					Info("Found service mapping")
				return svc
			}
		}
	}
	return nil
}

func (t *serviceRegistry) IsUsingMapping() bool {
	return t.mappingFile != ""
}

func (t *serviceRegistry) FindService(logger *logrus.Entry, scope, name string) *management.APIService {
	mappedSvc := t.findMappedService(logger, scope, name)
	if mappedSvc != nil {
		return mappedSvc
	}

	return t.GetAPIService(scope, name)
}

func (t *serviceRegistry) GetAPIService(env, name string) *management.APIService {
	apiServiceInfo, ok := t.APIServices[serviceKey(env, name)]
	if ok {
		return apiServiceInfo.APIService
	}
	return nil
}

func (t *serviceRegistry) readAPIServiceRevisions(logger *logrus.Entry, svcID, envName string) map[string]APIServiceRevisionInfo {
	revisionInfos := make(map[string]APIServiceRevisionInfo)

	r := management.NewAPIServiceRevision("", envName)
	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", svcID),
	}
	revisions, err := t.apicClient.GetAPIV1ResourceInstances(params, r.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to get APIServiceRevisions")
		return revisionInfos
	}
	for _, revision := range revisions {
		rev := management.NewAPIServiceRevision("", envName)
		rev.FromInstance(revision)
		logger = logger.WithField("apiServiceRevision", rev.Name)
		logger.Debug("Reading APIServiceRevision ok")
		instances := t.readAPIServiceInstances(logger, rev.Metadata.ID, envName)
		revisionInfo := APIServiceRevisionInfo{
			APIServiceRevision:  rev,
			APIServiceInstances: instances,
		}
		revisionInfos[revision.Metadata.ID] = revisionInfo
	}

	return revisionInfos
}

func (t *serviceRegistry) readAPIServiceInstances(logger *logrus.Entry, revisionID, envName string) map[string]*management.APIServiceInstance {
	revisionInfos := make(map[string]*management.APIServiceInstance)
	i := management.NewAPIServiceInstance("", envName)
	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", revisionID),
	}
	instances, err := t.apicClient.GetAPIV1ResourceInstances(params, i.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to get APIServiceInstance")
	}
	for _, instance := range instances {
		inst := management.NewAPIServiceInstance("", envName)
		inst.FromInstance(instance)
		revisionInfos[inst.Metadata.ID] = inst
		logger.WithField("apiServiceInstance", inst.Name).Debug("Reading APIServiceInstance ok")
	}

	return revisionInfos
}
