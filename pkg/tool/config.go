package tool

import (
	"github.com/Axway/agent-sdk/pkg/apic/auth"
)

// Config the configuration for the Watch client
type Config struct {
	TenantID    string      `mapstructure:"tenant_id"`
	URL         string      `mapstructure:"url"`
	PlatformURL string      `mapstructure:"platform_url"`
	Port        uint32      `mapstructure:"port"`
	Auth        auth.Config `mapstructure:"auth"`
	Level       string      `mapstructure:"log_level"`
	Format      string      `mapstructure:"log_format"`
}
