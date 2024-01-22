package export

import (
	"github.com/Axway/agent-sdk/pkg/apic"
	v1 "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/api/v1"
	utillog "github.com/Axway/agent-sdk/pkg/util/log"
	"github.com/sirupsen/logrus"
	"github.com/vivekschauhan/amplify-tool/pkg/log"
	"github.com/vivekschauhan/amplify-tool/pkg/service"
	"github.com/vivekschauhan/amplify-tool/pkg/tools"
)

type Tool interface {
	Run() error
}

type tool struct {
	apicClient      apic.Client
	cfg             *Config
	logger          *logrus.Logger
	assetCatalog    service.AssetCatalog
	serviceRegistry service.ServiceRegistry
}

func NewTool(cfg *Config) Tool {
	logger := log.GetLogger(cfg.Level, cfg.Format)
	apicClient := tools.CreateAPICClient(&cfg.Config)
	utillog.GlobalLoggerConfig.Level(cfg.Level).
		Format(cfg.Format).
		Apply()
	serviceRegistry := service.NewServiceRegistry(logger, apicClient, cfg.DryRun,
		service.WithGetInstances(),
		service.WithEnvironment(cfg.Environment),
		service.WithOutputFile(cfg.OutFile),
		service.WithIncludeData(false),
	)
	assetCatalog := service.NewAssetCatalog(logger, apicClient, cfg.DryRun, serviceRegistry, service.WithFilterUsingRegistry(), service.ForExport(), service.StripData())
	return &tool{
		logger:          logger,
		cfg:             cfg,
		apicClient:      apicClient,
		serviceRegistry: serviceRegistry,
		assetCatalog:    assetCatalog,
	}
}

func (t *tool) Run() error {
	t.logger.Info("Amplify Export Tool")
	resources := []v1.Interface{}
	t.serviceRegistry.ReadServices()
	svcRes := t.serviceRegistry.GetServicesOutput()
	resources = append(resources, svcRes...)
	t.assetCatalog.ReadAssets(false)
	catRes := t.assetCatalog.GetAssetOutput()
	resources = append(resources, catRes...)

	service.SaveToFile(t.logger, "export", "export.json", resources)
	return nil
}
