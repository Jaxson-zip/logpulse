package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze <file>",
	Short: "分析单个日志文件并输出总行数",
	Args:  cobra.ExactArgs(1),
	RunE:  runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}

// runAnalyze 读取指定日志文件,统计并输出总行数。
func runAnalyze(cmd *cobra.Command, args []string) error {
	path := args[0]
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %w", path, err)
	}
	defer file.Close()

	count, err := countLines(file)
	if err != nil {
		return fmt.Errorf("读取文件 %s 失败: %w", path, err)
	}

	fmt.Printf("文件 %s 总行数: %d\n", path, count)
	return nil
}

// countLines 逐行扫描文件并返回行数;单行最大 1MB 以兼容长日志。
func countLines(file *os.File) (int, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}
