package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/Axway/agent-sdk/pkg/cmd"
	"github.com/Axway/agent-sdk/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/vivekschauhan/amplify-tool/pkg/tools"
)

var cfg = &tools.Config{}

// NewRootCmd creates a new cobra.Command
func NewRootCmd() *cobra.Command {
	config.AgentTypeName = cmd.BuildAgentName
	config.AgentVersion = cmd.BuildVersion
	config.AgentDataPlaneType = cmd.BuildDataPlaneType
	config.SDKVersion = cmd.SDKBuildVersion

	rootCmd := &cobra.Command{Use: ""}
	rootCmd.AddCommand(newRepairCmd())
	rootCmd.AddCommand(newRepairProductCmd())
	rootCmd.AddCommand(newDuplicateCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newImportCmd())
	rootCmd.AddCommand(newMetricCmd())
	return rootCmd
}

func initViperConfig(cmd *cobra.Command) (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindFlagsToViperConfig(cmd, v)
	err := v.Unmarshal(cfg)
	if err != nil {
		return nil, err
	}

	return v, nil
}

// bindFlagsToViperConfig - For each flag, look up its corresponding env var, and use the env var if the flag is not set.
func bindFlagsToViperConfig(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		name := strings.ToUpper(f.Name)
		if err := v.BindPFlag(name, f); err != nil {
			panic(err)
		}

		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
			if err != nil {
				panic(err)
			}
		}
	})
}

func baseFlags(cmd *cobra.Command) {
	cmd.Flags().String("org_id", "", "The Amplify org ID")
	cmd.MarkFlagRequired("org_id")
	cmd.Flags().String("region", "us", "The central region (us, eu, apac)")
	cmd.Flags().String("url", "", "The central URL")
	cmd.Flags().String("platform_url", "", "The platform URL")
	cmd.Flags().String("traceability_host", "", "The traceability host to use for uploading metrics")
	cmd.Flags().String("auth.private_key", "./private_key.pem", "The private key associated with service account(default : ./private_key.pem)")
	cmd.Flags().String("auth.public_key", "./public_key.pem", "The public key associated with service account(default : ./public_key.pem)")
	cmd.Flags().String("auth.key_password", "", "The password for private key")
	cmd.Flags().String("auth.url", "", "The AxwayID auth URL")
	cmd.Flags().String("auth.client_id", "", "The service account client ID")
	cmd.MarkFlagRequired("auth.client_id")
	cmd.Flags().Duration("auth.timeout", 10*time.Second, "The connection timeout for AxwayID")
	cmd.Flags().String("log_level", "info", "log level")
	cmd.Flags().String("log_format", "json", "line or json")
	cmd.Flags().Bool("dry_run", false, "Run the tool with no update(true/false)")
}
