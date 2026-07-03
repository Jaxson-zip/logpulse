package report

import (
	"encoding/json"
	"strings"
	"testing"

	"logpulse/internal/stat"
)

func TestFromTopN(t *testing.T) {
	tests := []struct {
		name    string
		entries []stat.Entry
		want    []TopItem
	}{
		{"nil_to_empty", nil, []TopItem{}},
		{"empty_to_empty", []stat.Entry{}, []TopItem{}},
		{"two_items", []stat.Entry{{Key: "a", Count: 5}, {Key: "b", Count: 3}}, []TopItem{{Key: "a", Count: 5}, {Key: "b", Count: 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromTopN(tt.entries)
			if len(got) != len(tt.want) {
				t.Fatalf("FromTopN len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("FromTopN[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name    string
		report  *Report
		wantErr bool
	}{
		{
			name:    "nil_report_errors",
			report:  nil,
			wantErr: true,
		},
		{
			name: "full_report",
			report: &Report{
				Summary:     Summary{Format: "nginx", TotalLines: 10, Parsed: 8, Skipped: 2},
				LevelCounts: map[string]int{"ERROR": 3, "INFO": 5},
				TopIPs:      []TopItem{{"10.0.0.1", 4}},
				TopPaths:    []TopItem{{"/api", 4}},
				Alerts:      nil,
			},
			wantErr: false,
		},
		{
			name: "empty_slices_become_array",
			report: &Report{
				Summary:     Summary{Format: "generic", TotalLines: 0},
				LevelCounts: nil,
				TopIPs:      nil,
				TopPaths:    nil,
				Alerts:      nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Render(tt.report)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Render err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Render err = %v, want nil", err)
			}
			if !strings.Contains(out, "{") {
				t.Errorf("Render output missing JSON object: %s", out)
			}
			var v map[string]interface{}
			if err := json.Unmarshal([]byte(out), &v); err != nil {
				t.Fatalf("Render output not valid JSON: %v\n%s", err, out)
			}
		})
	}
}

func TestRenderEmptyArrayNotNull(t *testing.T) {
	r := &Report{
		Summary:     Summary{Format: "nginx", TotalLines: 0},
		LevelCounts: nil,
		TopIPs:      nil,
		TopPaths:    nil,
		Alerts:      nil,
	}
	out, err := Render(r)
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	mustContain := []string{
		`"top_ips": []`,
		`"top_paths": []`,
		`"alerts": []`,
		`"level_counts": {}`,
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("Render output missing %q\n%s", s, out)
		}
	}
}

func TestRenderSnakeCaseFields(t *testing.T) {
	r := &Report{Summary: Summary{Format: "generic", TotalLines: 1, Parsed: 1}}
	out, _ := Render(r)
	wantFields := []string{
		`"total_lines"`,
		`"level_counts"`,
		`"top_ips"`,
		`"top_paths"`,
	}
	for _, f := range wantFields {
		if !strings.Contains(out, f) {
			t.Errorf("Render output missing field %q\n%s", f, out)
		}
	}
}
