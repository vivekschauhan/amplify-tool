package asset

import "github.com/vivekschauhan/amplify-tool/pkg/tools"

// Config the configuration for the Watch client
type Config struct {
	tools.Config
	ServiceMappingFile string `mapstructure:"service_mapping_file"`
	ProductCatalogFile string `mapstructure:"product_catalog_file"`
}
