package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig 从文件加载配置
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	return &cfg, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg *Config, configPath string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Monitor: MonitorConfig{
			Interface:      "",
			ExcludeSubnets: "",
			Window:         30,
			Timeout:        60 * 60 * 24, // 24小时
		},
		Firewall: FirewallConfig{
			Chain: "NETBOUNCER",
		},
		Web: WebConfig{
			Listen: "0.0.0.0:8080",
		},
		Storage: StorageConfig{
			Type: "database",
			Database: DatabaseConfig{
				Driver:   "sqlite",
				Host:     "",
				Port:     0,
				Username: "",
				Password: "",
				Database: "netbouncer.db",
				DSN:      "",
			},
		},
		Debug: false,
	}
}
