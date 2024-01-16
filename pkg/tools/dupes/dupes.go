package dupes

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
	serviceRegistry service.ServiceRegistry
}

func NewTool(cfg *Config) Tool {
	logger := log.GetLogger(cfg.Level, cfg.Format)
	apicClient := tools.CreateAPICClient(&cfg.Config)
	utillog.GlobalLoggerConfig.Level(cfg.Level).
		Format(cfg.Format).
		Apply()
	serviceRegistry := service.NewServiceRegistry(logger, apicClient, "", cfg.DryRun)
	return &tool{
		logger:          logger,
		cfg:             cfg,
		apicClient:      apicClient,
		serviceRegistry: serviceRegistry,
	}
}

func (t *tool) Run() error {
	t.logger.Info("Amplify Duplication Tool")
	err := t.Read()
	t.Write()
	if err != nil {
		t.logger.WithError(err).Error("stopping the tool")
		return err
	}
	return nil
}

func (t *tool) Read() error {
	t.serviceRegistry.ReadServices()

	return nil
}

func (t *tool) Write() error {
	t.serviceRegistry.WriteServices()
	return nil
}
