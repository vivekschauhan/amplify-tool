package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Axway/agent-sdk/pkg/apic"
	v1 "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/api/v1"
	management "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/management/v1alpha1"
	"github.com/sirupsen/logrus"
)

type serviceRegistryOpt func(s *serviceRegistry)

type ServiceRegistry interface {
	ReadServices()
	WriteServices()
	GetServicesOutput() []v1.Interface
	GetEnvs() []string
	GetAPIServicesInfo(env string) map[string]APIServiceInfo
	GetAPIService(env, name string) *management.APIService
	GetAPIServiceInfo(env, name string) *APIServiceInfo
	FindService(logger *logrus.Entry, scope, name string) *management.APIService
	IsUsingMapping() bool
}

type serviceRegistry struct {
	logger          *logrus.Logger
	apicClient      apic.Client
	apiSvcLock      *sync.Mutex
	APIServices     map[string]map[string]APIServiceInfo
	ServiceMapping  map[string][]string
	envs            []string
	mappingFile     string
	outputFile      string
	envName         string
	getAllRevisions bool
	getInstances    bool
	stripData       bool
	dryRun          bool
}

func NewServiceRegistry(logger *logrus.Logger, apicClient apic.Client, dryRun bool, opts ...serviceRegistryOpt) ServiceRegistry {
	s := &serviceRegistry{
		logger:         logger,
		apicClient:     apicClient,
		apiSvcLock:     &sync.Mutex{},
		APIServices:    make(map[string]map[string]APIServiceInfo),
		ServiceMapping: make(map[string][]string),
		dryRun:         dryRun,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

func WithMappingFile(mappingFile string) serviceRegistryOpt {
	return func(s *serviceRegistry) {
		s.mappingFile = mappingFile
	}
}

func WithEnvironment(envName string) serviceRegistryOpt {
	return func(s *serviceRegistry) {
		s.envName = envName
	}
}

func WithOutputFile(outputFile string) serviceRegistryOpt {
	return func(s *serviceRegistry) {
		s.outputFile = outputFile
	}
}

func WithGetAllRevisions() serviceRegistryOpt {
	return func(s *serviceRegistry) {
		s.getAllRevisions = true
	}
}

func WithIncludeData(includeData bool) serviceRegistryOpt {
	return func(s *serviceRegistry) {
		s.stripData = !includeData
	}
}

func WithGetInstances() serviceRegistryOpt {
	return func(s *serviceRegistry) {
		s.getInstances = true
	}
}

func SaveToFile(logger *logrus.Logger, objType, fileName string, obj interface{}) {
	buf, err := json.Marshal(obj)
	if err != nil {
		logger.WithError(err).Errorf("unable to serialize %s to file %s", objType, fileName)
		return
	}
	os.WriteFile(fileName, buf, 0777)
}

func (t *serviceRegistry) WriteServices() {
	if t.outputFile == "" {
		t.outputFile = "service-registry.json"
	}

	SaveToFile(t.logger, "service-registry", t.outputFile, t.GetServicesOutput())
}

func (t *serviceRegistry) GetServicesOutput() []v1.Interface {
	objs := []v1.Interface{}
	for env := range t.APIServices {
		for _, svcInfo := range t.APIServices[env] {
			objs = append(objs, svcInfo.APIService)
			for _, rev := range svcInfo.APIServiceRevisions {
				objs = append(objs, rev)
			}
			for _, inst := range svcInfo.APIServiceInstances {
				objs = append(objs, inst)
			}
		}
	}

	// clean data
	if t.stripData {
		cleanObjs := []v1.Interface{}
		wg := sync.WaitGroup{}
		wg.Add(len(objs))
		for _, obj := range objs {
			func(i v1.Interface) {
				defer wg.Done()
				ri, _ := i.AsInstance()

				// clean sub resources by kind
				switch ri.Kind {
				case management.APIServiceGVK().Kind:
					delete(ri.SubResources, "details")
					delete(ri.SubResources, "compliance")
					delete(ri.SubResources, "status")
				case management.APIServiceInstanceGVK().Kind:
					apisi := management.NewAPIServiceInstance("", "")
					apisi.FromInstance(ri)
					apisi.Spec.AccessRequestDefinition = ""
					apisi.Spec.CredentialRequestDefinitions = []string{}
					ri, _ = apisi.AsInstance()
					delete(ri.SubResources, "references")
				case management.APIServiceRevisionGVK().Kind:
					delete(ri.SubResources, "compliance")
				}

				ri.Metadata.Audit = v1.AuditMetadata{}
				ri.Metadata.References = []v1.Reference{}
				ri.Metadata.ID = ""
				ri.Metadata.ResourceVersion = ""
				ri.Metadata.Scope.ID = ""
				ri.Metadata.Scope.SelfLink = ""
				ri.Metadata.SelfLink = ""
				ri.Owner = nil
				ri.Tags = []string{}
				ri.Attributes = map[string]string{}
				ri.Finalizers = make([]v1.Finalizer, 0)
				cleanObjs = append(cleanObjs, ri)
			}(obj)
		}
		wg.Wait()
		objs = cleanObjs
	}
	return objs
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

func (t *serviceRegistry) GetEnvs() []string {
	return t.envs
}

func (t *serviceRegistry) ReadServices() {
	e := management.NewEnvironment(t.envName)
	eInst, _ := e.AsInstance()
	envs := []*v1.ResourceInstance{eInst}

	if t.envName == "" {
		e := management.NewEnvironment("")
		var err error
		envs, err = t.apicClient.GetAPIV1ResourceInstances(nil, e.GetKindLink())
		if err != nil {
			t.logger.WithError(err).Error("unable to read environments")
			return
		}
	}

	t.loadServiceMapping()
	t.logger.Info("Reading API Service...")
	wg := sync.WaitGroup{}
	for _, env := range envs {
		wg.Add(1)
		envName := env.GetName()
		t.envs = append(t.envs, envName)
		go func(envName string) {
			defer wg.Done()
			logger := t.logger.WithField("env", envName)
			t.readAPIServices(logger, envName)
		}(envName)
	}
	wg.Wait()
}

func (t *serviceRegistry) readAPIServices(logger *logrus.Entry, envName string) {
	s := management.NewAPIService("", envName)
	services, err := t.apicClient.GetAPIV1ResourceInstances(nil, s.GetKindLink())
	if err != nil {
		t.logger.WithError(err).Error("unable to read assets")
	}

	limiter := make(chan *v1.ResourceInstance, 25)

	wg := sync.WaitGroup{}
	wg.Add(len(services))
	for _, service := range services {
		go func() {
			defer wg.Done()
			t.getSvcDetails(logger, <-limiter, envName)
		}()
		limiter <- service
	}

	wg.Wait()
	close(limiter)
}

func (t *serviceRegistry) getSvcDetails(logger *logrus.Entry, service *v1.ResourceInstance, envName string) {
	logger = logger.WithField("apiService", service.GetName())
	svc := management.NewAPIService("", "")
	svc.FromInstance(service)
	logger.Info("Reading APIService ok")
	if t.stripData {
		svc.Spec.Icon = management.ApiServiceSpecIcon{}
	}
	serviceInfo := APIServiceInfo{
		APIService: svc,
	}
	if _, found := t.APIServices[svc.Metadata.Scope.Name]; !found {
		t.apiSvcLock.Lock()
		t.APIServices[svc.Metadata.Scope.Name] = map[string]APIServiceInfo{}
		t.apiSvcLock.Unlock()
	}
	if t.getAllRevisions {
		serviceInfo.APIServiceRevisions = make(map[string]*management.APIServiceRevision)
		t.readAPIServiceRevisions(logger, svc.Metadata.ID, envName, &serviceInfo)
	}
	if t.getInstances {
		serviceInfo.APIServiceInstances = make(map[string]*management.APIServiceInstance)
		t.readAPIServiceInstances(logger, svc.Metadata.ID, envName, &serviceInfo)
	}
	t.apiSvcLock.Lock()
	t.APIServices[svc.Metadata.Scope.Name][svc.Name] = serviceInfo
	t.apiSvcLock.Unlock()
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

func (t *serviceRegistry) GetAPIServicesInfo(env string) map[string]APIServiceInfo {
	if apiServicesInfo, ok := t.APIServices[env]; ok {
		return apiServicesInfo
	}
	return nil
}

func (t *serviceRegistry) GetAPIServiceInfo(env, name string) *APIServiceInfo {
	apiServicesInfo := t.GetAPIServicesInfo(env)
	if apiServicesInfo == nil {
		return nil
	}
	if apiServiceInfo, ok := apiServicesInfo[name]; ok {
		return &apiServiceInfo
	}
	return nil
}

func (t *serviceRegistry) GetAPIService(env, name string) *management.APIService {
	apiServicesInfo := t.GetAPIServicesInfo(env)
	if apiServicesInfo == nil {
		return nil
	}
	if apiServiceInfo, ok := apiServicesInfo[name]; ok {
		return apiServiceInfo.APIService
	}
	return nil
}

func (t *serviceRegistry) readAPIServiceRevisions(logger *logrus.Entry, refID, envName string, serviceInfo *APIServiceInfo) {
	r := management.NewAPIServiceRevision("", envName)
	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", refID),
	}
	revisions, err := t.apicClient.GetAPIV1ResourceInstances(params, r.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to get APIServiceRevisions")
	}
	for _, revision := range revisions {
		rev := management.NewAPIServiceRevision("", envName)
		rev.FromInstance(revision)
		if t.stripData {
			rev.Spec.Definition.Type = "unstructured"
			rev.Spec.Definition.Value = ""
		}
		logger = logger.WithField("apiServiceRevision", rev.Name)
		logger.Debug("Reading APIServiceRevision ok")
		serviceInfo.APIServiceRevisions[revision.Metadata.ID] = rev
	}
}

func (t *serviceRegistry) readAPIServiceInstances(logger *logrus.Entry, refID, envName string, serviceInfo *APIServiceInfo) {
	serviceInfo.APIServiceInstances = make(map[string]*management.APIServiceInstance)
	i := management.NewAPIServiceInstance("", envName)
	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", refID),
	}
	instances, err := t.apicClient.GetAPIV1ResourceInstances(params, i.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to get APIServiceInstance")
	}
	for _, instance := range instances {
		inst := management.NewAPIServiceInstance("", envName)
		inst.FromInstance(instance)
		serviceInfo.APIServiceInstances[inst.Metadata.ID] = inst
		logger.WithField("apiServiceInstance", inst.Name).Debug("Reading APIServiceInstance ok")
		if !t.getAllRevisions {
			if serviceInfo.APIServiceRevisions == nil {
				serviceInfo.APIServiceRevisions = make(map[string]*management.APIServiceRevision)
			}
			t.readAPIServiceRevisionByName(logger, inst.Spec.ApiServiceRevision, envName, serviceInfo)
		}
	}
}

func (t *serviceRegistry) readAPIServiceRevisionByName(logger *logrus.Entry, revName, envName string, serviceInfo *APIServiceInfo) {
	r := management.NewAPIServiceRevision(revName, envName)
	revInst, err := t.apicClient.GetResource(r.GetSelfLink())
	if err != nil {
		logger.WithError(err).WithField("revName", revName).Error("unable to get APIServiceRevision")
		return
	}
	r.FromInstance(revInst)
	if t.stripData {
		r.Spec.Definition.Type = "unstructured"
		r.Spec.Definition.Value = ""
	}
	serviceInfo.APIServiceRevisions[r.Metadata.ID] = r
}
