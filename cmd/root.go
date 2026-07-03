package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "logpulse",
	Short:         "logpulse 是一个日志分析 CLI 工具",
	Long:          "logpulse 读取日志文件并输出统计报告,支持级别解析、Top N、过滤与异常检测等。",
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute 执行根命令;命令出错时打印友好提示并以非零码退出。
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
