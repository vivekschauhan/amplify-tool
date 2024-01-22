package cmd

import (
	"github.com/vivekschauhan/amplify-tool/pkg/tools/export"

	"github.com/spf13/cobra"
)

var exportCfg = &export.Config{}

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Amplify Export Tool",
		Version: "0.0.1",
		RunE:    runExport,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			v, err := initViperConfig(cmd)
			if err != nil {
				return err
			}
			err = v.Unmarshal(exportCfg)
			if err != nil {
				return err
			}

			exportCfg.Config = *cfg
			return nil
		},
	}

	initExportCmdFlags(cmd)

	return cmd
}

func initExportCmdFlags(cmd *cobra.Command) {
	baseFlags(cmd)
	cmd.Flags().String("environment", "", "The environment name to export")
	cmd.Flags().String("out_file", "export.json", "The name of the file to save to")
}

func runExport(_ *cobra.Command, _ []string) error {
	tool := export.NewTool(exportCfg)
	return tool.Run()
}
