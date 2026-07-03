package filter

import (
	"reflect"
	"testing"
	"time"
)

func TestParseTimeBound(t *testing.T) {
	utc := time.FixedZone("UTC", 0)
	tests := []struct {
		name string
		s    string
		isTo bool
		want time.Time
		ok   bool
	}{
		{"empty", "", false, time.Time{}, true},
		{"rfc3339", "2026-07-01T10:00:00Z", false, time.Date(2026, 7, 1, 10, 0, 0, 0, utc), true},
		{"date_from", "2026-07-01", false, time.Date(2026, 7, 1, 0, 0, 0, 0, utc), true},
		{"date_to_includes_full_day", "2026-07-02", true, time.Date(2026, 7, 2, 23, 59, 59, 999999999, utc), true},
		{"invalid", "2026/07/01", false, time.Time{}, false},
		{"garbage", "abc", false, time.Time{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeBound(tt.s, tt.isTo)
			if tt.ok && err != nil {
				t.Fatalf("ParseTimeBound(%q) err = %v, want nil", tt.s, err)
			}
			if !tt.ok {
				if err == nil {
					t.Fatalf("ParseTimeBound(%q) err = nil, want error", tt.s)
				}
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseTimeBound(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestMatchLevel(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		level   string
		want    bool
	}{
		{"empty_allows_all", nil, "unknown", true},
		{"single_match", []string{"ERROR"}, "ERROR", true},
		{"case_insensitive", []string{"ERROR"}, "error", true},
		{"multi_match", []string{"ERROR", "WARN"}, "WARN", true},
		{"multi_no_match", []string{"ERROR", "WARN"}, "INFO", false},
		{"unknown_excluded_when_filter", []string{"ERROR"}, "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchLevel(tt.allowed, tt.level); got != tt.want {
				t.Errorf("MatchLevel(%v,%q) = %v, want %v", tt.allowed, tt.level, got, tt.want)
			}
		})
	}
}

func TestExtractTime(t *testing.T) {
	utc := time.FixedZone("UTC", 0)
	tests := []struct {
		name string
		line string
		want time.Time
		ok   bool
	}{
		{"generic_space", "2026-07-01 10:00:00 [INFO] started", time.Date(2026, 7, 1, 10, 0, 0, 0, utc), true},
		{"generic_t", "2026-07-01T10:00:00 [INFO] started", time.Date(2026, 7, 1, 10, 0, 0, 0, utc), true},
		{"nginx_with_tz", `1.2.3.4 - - [01/Jul/2026:10:00:00 +0800] "GET / HTTP/1.1" 200 1`, time.Date(2026, 7, 1, 10, 0, 0, 0, time.FixedZone("+0800", 8*3600)), true},
		{"no_time", "random line without time", time.Time{}, false},
		{"empty", "", time.Time{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractTime(tt.line)
			if ok != tt.ok {
				t.Fatalf("ExtractTime(%q) ok = %v, want %v", tt.line, ok, tt.ok)
			}
			if ok && !got.Equal(tt.want) {
				t.Errorf("ExtractTime(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestFilterKeep(t *testing.T) {
	from, _ := ParseTimeBound("2026-07-01", false)
	to, _ := ParseTimeBound("2026-07-02", true)
	f := &Filter{From: from, To: to, Levels: []string{"ERROR", "WARN"}}

	tests := []struct {
		name  string
		line  string
		level string
		want  bool
	}{
		{"in_range_matching_level", "2026-07-01 10:00:00 [ERROR] x", "ERROR", true},
		{"in_range_wrong_level", "2026-07-01 10:00:00 [INFO] x", "INFO", false},
		{"out_of_range", "2026-07-03 10:00:00 [ERROR] x", "ERROR", false},
		{"no_time_with_time_filter", "no timestamp here [ERROR]", "ERROR", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.Keep(tt.line, tt.level); got != tt.want {
				t.Errorf("Keep(%q,%q) = %v, want %v", tt.line, tt.level, got, tt.want)
			}
		})
	}
}

func TestFilterZeroValueKeepsAll(t *testing.T) {
	var f Filter
	if !f.Keep("anything", "unknown") {
		t.Error("zero-value Filter should keep all lines")
	}
	if !reflect.DeepEqual(f.Levels, []string(nil)) {
		t.Error("zero-value Filter Levels should be nil")
	}
}
