package cmd

import (
	"time"

	"github.com/vivekschauhan/amplify-tool/pkg/tool"

	"github.com/spf13/cobra"
)

var cfg = &tool.Config{}

func newRepairCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "repairAsset",
		Short:   "Amplify Repair Asset Tool",
		Version: "0.0.1",
		RunE:    runRepairAsset,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initViperConfig(cmd)
		},
	}

	initRepairCmdFlags(cmd)

	return cmd
}

func initRepairCmdFlags(cmd *cobra.Command) {
	cmd.Flags().String("tenant_id", "", "The Amplify org ID")
	cmd.MarkFlagRequired("tenant_id")
	cmd.Flags().String("url", "https://apicentral.axway.com", "The central URL")
	cmd.Flags().String("platform_url", "https://platform.axway.com", "The platform URL")

	cmd.Flags().String("auth.private_key", "./private_key.pem", "The private key associated with service account(default : ./private_key.pem)")
	cmd.Flags().String("auth.public_key", "./public_key.pem", "The public key associated with service account(default : ./public_key.pem)")
	cmd.Flags().String("auth.key_password", "", "The password for private key")
	cmd.Flags().String("auth.url", "https://login.axway.com/auth", "The AxwayID auth URL")
	cmd.Flags().String("auth.client_id", "", "The service account client ID")
	cmd.MarkFlagRequired("auth.client_id")
	cmd.Flags().Duration("auth.timeout", 10*time.Second, "The connection timeout for AxwayID")

	cmd.Flags().String("log_level", "info", "log level")
	cmd.Flags().String("log_format", "json", "line or json")
}

func runRepairAsset(_ *cobra.Command, _ []string) error {
	tool := tool.NewTool(cfg)
	return tool.Run()
}
