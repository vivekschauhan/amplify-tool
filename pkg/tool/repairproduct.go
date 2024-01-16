package tool

func (t *tool) RunRepairProduct() error {
	t.logger.Info("Amplify Asset Tool")
	t.assetCatalog.ReadAssets(true)
	t.productCatalog.ReadProducts()
	t.productCatalog.RepairProductWithBackup()
	return nil
}
