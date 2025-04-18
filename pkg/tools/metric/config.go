package metric

import "github.com/vivekschauhan/amplify-tool/pkg/tools"

// Config the configuration for the Watch client
type Config struct {
	tools.Config
	EnvironmentID    string `mapstructure:"environment_id"`
	MetricCacheFile  string `mapstructure:"metric_cache_file"`
	UsageProduct     string `mapstructure:"usage_product"`
	SkipMetricUpload bool   `mapstructure:"skip_upload_metrics"`
	SkipUsageUpload  bool   `mapstructure:"skip_upload_usage"`
}
