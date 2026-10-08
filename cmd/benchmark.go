package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/graydovee/netbouncer/pkg/web"
	"github.com/spf13/cobra"
)

func init() {
	var parent string
	command := &cobra.Command{Use: "benchmark", Short: "Run isolated authenticated performance checks (synthetic data; no firewall changes)", RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Minute)
		defer cancel()
		report, err := web.RunBenchmark(ctx, parent)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err = encoder.Encode(report); err != nil {
			return err
		}
		if !report.Passed {
			return fmt.Errorf("performance budget exceeded")
		}
		return nil
	}}
	command.Flags().StringVar(&parent, "temp-dir", os.TempDir(), "Parent directory for the disposable fixture (needs approximately 1GiB free plus 2GiB reserve)")
	rootCmd.AddCommand(command)
}
