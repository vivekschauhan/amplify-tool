package tools

import (
	"github.com/Axway/agent-sdk/pkg/apic"
	"github.com/Axway/agent-sdk/pkg/apic/auth"
	"github.com/Axway/agent-sdk/pkg/config"
)

// Config the configuration for the Watch client
type Config struct {
	OrgID       string      `mapstructure:"org_id"`
	Region      string      `mapstructure:"region"`
	URL         string      `mapstructure:"url"`
	PlatformURL string      `mapstructure:"platform_url"`
	Port        uint32      `mapstructure:"port"`
	Auth        auth.Config `mapstructure:"auth"`
	Level       string      `mapstructure:"log_level"`
	Format      string      `mapstructure:"log_format"`
}

func CreateAPICClient(cfg *Config) apic.Client {
	c := config.NewCentralConfig(config.GenericService)
	centralCfg, _ := c.(*config.CentralConfiguration)
	centralCfg.URL = cfg.URL
	centralCfg.PlatformURL = cfg.PlatformURL
	acfg := centralCfg.GetAuthConfig()
	authCfg, _ := acfg.(*config.AuthConfiguration)
	authCfg.ClientID = cfg.Auth.ClientID
	authCfg.PrivateKey = cfg.Auth.PrivateKey
	authCfg.PublicKey = cfg.Auth.PublicKey
	authCfg.KeyPwd = cfg.Auth.KeyPassword
	authCfg.URL = cfg.Auth.URL
	authCfg.Timeout = cfg.Auth.Timeout
	authCfg.Realm = "Broker"

	tokenGetter := auth.NewPlatformTokenGetterWithCentralConfig(centralCfg)
	return apic.New(centralCfg, tokenGetter, nil)
}
