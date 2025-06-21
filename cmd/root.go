/*
Copyright © 2025 graydovee
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"dario.cat/mergo"
	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/graydovee/netbouncer/pkg/store"
	"github.com/graydovee/netbouncer/pkg/web"
	"github.com/spf13/cobra"
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
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
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
	rootCmd.Flags().StringVarP(&cfg.Firewall.Type, "firewall-type", "f", string(cfg.Firewall.Type), "防火墙类型 (iptables|ipset|mock)")

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

	// 添加使用示例
	rootCmd.Example = `  # 使用默认配置启动
  netbouncer

  # 生成默认配置文件
  netbouncer config generate

  # 使用配置文件启动
  netbouncer -c config.yaml

  # 指定网络接口和监听地址
  netbouncer -i eth0 -l 0.0.0.0:9090

  # 使用数据库存储
  netbouncer -s database --db-driver sqlite --db-name myapp.db

  # 排除特定子网
  netbouncer -e "127.0.0.1/8,192.168.0.0/16"

  # 调试模式
  netbouncer --debug`
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	// 加载配置文件（如果指定）
	var fileConfig *config.Config
	if configFile != "" {
		var err error
		fileConfig, err = config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("加载配置文件失败: %w", err)
		}
		fileConfigJson, _ := json.Marshal(fileConfig)
		slog.Info("已加载配置文件", "file", configFile, "data", string(fileConfigJson))

		// 合并配置，文件配置优先级高于命令行参数
		mergo.Merge(cfg, fileConfig, mergo.WithOverride)
		configJson, _ := json.Marshal(cfg)
		slog.Info("合并配置完成", "config", string(configJson))
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
	store, err := store.NewStore(&cfg.Database)
	if err != nil {
		return fmt.Errorf("创建数据库连接失败: %w", err)
	}

	// 创建防火墙
	fw, err := core.NewFirewallFromConfig(&cfg.Firewall)
	if err != nil {
		return fmt.Errorf("创建防火墙失败: %w", err)
	}

	svc := service.NewNetService(mon, fw, store)
	if err := svc.Init(); err != nil {
		return fmt.Errorf("初始化失败: %w", err)
	}

	server := web.NewServer(svc)
	slog.Info("Web服务已启动", "listen", cfg.Web.Listen)
	if err := server.Start(cfg.Web.Listen); err != nil {
		return err
	}
	return nil
}
