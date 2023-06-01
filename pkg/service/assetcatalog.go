package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/Axway/agent-sdk/pkg/apic"
	v1 "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/api/v1"
	catalog "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/catalog/v1alpha1"
	management "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/management/v1alpha1"
	"github.com/sirupsen/logrus"
)

type AssetCatalog interface {
	ReadAssets() error
	RepairAsset()
	PostRepairAsset()
}

type assetCatalog struct {
	logger          *logrus.Logger
	apicClient      apic.Client
	Assets          map[string]AssetInfo
	serviceRegistry ServiceRegistry
	dryRun          bool
}

func NewAssetCatalog(logger *logrus.Logger, serviceRegistry ServiceRegistry, apicClient apic.Client, dryRun bool) AssetCatalog {
	return &assetCatalog{
		logger:          logger,
		apicClient:      apicClient,
		Assets:          make(map[string]AssetInfo),
		serviceRegistry: serviceRegistry,
		dryRun:          dryRun,
	}
}

func (t *assetCatalog) ReadAssets() error {
	t.logger.Info("Reading Assets...")
	a := catalog.NewAsset("")
	assets, err := t.apicClient.GetAPIV1ResourceInstances(nil, a.GetKindLink())
	if err != nil {
		t.logger.WithError(err).Error("unable to read assets")
	}
	serviceReferencesFound := true
	for _, asset := range assets {
		ca := catalog.NewAsset("")
		ca.FromInstance(asset)
		logger := t.logger.
			WithField("asset", ca.GetName())
		if ca.Status != nil {
			logger = t.logger.
				WithField("assetStatus", ca.Status.Level)
		}
		logger.Info("Reading Asset ok")
		assetResources := t.readAssetResources(logger, ca.Name, catalog.AssetGVK().Kind)
		assetReleases := t.readAssetReleases(logger, asset.GetMetadata().ID)
		assetInfo := AssetInfo{
			Asset:                    ca,
			DeletedServiceReferences: make([]v1.Reference, 0),
			AssetResources:           assetResources,
			AssetReleases:            assetReleases,
		}
		t.Assets[asset.GetMetadata().ID] = assetInfo
		for _, assetDeletedRef := range ca.Metadata.DeletedReferences {
			if assetDeletedRef.Kind == management.APIServiceGVK().Kind {
				if svc := t.serviceRegistry.GetAPIService(assetDeletedRef.ScopeName, assetDeletedRef.Name); svc == nil {
					logger.
						WithField("apiServiceScopeName", assetDeletedRef.ScopeName).
						WithField("apiServiceName", assetDeletedRef.Name).
						Error("unable to find the APIService associated to asset")
					serviceReferencesFound = false
				}
			}
		}
	}

	if !serviceReferencesFound {
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

		// assetResources := t.readAssetResources(logger, ar.Name, catalog.AssetReleaseGVK().Kind)
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
			AssetRelease: ar,
			ReleaseTag:   releaseTag,
			// AssetResources: assetResources,
		}
		assetReleaseInfos[ar.Metadata.ID] = assetReleaseInfo
	}

	return assetReleaseInfos
}

func (t *assetCatalog) readAssetResources(logger *logrus.Entry, assetRelease, scope string) map[string]AssetResourceInfo {
	assetResourceInfos := make(map[string]AssetResourceInfo)
	a, _ := catalog.NewAssetResource("", scope, assetRelease)
	assetReleaseResources, err := t.apicClient.GetAPIV1ResourceInstances(nil, a.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to read asset resources")
		return assetResourceInfos
	}
	for _, assetReleaseResource := range assetReleaseResources {
		ar, _ := catalog.NewAssetResource("", scope, "")
		ar.FromInstance(assetReleaseResource)
		assetResourceInfo := AssetResourceInfo{
			AssetResource: ar,
		}

		assetResourceInfos[ar.Metadata.ID] = assetResourceInfo

		logger.
			WithField("assetResource", ar.Name).
			WithField("assetResourceStatus", ar.Spec.Status).
			WithField("apiServiceRevision", ar.References.ApiServiceRevision).
			WithField("apiServiceInstance", ar.References.ApiServiceInstance).
			Debug("Reading AssetResource ok")
	}

	return assetResourceInfos
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
	logger.Info("Creating asset mapping")

	am := catalog.NewAssetMapping("", assetName)
	am.Spec.Inputs.ApiService = assetSvcRef.Group + "/" + assetSvcRef.ScopeName + "/" + assetSvcRef.Name
	am.Spec.Inputs.Stage = "default"
	ri, err := am.AsInstance()
	if !t.dryRun {
		ri, err = t.apicClient.CreateResourceInstance(am)
		if err != nil {
			logger.WithError(err).Error("unable to create new asset mapping")
		}
	}
	if err == nil {
		logger = logger.
			WithField("newAssetMappingID", ri.Metadata.ID).
			WithField("newAssetMappingName", ri.Name)
		logger.Info("Created asset mapping")
	}
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
