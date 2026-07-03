package report

import (
	"bytes"
	"encoding/json"
	"fmt"

	"logpulse/internal/stat"
)

// Summary 汇总基础统计指标。
type Summary struct {
	Format     string `json:"format"`
	TotalLines int    `json:"total_lines"`
	Parsed     int    `json:"parsed"`
	Skipped    int    `json:"skipped"`
}

// TopItem 是 Top N 列表的单条条目。
type TopItem struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// Alert 是一条异常告警(R7 预留,R6 恒为空数组占位)。
type Alert struct {
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
	Count     int `json:"count"`
}

// Report 是日志分析的完整结构化报告。
type Report struct {
	Summary     Summary             `json:"summary"`
	LevelCounts map[string]int      `json:"level_counts"`
	TopIPs      []TopItem           `json:"top_ips"`
	TopPaths    []TopItem           `json:"top_paths"`
	Alerts      []Alert             `json:"alerts"`
	Extra       map[string]string   `json:"extra,omitempty"`
}

// FromTopN 将 stat.Entry 列表转为 TopItem 列表;nil 时返回空切片。
func FromTopN(entries []stat.Entry) []TopItem {
	items := make([]TopItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, TopItem{Key: e.Key, Count: e.Count})
	}
	return items
}

// Render 将报告序列化为带缩进的 JSON 字符串。
// 空切片字段输出 [] 而非 null;map 为空时输出 {}。
func Render(r *Report) (string, error) {
	if r == nil {
		return "", fmt.Errorf("report is nil")
	}
	ensureNonNil(r)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return "", fmt.Errorf("json 编码失败: %w", err)
	}
	return buf.String(), nil
}

// ensureNonNil 将切片字段保证为非 nil,使 JSON 输出 [] 而非 null。
func ensureNonNil(r *Report) {
	if r.TopIPs == nil {
		r.TopIPs = []TopItem{}
	}
	if r.TopPaths == nil {
		r.TopPaths = []TopItem{}
	}
	if r.Alerts == nil {
		r.Alerts = []Alert{}
	}
	if r.LevelCounts == nil {
		r.LevelCounts = map[string]int{}
	}
}
