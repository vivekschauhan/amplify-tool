package cmd

import (
	"github.com/vivekschauhan/amplify-tool/pkg/tools/asset"

	"github.com/spf13/cobra"
)

var assetCfg = &asset.Config{}

func newRepairCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "repairAsset",
		Short:   "Amplify Repair Asset Tool",
		Version: "0.0.2",
		RunE:    runRepairAsset,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			v, err := initViperConfig(cmd)
			if err != nil {
				return err
			}

			err = v.Unmarshal(assetCfg)
			if err != nil {
				return err
			}

			assetCfg.Config = *cfg
			return nil
		},
	}

	initRepairCmdFlags(cmd)

	return cmd
}

func initRepairCmdFlags(cmd *cobra.Command) {
	baseFlags(cmd)
	cmd.Flags().String("service_mapping_file", "", "The path of the service mapping file")
	cmd.Flags().String("product_catalog_file", "", "The path of the product-catalog.json")
}

func runRepairAsset(_ *cobra.Command, _ []string) error {
	tool := asset.NewTool(assetCfg)
	return tool.Run()
}
