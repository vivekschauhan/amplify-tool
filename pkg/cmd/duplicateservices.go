package cmd

import (
	"github.com/vivekschauhan/amplify-tool/pkg/tools/dupes"

	"github.com/spf13/cobra"
)

var dupeCfg = &dupes.Config{}

func newDuplicateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "duplicate",
		Short:   "Amplify Duplicate Repair Tool",
		Version: "0.0.1",
		RunE:    runDeduplicate,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			v, err := initViperConfig(cmd)
			if err != nil {
				return err
			}
			err = v.Unmarshal(dupeCfg)
			if err != nil {
				return err
			}

			dupeCfg.Config = *cfg
			return nil
		},
	}

	initDuplicateCmdFlags(cmd)

	return cmd
}

func initDuplicateCmdFlags(cmd *cobra.Command) {
	baseFlags(cmd)
	cmd.Flags().Bool("dry_run", false, "Run the tool with no update(true/false)")
}

func runDeduplicate(_ *cobra.Command, _ []string) error {
	tool := dupes.NewTool(dupeCfg)
	return tool.Run()
}
