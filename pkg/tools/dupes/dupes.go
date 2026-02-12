package dupes

import (
	"encoding/json"
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
	subResources    []string
	outFile         string
	backup          []string
	backupFile      string
	noIssues        []string
	noAssetsSI      []string
	noMerge         []string
	noSpecHash      []string
	multipleAssets  []string
	mergeRevisions  []string
}

func NewTool(cfg *Config) Tool {
	logger := log.GetLogger(cfg.Level, cfg.Format)
	apicClient, _ := tools.CreateAPICClient(&cfg.Config)
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
	subResources := []string{}
	if len(cfg.SubResources) > 0 {
		subs := strings.Split(cfg.SubResources, ",")
		for i := range subs {
			if str := strings.Trim(subs[i], " "); str != "" {
				subResources = append(subResources, str)
			}
		}
	}
	assetCatalog := service.NewAssetCatalog(logger, apicClient, cfg.DryRun, serviceRegistry)
	return &tool{
		logger:          logger,
		cfg:             cfg,
		apicClient:      apicClient,
		serviceRegistry: serviceRegistry,
		assetCatalog:    assetCatalog,
		subResources:    subResources,
		outFile:         cfg.OutFile,
		backup:          []string{},
		backupFile:      cfg.BackupFile,
		noIssues:        []string{},
		noAssetsSI:      []string{},
		noMerge:         []string{},
		noSpecHash:      []string{},
		multipleAssets:  []string{},
		mergeRevisions:  []string{},
	}
}

func (t *tool) Run() error {
	t.logger.Info("Amplify Duplication Tool")
	err := t.Read()
	if err != nil {
		t.logger.WithError(err).Error("could not read resources: stopping the tool")
		return err
	}

	return t.findDupes()
}

func (t *tool) Read() error {
	t.logger.Debug("gathering resources from amplify")
	t.serviceRegistry.ReadServices()
	t.assetCatalog.ReadAssets(false)

	return nil
}

func (t *tool) findDupes() error {
	t.logger.Debug("starting to find possible duplicates")
	envs := t.serviceRegistry.GetEnvs()
	for _, env := range envs {
		logger := t.logger.WithField("env", env)
		groupings := t.groupServicesInEnv(env)
		logger.WithField("groups", groupings).Debug("finished grouping for env")

		// process each grouping
		for key, group := range groupings {
			logger = logger.WithField("groupKey", key)
			logger.WithField("copies", len(group)).Debug("found duplicates")
			t.handleGroup(logger, env, group)
		}
	}

	outputLines := []string{}
	// add all services without assets to the end of the output for visibility that they were checked and have no assets
	if len(t.noAssetsSI) > 0 {
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "#\tThe following actions can be taken as no api service instances or assets found for the service")
		outputLines = append(outputLines, sep2)
		for _, action := range t.noAssetsSI {
			outputLines = append(outputLines, action)
		}
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "")
	}

	// add all services that do not need merged as their surviving service has the spec hash they refer to
	if len(t.noMerge) > 0 {
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "#\tThe following actions can be taken without merging their api service revisions as the hash")
		outputLines = append(outputLines, "#\t\texists on the service that was duplicated and as no assets linked to them")
		outputLines = append(outputLines, sep2)
		for _, action := range t.noMerge {
			outputLines = append(outputLines, action)
		}
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "")
	}

	// add all services that have multiple assets linked to them and need more investigation to the end of the output for visibility that they were checked and need more investigation
	if len(t.multipleAssets) > 0 {
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "#\tThe following services require more investigation as they have multiple assets linked to them")
		outputLines = append(outputLines, sep2)
		for _, action := range t.multipleAssets {
			outputLines = append(outputLines, action)
			outputLines = append(outputLines, sep2)
		}
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "")
	}

	// add all services that a spec hash could not be found in the x-agent-details of the service
	if len(t.noSpecHash) > 0 {
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "#\tThe following services were checked and require more investigation. No spec hash found for the revision referenced in the instance")
		outputLines = append(outputLines, "#\t\tThese will need to be investigated and manually merged if they are to be deleted")
		outputLines = append(outputLines, sep2)
		for _, svc := range t.noSpecHash {
			outputLines = append(outputLines, fmt.Sprintf("#\t\t%v", svc))
		}
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "")
	}

	// add all services that need to be merged and have commands to do so to the end of the output for visibility that they were checked and need to be merged
	if len(t.mergeRevisions) > 0 {
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "#\tThe following services were checked and need to have their revisions merged to the surviving services")
		outputLines = append(outputLines, "#\t\tOnce merged their API Service Instances may need updates to point to the latest revision.")
		outputLines = append(outputLines, sep)
		for _, action := range t.mergeRevisions {
			outputLines = append(outputLines, action)
		}
	}
	outputLines = append(outputLines, "")

	// add all services without issues to the end of the output for visibility that they were checked and have no duplicates
	if len(t.noIssues) > 0 {
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "#\tThe following services were checked and no duplicates found, no actions needed")
		outputLines = append(outputLines, sep2)
		for _, svc := range t.noIssues {
			outputLines = append(outputLines, fmt.Sprintf("#\t\t%v", svc))
		}
		outputLines = append(outputLines, sep)
		outputLines = append(outputLines, "")
	}

	output := strings.Join(outputLines, "\n")
	if t.outFile == "" || t.cfg.DryRun {
		fmt.Print(output)
	}
	if t.cfg.DryRun {
		return nil
	}

	if t.outFile != "" {
		os.WriteFile(t.outFile, []byte(output), 0777)
	}
	if t.backupFile != "" {
		backup := strings.Join(t.backup, "\n")
		os.WriteFile(t.backupFile, []byte(backup), 0777)
	}
	return nil
}

func (t *tool) handleGroup(logger *logrus.Entry, env string, services []string) {
	sort.Strings(services)

	itemToAssets := map[string]int{}
	totalAssets := 0

	if len(services) == 1 {
		svcInfo := t.serviceRegistry.GetAPIServiceInfo(env, services[0])
		if len(svcInfo.APIServiceInstances) == 0 {
			t.noAssetsSI = append(t.noAssetsSI, fmt.Sprintf("axway central delete -s %v apiservice %v", env, services[0]))
			return
		}
		t.noIssues = append(t.noIssues, services[0])
		return
	}

	if len(services) < 1 {
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

	// when greater than 2 output that more care needs to be taken
	if servicesWithAssets == 2 {
		multipleAssetsOutput := ""
		for _, service := range services {
			multipleAssetsOutput += fmt.Sprintf("#\t\t%v has %v assets\n", service, itemToAssets[service])
		}
		t.multipleAssets = append(t.multipleAssets, multipleAssetsOutput)
		return
	}

	// 1 or fewer services with assets
	t.backup = append(t.backup, sep)
	t.backup = append(t.backup, "#\tAll backups for "+serviceToKeep)

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
	backups := []*service.APIServiceInfo{}
	for _, service := range services {
		if service == serviceToKeep {
			continue
		}

		logger = logger.WithField("service", service)
		logger.Debug("comparing hash of revision on instance to hashes in service to keep")
		svcInfo := t.serviceRegistry.GetAPIServiceInfo(env, service)
		backups = append(backups, svcInfo)
		for _, inst := range svcInfo.APIServiceInstances {
			hash, err := util.GetAgentDetailsValue(inst, "tempHash")
			if err != nil {
				t.noSpecHash = append(t.noSpecHash, service)
				continue
			}
			logger = logger.WithField("hash", hash)

			logger.Debug("handling instance hash compare")
			if _, found := hashes[hash]; found {
				t.noMerge = append(t.noMerge, fmt.Sprintf("axway central delete -s %v apiservice %v", env, service))
			} else {
				actionOutput += fmt.Sprintf("#\t\t%v can be deleted after merging revision %v to %v\n", service, inst.Spec.ApiServiceRevision, serviceToKeep)
				commandOutput += fmt.Sprintf("axway central get -o json -s %v apiservicerevision %v > %v.json\n", env, inst.Spec.ApiServiceRevision, inst.Spec.ApiServiceRevision)
				commandOutput += fmt.Sprintf("jq '.spec.apiService |= \"%v\"' %v.json > %v-new.json\n", serviceToKeep, inst.Spec.ApiServiceRevision, inst.Spec.ApiServiceRevision)
				commandOutput += fmt.Sprintf("cp %v-new.json %v-new-bu.json\n", inst.Spec.ApiServiceRevision, inst.Spec.ApiServiceRevision)
				for _, s := range t.subResources {
					commandOutput += fmt.Sprintf("jq 'del(.%v)' %v-new.json > %v-new.json\n", s, inst.Spec.ApiServiceRevision, inst.Spec.ApiServiceRevision)
				}
				commandOutput += fmt.Sprintf("axway central apply -f %v-new.json\n", inst.Spec.ApiServiceRevision)
			}
		}
	}

	// add app actions and commands to merge array
	if actionOutput != "" {
		t.mergeRevisions = append(t.mergeRevisions, fmt.Sprintf("%v\n%v", strings.TrimRight(actionOutput, "\n"), sep2))
		t.mergeRevisions = append(t.mergeRevisions, fmt.Sprintf("%v\n%v", strings.TrimRight(commandOutput, "\n"), sep))
	}

	// append backup data to log
	j, _ := json.Marshal(backups)
	t.backup = append(t.backup, string(j))
	t.backup = append(t.backup, sep)
	t.backup = append(t.backup, "")
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
		if len(serviceInfo.APIServiceInstances) == 0 {
			grouping[serviceInfo.APIService.Metadata.ID] = append(grouping[serviceInfo.APIService.Metadata.ID], service)
			continue
		}
		for _, inst := range serviceInfo.APIServiceInstances {
			logger = logger.WithField("instance", inst.Name)

			details := util.GetAgentDetailStrings(inst)
			if groupBy == "" {
				// use the first service to determine if we will group by api id or primary key
				if _, found := details[definitions.AttrExternalAPIID]; found {
					groupBy = definitions.AttrExternalAPIID
				}
				if _, found := details[definitions.AttrExternalAPIPrimaryKey]; found {
					groupBy = definitions.AttrExternalAPIPrimaryKey
				}
				if groupBy == "" {
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
