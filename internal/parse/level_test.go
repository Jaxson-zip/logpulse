package parse

import "testing"

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"error", "2026-07-01 10:00:00 [ERROR] timeout", "ERROR"},
		{"warn", "[WARN] cache miss", "WARN"},
		{"info", "2026-07-01 [INFO] started", "INFO"},
		{"debug", "[DEBUG] detail=x", "DEBUG"},
		{"lowercase", "[error] something failed", "ERROR"},
		{"mixed_case", "[Warn] retry once", "WARN"},
		{"nested_brackets", "[2026-07-01] [ERROR] msg [detail]", "ERROR"},
		{"multiple_levels", "[ERROR] then [INFO] follows", "ERROR"},
		{"warning_not_warn", "[WARNING] boom", "unknown"},
		{"trace_unknown", "[TRACE] low level", "unknown"},
		{"no_bracket", "2026-07-01 INFO no brackets here", "unknown"},
		{"empty", "", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLevel(tt.line); got != tt.want {
				t.Errorf("ParseLevel(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}
