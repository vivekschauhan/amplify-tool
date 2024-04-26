## Overview

Tool for Amplify Service Registry, Asset and Product catalog

## Usage

```
./amplify-tool help
Usage:
   [command]

Available Commands:
  completion    Generate the autocompletion script for the specified shell
  duplicate     Amplify Duplicate Repair Tool
  export        Amplify Export Tool
  help          Help about any command
  import        Amplify Import Tool
  repairAsset   Amplify Repair Asset Tool
  repairProduct Amplify Repair Product Tool

Flags:
  -h, --help   help for this command

Use " [command] --help" for more information about a command.
```

### repairAsset

```
./amplify-tool help repairAsset
Amplify Repair Asset Tool

Usage:
   repairAsset [flags]

Flags:
      --auth.client_id string         The service account client ID
      --auth.key_password string      The password for private key
      --auth.private_key string       The private key associated with service account(default : ./private_key.pem) (default "./private_key.pem")
      --auth.public_key string        The public key associated with service account(default : ./public_key.pem) (default "./public_key.pem")
      --auth.timeout duration         The connection timeout for AxwayID (default 10s)
      --auth.url string               The AxwayID auth URL
      --dry_run                       Run the tool with no update(true/false)
  -h, --help                          help for repairAsset
      --log_format string             line or json (default "json")
      --log_level string              log level (default "info")
      --org_id string                 The Amplify org ID
      --platform_url string           The platform URL
      --product_catalog_file string   The path of the product-catalog.json
      --region string                 The central region (us, eu, apac) (default "us")
      --service_mapping_file string   The path of the service mapping file
      --url string                    The central URL
  -v, --version                       version for repairAsset
```

### duplicate

```
./amplify-tool help duplicate 
Amplify Duplicate Repair Tool

Usage:
   duplicate [flags]

Flags:
      --auth.client_id string      The service account client ID
      --auth.key_password string   The password for private key
      --auth.private_key string    The private key associated with service account(default : ./private_key.pem) (default "./private_key.pem")
      --auth.public_key string     The public key associated with service account(default : ./public_key.pem) (default "./public_key.pem")
      --auth.timeout duration      The connection timeout for AxwayID (default 10s)
      --auth.url string            The AxwayID auth URL
      --backup_file string         The name of the file to backup to, not created in dry runs
      --dry_run                    Run the tool with no update(true/false)
      --environments string        The environments to run the deduplication against, comma separated
  -h, --help                       help for duplicate
      --log_format string          line or json (default "json")
      --log_level string           log level (default "info")
      --org_id string              The Amplify org ID
      --out_file string            The name of the file to save to
      --platform_url string        The platform URL
      --region string              The central region (us, eu, apac) (default "us")
      --url string                 The central URL
  -v, --version                    version for duplicate
```

When running the duplicate tool the output will be a file names `actions.log`. In the file the tool will group services it thinks are duplicates. Based on other resources found the tool will output information about this group and actions it feels are safe to execute. *No actions are taken by the tool.*

The tool follows the following process:

* Gather all services, revisions, and instances in an environment
* Gather all assets created in the system
* For all services group them based off the External API ID found on the related API Service Instance
* For each grouping determine the number of assets that each service is referenced in
* Output an action based off the number of services in a group that are referenced in assets

When running this tool follow the steps below.

1. Execute the duplicate detect tool
  * Optionally use the `backup_file` option to save the resources being referneced in the actions to a backup file
2. Review the output file and remove commands that should not be executed and save the updated file
3. Stop all agents and clean up their persistent cache, if in use
4. Run commands in the reviewed actions file (*NOTE: These actions can not be undone!!!!*)
5. Restart your agents
