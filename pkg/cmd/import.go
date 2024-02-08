package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "import",
		Short:   "Amplify Import Tool",
		Version: "0.0.1",
		RunE:    runImport,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	return cmd
}

func runImport(_ *cobra.Command, _ []string) error {
	fmt.Printf("To import an export use the axway cli command `axway central applly -f export-file.json`")
	return nil
}
