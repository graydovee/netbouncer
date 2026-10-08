package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigKeepsDefaultsAndExplicitDisabledHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("monitor:\n  history_interval: 0\npolicy:\n  eval_interval: 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Monitor.HistoryInterval != 0 || cfg.Policy.EvalInterval != 0 {
		t.Fatal("explicit disable lost")
	}
	if cfg.Database.Driver != "sqlite" || cfg.History.BudgetBytes != 2<<30 || cfg.Monitor.HistoryRetentionDays != 30 {
		t.Fatalf("defaults missing %+v", cfg)
	}
	if err = cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
