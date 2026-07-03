package parse

import "regexp"

// nginxRe 匹配 nginx combined 日志格式,提取客户端 IP、HTTP 方法与请求路径。
// 格式示例: 192.168.1.1 - - [01/Jul/2026:10:00:00 +0800] "GET /api/users HTTP/1.1" 200 1234 "-" "Mozilla/5.1"
// 要求请求行引号内以大写方法开头,以避免 "-" 等无效请求被误匹配。
var nginxRe = regexp.MustCompile(`^(\S+) \S+ \S+ \[[^\]]*\] "([A-Z]+) (\S+)[^"]*"`)

// ParseNginx 解析一行 nginx combined 日志,返回客户端 IP、HTTP 方法与请求路径。
// 解析失败(格式不匹配)时 ok 为 false。
func ParseNginx(line string) (ip, method, path string, ok bool) {
	m := nginxRe.FindStringSubmatch(line)
	if len(m) < 4 {
		return "", "", "", false
	}
	return m[1], m[2], m[3], true
}
