package product

import (
	"github.com/Axway/agent-sdk/pkg/apic"
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
	productCatalog  service.ProductCatalog
	assetCatalog    service.AssetCatalog
	serviceRegistry service.ServiceRegistry
}

func NewTool(cfg *Config) Tool {
	logger := log.GetLogger(cfg.Level, cfg.Format)
	apicClient := tools.CreateAPICClient(&cfg.Config)
	utillog.GlobalLoggerConfig.Level(cfg.Level).
		Format(cfg.Format).
		Apply()
	serviceRegistry := service.NewServiceRegistry(logger, apicClient, cfg.ServiceMappingFile, cfg.DryRun)
	assetCatalog := service.NewAssetCatalog(logger, serviceRegistry, apicClient, cfg.DryRun)
	productCatalog := service.NewProductCatalog(logger, assetCatalog, apicClient, cfg.ProductCatalogFile, cfg.DryRun)
	return &tool{
		logger:          logger,
		cfg:             cfg,
		apicClient:      apicClient,
		serviceRegistry: serviceRegistry,
		assetCatalog:    assetCatalog,
		productCatalog:  productCatalog,
	}
}

func (t *tool) Run() error {
	t.logger.Info("Amplify Product Tool")
	t.assetCatalog.ReadAssets(true)
	t.productCatalog.ReadProducts()
	t.productCatalog.RepairProductWithBackup()
	return nil
}
