## Overview

Tool for Amplify Service Registry, Asset and Product catalog

## Usage

```
./amplify-tool help repairAsset

Amplify Service Registry, Asset, and Product Repair Asset Tool

Usage:
   repairAsset [flags]

Flags:
      --auth.client_id string      The service account client ID
      --auth.key_password string   The password for private key
      --auth.private_key string    The private key associated with service account(default : ./private_key.pem) (default "./private_key.pem")
      --auth.public_key string     The public key associated with service account(default : ./public_key.pem) (default "./public_key.pem")
      --auth.timeout duration      The connection timeout for AxwayID (default 10s)
      --auth.url string            The AxwayID auth URL (default "https://login.axway.com/auth")
  -h, --help                       help for repairAsset
      --log_format string          line or json (default "json")
      --log_level string           log level (default "info")
      --platform_url string        The platform URL (default "https://platform.axway.com")
      --tenant_id string           The Amplify org ID
      --url string                 The central URL (default "https://apicentral.axway.com")
  -v, --version                    version for repairAsset
  ```