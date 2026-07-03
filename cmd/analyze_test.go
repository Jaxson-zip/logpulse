package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"logpulse/internal/alert"
	"logpulse/internal/report"
	"logpulse/internal/stat"
)

// resetFlags 将包级 flag 恢复为默认值,隔离各测试。
func resetFlags() {
	formatFlag = "generic"
	topNFlag = 10
	fromFlag = ""
	toFlag = ""
	levelFlag = ""
	formatOutFlag = "table"
	alertThresholdFlag = 5
}

// captureStdout 捕获 fn 期间写入 os.Stdout 的内容。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}

// writeTemp 创建临时日志文件并写入 content,返回路径与已打开且定位到首部的句柄。
func writeTemp(t *testing.T, content string) (*os.File, string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.log")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatalf("open temp: %v", err)
	}
	return f, p
}

func TestNewLevelCounts(t *testing.T) {
	m := newLevelCounts()
	if len(m) != len(levels) {
		t.Fatalf("len = %d, want %d", len(m), len(levels))
	}
	for _, l := range levels {
		if m[l] != 0 {
			t.Errorf("level %s = %d, want 0", l, m[l])
		}
	}
}

func TestTopNFromItems(t *testing.T) {
	tests := []struct {
		name  string
		items []report.TopItem
		want  []stat.Entry
	}{
		{"nil_to_empty", nil, []stat.Entry{}},
		{"empty_to_empty", []report.TopItem{}, []stat.Entry{}},
		{"two_items", []report.TopItem{{Key: "a", Count: 5}, {Key: "b", Count: 3}},
			[]stat.Entry{{Key: "a", Count: 5}, {Key: "b", Count: 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := topNFromItems(tt.items)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestToReportAlerts(t *testing.T) {
	tests := []struct {
		name  string
		spans []alert.Span
		want  []report.Alert
	}{
		{"nil_to_empty", nil, []report.Alert{}},
		{"empty_to_empty", []alert.Span{}, []report.Alert{}},
		{"one_span", []alert.Span{{StartLine: 1, EndLine: 3, Count: 3}},
			[]report.Alert{{StartLine: 1, EndLine: 3, Count: 3}}},
		{"two_spans", []alert.Span{{StartLine: 1, EndLine: 2, Count: 2}, {StartLine: 5, EndLine: 9, Count: 5}},
			[]report.Alert{{StartLine: 1, EndLine: 2, Count: 2}, {StartLine: 5, EndLine: 9, Count: 5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toReportAlerts(tt.spans)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildFilter(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		to       string
		level    string
		wantErr  bool
		wantLvl  []string
		hasBound bool
	}{
		{"empty_no_filter", "", "", "", false, nil, false},
		{"valid_levels_spaces", "", "", "ERROR, WARN , INFO", false, []string{"ERROR", "WARN", "INFO"}, false},
		{"single_level", "", "", "error", false, []string{"error"}, false},
		{"level_only_comma", "", "", ",", false, nil, false},
		{"valid_from_to", "2026-07-01", "2026-07-02", "", false, nil, true},
		{"bad_from", "2026/07/01", "", "", true, nil, false},
		{"bad_to", "", "notadate", "", true, nil, false},
		{"rfc3339_bounds", "2026-07-01T00:00:00Z", "2026-07-02T23:59:59Z", "ERROR", false, []string{"ERROR"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			fromFlag, toFlag, levelFlag = tt.from, tt.to, tt.level
			f, err := buildFilter()
			if tt.wantErr {
				if err == nil {
					t.Fatal("buildFilter err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildFilter err = %v", err)
			}
			if !equalStrings(f.Levels, tt.wantLvl) {
				t.Errorf("Levels = %v, want %v", f.Levels, tt.wantLvl)
			}
			if tt.hasBound && f.From.IsZero() && f.To.IsZero() {
				t.Error("expected at least one time bound set")
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEmit(t *testing.T) {
	r := &report.Report{
		Summary:     report.Summary{Format: "generic", TotalLines: 1, Parsed: 1},
		LevelCounts: map[string]int{"ERROR": 1},
		TopIPs:      []report.TopItem{{Key: "1.2.3.4", Count: 1}},
		TopPaths:    []report.TopItem{{Key: "/x", Count: 1}},
		Alerts:      []report.Alert{{StartLine: 1, EndLine: 1, Count: 1}},
	}
	t.Run("json_valid", func(t *testing.T) {
		resetFlags()
		formatOutFlag = "json"
		out := captureStdout(t, func() {
			if err := emit(r); err != nil {
				t.Fatalf("emit err = %v", err)
			}
		})
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("not valid JSON: %v\n%s", err, out)
		}
	})
	t.Run("table_valid", func(t *testing.T) {
		resetFlags()
		formatOutFlag = "table"
		out := captureStdout(t, func() {
			if err := emit(r); err != nil {
				t.Fatalf("emit err = %v", err)
			}
		})
		for _, want := range []string{"总行数", "级别统计", "高频 IP Top", "告警", "行 1-1"} {
			if !strings.Contains(out, want) {
				t.Errorf("table output missing %q\n%s", want, out)
			}
		}
	})
	t.Run("bad_format_errors", func(t *testing.T) {
		resetFlags()
		formatOutFlag = "xml"
		if err := emit(r); err == nil {
			t.Fatal("emit err = nil, want error for bad format")
		}
	})
}

func TestAnalyzeGenericTable(t *testing.T) {
	resetFlags()
	content := "2026-07-01 10:00:00 [ERROR] e1\n" +
		"2026-07-01 10:00:01 [ERROR] e2\n" +
		"2026-07-01 10:00:02 [INFO] i1\n" +
		"no level line\n"
	f, p := writeTemp(t, content)
	defer f.Close()
	formatOutFlag = "table"
	fltr, _ := buildFilter()
	out := captureStdout(t, func() {
		if err := analyzeGeneric(p, f, fltr); err != nil {
			t.Fatalf("analyzeGeneric err = %v", err)
		}
	})
	for _, want := range []string{"总行数: 4", "ERROR", "INFO", "unknown", "告警: (无)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestAnalyzeGenericJSON(t *testing.T) {
	resetFlags()
	content := "2026-07-01 10:00:00 [ERROR] e1\n2026-07-01 10:00:01 [INFO] i1\n"
	f, p := writeTemp(t, content)
	defer f.Close()
	formatOutFlag = "json"
	fltr, _ := buildFilter()
	out := captureStdout(t, func() {
		if err := analyzeGeneric(p, f, fltr); err != nil {
			t.Fatalf("analyzeGeneric err = %v", err)
		}
	})
	var r report.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if r.Summary.TotalLines != 2 {
		t.Errorf("total_lines = %d, want 2", r.Summary.TotalLines)
	}
	if r.LevelCounts["ERROR"] != 1 || r.LevelCounts["INFO"] != 1 {
		t.Errorf("level_counts = %v", r.LevelCounts)
	}
}

func TestAnalyzeGenericAlerts(t *testing.T) {
	resetFlags()
	alertThresholdFlag = 3
	var lines []string
	for i := 0; i < 4; i++ {
		lines = append(lines, "2026-07-01 10:00:0"+string(rune('0'+i))+" [ERROR] e")
	}
	content := strings.Join(lines, "\n") + "\n"
	f, p := writeTemp(t, content)
	defer f.Close()
	formatOutFlag = "json"
	fltr, _ := buildFilter()
	out := captureStdout(t, func() {
		if err := analyzeGeneric(p, f, fltr); err != nil {
			t.Fatalf("analyzeGeneric err = %v", err)
		}
	})
	var r report.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if len(r.Alerts) != 1 {
		t.Fatalf("alerts len = %d, want 1", len(r.Alerts))
	}
	a := r.Alerts[0]
	if a.StartLine != 1 || a.EndLine != 4 || a.Count != 4 {
		t.Errorf("alert = %+v, want start=1 end=4 count=4", a)
	}
}

func TestAnalyzeGenericEmpty(t *testing.T) {
	resetFlags()
	f, p := writeTemp(t, "")
	defer f.Close()
	formatOutFlag = "table"
	fltr, _ := buildFilter()
	out := captureStdout(t, func() {
		if err := analyzeGeneric(p, f, fltr); err != nil {
			t.Fatalf("analyzeGeneric err = %v", err)
		}
	})
	if !strings.Contains(out, "无匹配日志行") {
		t.Errorf("expected empty hint, got:\n%s", out)
	}
}

func TestAnalyzeNginxTable(t *testing.T) {
	resetFlags()
	content := `192.168.1.1 - - [01/Jul/2026:10:00:00 +0800] "GET /api HTTP/1.1" 200 10 "-" "M"
bad line not nginx
10.0.0.5 - - [01/Jul/2026:10:00:05 +0800] "POST /login HTTP/1.1" 200 12 "-" "c"
`
	f, p := writeTemp(t, content)
	defer f.Close()
	topNFlag = 5
	fltr, _ := buildFilter()
	out := captureStdout(t, func() {
		if err := analyzeNginx(p, f, fltr); err != nil {
			t.Fatalf("analyzeNginx err = %v", err)
		}
	})
	for _, want := range []string{"总行数: 3", "成功解析: 2", "跳过(解析失败): 1", "192.168.1.1", "/api"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestAnalyzeNginxJSON(t *testing.T) {
	resetFlags()
	content := `1.2.3.4 - - [01/Jul/2026:10:00:00 +0800] "GET /x HTTP/1.1" 200 1 "-" "M"`
	f, p := writeTemp(t, content+"\n")
	defer f.Close()
	formatOutFlag = "json"
	fltr, _ := buildFilter()
	out := captureStdout(t, func() {
		if err := analyzeNginx(p, f, fltr); err != nil {
			t.Fatalf("analyzeNginx err = %v", err)
		}
	})
	var r report.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if r.Summary.Parsed != 1 || r.Summary.Skipped != 0 {
		t.Errorf("summary = %+v", r.Summary)
	}
	if len(r.TopIPs) != 1 || r.TopIPs[0].Key != "1.2.3.4" {
		t.Errorf("top_ips = %v", r.TopIPs)
	}
}

func TestAnalyzeNginxNoParsed(t *testing.T) {
	resetFlags()
	f, p := writeTemp(t, "totally not nginx\nanother junk\n")
	defer f.Close()
	fltr, _ := buildFilter()
	out := captureStdout(t, func() {
		if err := analyzeNginx(p, f, fltr); err != nil {
			t.Fatalf("analyzeNginx err = %v", err)
		}
	})
	if !strings.Contains(out, "无匹配日志行") {
		t.Errorf("expected empty hint, got:\n%s", out)
	}
}

func TestRunAnalyzeErrors(t *testing.T) {
	t.Run("bad_format_output", func(t *testing.T) {
		resetFlags()
		formatOutFlag = "xml"
		err := runAnalyze(nil, []string{"any"})
		if err == nil || !strings.Contains(err.Error(), "不支持的输出格式") {
			t.Fatalf("err = %v, want format error", err)
		}
	})
	t.Run("bad_format", func(t *testing.T) {
		resetFlags()
		formatFlag = "xml"
		fh, p := writeTemp(t, "x\n")
		fh.Close()
		err := runAnalyze(nil, []string{p})
		if err == nil || !strings.Contains(err.Error(), "不支持的格式") {
			t.Fatalf("err = %v, want format error", err)
		}
	})
	t.Run("missing_file", func(t *testing.T) {
		resetFlags()
		err := runAnalyze(nil, []string{filepath.Join(t.TempDir(), "nope.log")})
		if err == nil || !strings.Contains(err.Error(), "无法打开文件") {
			t.Fatalf("err = %v, want open error", err)
		}
	})
	t.Run("bad_from_time", func(t *testing.T) {
		resetFlags()
		fromFlag = "notadate"
		fh, p := writeTemp(t, "x\n")
		fh.Close()
		err := runAnalyze(nil, []string{p})
		if err == nil {
			t.Fatal("err = nil, want time parse error")
		}
	})
}
