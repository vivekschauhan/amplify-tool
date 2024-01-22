package export

import "github.com/vivekschauhan/amplify-tool/pkg/tools"

// Config the configuration for the Watch client
type Config struct {
	tools.Config
	Environment string `mapstructure:"environment"`
	OutFile     string `mapstructure:"out_file"`
}
