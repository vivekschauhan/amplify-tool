package tools

import (
	"strings"

	"github.com/Axway/agent-sdk/pkg/apic"
	"github.com/Axway/agent-sdk/pkg/apic/auth"
	"github.com/Axway/agent-sdk/pkg/config"
)

// Config the configuration for the Watch client
type Config struct {
	OrgID            string      `mapstructure:"org_id"`
	Region           string      `mapstructure:"region"`
	URL              string      `mapstructure:"url"`
	Port             uint32      `mapstructure:"port"`
	Auth             auth.Config `mapstructure:"auth"`
	Level            string      `mapstructure:"log_level"`
	Format           string      `mapstructure:"log_format"`
	DryRun           bool        `mapstructure:"dry_run"`
	SingleURL        string      `mapstructure:"single_url"`
	TraceabilityHost string      `mapstructure:"traceability_host"`
	PlatformURL      string      `mapstructure:"platform_url"`
}

var nameToRegionMap = map[string]config.Region{
	"US": config.US,
	"EU": config.EU,
	"AP": config.AP,
}

type regionalSettings struct {
	SingleURL        string
	CentralURL       string
	AuthURL          string
	PlatformURL      string
	TraceabilityHost string
	Deployment       string
}

var regionalSettingsMap = map[config.Region]regionalSettings{
	config.US: {
		SingleURL:        "https://ingestion.platform.axway.com",
		CentralURL:       "https://apicentral.axway.com",
		AuthURL:          "https://login.axway.com/auth",
		PlatformURL:      "https://platform.axway.com",
		TraceabilityHost: "ingestion.datasearch.axway.com:5044",
		Deployment:       "prod",
	},
	config.EU: {
		SingleURL:        "https://ingestion-eu.platform.axway.com",
		CentralURL:       "https://central.eu-fr.axway.com",
		AuthURL:          "https://login.axway.com/auth",
		PlatformURL:      "https://platform.axway.com",
		TraceabilityHost: "ingestion.visibility.eu-fr.axway.com:5044",
		Deployment:       "prod-eu",
	},
	config.AP: {
		SingleURL:        "https://ingestion-ap-sg.platform.axway.com",
		CentralURL:       "https://central.ap-sg.axway.com",
		AuthURL:          "https://login.axway.com/auth",
		PlatformURL:      "https://platform.axway.com",
		TraceabilityHost: "ingestion.visibility.ap-sg.axway.com:5044",
		Deployment:       "prod-ap",
	},
}

func CreateAPICClient(cfg *Config) (apic.Client, auth.PlatformTokenGetter) {
	c := config.NewCentralConfig(config.GenericService)
	centralCfg, _ := c.(*config.CentralConfiguration)
	centralCfg.Region = nameToRegionMap[strings.ToUpper(cfg.Region)]
	urls := regionalSettingsMap[centralCfg.GetRegion()]
	if cfg.SingleURL == "" {
		cfg.SingleURL = urls.SingleURL
		centralCfg.SingleURL = urls.SingleURL
	}
	if cfg.URL == "" {
		cfg.URL = urls.CentralURL
		centralCfg.URL = urls.CentralURL
	}
	if cfg.PlatformURL == "" {
		cfg.PlatformURL = urls.PlatformURL
		centralCfg.PlatformURL = urls.PlatformURL
	}
	if cfg.TraceabilityHost == "" {
		// convert to http
		cfg.TraceabilityHost = strings.Split(urls.TraceabilityHost, ":")[0] + ":443"
	}
	acfg := centralCfg.GetAuthConfig()
	authCfg, _ := acfg.(*config.AuthConfiguration)
	authCfg.ClientID = cfg.Auth.ClientID
	authCfg.PrivateKey = cfg.Auth.PrivateKey
	authCfg.PublicKey = cfg.Auth.PublicKey
	authCfg.KeyPwd = cfg.Auth.KeyPassword
	if cfg.Auth.URL == "" {
		cfg.Auth.URL = urls.AuthURL
	}
	authCfg.URL = cfg.Auth.URL
	authCfg.Timeout = cfg.Auth.Timeout
	authCfg.Realm = "Broker"

	tokenGetter := auth.NewPlatformTokenGetterWithCentralConfig(centralCfg)
	return apic.New(centralCfg, tokenGetter, nil), tokenGetter
}
