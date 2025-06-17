/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/graydovee/netbouncer/pkg/monitor"
	"github.com/graydovee/netbouncer/pkg/web"
	"github.com/spf13/cobra"
)

var (
	device  string
	webAddr string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "netbouncer",
	Short: "网络流量监控工具",
	Long:  `netbouncer 是一个网络流量监控工具，支持Web页面实时查看流量统计。`,
	Run: func(cmd *cobra.Command, args []string) {
		mon, err := monitor.NewMonitor(device)
		if err != nil {
			fmt.Printf("创建监控器失败: %v\n", err)
			os.Exit(1)
		}

		if err := mon.Start(); err != nil {
			fmt.Printf("启动监控失败: %v\n", err)
			os.Exit(1)
		}
		defer mon.Stop()

		server := web.NewServer(mon)
		fmt.Printf("Web服务已启动: http://%s\n", webAddr)
		if err := server.Start(webAddr); err != nil {
			fmt.Printf("Web服务启动失败: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&device, "device", "d", "", "网络接口名称（留空自动选择）")
	rootCmd.Flags().StringVarP(&webAddr, "web", "w", "0.0.0.0:8080", "Web服务监听地址")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
