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
	Short: "分析单个日志文件,输出统计报告",
	Args:  cobra.ExactArgs(1),
	RunE:  runAnalyze,
}

var (
	formatFlag string
	levels     = []string{"ERROR", "WARN", "INFO", "DEBUG", "unknown"}
)

func init() {
	analyzeCmd.Flags().StringVar(&formatFlag, "format", "generic", "日志格式: generic|nginx")
	rootCmd.AddCommand(analyzeCmd)
}

// runAnalyze 根据格式分发到对应解析器。
func runAnalyze(cmd *cobra.Command, args []string) error {
	path := args[0]
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %w", path, err)
	}
	defer file.Close()

	switch formatFlag {
	case "generic":
		return analyzeGeneric(path, file)
	case "nginx":
		return analyzeNginx(path, file)
	default:
		return fmt.Errorf("不支持的格式: %s (可选 generic|nginx)", formatFlag)
	}
}

// analyzeGeneric 解析通用 [LEVEL] 格式,输出总行数与级别计数。
func analyzeGeneric(path string, file *os.File) error {
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

// nginxEntry 保存一行 nginx 日志的提取结果。
type nginxEntry struct {
	IP     string
	Method string
	Path   string
}

// analyzeNginx 解析 nginx combined 格式,输出解析计数与 IP/路径示例。
func analyzeNginx(path string, file *os.File) error {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var parsed, skipped int
	var sample []nginxEntry
	const sampleLimit = 10
	for scanner.Scan() {
		ip, method, p, ok := parse.ParseNginx(scanner.Text())
		if !ok {
			skipped++
			continue
		}
		parsed++
		if len(sample) < sampleLimit {
			sample = append(sample, nginxEntry{ip, method, p})
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取文件 %s 失败: %w", path, err)
	}
	fmt.Printf("文件 %s (nginx 模式) 总行数: %d\n", path, parsed+skipped)
	fmt.Printf("成功解析: %d 行,跳过(解析失败): %d 行\n", parsed, skipped)
	printNginxSample(sample)
	return nil
}

// printNginxSample 打印前若干条 IP/方法/路径提取结果。
func printNginxSample(sample []nginxEntry) {
	if len(sample) == 0 {
		fmt.Println("无有效解析结果")
		return
	}
	fmt.Println("IP 与请求路径示例:")
	for _, e := range sample {
		fmt.Printf("  %-15s %-6s %s\n", e.IP, e.Method, e.Path)
	}
}
