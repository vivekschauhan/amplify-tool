package cmd

import (
	"github.com/vivekschauhan/amplify-tool/pkg/tools/product"

	"github.com/spf13/cobra"
)

var prodCfg = &product.Config{}

func newRepairProductCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repairProduct",
		Short:   "Amplify Repair Product Tool",
		Version: "0.0.2",
		RunE:    runRepairProduct,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			v, err := initViperConfig(cmd)
			if err != nil {
				return err
			}

			err = v.Unmarshal(prodCfg)
			if err != nil {
				return err
			}

			prodCfg.Config = *cfg
			return nil
		},
	}

	initRepairProductCmdFlags(cmd)

	return cmd
}

func initRepairProductCmdFlags(cmd *cobra.Command) {
	baseFlags(cmd)
	cmd.Flags().String("service_mapping_file", "", "The path of the service mapping file")
	cmd.Flags().String("product_catalog_file", "", "The path of the product-catalog.json")
	cmd.Flags().Bool("dry_run", false, "Run the tool with no update(true/false)")
}

func runRepairProduct(_ *cobra.Command, _ []string) error {
	tool := product.NewTool(prodCfg)
	return tool.Run()
}
