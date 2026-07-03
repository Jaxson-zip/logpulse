package filter

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// 预编译正则:提取日志行中的时间字段,避免循环内重复编译。
var (
	nginxTimeRe   = regexp.MustCompile(`\[(\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4})\]`)
	genericTimeRe = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2})`)
)

const (
	dateOnlyLayout = "2006-01-02"
	daySpan        = 24*time.Hour - time.Nanosecond
)

// Filter 表示一次过滤条件集合;零值 Filter 不过滤任何行。
type Filter struct {
	From   time.Time
	To     time.Time
	Levels []string
}

// ParseTimeBound 解析 --from/--to 边界值。
// 支持 RFC3339 与 YYYY-MM-DD 简写;空串返回零值(表示无界)。
// isTo 为 true 时,日期简写补齐为当日 23:59:59.999999999 以包含整天。
func ParseTimeBound(s string, isTo bool) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(dateOnlyLayout, s); err == nil {
		if isTo {
			return t.Add(daySpan), nil
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("无法解析时间 %q (支持 RFC3339 或 YYYY-MM-DD)", s)
}

// MatchLevel 判断 level 是否在允许集合中;空集合表示不过滤(恒真)。
// 比较大小写不敏感。
func MatchLevel(allowed []string, level string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if strings.EqualFold(a, level) {
			return true
		}
	}
	return false
}

// ExtractTime 从日志行提取时间。
// 依次尝试 nginx combined 时间与通用行首时间;失败返回零值与 false。
func ExtractTime(line string) (time.Time, bool) {
	if m := nginxTimeRe.FindStringSubmatch(line); len(m) == 2 {
		if t, err := time.Parse("02/Jan/2006:15:04:05 -0700", m[1]); err == nil {
			return t, true
		}
	}
	if m := genericTimeRe.FindStringSubmatch(line); len(m) == 2 {
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
			if t, err := time.Parse(layout, m[1]); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// Keep 判断一行日志是否通过过滤。
// 级别不匹配则丢弃;启用时间过滤但无法提取时间的行也丢弃。
func (f *Filter) Keep(line, level string) bool {
	if !MatchLevel(f.Levels, level) {
		return false
	}
	if f.From.IsZero() && f.To.IsZero() {
		return true
	}
	t, ok := ExtractTime(line)
	if !ok {
		return false
	}
	if !f.From.IsZero() && t.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && t.After(f.To) {
		return false
	}
	return true
}
