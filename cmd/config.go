package cmd

import (
	"fmt"
	"os"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
	Long:  `配置管理相关命令，包括生成默认配置文件等。`,
}

var generateConfigCmd = &cobra.Command{
	Use:   "generate [config-file]",
	Short: "生成默认配置文件",
	Long:  `生成默认的YAML配置文件，如果未指定文件名则使用config.yaml`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configFile := "config.yaml"
		if len(args) > 0 {
			configFile = args[0]
		}

		// 检查文件是否已存在
		if _, err := os.Stat(configFile); err == nil {
			fmt.Printf("配置文件 %s 已存在，是否覆盖？(y/N): ", configFile)
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("操作已取消")
				return
			}
		}

		// 生成默认配置
		defaultCfg := config.DefaultConfig()

		// 保存配置文件
		if err := config.SaveConfig(defaultCfg, configFile); err != nil {
			fmt.Printf("生成配置文件失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("默认配置文件已生成: %s\n", configFile)
		fmt.Println("请根据您的需求修改配置文件，然后使用 -c 参数指定配置文件启动程序。")
	},
}

func init() {
	configCmd.AddCommand(generateConfigCmd)
	rootCmd.AddCommand(configCmd)
}
