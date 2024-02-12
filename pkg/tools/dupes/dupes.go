package dupes

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Axway/agent-sdk/pkg/apic"
	"github.com/Axway/agent-sdk/pkg/apic/definitions"
	"github.com/Axway/agent-sdk/pkg/util"
	utillog "github.com/Axway/agent-sdk/pkg/util/log"
	"github.com/sirupsen/logrus"
	"github.com/vivekschauhan/amplify-tool/pkg/log"
	"github.com/vivekschauhan/amplify-tool/pkg/service"
	"github.com/vivekschauhan/amplify-tool/pkg/tools"
)

const (
	sep  = "############################################################################################################################################################"
	sep2 = "#**********************************************************************************************************************************************************#"
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
	if len(cfg.Environments) > 0 {
		envs := strings.Split(cfg.Environments, ",")
		for i := range envs {
			envs[i] = strings.Trim(envs[i], " ")
		}
		serviceRegistry = service.NewServiceRegistry(logger, apicClient, cfg.DryRun, service.WithGetInstances(), service.WithEnvironments(envs))
	}
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
	sort.Strings(services)

	itemToAssets := map[string]int{}
	totalAssets := 0
	if len(services) <= 1 {
		return
	}

	// loop through all services in groups and count the number of assets
	serviceToKeep := services[0]
	serviceToKeepTime := time.Now()
	for _, service := range services {
		svcInfo := t.serviceRegistry.GetAPIServiceInfo(env, service)
		if svcInfo == nil {
			continue
		}

		// find oldest service and set that one to keep
		if t := time.Time(svcInfo.APIService.GetMetadata().Audit.CreateTimestamp); t.Before(serviceToKeepTime) {
			serviceToKeepTime = t
			serviceToKeep = service
		}

		itemToAssets[service] = 0
		for _, inst := range svcInfo.APIServiceInstances {
			assets := t.assetCatalog.AssetsForInstance(inst.Group, env, inst.Name)
			itemToAssets[service] += len(assets)
			totalAssets += len(assets)
		}
		logger.WithField("svc", service).WithField("assetsPerSvc", itemToAssets[service]).Debug("done finding assets for service")
	}
	logger.WithField("asset", itemToAssets).WithField("numAssets", totalAssets).Info("counted assets")

	servicesWithAssets := 0

	// check how many of the services are linked to assets
	for service, assets := range itemToAssets {
		if assets > 0 {
			serviceToKeep = service
			servicesWithAssets++
		}
	}

	t.output = append(t.output, sep)
	// when greater than 2 output that more care needs to be taken
	if servicesWithAssets == 2 {
		t.output = append(t.output, "#\tACTION: For the following services more investigation needed as multiple services have assets")
		for _, service := range services {
			t.output = append(t.output, fmt.Sprintf("#\t\t%v has %v assets", service, itemToAssets[service]))
		}
		t.output = append(t.output, sep)
		t.output = append(t.output, "")
		return
	}

	// 1 or fewer services with assets
	t.output = append(t.output, fmt.Sprintf("#\tACTION: For the following services combine all revisions to %s and remove others", serviceToKeep))

	logger = logger.WithField("serviceToKeep", serviceToKeep)
	logger.Info("starting to compare spec hashes")
	svcKeepInfo := t.serviceRegistry.GetAPIServiceInfo(env, serviceToKeep)
	svcKeepDetails := util.GetAgentDetails(svcKeepInfo.APIService)
	hashes := map[string]interface{}{}
	if v, found := svcKeepDetails["specHashes"]; found {
		hashes = v.(map[string]interface{})
	}

	// check hashes for revision referenced by instance to see if they need merged
	actionOutput := ""
	commandOutput := ""
	logger = logger.WithField("hashData", hashes)
	for _, service := range services {
		if service == serviceToKeep {
			continue
		}

		logger = logger.WithField("service", service)
		logger.Debug("comparing hash of revision on instance to hashes in service to keep")
		svcInfo := t.serviceRegistry.GetAPIServiceInfo(env, service)
		for _, inst := range svcInfo.APIServiceInstances {
			hash, err := util.GetAgentDetailsValue(inst, "tempHash")
			if err != nil {
				actionOutput += fmt.Sprintf("#\t\t%v no hash found, take care with removing\n", service)
				continue
			}
			logger = logger.WithField("hash", hash)

			logger.Debug("handling instance hash compare")
			if _, found := hashes[hash]; found {
				actionOutput += fmt.Sprintf("#\t\t%v can be deleted without any merge as hash exists on %v and it has %v related assets\n", service, serviceToKeep, itemToAssets[service])
				commandOutput += fmt.Sprintf("axway central delete -s %v apiservice %v\n", env, service)
			} else {
				actionOutput += fmt.Sprintf("#\t\t%v can be deleted after merging revision %v to %v\n", service, inst.Spec.ApiServiceRevision, serviceToKeep)
				commandOutput += fmt.Sprintf("axway central get -o json -s %v apiservicerevision %v > %v.json\n", env, inst.Spec.ApiServiceRevision, inst.Spec.ApiServiceRevision)
				commandOutput += fmt.Sprintf("jq '.spec.apiService |= \"%v\" %v.json > %v.json\n", service, inst.Spec.ApiServiceRevision, inst.Spec.ApiServiceRevision)
				commandOutput += fmt.Sprintf("axway central apply -f %v.json\n", inst.Spec.ApiServiceRevision)
				commandOutput += fmt.Sprintf("%v\n", sep2)
				commandOutput += fmt.Sprintf("#\tIn environment %v an update to the APIServiceInstance(s) related to %v may be necessary, in order to point to new revision %v\n", env, service, inst.Spec.ApiServiceRevision)
				commandOutput += fmt.Sprintf("%v\n", sep2)
			}
		}
	}
	// append actionOutput to log
	actionOutput = strings.TrimRight(actionOutput, "\n")
	t.output = append(t.output, actionOutput)
	t.output = append(t.output, sep2)

	// append commandOutput to log
	commandOutput = strings.TrimRight(commandOutput, "\n")
	t.output = append(t.output, "#\tExecute the following commands to clean these duplicated services")
	t.output = append(t.output, commandOutput)
	t.output = append(t.output, sep)
	t.output = append(t.output, "")
}

func (t *tool) groupServicesInEnv(env string) map[string][]string {
	grouping := make(map[string][]string)
	logger := t.logger.WithField("env", env)
	servicesInfo := t.serviceRegistry.GetAPIServicesInfo(env)

	groupBy := ""

	for service, serviceInfo := range servicesInfo {
		logger = logger.WithField("svc", service)
		svcDetails := util.GetAgentDetails(serviceInfo.APIService)
		hashes := map[string]interface{}{}
		if v, found := svcDetails["specHashes"]; found {
			hashes = v.(map[string]interface{})
		}
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

			// get revision hash from service and add to tempHash x-agent-detail on instance for duplication processing
			rev := inst.Spec.ApiServiceRevision
			logger.WithField("hashData", hashes).WithField("rev.Name", rev).Debug("looking for revision hash on service")
			for hash, revName := range hashes {
				if rev == revName.(string) {
					util.SetAgentDetailsKey(inst, "tempHash", hash)
					t.serviceRegistry.UpdateAPIServiceInst(env, service, inst)
					break
				}
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
