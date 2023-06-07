package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Axway/agent-sdk/pkg/apic"
	v1 "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/api/v1"
	catalog "github.com/Axway/agent-sdk/pkg/apic/apiserver/models/catalog/v1alpha1"
	"github.com/sirupsen/logrus"
)

type ProductCatalog interface {
	ReadProducts()
	WriteProducts()
	PreProcessProductForAssetRepair()
	PostProcessProductForAssetRepair()
}

type productCatalog struct {
	logger       *logrus.Logger
	apicClient   apic.Client
	Products     map[string]ProductInfo
	assetCatalog AssetCatalog
	dryRun       bool
}

func NewProductCatalog(logger *logrus.Logger, assetCatalog AssetCatalog, apicClient apic.Client, dryRun bool) ProductCatalog {
	return &productCatalog{
		logger:       logger,
		apicClient:   apicClient,
		Products:     make(map[string]ProductInfo),
		assetCatalog: assetCatalog,
		dryRun:       dryRun,
	}
}

func (t *productCatalog) WriteProducts() {
	saveToFile(t.logger, "product-catalog", t.Products)
}

func (t *productCatalog) ReadProducts() {
	t.logger.Info("Reading Products...")
	p := catalog.NewProduct("")
	products, err := t.apicClient.GetAPIV1ResourceInstances(nil, p.GetKindLink())
	if err != nil {
		t.logger.WithError(err).Error("unable to read products")
		return
	}

	for _, product := range products {
		cp := catalog.NewProduct("")
		cp.FromInstance(product)
		logger := t.logger.WithField("productName", cp.Name)
		if cp.Status != nil {
			logger = logger.WithField("productStatus", cp.Status.Level)
		}

		logger.Info("Reading Product ok")
		productReleases := t.readProductReleases(logger, product.GetMetadata().ID, cp.Name)
		plansWithNoRelease := t.readProductPlans(logger, product.GetMetadata().ID, true)

		productInfo := ProductInfo{
			Product:            cp,
			ProductReleases:    productReleases,
			PlansWithNoRelease: plansWithNoRelease,
		}

		t.Products[product.GetMetadata().ID] = productInfo
	}
}

func (t *productCatalog) readProductReleases(logger *logrus.Entry, productID, productName string) map[string]ProductReleaseInfo {
	productReleaseInfos := make(map[string]ProductReleaseInfo)
	p := catalog.NewProductRelease("")
	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", productID),
	}
	productReleases, err := t.apicClient.GetAPIV1ResourceInstances(params, p.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to read product releases")
		return productReleaseInfos
	}
	for _, productRelease := range productReleases {
		pr := catalog.NewProductRelease("")
		pr.FromInstance(productRelease)
		logger = logger.WithField("productRelease", pr.Name)
		if pr.Status != nil {
			logger = logger.WithField("productReleaseStatus", pr.Status.Level)
		}

		releaseTagName := strings.Split(pr.Spec.ReleaseTag, "/")
		releaseTag := readReleaseTag(logger, t.apicClient, releaseTagName[1], catalog.ProductGVK().Kind, productName)
		if releaseTag != nil {
			logger = logger.WithField("releaseTag", releaseTag.Name)
		}
		logger.Debug("Reading ProductRelease ok")

		plans := t.readProductPlans(logger, pr.GetMetadata().ID, false)
		productReleaseInfo := ProductReleaseInfo{
			ProductRelease: pr,
			Plans:          plans,
			ReleaseTag:     releaseTag,
		}
		productReleaseInfos[pr.Metadata.ID] = productReleaseInfo
	}

	return productReleaseInfos
}

func (t *productCatalog) getProductReleaseForReleaseTag(releaseTagID string) *catalog.ProductRelease {
	p := catalog.NewProductRelease("")

	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", releaseTagID),
	}
	productReleases, err := t.apicClient.GetAPIV1ResourceInstances(params, p.GetKindLink())
	if err != nil {
		t.logger.Error(err)
	}
	for _, productRelease := range productReleases {
		p.FromInstance(productRelease)
		return p
	}

	return nil
}

func readReleaseTag(logger *logrus.Entry, apicClient apic.Client, releaseName, releaseScopeKind, releaseScope string) *catalog.ReleaseTag {
	releaseTag, _ := catalog.NewReleaseTag(releaseName, releaseScopeKind, releaseScope)
	resource, err := apicClient.GetResource(releaseTag.GetSelfLink())
	if err != nil {
		logger.WithError(err).Error("unable to read release tag")
		return nil
	}
	ri, _ := resource.AsInstance()
	releaseTag.FromInstance(ri)
	return releaseTag
}

func (t *productCatalog) readProductPlans(logger *logrus.Entry, referenceID string, withNoReference bool) map[string]PlanInfo {
	plans := make(map[string]PlanInfo)
	p := catalog.NewProductPlan("")
	params := map[string]string{
		"query": fmt.Sprintf("metadata.references.id==%s", referenceID),
	}
	productPlans, err := t.apicClient.GetAPIV1ResourceInstances(params, p.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to read product plans")
		return plans
	}

	for _, productPlan := range productPlans {
		pr := catalog.NewProductPlan("")
		pr.FromInstance(productPlan)
		logger = logger.WithField("productPlan", pr.Name)
		if pr.Status != nil {
			logger = logger.WithField("productPlanStatus", pr.Status.Level)
		}
		if withNoReference && pr.References != nil && pr.References.Product.Release != "" {
			continue
		}

		logger.Debug("Reading ProductPlan ok")
		quotas := t.readPlanQuotas(logger, pr.GetName())
		planInfo := PlanInfo{
			Plan:   pr,
			Quotas: quotas,
		}
		plans[pr.Metadata.ID] = planInfo
	}

	return plans
}

func (t *productCatalog) readPlanQuotas(logger *logrus.Entry, planName string) map[string]QuotaInfo {
	quotaInfos := make(map[string]QuotaInfo)
	q := catalog.NewQuota("", planName)
	planQuotas, err := t.apicClient.GetAPIV1ResourceInstances(nil, q.GetKindLink())
	if err != nil {
		logger.WithError(err).Error("unable to read quotas")
		return quotaInfos
	}

	for _, quota := range planQuotas {
		pq := catalog.NewQuota("", planName)
		pq.FromInstance(quota)
		logger = logger.WithField("quota", pq.Name)
		logger.Debug("Reading Quota ok")
		// quotaAssetResources := t.readQuotaAssetResources(logger, pq)
		quotaInfo := QuotaInfo{
			Quota: pq,
			// AssetResources: quotaAssetResources,
		}
		quotaInfos[pq.Metadata.ID] = quotaInfo
	}

	return quotaInfos
}

func (t *productCatalog) readQuotaAssetResources(logger *logrus.Entry, quota *catalog.Quota) map[string]*catalog.AssetResource {
	quotaAssetResources := make(map[string]*catalog.AssetResource)
	for _, resource := range quota.Spec.Resources {
		buf, _ := json.Marshal(resource)
		qar := &catalog.QuotaSpecAssetResourceRef{}
		json.Unmarshal(buf, qar)
		nameElements := strings.Split(qar.Name, "/")
		ar, _ := catalog.NewAssetResource(nameElements[1], catalog.AssetGVK().Kind, nameElements[0])
		quotaAssetResources[ar.Name] = ar
		logger.
			WithField("quotaAssetResource", ar.Name).
			WithField("quotaAssetResourceScope", ar.Metadata.Scope.Name).
			Debug("Quota AssetResource")
	}

	return quotaAssetResources
}

func (t *productCatalog) PreProcessProductForAssetRepair() {
	for _, product := range t.Products {
		if product.Product.Status != nil && product.Product.Status.Level == "Error" {
			logger := t.logger.
				WithField("productID", product.Product.Metadata.ID).
				WithField("productName", product.Product.Name)
			logger.Info("Preprocessing product")
			for _, productRelease := range product.ProductReleases {
				for _, plan := range productRelease.Plans {
					t.removeProductPlan(logger, plan)
				}
				t.deprecateCurrentProductRelease(logger, productRelease)
			}
			for _, plan := range product.PlansWithNoRelease {
				t.removeProductPlan(logger, plan)
			}
			statusErr := t.setProductStateToDraft(logger, product)
			if statusErr == nil {
				t.setProductReleaseTypeToManual(logger, product)
			}
		}
	}
}

func (t *productCatalog) PostProcessProductForAssetRepair() {
	for _, product := range t.Products {
		if product.Product.Status != nil && product.Product.Status.Level == "Error" {
			logger := t.logger.
				WithField("productID", product.Product.Metadata.ID).
				WithField("productName", product.Product.Name)

			t.reapplyAutoRelease(logger, product)
			if product.Product.State == catalog.ProductStateDRAFT {
				for _, plan := range product.PlansWithNoRelease {
					logger := logger.WithField("existingProductPlanName", plan.Plan.Name)
					newPlanRI, err := t.recreatePlan(logger, product, plan, nil)
					if err == nil {
						logger := logger.
							WithField("newProductPlanID", newPlanRI.Metadata.ID).
							WithField("newProductPlanName", newPlanRI.Name)
						logger.Infof("Recreated product plan")
						t.recreateQuota(logger, plan, newPlanRI)
					}
				}
				continue
			}
			releaseTagRI, err := t.createReleaseTag(logger, product)
			if err == nil {
				logger = logger.
					WithField("newReleasTagID", releaseTagRI.Metadata.ID).
					WithField("newReleaseTagName", releaseTagRI.Name)
				logger.Infof("Created new release tag")

				t.waitForProductRelease(releaseTagRI.Metadata.ID)

				lastProductRelease := t.getPreRepairLastRelease(product)
				for _, plan := range lastProductRelease.Plans {
					logger := logger.WithField("existingProductPlanName", plan.Plan.Name)
					newPlanRI, err := t.recreatePlan(logger, product, plan, releaseTagRI)
					if err == nil {
						logger := logger.
							WithField("newProductPlanID", newPlanRI.Metadata.ID).
							WithField("newProductPlanName", newPlanRI.Name)
						logger.Infof("Recreated product plan")
						quotaCreateError := t.recreateQuota(logger, plan, newPlanRI)
						if !quotaCreateError && plan.Plan.State == catalog.ProductPlanStateACTIVE {
							t.ActivateProductPlan(newPlanRI)
						}
					}
				}
				for _, productRelease := range product.ProductReleases {
					t.archiveCurrentProductRelease(logger, productRelease)
				}
			}
		}
	}
}

func (t *productCatalog) removeProductPlan(logger *logrus.Entry, plan PlanInfo) {
	logger = logger.
		WithField("planID", plan.Plan.Metadata.ID).
		WithField("planName", plan.Plan.Name)
	switch plan.Plan.State {
	case catalog.ProductPlanStateACTIVE:
		// deprecate the plan
		logger.Info("Deprecating Plan")
		if !t.dryRun {
			statusErr := t.apicClient.CreateSubResource(plan.Plan.ResourceMeta, map[string]interface{}{"state": catalog.ProductPlanStateDEPRECATED})
			if statusErr != nil {
				logger.WithError(statusErr).Error("error deprecating plan")
				break
			}
		}
		fallthrough
	case catalog.ProductPlanStateDEPRECATED:
		// archive the plan
		logger.Info("Archiving Plan")
		if !t.dryRun {
			statusErr := t.apicClient.CreateSubResource(plan.Plan.ResourceMeta, map[string]interface{}{"state": catalog.ProductPlanStateARCHIVED})
			if statusErr != nil {
				logger.WithError(statusErr).Error("error deprecating plan")
				break
			}
		}
		fallthrough
	case catalog.ProductPlanStateARCHIVED:
		fallthrough
	case catalog.ProductPlanStateDRAFT:
		//delete the plan
		logger.Info("Removing Plan")
		if !t.dryRun {
			statusErr := t.apicClient.DeleteResourceInstance(plan.Plan)
			if statusErr != nil {
				logger.WithError(statusErr).Error("error deleting plan")
				break
			}
		}
	}
}

func (t *productCatalog) archiveCurrentProductRelease(logger *logrus.Entry, productRelease ProductReleaseInfo) {
	releaseTag := productRelease.ReleaseTag
	logger = logger.
		WithField("productReleaseID", productRelease.ProductRelease.Metadata.ID).
		WithField("productReleaseName", productRelease.ProductRelease.Name).
		WithField("releaseTag", productRelease.ProductRelease.Spec.ReleaseTag)
	if productRelease.ProductRelease.Status != nil && productRelease.ProductRelease.Status.Level == "Error" {
		switch releaseTag.State.(string) {
		case string(catalog.ProductStateDEPRECATED):
			// archive the product release
			logger.Info("Archiving ProductRelease")
			if !t.dryRun {
				statusErr := t.apicClient.CreateSubResource(releaseTag.ResourceMeta, map[string]interface{}{"state": catalog.ProductStateARCHIVED})
				if statusErr != nil {
					logger.WithError(statusErr).Error("error deprecating plan")
					break
				}
			}
		}
	}
}

func (t *productCatalog) deprecateCurrentProductRelease(logger *logrus.Entry, productRelease ProductReleaseInfo) {
	releaseTag := productRelease.ReleaseTag
	logger = logger.
		WithField("productReleaseID", productRelease.ProductRelease.Metadata.ID).
		WithField("productReleaseName", productRelease.ProductRelease.Name).
		WithField("releaseTag", productRelease.ProductRelease.Spec.ReleaseTag)
	if productRelease.ProductRelease.Status != nil && productRelease.ProductRelease.Status.Level == "Error" {
		switch releaseTag.State.(string) {
		case string(catalog.ProductStateACTIVE):
			// deprecate the product release
			logger.Info("Deprecating ProductRelease")
			if !t.dryRun {
				statusErr := t.apicClient.CreateSubResource(releaseTag.ResourceMeta, map[string]interface{}{"state": catalog.ProductStateDEPRECATED})
				if statusErr != nil {
					logger.WithError(statusErr).Error("error deprecating plan")
					break
				}
			}
			releaseTag.State = string(catalog.ProductStateDEPRECATED)
		}
	}
}

func (t *productCatalog) setProductStateToDraft(logger *logrus.Entry, product ProductInfo) error {
	logger.Info("Setting product to draft")
	if !t.dryRun {
		statusErr := t.apicClient.CreateSubResource(product.Product.ResourceMeta, map[string]interface{}{"state": catalog.ProductStateDRAFT})
		if statusErr != nil {
			logger.WithError(statusErr).Errorf("unable to transition asset %s to draft", product.Product.Name)
			return statusErr
		}
	}
	return nil
}

func (t *productCatalog) setProductReleaseTypeToManual(logger *logrus.Entry, product ProductInfo) {
	logger.Info("Updating product release type to manual")
	p := catalog.NewProduct(product.Product.Name)
	p.Title = product.Product.Title
	p.Owner = product.Product.Owner
	p.Tags = product.Product.Tags
	p.Attributes = product.Product.Attributes
	p.Finalizers = product.Product.Finalizers
	p.Spec = product.Product.Spec
	p.Spec.AutoRelease = nil
	if !t.dryRun {
		_, err := t.apicClient.UpdateResourceInstance(p)
		if err != nil {
			logger.WithError(err).Errorf("unable to update asset %s for draft", product.Product.Name)
		}
	}
}

func (t *productCatalog) reapplyAutoRelease(logger *logrus.Entry, product ProductInfo) {
	if product.Product.Spec.AutoRelease != nil {
		logger = logger.WithField("originalReleaseType", product.Product.Spec.AutoRelease.ReleaseType)
	}
	logger.Info("Updating product release type to original release type")
	ri, _ := t.apicClient.GetResource(product.Product.GetSelfLink())
	p := catalog.NewProduct("")
	p.FromInstance(ri)
	p.ResourceMeta.Metadata.ResourceVersion = ""
	if product.Product.Spec.AutoRelease != nil {
		p.Spec.AutoRelease = &catalog.ProductSpecAutoRelease{
			ReleaseType: product.Product.Spec.AutoRelease.ReleaseType,
		}
	}
	if !t.dryRun {
		_, err := t.apicClient.UpdateResourceInstance(p)
		if err != nil {
			logger.WithError(err).Error("unable to update product auto release")
		}
	}
}

func (t *productCatalog) createReleaseTag(logger *logrus.Entry, product ProductInfo) (*v1.ResourceInstance, error) {
	logger.Infof("Creating new product release")
	releaseTag, _ := catalog.NewReleaseTag("", catalog.ProductGVK().Kind, product.Product.Name)
	releaseTag.Spec.ReleaseType = "patch"
	releaseTag.Title = product.Product.Title
	if t.dryRun {
		releaseTag.Name = "dry-run"
		return releaseTag.AsInstance()
	}

	releaseTagRI, err := t.apicClient.CreateResourceInstance(releaseTag)
	if err != nil {
		logger.WithError(err).Errorf("unable to create new release tag for product:%s", product.Product.Name)
		return nil, err
	}
	return releaseTagRI, err
}

func (t *productCatalog) waitForProductRelease(releaseTagID string) {
	if t.dryRun {
		return
	}
	n := 0
	for newProductRelease := t.getProductReleaseForReleaseTag(releaseTagID); newProductRelease == nil && n < 5; {
		newProductRelease = t.getProductReleaseForReleaseTag(releaseTagID)
		n++
		time.Sleep(time.Second)
	}
}
func (t *productCatalog) getPreRepairLastRelease(product ProductInfo) ProductReleaseInfo {
	lastProductRelease := ProductReleaseInfo{}
	for _, productRelease := range product.ProductReleases {
		if lastProductRelease.ProductRelease == nil {
			lastProductRelease = productRelease
		} else {
			lastReleaseTime := lastProductRelease.ProductRelease.Metadata.Audit.CreateTimestamp
			productReleaseTime := productRelease.ProductRelease.Metadata.Audit.CreateTimestamp
			if time.Time(lastReleaseTime).Before(time.Time(productReleaseTime)) {
				lastProductRelease = productRelease
			}
		}
	}
	return lastProductRelease
}

func (t *productCatalog) recreatePlan(logger *logrus.Entry, product ProductInfo, plan PlanInfo, releaseTagRI *v1.ResourceInstance) (*v1.ResourceInstance, error) {
	logger.Infof("Recreating product plan")
	newPlan := catalog.NewProductPlan("")
	newPlan.Title = plan.Plan.Title
	newPlan.Tags = plan.Plan.Tags
	newPlan.Attributes = plan.Plan.Attributes
	newPlan.Spec = plan.Plan.Spec
	newPlan.Owner = plan.Plan.Owner
	if releaseTagRI != nil {
		newPlan.References = &catalog.ProductPlanReferences{
			Product: catalog.ProductPlanReferencesProduct{
				Release: releaseTagRI.Name,
			},
		}
	}
	if t.dryRun {
		newPlan.Name = "dry-run"
		return newPlan.AsInstance()
	}
	newPlanRI, err := t.apicClient.CreateResourceInstance(newPlan)
	if err != nil {
		logger.WithError(err).Error("unable to recreate product plan")
		return nil, err
	}
	return newPlanRI, nil
}

func (t *productCatalog) recreateQuota(logger *logrus.Entry, existingPlanInfo PlanInfo, newPlanRI *v1.ResourceInstance) bool {
	quotaCreateError := false
	logger.Info("Recreating quota for plan")
	if t.dryRun {
		return false
	}
	for _, quota := range existingPlanInfo.Quotas {
		newQuota := catalog.NewQuota("", newPlanRI.Name)
		newQuota.Title = quota.Quota.Title
		newQuota.Tags = quota.Quota.Tags
		newQuota.Attributes = quota.Quota.Attributes
		newQuota.Spec = t.recreateQuotaSpec(logger, quota.Quota.Spec)
		newQuota.Owner = quota.Quota.Owner
		if len(newQuota.Spec.Resources) == 0 {
			logger.
				WithField("quotaName", quota.Quota.Name).
				WithField("planName", newPlanRI.Name).
				Error("unable to recreate quota, no asset resources were found")
			return false
		}
		newQuotaRI, err := t.apicClient.CreateResourceInstance(newQuota)
		if err != nil {
			logger.
				WithField("quotaName", quota.Quota.Name).
				WithField("planName", newPlanRI.Name).
				WithError(err).
				Errorf("unable to recreate quota")
			quotaCreateError = true
		} else {
			t.logger.Infof("Recreated quota id:%s, name: %s, plan: %s",
				newQuotaRI.Metadata.ID,
				newQuotaRI.Name,
				newPlanRI.Name)
		}
	}
	return quotaCreateError
}

func (t *productCatalog) recreateQuotaSpec(logger *logrus.Entry, existingQuotaSpec catalog.QuotaSpec) catalog.QuotaSpec {
	quotaSpec := catalog.QuotaSpec{
		Description: existingQuotaSpec.Description,
		Unit:        existingQuotaSpec.Unit,
		Pricing:     existingQuotaSpec.Pricing,
	}
	quotaResources := make([]interface{}, 0)
	for _, quotaResource := range existingQuotaSpec.Resources {
		buf, _ := json.Marshal(quotaResource)
		qar := &catalog.QuotaSpecAssetResourceRef{}
		json.Unmarshal(buf, qar)
		if qar.Kind == "AssetResource" {
			assetResName := t.assetCatalog.FindAssetResource(logger, qar.Name)
			if assetResName == "" {
				logger.WithField("assetName", qar.Name).Warn("missing asset resource")
				continue
			}
		}
		quotaResources = append(quotaResources, qar)
	}
	quotaSpec.Resources = quotaResources
	return quotaSpec
}

func (t *productCatalog) ActivateProductPlan(plan v1.Interface) {
	planRI, _ := plan.AsInstance()
	t.logger.Infof("Activating Product plan: %s", planRI.Title)
	if !t.dryRun {
		statusErr := t.apicClient.CreateSubResource(planRI.ResourceMeta, map[string]interface{}{"state": catalog.ProductPlanStateACTIVE})
		if statusErr != nil {
			t.logger.WithError(statusErr).Error("error activating plan")
		}
	}
}
