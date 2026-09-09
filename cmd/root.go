/*
Copyright © 2025 graydovee
*/
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"dario.cat/mergo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/graydovee/netbouncer/pkg/store"
	"github.com/graydovee/netbouncer/pkg/web"
)

var (
	cfg        = config.DefaultConfig()
	configFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "netbouncer",
	Short: "网络流量监控工具",
	Long:  `netbouncer 是一个网络流量监控工具，支持Web页面实时查看流量统计。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func init() {
	// 配置文件参数
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "配置文件路径 (YAML格式)")

	// 监控配置
	rootCmd.Flags().StringVarP(&cfg.Monitor.Interface, "monitor-interface", "i", cfg.Monitor.Interface, "网络接口名称（留空自动选择）")
	rootCmd.Flags().StringVarP(&cfg.Monitor.ExcludeSubnets, "monitor-exclude-subnets", "e", cfg.Monitor.ExcludeSubnets, "排除的子网（逗号分隔，如：127.0.0.1/8,192.168.0.0/16）")
	rootCmd.Flags().IntVarP(&cfg.Monitor.Window, "monitor-window", "w", cfg.Monitor.Window, "监控时间窗口（秒）")
	rootCmd.Flags().IntVarP(&cfg.Monitor.Timeout, "monitor-timeout", "t", cfg.Monitor.Timeout, "连接超时时间（秒）")

	// 防火墙配置
	rootCmd.Flags().StringVarP(&cfg.Firewall.Chain, "firewall-chain", "n", cfg.Firewall.Chain, "iptables链名称")
	rootCmd.Flags().StringVarP(&cfg.Firewall.IpSet, "firewall-ipset", "p", cfg.Firewall.IpSet, "ipset名称")
	rootCmd.Flags().StringVarP(&cfg.Firewall.Type, "firewall-type", "f", cfg.Firewall.Type, "防火墙类型 (iptables|ipset|mock)")

	// Web配置
	rootCmd.Flags().StringVarP(&cfg.Web.Listen, "listen", "l", cfg.Web.Listen, "Web服务监听地址")

	// 数据库配置
	rootCmd.Flags().StringVar(&cfg.Database.Driver, "db-driver", cfg.Database.Driver, "数据库驱动 (sqlite|mysql|postgres)")
	rootCmd.Flags().StringVar(&cfg.Database.Host, "db-host", cfg.Database.Host, "数据库主机地址")
	rootCmd.Flags().IntVar(&cfg.Database.Port, "db-port", cfg.Database.Port, "数据库端口号")
	rootCmd.Flags().StringVar(&cfg.Database.Username, "db-username", cfg.Database.Username, "数据库用户名")
	rootCmd.Flags().StringVar(&cfg.Database.Password, "db-password", cfg.Database.Password, "数据库密码")
	rootCmd.Flags().StringVar(&cfg.Database.Database, "db-name", cfg.Database.Database, "数据库名称或文件路径")
	rootCmd.Flags().StringVar(&cfg.Database.DSN, "db-dsn", cfg.Database.DSN, "数据库连接字符串")
	rootCmd.Flags().StringVar(&cfg.Database.LogLevel, "db-log-level", cfg.Database.LogLevel, "SQL日志级别 (silent|error|warn|info)")

	// 添加使用示例
	rootCmd.Example = `  # 使用默认配置启动（ipset模式）
  netbouncer

  # 生成默认配置文件
  netbouncer config generate

  # 使用配置文件启动
  netbouncer -c config.yaml

  # 指定网络接口和监听地址
  netbouncer -i eth0 -l 0.0.0.0:9090

  # 使用iptables防火墙模式
  netbouncer -f iptables

  # 使用mock模式（调试用）
  netbouncer -f mock

  # 排除特定子网
  netbouncer -e "127.0.0.1/8,192.168.0.0/16"

  # 使用MySQL数据库
  netbouncer --db-driver mysql --db-host localhost --db-name netbouncer`
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// applyExplicitFlags 将用户显式传入的命令行参数重新应用到配置上，
// 保证优先级为：默认值 < 配置文件 < 显式命令行参数
func applyExplicitFlags(cmd *cobra.Command, cfg *config.Config, bindings map[string]func(*config.Config, string) error) {
	var explicit []string
	cmd.Flags().Visit(func(f *pflag.Flag) {
		explicit = append(explicit, f.Name)
	})

	for _, name := range explicit {
		apply, ok := bindings[name]
		if !ok {
			continue
		}
		value, err := cmd.Flags().GetString(name)
		if err != nil {
			continue
		}
		if err := apply(cfg, value); err != nil {
			slog.Warn("应用命令行参数失败", "flag", name, "error", err)
		}
	}
}

func run(cmd *cobra.Command) error {
	// 配置文件中各字段与flag的对应关系（用于显式flag的回填）
	bindings := map[string]func(*config.Config, string) error{
		"monitor-interface":       func(c *config.Config, v string) error { c.Monitor.Interface = v; return nil },
		"monitor-exclude-subnets": func(c *config.Config, v string) error { c.Monitor.ExcludeSubnets = v; return nil },
		"monitor-window":          func(c *config.Config, v string) error { return parseIntFlag(v, func(n int) { c.Monitor.Window = n }) },
		"monitor-timeout":         func(c *config.Config, v string) error { return parseIntFlag(v, func(n int) { c.Monitor.Timeout = n }) },
		"firewall-chain":          func(c *config.Config, v string) error { c.Firewall.Chain = v; return nil },
		"firewall-ipset":          func(c *config.Config, v string) error { c.Firewall.IpSet = v; return nil },
		"firewall-type":           func(c *config.Config, v string) error { c.Firewall.Type = v; return nil },
		"listen":                  func(c *config.Config, v string) error { c.Web.Listen = v; return nil },
		"db-driver":               func(c *config.Config, v string) error { c.Database.Driver = v; return nil },
		"db-host":                 func(c *config.Config, v string) error { c.Database.Host = v; return nil },
		"db-port":                 func(c *config.Config, v string) error { return parseIntFlag(v, func(n int) { c.Database.Port = n }) },
		"db-username":             func(c *config.Config, v string) error { c.Database.Username = v; return nil },
		"db-password":             func(c *config.Config, v string) error { c.Database.Password = v; return nil },
		"db-name":                 func(c *config.Config, v string) error { c.Database.Database = v; return nil },
		"db-dsn":                  func(c *config.Config, v string) error { c.Database.DSN = v; return nil },
		"db-log-level":            func(c *config.Config, v string) error { c.Database.LogLevel = v; return nil },
	}

	// 加载配置文件（如果指定）。优先级：默认值 < 配置文件 < 显式命令行参数
	if configFile != "" {
		fileConfig, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("加载配置文件失败: %w", err)
		}

		merged, err := mergeConfig(cfg, fileConfig)
		if err != nil {
			return fmt.Errorf("合并配置失败: %w", err)
		}
		*cfg = *merged

		redacted, _ := json.Marshal(cfg.Redacted())
		slog.Info("已加载配置文件", "file", configFile, "config", string(redacted))
	}

	// 显式传入的命令行参数拥有最高优先级
	applyExplicitFlags(cmd, cfg, bindings)

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("配置校验失败: %w", err)
	}

	// 创建监控器
	mon, err := core.NewMonitor(&cfg.Monitor)
	if err != nil {
		return fmt.Errorf("创建监控器失败: %w", err)
	}
	if err := mon.Start(); err != nil {
		return fmt.Errorf("启动监控失败: %w", err)
	}
	defer mon.Stop()

	// 创建数据库连接
	st, err := store.NewStore(&cfg.Database)
	if err != nil {
		return fmt.Errorf("创建数据库连接失败: %w", err)
	}

	// 创建防火墙
	fw, err := core.NewFirewallFromConfig(&cfg.Firewall)
	if err != nil {
		return fmt.Errorf("创建防火墙失败: %w", err)
	}

	svc := service.NewNetService(mon, fw, st)
	if err := svc.Init(cfg.Rules); err != nil {
		return fmt.Errorf("初始化失败: %w", err)
	}

	// 创建认证处理器
	authHandler, err := web.NewAuthHandler(context.Background(), &web.AuthConfig{
		Enabled:       cfg.Web.Auth.Enabled,
		Type:          cfg.Web.Auth.Type,
		BasicUsername: cfg.Web.Auth.Basic.Username,
		BasicPassword: cfg.Web.Auth.Basic.Password,
		ClientID:      cfg.Web.Auth.OIDC.ClientID,
		ClientSecret:  cfg.Web.Auth.OIDC.ClientSecret,
		IssuerURL:     cfg.Web.Auth.OIDC.IssuerURL,
		RedirectURL:   cfg.Web.Auth.OIDC.RedirectURL,
		SessionSecret: cfg.Web.Auth.OIDC.SessionSecret,
	})
	if err != nil {
		return fmt.Errorf("创建认证处理器失败: %w", err)
	}

	if cfg.Web.Auth.Enabled {
		if cfg.Web.Auth.Type == "basic" {
			slog.Info("BasicAuth认证已启用", "username", cfg.Web.Auth.Basic.Username)
		} else {
			slog.Info("OIDC认证已启用", "issuer", cfg.Web.Auth.OIDC.IssuerURL)
		}
	}

	// 监听退出信号，统一走优雅退出流程：
	// 停止Web服务 -> 停止流量监控 -> 清理防火墙规则
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	server := web.NewServer(svc, authHandler)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Web服务已启动", "listen", cfg.Web.Listen)
		serverErr <- server.Start(cfg.Web.Listen)
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("收到退出信号，开始优雅退出")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("停止Web服务失败", "error", err)
		}
	}

	if err := fw.Cleanup(); err != nil {
		slog.Error("清理防火墙规则失败", "error", err)
	}

	return nil
}

// mergeConfig 将文件配置合并到默认配置上（文件中设置的字段覆盖默认值）
func mergeConfig(defaultCfg, fileCfg *config.Config) (*config.Config, error) {
	merged := *defaultCfg
	if err := mergo.Merge(&merged, fileCfg, mergo.WithOverride); err != nil {
		return nil, err
	}
	return &merged, nil
}

func parseIntFlag(v string, set func(int)) error {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("无效的数字: %s", v)
	}
	set(n)
	return nil
}
