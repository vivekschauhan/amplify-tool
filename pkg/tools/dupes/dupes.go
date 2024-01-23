package dupes

import (
	"fmt"
	"os"
	"strings"

	"github.com/Axway/agent-sdk/pkg/apic"
	"github.com/Axway/agent-sdk/pkg/apic/definitions"
	"github.com/Axway/agent-sdk/pkg/util"
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
	assetCatalog    service.AssetCatalog
	// productCatalog  service.ProductCatalog
	output  []string
	outfile string
}

func NewTool(cfg *Config) Tool {
	logger := log.GetLogger(cfg.Level, cfg.Format)
	apicClient := tools.CreateAPICClient(&cfg.Config)
	utillog.GlobalLoggerConfig.Level(cfg.Level).
		Format(cfg.Format).
		Apply()
	serviceRegistry := service.NewServiceRegistry(logger, apicClient, cfg.DryRun, service.WithGetInstances())
	assetCatalog := service.NewAssetCatalog(logger, apicClient, cfg.DryRun, serviceRegistry)
	// productCatalog := service.NewProductCatalog(logger, assetCatalog, apicClient, "", cfg.DryRun)
	return &tool{
		logger:          logger,
		cfg:             cfg,
		apicClient:      apicClient,
		serviceRegistry: serviceRegistry,
		assetCatalog:    assetCatalog,
		// productCatalog:  productCatalog,
		output:  []string{},
		outfile: cfg.OutFile,
	}
}

func (t *tool) Run() error {
	t.logger.Info("Amplify Duplication Tool")
	err := t.Read()
	if err != nil {
		t.logger.WithError(err).Error("could not read resources: stopping the tool")
		return err
	}
	// err = t.Write()
	// if err != nil {
	// 	t.logger.WithError(err).Error("could not write resources: stopping the tool")
	// 	return err
	// }
	return t.findDupes()
}

func (t *tool) Read() error {
	t.logger.Debug("gathering resources from amplify")
	t.serviceRegistry.ReadServices()
	t.assetCatalog.ReadAssets(false)
	// t.productCatalog.ReadProducts()

	// cycle through all envs
	return nil
}

func (t *tool) findDupes() error {
	t.logger.Debug("starting to find possible duplicates")
	envs := t.serviceRegistry.GetEnvs()
	for _, env := range envs {
		logger := t.logger.WithField("env", env)
		grouping := t.groupServicesInEnv(env)
		logger.WithField("groups", grouping).Debug("finished grouping for env")

		// process each grouping
		for key, group := range grouping {
			logger = logger.WithField("groupKey", key)
			if len(group) <= 1 {
				continue
			}
			logger.WithField("copies", len(group)).Debug("found duplicates")
			t.handleGroup(logger, env, group)
		}
	}

	output := strings.Join(t.output, "\n")
	if t.outfile == "" {
		fmt.Print(output)
	} else {
		os.WriteFile(t.outfile, []byte(output), 0777)
	}
	return nil
}

func (t *tool) handleGroup(logger *logrus.Entry, env string, services []string) {
	itemToAssets := map[string]int{}
	totalAssets := 0
	if len(services) <= 1 {
		return
	}
	t.output = append(t.output, fmt.Sprintf("Found possible duplicate services: %s", strings.Join(services, ", ")))
	for _, service := range services {
		svcInfo := t.serviceRegistry.GetAPIServiceInfo(env, service)
		if svcInfo == nil {
			continue
		}
		temp := strings.HasPrefix(service, "wss-weather-standard-solution-api")
		_ = temp
		itemToAssets[service] = 0
		for _, inst := range svcInfo.APIServiceInstances {
			assets := t.assetCatalog.AssetsForInstance(inst.Group, env, inst.Name)
			itemToAssets[service] += len(assets)
			totalAssets += len(assets)
		}
		logger.WithField("svc", service).WithField("assetsPerSvc", itemToAssets[service]).Debug("done finding assets for service")
	}
	logger.WithField("asset", itemToAssets).WithField("numAssets", totalAssets).Info("counted assets")
	svcsWithAssets := 0
	svcWithAsset := ""
	for service, assets := range itemToAssets {
		svcOutput := fmt.Sprintf("%s: %v assets", service, assets)
		t.output = append(t.output, svcOutput)
		if assets > 0 {
			svcWithAsset = service
			svcsWithAssets++
		}
	}
	if svcsWithAssets == 0 {
		t.output = append(t.output, fmt.Sprintf("ACTION: For services (%s) combine all revisions to any and remove others", strings.Join(services, ", ")))
	} else if svcsWithAssets == 1 {
		t.output = append(t.output, fmt.Sprintf("ACTION: For services (%s) combine all revisions to %s and remove others", strings.Join(services, ", "), svcWithAsset))
	} else if svcsWithAssets == 2 {
		t.output = append(t.output, fmt.Sprintf("ACTION: For services (%s) more investigation needed as multiple services have assets", strings.Join(services, ", ")))
	}
}

func (t *tool) groupServicesInEnv(env string) map[string][]string {
	grouping := make(map[string][]string)
	logger := t.logger.WithField("env", env)
	servicesInfo := t.serviceRegistry.GetAPIServicesInfo(env)

	groupBy := ""

	for service, serviceInfo := range servicesInfo {
		logger = logger.WithField("svc", service)
		for _, inst := range serviceInfo.APIServiceInstances {
			logger = logger.WithField("instance", inst.Name)

			details := util.GetAgentDetailStrings(inst)
			if groupBy == "" {
				// use the first service to determine if we will group by api id or primary key
				if _, found := details[definitions.AttrExternalAPIID]; found {
					groupBy = definitions.AttrExternalAPIID
				} else if _, found := details[definitions.AttrExternalAPIPrimaryKey]; found {
					groupBy = definitions.AttrExternalAPIPrimaryKey
				} else {
					logger.Error("can't determine how to group services")
					break
				}
				logger = logger.WithField("groupBy", groupBy)
			}

			if key, found := details[groupBy]; found {
				if _, ok := grouping[key]; !ok {
					grouping[key] = []string{}
				}
				grouping[key] = append(grouping[key], service)
				break
			} else {
				logger.Warn("can't find grouping attribute on service")
			}
		}
		logger.Debug("finished grouping instances in service")
	}
	logger.Debug("finished grouping services in environment")
	return grouping
}

func (t *tool) Write() error {
	t.serviceRegistry.WriteServices()
	return nil
}
