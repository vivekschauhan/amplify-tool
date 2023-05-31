package service

import (
	v1 "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/api/v1"
	catalog "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/catalog/v1alpha1"
	management "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/management/v1alpha1"
)

type APIServiceInfo struct {
	APIService          *management.APIService
	APIServiceRevisions map[string]APIServiceRevisionInfo
}

type APIServiceRevisionInfo struct {
	APIServiceRevision  *management.APIServiceRevision
	APIServiceInstances map[string]*management.APIServiceInstance
}

type AssetInfo struct {
	Asset                    *catalog.Asset
	DeletedServiceReferences []v1.Reference
	AssetResources           map[string]AssetResourceInfo
	AssetReleases            map[string]AssetReleaseInfo
}

type AssetReleaseInfo struct {
	AssetRelease   *catalog.AssetRelease
	ReleaseTag     *catalog.ReleaseTag
	AssetResources map[string]AssetResourceInfo
}
type AssetResourceInfo struct {
	AssetResource *catalog.AssetResource
}

type ProductInfo struct {
	Product         *catalog.Product
	ProductReleases map[string]ProductReleaseInfo
}

type ProductReleaseInfo struct {
	ProductRelease *catalog.ProductRelease
	ReleaseTag     *catalog.ReleaseTag
	Plans          map[string]PlanInfo
}

type PlanInfo struct {
	Plan   *catalog.ProductPlan
	Quotas map[string]QuotaInfo
}

type QuotaInfo struct {
	Quota          *catalog.Quota
	AssetResources map[string]*catalog.AssetResource
}
