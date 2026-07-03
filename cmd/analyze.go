package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"logpulse/internal/filter"
	"logpulse/internal/parse"
	"logpulse/internal/stat"

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
	topNFlag   int
	fromFlag   string
	toFlag     string
	levelFlag  string
	levels     = []string{"ERROR", "WARN", "INFO", "DEBUG", "unknown"}
)

func init() {
	analyzeCmd.Flags().StringVar(&formatFlag, "format", "generic", "日志格式: generic|nginx")
	analyzeCmd.Flags().IntVarP(&topNFlag, "n", "n", 10, "Top N 输出条数(nginx 模式生效)")
	analyzeCmd.Flags().StringVar(&fromFlag, "from", "", "起始时间(RFC3339 或 YYYY-MM-DD)")
	analyzeCmd.Flags().StringVar(&toFlag, "to", "", "结束时间(RFC3339 或 YYYY-MM-DD)")
	analyzeCmd.Flags().StringVar(&levelFlag, "level", "", "级别过滤(逗号分隔,如 ERROR,WARN)")
	rootCmd.AddCommand(analyzeCmd)
}

// runAnalyze 根据格式分发到对应解析器;统一构造过滤条件。
func runAnalyze(cmd *cobra.Command, args []string) error {
	path := args[0]
	f, err := buildFilter()
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %w", path, err)
	}
	defer file.Close()

	switch formatFlag {
	case "generic":
		return analyzeGeneric(path, file, f)
	case "nginx":
		return analyzeNginx(path, file, f)
	default:
		return fmt.Errorf("不支持的格式: %s (可选 generic|nginx)", formatFlag)
	}
}

// buildFilter 解析 --from/--to/--level 为 Filter;未设任何过滤时为零值 Filter。
func buildFilter() (*filter.Filter, error) {
	from, err := filter.ParseTimeBound(fromFlag, false)
	if err != nil {
		return nil, err
	}
	to, err := filter.ParseTimeBound(toFlag, true)
	if err != nil {
		return nil, err
	}
	var lvls []string
	if strings.TrimSpace(levelFlag) != "" {
		for _, l := range strings.Split(levelFlag, ",") {
			if l = strings.TrimSpace(l); l != "" {
				lvls = append(lvls, l)
			}
		}
	}
	return &filter.Filter{From: from, To: to, Levels: lvls}, nil
}

// analyzeGeneric 解析通用 [LEVEL] 格式,应用过滤后输出总行数与级别计数。
func analyzeGeneric(path string, file *os.File, f *filter.Filter) error {
	total, counts, err := scanLog(file, f)
	if err != nil {
		return fmt.Errorf("读取文件 %s 失败: %w", path, err)
	}
	fmt.Printf("文件 %s 总行数: %d\n", path, total)
	if total == 0 {
		fmt.Println("提示: 过滤后无匹配日志行")
		return nil
	}
	printLevelCounts(counts)
	return nil
}

// scanLog 逐行扫描日志,应用过滤后统计总行数与各级别计数;单行最大 1MB。
func scanLog(file *os.File, f *filter.Filter) (int, map[string]int, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	counts := newLevelCounts()
	total := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !f.Keep(line, parse.ParseLevel(line)) {
			continue
		}
		total++
		counts[parse.ParseLevel(line)]++
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

// analyzeNginx 解析 nginx combined 格式,应用过滤后统计 IP/路径并输出 Top N。
func analyzeNginx(path string, file *os.File, f *filter.Filter) error {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var parsed, skipped int
	ipCounts := make(map[string]int)
	pathCounts := make(map[string]int)
	for scanner.Scan() {
		line := scanner.Text()
		ip, _, p, ok := parse.ParseNginx(line)
		if !ok {
			skipped++
			continue
		}
		if !f.Keep(line, parse.ParseLevel(line)) {
			continue
		}
		parsed++
		ipCounts[ip]++
		pathCounts[p]++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取文件 %s 失败: %w", path, err)
	}
	fmt.Printf("文件 %s (nginx 模式) 总行数: %d\n", path, parsed+skipped)
	fmt.Printf("成功解析: %d 行,跳过(解析失败): %d 行\n", parsed, skipped)
	if parsed == 0 {
		fmt.Println("提示: 过滤后无匹配日志行")
		return nil
	}
	printTopN("高频 IP Top", stat.TopN(ipCounts, topNFlag))
	printTopN("高频路径 Top", stat.TopN(pathCounts, topNFlag))
	return nil
}

// printTopN 输出 Top N 条目,标题带条数。
func printTopN(title string, entries []stat.Entry) {
	fmt.Printf("%s %d:\n", title, len(entries))
	if len(entries) == 0 {
		fmt.Println("  (无数据)")
		return
	}
	for i, e := range entries {
		fmt.Printf("  %2d. %-30s %d\n", i+1, e.Key, e.Count)
	}
}
