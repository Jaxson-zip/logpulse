package cmd

import (
	"bufio"
	"fmt"
	"os"

	"logpulse/internal/parse"

	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze <file>",
	Short: "分析单个日志文件,输出总行数与各级别计数",
	Args:  cobra.ExactArgs(1),
	RunE:  runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}

// levels 为输出顺序固定的级别列表,unknown 排在最后。
var levels = []string{"ERROR", "WARN", "INFO", "DEBUG", "unknown"}

// runAnalyze 读取指定日志文件,统计并输出总行数与各级别计数。
func runAnalyze(cmd *cobra.Command, args []string) error {
	path := args[0]
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %w", path, err)
	}
	defer file.Close()

	total, counts, err := scanLog(file)
	if err != nil {
		return fmt.Errorf("读取文件 %s 失败: %w", path, err)
	}

	fmt.Printf("文件 %s 总行数: %d\n", path, total)
	printLevelCounts(counts)
	return nil
}

// scanLog 逐行扫描日志,返回总行数与各级别计数;单行最大 1MB 以兼容长日志。
func scanLog(file *os.File) (int, map[string]int, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	counts := newLevelCounts()
	total := 0
	for scanner.Scan() {
		total++
		counts[parse.ParseLevel(scanner.Text())]++
	}
	if err := scanner.Err(); err != nil {
		return 0, nil, err
	}
	return total, counts, nil
}

// newLevelCounts 初始化包含全部预定义级别的计数 map,保证输出稳定。
func newLevelCounts() map[string]int {
	m := make(map[string]int, len(levels))
	for _, l := range levels {
		m[l] = 0
	}
	return m
}

// printLevelCounts 按 levels 顺序输出各级别计数。
func printLevelCounts(counts map[string]int) {
	fmt.Println("级别统计:")
	for _, l := range levels {
		fmt.Printf("  %-7s %d\n", l, counts[l])
	}
}
