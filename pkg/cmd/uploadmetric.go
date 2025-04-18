package cmd

import (
	"github.com/vivekschauhan/amplify-tool/pkg/tools/metric"

	"github.com/spf13/cobra"
)

var metricCfg = &metric.Config{}

func newMetricCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "uploadMetrics",
		Short:   "Amplify Cached Metic Upload Tool",
		Version: "0.0.2",
		RunE:    runUploadMetrics,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			v, err := initViperConfig(cmd)
			if err != nil {
				return err
			}

			err = v.Unmarshal(metricCfg)
			if err != nil {
				return err
			}

			metricCfg.Config = *cfg
			return nil
		},
	}

	initMetricCmdFlags(cmd)

	return cmd
}

func initMetricCmdFlags(cmd *cobra.Command) {
	baseFlags(cmd)
	cmd.Flags().String("metric_cache_file", "", "The path of the metric cache file created by the agent")
	cmd.Flags().Bool("skip_upload_metrics", false, "Set if the tool should skip uploading metrics")
	cmd.Flags().Bool("skip_upload_usage", false, "Set if the tool should skip uploading usage details")
	cmd.Flags().String("usage_product", "", "Set the product name to use with the Usage Report")
	cmd.Flags().String("environment_id", "", "Set the environment id to use with the Usage Report")
}

func runUploadMetrics(_ *cobra.Command, _ []string) error {
	tool := metric.NewTool(metricCfg)
	return tool.Run()
}
