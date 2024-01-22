package service

import (
	v1 "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/api/v1"
	catalog "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/catalog/v1alpha1"
	management "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/management/v1alpha1"
)

type APIServiceInfo struct {
	APIService          *management.APIService                    `json:"apiService,omitempty"`
	APIServiceRevisions map[string]*management.APIServiceRevision `json:"apiServiceRevisions,omitempty"`
	APIServiceInstances map[string]*management.APIServiceInstance `json:"apiServiceInstances,omitempty"`
}
type AssetInfo struct {
	Asset                    *catalog.Asset               `json:"asset,omitempty"`
	DeletedServiceReferences []v1.Reference               `json:"deletedServiceReferences,omitempty"`
	AssetResources           map[string]AssetResourceInfo `json:"assetResources,omitempty"`
	AssetReleases            map[string]AssetReleaseInfo  `json:"assetReleases,omitempty"`
	AssetMappings            []*v1.ResourceInstance
}

type AssetReleaseInfo struct {
	AssetRelease   *catalog.AssetRelease        `json:"assetRelease,omitempty"`
	ReleaseTag     *catalog.ReleaseTag          `json:"releaseTag,omitempty"`
	AssetResources map[string]AssetResourceInfo `json:"assetResources,omitempty"`
}
type AssetResourceInfo struct {
	AssetResource *catalog.AssetResource `json:"assetResource,omitempty"`
}

type ProductInfo struct {
	Product            *catalog.Product              `json:"product,omitempty"`
	ProductReleases    map[string]ProductReleaseInfo `json:"productReleases,omitempty"`
	PlansWithNoRelease map[string]PlanInfo           `json:"planWithNoRelease,omitempty"`
}

type ProductReleaseInfo struct {
	ProductRelease *catalog.ProductRelease `json:"productRelease,omitempty"`
	ReleaseTag     *catalog.ReleaseTag     `json:"releaseTag,omitempty"`
	Plans          map[string]PlanInfo     `json:"productPlans,omitempty"`
}

type PlanInfo struct {
	Plan   *catalog.ProductPlan `json:"productPlan,omitempty"`
	Quotas map[string]QuotaInfo `json:"quotas,omitempty"`
}

type QuotaInfo struct {
	Quota          *catalog.Quota                    `json:"quota,omitempty"`
	AssetResources map[string]*catalog.AssetResource `json:"assetResources,omitempty"`
}
