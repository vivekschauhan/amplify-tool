package cmd

import (
	"fmt"
	"strings"

	"github.com/Axway/agent-sdk/pkg/cmd"
	"github.com/Axway/agent-sdk/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// NewRootCmd creates a new cobra.Command
func NewRootCmd() *cobra.Command {
	config.AgentTypeName = cmd.BuildAgentName
	config.AgentVersion = cmd.BuildVersion
	config.AgentDataPlaneType = cmd.BuildDataPlaneType
	config.SDKVersion = cmd.SDKBuildVersion

	rootCmd := &cobra.Command{Use: ""}
	rootCmd.AddCommand(newRepairCmd())
	return rootCmd
}

func initViperConfig(cmd *cobra.Command) error {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindFlagsToViperConfig(cmd, v)

	err := v.Unmarshal(cfg)
	if err != nil {
		return err
	}

	return nil
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
