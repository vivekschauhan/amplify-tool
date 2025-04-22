package metric

import "github.com/vivekschauhan/amplify-tool/pkg/tools"

// Config the configuration for the Watch client
type Config struct {
	tools.Config
	EnvironmentID    string `mapstructure:"environment_id"`
	MetricCacheFile  string `mapstructure:"metric_cache_file"`
	UsageProduct     string `mapstructure:"usage_product"`
	AgentName        string `mapstructure:"agent_name"`
	AgentVersion     string `mapstructure:"agent_version"`
	AgentSDKVersion  string `mapstructure:"agent_sdk_version"`
	AgentType        string `mapstructure:"agent_type"`
	SkipMetricUpload bool   `mapstructure:"skip_upload_metrics"`
	SkipUsageUpload  bool   `mapstructure:"skip_upload_usage"`
	BatchSize        int    `mapstructure:"batch_size"`
}
