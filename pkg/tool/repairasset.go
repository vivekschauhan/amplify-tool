package tool

import (
	"github.com/Axway/agent-sdk/pkg/apic"
	"github.com/Axway/agent-sdk/pkg/apic/auth"
	"github.com/Axway/agent-sdk/pkg/config"
	utillog "github.com/Axway/agent-sdk/pkg/util/log"
	"github.com/sirupsen/logrus"
	"github.com/vivekschauhan/amplify-tool/pkg/log"
	"github.com/vivekschauhan/amplify-tool/pkg/service"
)

type Tool interface {
	Run() error
	RunRepairProduct() error
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
	apicClient := createAPICClient(cfg)
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

func createAPICClient(cfg *Config) apic.Client {
	c := config.NewCentralConfig(config.GenericService)
	centralCfg, _ := c.(*config.CentralConfiguration)
	centralCfg.URL = cfg.URL
	centralCfg.PlatformURL = cfg.PlatformURL
	acfg := centralCfg.GetAuthConfig()
	authCfg, _ := acfg.(*config.AuthConfiguration)
	authCfg.ClientID = cfg.Auth.ClientID
	authCfg.PrivateKey = cfg.Auth.PrivateKey
	authCfg.PublicKey = cfg.Auth.PublicKey
	authCfg.KeyPwd = cfg.Auth.KeyPassword
	authCfg.URL = cfg.Auth.URL
	authCfg.Timeout = cfg.Auth.Timeout
	authCfg.Realm = "Broker"

	tokenGetter := auth.NewPlatformTokenGetterWithCentralConfig(centralCfg)
	return apic.New(centralCfg, tokenGetter, nil)
}

func (t *tool) Run() error {
	t.logger.Info("Amplify Asset Tool")
	err := t.Read()
	t.Write()
	if err != nil {
		t.logger.WithError(err).Error("stopping the repair")
		return err
	}
	t.productCatalog.PreProcessProductForAssetRepair()
	t.assetCatalog.RepairAsset()
	//t.productCatalog.PostProcessProductForAssetRepair()
	t.assetCatalog.PostRepairAsset()
	return nil
}

func (t *tool) Read() error {
	t.serviceRegistry.ReadServices()
	err := t.assetCatalog.ReadAssets(false)
	t.productCatalog.ReadProducts()
	if err != nil {
		return err
	}

	return nil
}

func (t *tool) Write() error {
	t.serviceRegistry.WriteServices()
	t.assetCatalog.WriteAssets()
	t.productCatalog.WriteProducts()
	return nil
}
