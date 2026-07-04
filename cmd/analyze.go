package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"logpulse/internal/alert"
	"logpulse/internal/filter"
	"logpulse/internal/parse"
	"logpulse/internal/report"
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
	formatFlag        string
	topNFlag          int
	fromFlag          string
	toFlag            string
	levelFlag         string
	formatOutFlag     string
	alertThresholdFlag int
	levels            = []string{"ERROR", "WARN", "INFO", "DEBUG", "unknown"}
)

func init() {
	analyzeCmd.Flags().StringVar(&formatFlag, "format", "generic", "日志格式: generic|nginx")
	analyzeCmd.Flags().IntVarP(&topNFlag, "n", "n", 10, "Top N 输出条数(nginx 模式生效)")
	analyzeCmd.Flags().StringVar(&fromFlag, "from", "", "起始时间(RFC3339 或 YYYY-MM-DD)")
	analyzeCmd.Flags().StringVar(&toFlag, "to", "", "结束时间(RFC3339 或 YYYY-MM-DD)")
	analyzeCmd.Flags().StringVar(&levelFlag, "level", "", "级别过滤(逗号分隔,如 ERROR,WARN)")
	analyzeCmd.Flags().StringVar(&formatOutFlag, "format-output", "table", "输出格式: table|json")
	analyzeCmd.Flags().IntVar(&alertThresholdFlag, "alert-threshold", 5, "连续 ERROR 告警阈值")
	rootCmd.AddCommand(analyzeCmd)
}

// runAnalyze 根据格式分发到对应解析器;统一构造过滤条件。
func runAnalyze(cmd *cobra.Command, args []string) error {
	if formatOutFlag != "table" && formatOutFlag != "json" {
		return fmt.Errorf("不支持的输出格式: %s (可选 table|json)", formatOutFlag)
	}
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

// emit 根据 --format-output 渲染并打印报告;json 模式输出结构化 JSON。
func emit(r *report.Report) error {
	switch formatOutFlag {
	case "json":
		out, err := report.Render(r)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	case "table", "":
		printReportTable(r)
		return nil
	default:
		return fmt.Errorf("不支持的输出格式: %s (可选 table|json)", formatOutFlag)
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

// analyzeGeneric 解析通用 [LEVEL] 格式,应用过滤后构建并输出报告。
func analyzeGeneric(path string, file *os.File, f *filter.Filter) error {
	det := alert.NewDetector(alertThresholdFlag)
	total, counts, err := scanLog(file, f, det)
	if err != nil {
		return fmt.Errorf("读取文件 %s 失败: %w", path, err)
	}
	r := &report.Report{
		Summary:     report.Summary{Format: "generic", TotalLines: total, Parsed: total, Skipped: 0},
		LevelCounts: counts,
		TopIPs:      []report.TopItem{},
		TopPaths:    []report.TopItem{},
		Alerts:      toReportAlerts(det.Spans()),
	}
	if total == 0 {
		if formatOutFlag == "json" {
			return emit(r)
		}
		fmt.Printf("文件 %s 总行数: %d\n", path, total)
		fmt.Println("提示: 过滤后无匹配日志行")
		return nil
	}
	if formatOutFlag == "json" {
		return emit(r)
	}
	fmt.Printf("文件 %s 总行数: %d\n", path, total)
	printLevelCounts(counts)
	printAlerts(r.Alerts)
	return nil
}

// toReportAlerts 将 alert.Span 列表转为 report.Alert 列表;恒返回非 nil。
func toReportAlerts(spans []alert.Span) []report.Alert {
	alerts := make([]report.Alert, 0, len(spans))
	for _, s := range spans {
		alerts = append(alerts, report.Alert{
			StartLine: s.StartLine,
			EndLine:   s.EndLine,
			Count:     s.Count,
		})
	}
	return alerts
}

// scanLog 逐行扫描日志,应用过滤后统计总行数与各级别计数;
// det 非 nil 时同步驱动连续 ERROR 检测。单行最大 1MB。
func scanLog(file *os.File, f *filter.Filter, det *alert.Detector) (int, map[string]int, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	counts := newLevelCounts()
	total := 0
	for scanner.Scan() {
		line := scanner.Text()
		lvl := parse.ParseLevel(line)
		if !f.Keep(line, lvl) {
			continue
		}
		total++
		counts[lvl]++
		if det != nil {
			det.Observe(lvl == "ERROR")
		}
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

// analyzeNginx 解析 nginx combined 格式,应用过滤后构建并输出报告。
func analyzeNginx(path string, file *os.File, f *filter.Filter) error {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var parsed, skipped int
	ipCounts := make(map[string]int)
	pathCounts := make(map[string]int)
	// 惰性求值:nginx combined 格式本身不含 [LEVEL] 标记,仅当用户配置了 --level 过滤时
	// 才调用 ParseLevel(触发正则),否则跳过级别检查,避免对每行做冗余正则匹配。
	levelFiltered := len(f.Levels) > 0
	for scanner.Scan() {
		line := scanner.Text()
		ip, _, p, ok := parse.ParseNginx(line)
		if !ok {
			skipped++
			continue
		}
		lvl := ""
		if levelFiltered {
			lvl = parse.ParseLevel(line)
		}
		if !f.Keep(line, lvl) {
			continue
		}
		parsed++
		ipCounts[ip]++
		pathCounts[p]++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取文件 %s 失败: %w", path, err)
	}
	r := &report.Report{
		Summary:     report.Summary{Format: "nginx", TotalLines: parsed + skipped, Parsed: parsed, Skipped: skipped},
		LevelCounts: newLevelCounts(),
		TopIPs:      report.FromTopN(stat.TopN(ipCounts, topNFlag)),
		TopPaths:    report.FromTopN(stat.TopN(pathCounts, topNFlag)),
		Alerts:      []report.Alert{},
	}
	if formatOutFlag == "json" {
		return emit(r)
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

// printReportTable 以表格风格打印报告摘要与 Top N 列表。
func printReportTable(r *report.Report) {
	fmt.Printf("格式: %s  总行数: %d  解析: %d  跳过: %d\n",
		r.Summary.Format, r.Summary.TotalLines, r.Summary.Parsed, r.Summary.Skipped)
	if len(r.LevelCounts) > 0 {
		printLevelCounts(r.LevelCounts)
	}
	printTopN("高频 IP Top", topNFromItems(r.TopIPs))
	printTopN("高频路径 Top", topNFromItems(r.TopPaths))
	printAlerts(r.Alerts)
}

// printAlerts 打印告警列表;无告警时输出占位提示。
func printAlerts(alerts []report.Alert) {
	if len(alerts) == 0 {
		fmt.Println("告警: (无)")
		return
	}
	fmt.Println("告警:")
	for i, a := range alerts {
		fmt.Printf("  %d. 行 %d-%d  连续 %d 次\n", i+1, a.StartLine, a.EndLine, a.Count)
	}
}

// topNFromItems 将 TopItem 转回 stat.Entry 以复用 printTopN。
func topNFromItems(items []report.TopItem) []stat.Entry {
	entries := make([]stat.Entry, 0, len(items))
	for _, it := range items {
		entries = append(entries, stat.Entry{Key: it.Key, Count: it.Count})
	}
	return entries
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
