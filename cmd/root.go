/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/graydovee/netbouncer/pkg/web"
	"github.com/spf13/cobra"
)

var cfg config.Config

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
	rootCmd.Flags().StringVarP(&cfg.Device, "device", "d", "", "网络接口名称（留空自动选择）")
	rootCmd.Flags().StringVarP(&cfg.WebAddr, "addr", "a", "0.0.0.0:8080", "Web服务监听地址")
	rootCmd.Flags().IntVarP(&cfg.MonitorWindowSize, "window-size", "w", 30, "监控窗口大小（秒）")
	rootCmd.Flags().IntVarP(&cfg.MonitorConnectionTimeout, "connection-timeout", "t", 60*60*24, "连接超时时间（秒）")
	rootCmd.Flags().StringVarP(&cfg.IptablesChainName, "iptables-chain", "c", "netbouncer", "iptables链名")
	rootCmd.Flags().BoolVar(&cfg.Debug, "debug", false, "调试模式")
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
	mon, err := core.NewMonitor(cfg.Device, time.Duration(cfg.MonitorWindowSize)*time.Second, time.Duration(cfg.MonitorConnectionTimeout)*time.Second)
	if err != nil {
		fmt.Printf("创建监控器失败: %v\n", err)
		os.Exit(1)
	}

	var fw core.Firewall
	if cfg.Debug {
		fw = core.NewMockFirewall()
	} else {
		fw, err = core.NewIptablesFirewall(cfg.IptablesChainName)
		if err != nil {
			fmt.Printf("初始化防火墙失败: %v\n", err)
			os.Exit(1)
		}
	}
	if err := mon.Start(); err != nil {
		fmt.Printf("启动监控失败: %v\n", err)
		os.Exit(1)
	}
	defer mon.Stop()

	svc := service.NewNetService(mon, fw)
	server := web.NewServer(svc)
	fmt.Printf("Web服务已启动: http://%s\n", cfg.WebAddr)
	if err := server.Start(cfg.WebAddr); err != nil {
		return err
	}
	return nil
}
