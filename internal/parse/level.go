package parse

import (
	"regexp"
	"strings"
)

var levelRe = regexp.MustCompile(`(?i)\[(ERROR|WARN|INFO|DEBUG)\]`)

// ParseLevel 从一行日志中提取 [LEVEL] 标记,返回大写级别名。
// 未识别(无标记或级别不在四级内)返回 "unknown"。
func ParseLevel(line string) string {
	m := levelRe.FindStringSubmatch(line)
	if len(m) < 2 {
		return "unknown"
	}
	return strings.ToUpper(m[1])
}
