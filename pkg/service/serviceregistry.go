package service

import (
	"fmt"

	"github.com/Axway/agent-sdk/pkg/apic"
	management "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/management/v1alpha1"
	"github.com/sirupsen/logrus"
)

type ServiceRegistry interface {
	ReadServices()
}

type serviceRegistry struct {
	logger      *logrus.Logger
	apicClient  apic.Client
	APIServices map[string]APIServiceInfo
}

func NewServiceRegistry(logger *logrus.Logger, apicClient apic.Client) ServiceRegistry {
	return &serviceRegistry{
		logger:      logger,
		apicClient:  apicClient,
		APIServices: make(map[string]APIServiceInfo),
	}
}

func (t *serviceRegistry) ReadServices() {
	e := management.NewEnvironment("")
	envs, _ := t.apicClient.GetResources(e)
	t.logger.Info("Reading API Service...")
	for _, env := range envs {
		envName := env.GetName()
		logger := t.logger.WithField("env", envName)
		t.readAPIServices(logger, envName)
	}
}

func (t *serviceRegistry) readAPIServices(logger *logrus.Entry, envName string) {
	s := management.NewAPIService("", envName)
	services, err := t.apicClient.GetResources(s)
	if err != nil {
		t.logger.Error(err)
	}
	for _, service := range services {
		logger = logger.WithField("apiService", service.GetName())
		svc := management.NewAPIService("", "")

		ri, _ := service.AsInstance()
		svc.FromInstance(ri)
		logger.Info("Reading APIService ok")
		revisions := t.readAPIServiceRevisions(logger, svc.GetMetadata().ID, envName)

		serviceInfo := APIServiceInfo{
			APIService:          svc,
			APIServiceRevisions: revisions,
		}
		t.APIServices[svc.GetMetadata().ID] = serviceInfo
	}
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
