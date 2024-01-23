package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Axway/agent-sdk/pkg/apic"
	v1 "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/api/v1"
	catalog "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/catalog/v1alpha1"
	management "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/management/v1alpha1"
	"github.com/sirupsen/logrus"
)

type assetCatalogOpt func(s *assetCatalog)

type AssetCatalog interface {
	ReadAssets(repairProduct bool) error
	WriteAssets()
	GetAssetOutput() []v1.Interface
	RepairAsset()
	PostRepairAsset()
	GetAssetInfo(logger *logrus.Entry, id string) AssetInfo
	FindAssetResource(logger *logrus.Entry, nameWithScope string) string
	AssetsForInstance(group, env, instance string) []string
}

type assetCatalog struct {
	logger                *logrus.Logger
	apicClient            apic.Client
	Assets                map[string]AssetInfo
	AssetResourcesMap     map[string]string
	InstanceToResourceMap map[string][]string
	resourceLock          sync.Mutex
	serviceRegistry       ServiceRegistry
	filterUsingRegistry   bool
	assetRelRes           bool
	forExport             bool
	stripData             bool
	dryRun                bool
}

func NewAssetCatalog(logger *logrus.Logger, apicClient apic.Client, dryRun bool, serviceRegistry ServiceRegistry, opt ...assetCatalogOpt) AssetCatalog {
	a := &assetCatalog{
		logger:                logger,
		apicClient:            apicClient,
		Assets:                make(map[string]AssetInfo),
		AssetResourcesMap:     make(map[string]string),
		InstanceToResourceMap: make(map[string][]string),
		resourceLock:          sync.Mutex{},
		serviceRegistry:       serviceRegistry,
		dryRun:                dryRun,
	}

	for _, o := range opt {
		o(a)
	}

	return a
}

func WithFilterUsingRegistry() assetCatalogOpt {
	return func(a *assetCatalog) {
		a.filterUsingRegistry = true
	}
}

func WithAssetReleaseResources() assetCatalogOpt {
	return func(a *assetCatalog) {
		a.assetRelRes = true
	}
}

func ForExport() assetCatalogOpt {
	return func(a *assetCatalog) {
		a.forExport = true
	}
}

func StripData() assetCatalogOpt {
	return func(a *assetCatalog) {
		a.stripData = true
	}
}

func (t *assetCatalog) WriteAssets() {
	SaveToFile(t.logger, "asset-catalog", "asset-catalog.json", t.Assets)
}

func (t *assetCatalog) GetAssetOutput() []v1.Interface {
	objs := []v1.Interface{}
	for _, assetInfo := range t.Assets {
		objs = append(objs, assetInfo.Asset)
		for _, mpng := range assetInfo.AssetMappings {
			objs = append(objs, mpng)
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
				case catalog.AssetGVK().Kind:
					a := catalog.NewAsset("")
					a.FromInstance(ri)
					a.Icon = struct{}{}
					ri, _ = a.AsInstance()
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

func (t *assetCatalog) ReadAssets(repairProduct bool) error {
	t.logger.Info("Reading Assets...")
	a := catalog.NewAsset("")
	assets := []*v1.ResourceInstance{}
	validEnvs := map[string]struct{}{}
	if t.filterUsingRegistry {
		wg := sync.WaitGroup{}
		aMutex := sync.Mutex{}
		wg.Add(len(t.serviceRegistry.GetEnvs()))
		for _, d := range t.serviceRegistry.GetEnvs() {
			validEnvs[d] = struct{}{}
			go func(env string) {
				defer wg.Done()
				params := map[string]string{
					"query": fmt.Sprintf("metadata.references.name==%s", env),
				}
				envAssets, err := t.apicClient.GetAPIV1ResourceInstances(params, a.GetKindLink())
				if err != nil {
					t.logger.WithError(err).Error("unable to read assets")
				}
				aMutex.Lock()
				defer aMutex.Unlock()
				assets = append(assets, envAssets...)
			}(d)
		}
		wg.Wait()
	} else {
		var err error
		assets, err = t.apicClient.GetAPIV1ResourceInstances(nil, a.GetKindLink())
		if err != nil {
			t.logger.WithError(err).Error("unable to read assets")
		}
	}
	if t.forExport {
		limiter := make(chan *v1.ResourceInstance, 25)

		lock := sync.Mutex{}

		wg := sync.WaitGroup{}
		wg.Add(len(assets))
		for _, d := range assets {
			go func() {
				defer wg.Done()
				in := <-limiter

				ca := catalog.NewAsset("")
				ca.FromInstance(in)
				logger := t.logger.WithField("asset", in.Name)
				lock.Lock()
				defer lock.Unlock()
				t.Assets[in.Metadata.ID] = AssetInfo{
					Asset:         ca,
					AssetMappings: t.readAssetMappings(logger, in.Name, validEnvs),
				}
			}()
			limiter <- d
		}

		wg.Wait()
		close(limiter)
		return nil
	}
	serviceReferencesFound := true

	limiter := make(chan *v1.ResourceInstance, 25)
	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(assets))

	for _, d := range assets {
		go func() {
			defer wg.Done()
			asset := <-limiter

			ca := catalog.NewAsset("")
			ca.FromInstance(asset)
			logger := t.logger.
				WithField("asset", ca.GetName())
			if ca.Status != nil {
				logger = t.logger.
					WithField("assetStatus", ca.Status.Level)
			}
			logger.Info("Reading Asset ok")
			assetResources := t.readAssetResources(logger, ca.Name, catalog.AssetGVK().Kind, ca.Metadata.ID)
			assetInfo := AssetInfo{
				Asset:                    ca,
				DeletedServiceReferences: make([]v1.Reference, 0),
				AssetResources:           assetResources,
			}
			if !repairProduct {
				assetReleases := t.readAssetReleases(logger, asset.GetMetadata().ID)
				assetInfo.AssetReleases = assetReleases
			}
			lock.Lock()
			t.Assets[asset.GetMetadata().ID] = assetInfo
			lock.Unlock()
			if !repairProduct {
				for _, assetDeletedRef := range ca.Metadata.DeletedReferences {
					if assetDeletedRef.Kind == management.APIServiceGVK().Kind {
						if svc := t.serviceRegistry.FindService(logger, assetDeletedRef.ScopeName, assetDeletedRef.Name); svc == nil {
							logger.
								WithField("apiServiceScopeName", assetDeletedRef.ScopeName).
								WithField("apiServiceName", assetDeletedRef.Name).
								Error("unable to find the APIService associated to asset")
							serviceReferencesFound = false
						}
					}
				}
			}
		}()
		limiter <- d
	}
	wg.Wait()
	close(limiter)

	if !serviceReferencesFound && !t.serviceRegistry.IsUsingMapping() {
		return fmt.Errorf("unable to identify the APIService associated to the assets")
	}
	return nil
}

func (t *assetCatalog) readAssetReleases(logger *logrus.Entry, assetID string) map[string]AssetReleaseInfo {
	t.logger.Debug("Reading AssetReleases...")
	assetReleaseInfos := make(map[string]AssetReleaseInfo)
	a := catalog.NewAssetRelease("")
	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", assetID),
	}
	assetReleases, err := t.apicClient.GetAPIV1ResourceInstances(params, a.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to read asset releases")
		return assetReleaseInfos
	}
	for _, assetRelease := range assetReleases {
		ar := catalog.NewAssetRelease("")
		ar.FromInstance(assetRelease)
		logger = logger.WithField("assetRelease", ar.Name)
		if ar.Status != nil {
			logger = logger.WithField("assetReleaseStatus", ar.Status.Level)
		}

		assetResources := map[string]AssetResourceInfo{}
		if t.assetRelRes {
			assetResources = t.readAssetResources(logger, ar.Name, ar.Kind, ar.Metadata.ID)
		}
		var releaseTag *catalog.ReleaseTag
		if refs, ok := ar.References.([]interface{}); ok {
			for _, reference := range refs {
				if ref, ok := reference.(map[string]interface{}); ok {
					if kind := ref["kind"].(string); kind == catalog.ReleaseTagGVK().Kind {
						name := ref["name"].(string)
						releaseTagElements := strings.Split(name, "/")
						releaseTag = readReleaseTag(logger, t.apicClient, releaseTagElements[2], catalog.AssetGVK().Kind, releaseTagElements[1])
					}
				}
			}
		}
		if releaseTag != nil {
			logger = logger.WithField("releaseTag", releaseTag.Name)
		}
		logger.Debug("Reading AssetRelease ok")
		assetReleaseInfo := AssetReleaseInfo{
			AssetRelease:   ar,
			ReleaseTag:     releaseTag,
			AssetResources: assetResources,
		}
		assetReleaseInfos[ar.Metadata.ID] = assetReleaseInfo
	}

	return assetReleaseInfos
}

func (t *assetCatalog) readAssetMappings(logger *logrus.Entry, scopeName string, validEnvs map[string]struct{}) []*v1.ResourceInstance {
	a := catalog.NewAssetMapping("", scopeName)
	assetMappings, err := t.apicClient.GetAPIV1ResourceInstances(nil, a.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to read asset mappings")
		return []*v1.ResourceInstance{}
	}
	if len(validEnvs) == 0 {
		return assetMappings
	}

	filteredMappings := []*v1.ResourceInstance{}
	for _, ri := range assetMappings {
		a.FromInstance(ri)
		apiSvcParts := strings.Split(a.Spec.Inputs.ApiService, "/")
		if _, found := validEnvs[apiSvcParts[1]]; !found {
			// not part of an env that is being exported
			continue
		}
		apiRevParts := strings.Split(a.Spec.Inputs.ApiServiceRevision, "/")
		svcInfo := t.serviceRegistry.GetAPIServiceInfo(apiSvcParts[1], apiSvcParts[2])

		// check that the revision is being exported
		found := false
		for _, rev := range svcInfo.APIServiceRevisions {
			if rev.Name == apiRevParts[2] {
				found = true
				break
			}
		}
		if found {
			filteredMappings = append(filteredMappings, ri)
		}
	}
	return filteredMappings
}

func (t *assetCatalog) readAssetResources(logger *logrus.Entry, scopeName, scopeKind, scopeID string) map[string]AssetResourceInfo {
	assetResourceInfos := make(map[string]AssetResourceInfo)
	a, _ := catalog.NewAssetResource("", scopeKind, scopeName)
	assetResources, err := t.apicClient.GetAPIV1ResourceInstances(nil, a.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to read asset resources")
		return assetResourceInfos
	}
	for _, assetResource := range assetResources {
		ar, _ := catalog.NewAssetResource("", scopeKind, "")
		ar.FromInstance(assetResource)
		assetResourceInfo := AssetResourceInfo{
			AssetResource: ar,
		}

		assetResourceInfos[ar.Metadata.ID] = assetResourceInfo
		t.resourceLock.Lock()
		t.AssetResourcesMap[assetResourceMapKey(scopeName, ar.Name)] = scopeID
		t.InstanceToResourceMap[ar.References.ApiServiceInstance] = append(t.InstanceToResourceMap[ar.References.ApiServiceInstance], assetResourceMapKey(scopeName, ar.Name))
		t.resourceLock.Unlock()
		logger.
			WithField("assetResource", ar.Name).
			WithField("assetResourceStatus", ar.Spec.Status).
			WithField("apiServiceRevision", ar.References.ApiServiceRevision).
			WithField("apiServiceInstance", ar.References.ApiServiceInstance).
			Debug("Reading AssetResource ok")
	}

	return assetResourceInfos
}

func assetResourceMapKey(scopeName, assetResourceName string) string {
	return fmt.Sprintf("%s/%s", scopeName, assetResourceName)
}

func (t *assetCatalog) FindAssetResource(logger *logrus.Entry, nameWithScope string) string {
	return t.AssetResourcesMap[nameWithScope]
}

func (t *assetCatalog) GetAssetInfo(logger *logrus.Entry, id string) AssetInfo {
	if i, found := t.Assets[id]; found {
		return i
	}
	return AssetInfo{}
}

func (t *assetCatalog) AssetsForInstance(group, env, instance string) []string {
	instancePath := fmt.Sprintf("%s/%s/%s", group, env, instance)
	if assets, found := t.InstanceToResourceMap[instancePath]; found {
		return assets
	}
	return nil
}

func (t *assetCatalog) RepairAsset() {
	for _, asset := range t.Assets {
		if asset.Asset.Status != nil && asset.Asset.Status.Level == "Error" {
			logger := t.logger.
				WithField("assetID", asset.Asset.Metadata.ID).
				WithField("assetName", asset.Asset.Name)
			logger.Infof("Processing asset")
			t.deleteAssetResources(logger, asset)
			t.recreateAssetMapping(logger, asset)
			err := t.setAssetToDraft(logger, asset)
			if err == nil {
				// ri, _ := t.apicClient.GetResource(asset.Asset.GetSelfLink())
				// a := catalog.NewAsset("")
				// a.FromInstance(ri)
				// a.Spec.AutoRelease = nil
				// _, err := t.apicClient.UpdateResourceInstance(a)
				// if err != nil {
				// 	t.logger.WithError(err).Error("unable to update asset %s for draft", asset.Asset.Name)
				// }
				releaseTagRI, err := t.createAssetRelease(logger, asset)
				if err == nil {
					logger = logger.
						WithField("newReleaseTagID", releaseTagRI.Metadata.ID).
						WithField("newReleaseTagName", releaseTagRI.Name)
					logger.Info("Created new ReleaseTag for Asset")

					t.waitForAssetRelease(releaseTagRI.Metadata.ID)

					for _, assetRelease := range asset.AssetReleases {
						t.deprecatePreviousAssetRelease(logger, assetRelease)
					}
				}
			}

		}
	}
}

func (t *assetCatalog) waitForAssetRelease(releaseTagID string) {
	if t.dryRun {
		return
	}
	n := 0
	for newAssetRelease := t.getAssetReleaseForReleaseTag(releaseTagID); newAssetRelease == nil && n < 5; {
		newAssetRelease = t.getAssetReleaseForReleaseTag(releaseTagID)
		n++
		time.Sleep(time.Second)
	}
}

func (t *assetCatalog) getAssetReleaseForReleaseTag(releaseTagID string) *catalog.AssetRelease {
	a := catalog.NewAssetRelease("")

	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", releaseTagID),
	}
	assetReleases, err := t.apicClient.GetAPIV1ResourceInstances(params, a.GetKindLink())
	if err != nil {
		t.logger.Error(err)
	}
	for _, assetRelease := range assetReleases {
		a.FromInstance(assetRelease)
		return a
	}

	return nil
}

func (t *assetCatalog) PostRepairAsset() {
	for _, asset := range t.Assets {
		if asset.Asset.Status != nil && asset.Asset.Status.Level == "Error" {
			logger := t.logger.
				WithField("assetID", asset.Asset.Metadata.ID).
				WithField("assetName", asset.Asset.Name)
			logger.Infof("Post processing asset")
			for _, assetRelease := range asset.AssetReleases {
				t.archivePreviousAssetRelease(logger, assetRelease)
			}
		}
	}
}

func (t *assetCatalog) deleteAssetResources(logger *logrus.Entry, asset AssetInfo) {
	for _, assetResource := range asset.AssetResources {
		logger = logger.
			WithField("assetResourceID", assetResource.AssetResource.Metadata.ID).
			WithField("assetResourceName", assetResource.AssetResource.Name)
		logger.Info("Removing AssetResource")
		if !t.dryRun {
			err := t.apicClient.DeleteResourceInstance(assetResource.AssetResource)
			if err != nil {
				logger.WithError(err).Error("Unable to delete the corrupted asset resource")
			}
			key := assetResourceMapKey(asset.Asset.Name, assetResource.AssetResource.Name)
			delete(t.AssetResourcesMap, key)
		}
	}
}

func (t *assetCatalog) recreateAssetMapping(logger *logrus.Entry, asset AssetInfo) {
	for _, assetDeletedRef := range asset.Asset.Metadata.DeletedReferences {
		if assetDeletedRef.Kind == management.APIServiceGVK().Kind {
			t.createAssetMapping(logger, asset.Asset.Name, assetDeletedRef)
		}
	}
	for _, assetRef := range asset.Asset.Metadata.References {
		if assetRef.Kind == management.APIServiceGVK().Kind {
			t.createAssetMapping(logger, asset.Asset.Name, assetRef)
		}
	}
}

func (t *assetCatalog) createAssetMapping(logger *logrus.Entry, assetName string, assetSvcRef v1.Reference) {
	logger = logger.WithField("apiService", assetSvcRef.Name)
	svc := t.serviceRegistry.FindService(logger, assetSvcRef.ScopeName, assetSvcRef.Name)
	if svc != nil {
		logger.Info("Creating asset mapping")
		am := catalog.NewAssetMapping("", assetName)

		am.Spec.Inputs.ApiService = assetSvcRef.Group + "/" + svc.Metadata.Scope.Name + "/" + svc.Name
		am.Spec.Inputs.Stage = "default"
		ri, err := am.AsInstance()
		if !t.dryRun {
			ri, err = t.apicClient.CreateResourceInstance(am)
			if err != nil {
				logger.WithError(err).Error("unable to create new asset mapping")
			}
		}
		if err == nil {
			// wait for asset resource
			am := t.waitForAssetMappingStatus(ri.Name, assetName, svc.Name)
			if am != nil {
				refName := am.Status.Outputs[0].Resource.AssetResource.Ref
				element := strings.Split(refName, "/")
				if len(element) == 3 {
					t.AssetResourcesMap[assetResourceMapKey(assetName, assetSvcRef.Name)] = assetResourceMapKey(assetName, element[2])
				}
			}
			logger = logger.
				WithField("newAssetMappingID", ri.Metadata.ID).
				WithField("newAssetMappingName", ri.Name)
			logger.Info("Created asset mapping")
		}
	}
}

func (t *assetCatalog) waitForAssetMappingStatus(assetMappingName, assetName, svcName string) *catalog.AssetMapping {
	if t.dryRun {
		return &catalog.AssetMapping{Status: catalog.AssetMappingStatus{
			Outputs: []catalog.AssetMappingStatusOutputs{
				{
					Resource: catalog.AssetMappingStatusResource{
						AssetResource: catalog.AssetMappingStatusResourceAssetResource{
							Ref: fmt.Sprintf("catalog/%s/%s", assetName, svcName),
						},
					},
				},
			},
		}}
	}
	n := 0
	for updatedAssetMapping := t.getAssetMapping(assetMappingName, assetName); n < 5; {
		if updatedAssetMapping != nil {
			return updatedAssetMapping
		}
		updatedAssetMapping = t.getAssetMapping(assetMappingName, assetName)

		n++
		time.Sleep(time.Second)
	}
	return nil
}

func (t *assetCatalog) getAssetMapping(assetMappingName, assetName string) *catalog.AssetMapping {
	a := catalog.NewAssetMapping(assetMappingName, assetName)
	ri, err := t.apicClient.GetResource(a.GetSelfLink())
	if err == nil {
		am := catalog.NewAssetMapping("", assetName)
		am.FromInstance(ri)
		if len(am.Status.Outputs) != 0 {
			return am
		}
	}
	return nil
}

func (t *assetCatalog) setAssetToDraft(logger *logrus.Entry, asset AssetInfo) error {
	logger.Info("Setting asset to draft")
	if !t.dryRun {
		statusErr := t.apicClient.CreateSubResource(asset.Asset.ResourceMeta, map[string]interface{}{"state": catalog.AssetStateDRAFT})
		if statusErr != nil {
			logger.WithError(statusErr).Error("unable to transition asset to draft")
			return statusErr
		}
	}
	return nil
}

func (t *assetCatalog) createAssetRelease(logger *logrus.Entry, asset AssetInfo) (*v1.ResourceInstance, error) {
	logger.Info("Creating new asset release")
	releaseTag, _ := catalog.NewReleaseTag("", catalog.AssetGVK().Kind, asset.Asset.Name)
	releaseTag.Spec.ReleaseType = "patch"
	releaseTag.Title = asset.Asset.Title
	if t.dryRun {
		releaseTag.Name = "dry-run"
		return releaseTag.AsInstance()
	}

	releaseTagRI, err := t.apicClient.CreateResourceInstance(releaseTag)
	if err != nil {
		logger.WithError(err).Errorf("unable to create new release tag for asset: %s", asset.Asset.Name)
		return nil, err
	}
	return releaseTagRI, err
}

func (t *assetCatalog) deprecatePreviousAssetRelease(logger *logrus.Entry, assetRelease AssetReleaseInfo) {
	releaseTag := assetRelease.ReleaseTag
	logger = logger.
		WithField("assetReleaseID", releaseTag.Metadata.ID).
		WithField("assetReleaseName", releaseTag.Name)

	if assetRelease.AssetRelease.Status != nil && assetRelease.AssetRelease.Status.Level == "Error" {
		switch releaseTag.State.(string) {
		case string(catalog.AssetStateACTIVE):
			// deprecate the asset release
			logger.Info("Deprecating AssetRelease")
			if !t.dryRun {
				statusErr := t.apicClient.CreateSubResource(releaseTag.ResourceMeta, map[string]interface{}{"state": catalog.ProductStateDEPRECATED})
				if statusErr != nil {
					logger.WithError(statusErr).Error("error deprecating AssetRelease")
					break
				}
				releaseTag.State = string(catalog.AssetStateDEPRECATED)
			}
		}
	}
}

func (t *assetCatalog) archivePreviousAssetRelease(logger *logrus.Entry, assetRelease AssetReleaseInfo) {
	releaseTag := assetRelease.ReleaseTag
	logger = logger.
		WithField("assetReleaseID", releaseTag.Metadata.ID).
		WithField("assetReleaseName", releaseTag.Name)

	if assetRelease.AssetRelease.Status != nil && assetRelease.AssetRelease.Status.Level == "Error" {
		switch releaseTag.State.(string) {
		case string(catalog.AssetStateDEPRECATED):
			// archive the asset release
			logger.Info("Archiving AssetRelease")
			if !t.dryRun {
				statusErr := t.apicClient.CreateSubResource(releaseTag.ResourceMeta, map[string]interface{}{"state": catalog.ProductStateARCHIVED})
				if statusErr != nil {
					logger.WithError(statusErr).Error("error archiving AssetRelease")
					break
				}
			}
		}
	}
}
