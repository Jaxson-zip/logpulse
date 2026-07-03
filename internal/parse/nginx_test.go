package parse

import "testing"

func TestParseNginx(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		ip     string
		method string
		path   string
		ok     bool
	}{
		{
			name:   "standard_combined",
			line:   `192.168.1.1 - - [01/Jul/2026:10:00:00 +0800] "GET /api/users HTTP/1.1" 200 1234 "-" "Mozilla/5.1"`,
			ip:     "192.168.1.1",
			method: "GET",
			path:   "/api/users",
			ok:     true,
		},
		{
			name:   "post_with_user_referer",
			line:   `10.0.0.5 - frank [01/Jul/2026:10:00:05 +0800] "POST /login HTTP/1.1" 302 0 "https://example.com/" "curl/8.0"`,
			ip:     "10.0.0.5",
			method: "POST",
			path:   "/login",
			ok:     true,
		},
		{
			name:   "path_with_query",
			line:   `203.0.113.9 - - [01/Jul/2026:10:00:10 +0800] "GET /search?q=logpulse HTTP/1.1" 200 512 "-" "Go-http-client"`,
			ip:     "203.0.113.9",
			method: "GET",
			path:   "/search?q=logpulse",
			ok:     true,
		},
		{
			name: "dash_request_invalid",
			line: `192.168.1.1 - - [01/Jul/2026:10:00:11 +0800] "-" 400 0 "-" "-"`,
			ok:   false,
		},
		{
			name: "missing_time_brackets",
			line: `192.168.1.1 - - 01/Jul/2026:10:00:00 +0800 "GET /api HTTP/1.1" 200 1234`,
			ok:   false,
		},
		{
			name: "no_quote_request",
			line: `192.168.1.1 - - [01/Jul/2026:10:00:00 +0800] GET /api HTTP/1.1 200 1234`,
			ok:   false,
		},
		{
			name: "empty",
			line: ``,
			ok:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, method, path, ok := ParseNginx(tt.line)
			if ok != tt.ok {
				t.Errorf("ParseNginx(%q) ok = %v, want %v", tt.line, ok, tt.ok)
				return
			}
			if ok && (ip != tt.ip || method != tt.method || path != tt.path) {
				t.Errorf("ParseNginx(%q) = (%q,%q,%q), want (%q,%q,%q)",
					tt.line, ip, method, path, tt.ip, tt.method, tt.path)
			}
		})
	}
}
