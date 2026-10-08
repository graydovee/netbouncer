package cmd

import (
	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/spf13/cobra"
	"testing"
)

func TestMergeConfigAndExplicitIntegerZero(t *testing.T) {
	defaults := config.DefaultConfig()
	file := config.DefaultConfig()
	file.Monitor.HistoryInterval = 0
	file.Policy.EvalInterval = 0
	merged, err := mergeConfig(defaults, file)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Monitor.HistoryInterval != 0 || merged.Policy.EvalInterval != 0 {
		t.Fatal("file zero overwritten by defaults")
	}
	command := &cobra.Command{}
	command.Flags().Int("interval", 60, "")
	if err = command.Flags().Set("interval", "0"); err != nil {
		t.Fatal(err)
	}
	merged.Monitor.HistoryInterval = 60
	applyExplicitFlags(command, merged, map[string]func(*config.Config, string) error{"interval": func(c *config.Config, v string) error {
		return parseIntFlag(v, func(n int) { c.Monitor.HistoryInterval = n })
	}})
	if merged.Monitor.HistoryInterval != 0 {
		t.Fatal("integer CLI zero ignored")
	}
}
