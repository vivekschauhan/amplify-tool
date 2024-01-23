package dupes

import (
	"github.com/vivekschauhan/amplify-tool/pkg/tools"
)

// Config the configuration for the Watch client
type Config struct {
	tools.Config
	OutFile string `mapstructure:"out_file"`
}
