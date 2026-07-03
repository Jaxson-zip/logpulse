package stat

import (
	"reflect"
	"testing"
)

func TestTopN(t *testing.T) {
	tests := []struct {
		name   string
		counts map[string]int
		n      int
		want   []Entry
	}{
		{
			name:   "basic_top2",
			counts: map[string]int{"a": 5, "b": 3, "c": 1},
			n:      2,
			want:   []Entry{{"a", 5}, {"b", 3}},
		},
		{
			name:   "tie_break_by_key",
			counts: map[string]int{"b": 2, "a": 2, "c": 2},
			n:      2,
			want:   []Entry{{"a", 2}, {"b", 2}},
		},
		{
			name:   "n_exceeds_size_returns_all",
			counts: map[string]int{"a": 1, "b": 2},
			n:      10,
			want:   []Entry{{"b", 2}, {"a", 1}},
		},
		{
			name:   "n_equals_size",
			counts: map[string]int{"a": 1, "b": 2, "c": 3},
			n:      3,
			want:   []Entry{{"c", 3}, {"b", 2}, {"a", 1}},
		},
		{
			name:   "n_zero_returns_empty",
			counts: map[string]int{"a": 5, "b": 3},
			n:      0,
			want:   []Entry{},
		},
		{
			name:   "n_negative_returns_empty",
			counts: map[string]int{"a": 5, "b": 3},
			n:      -1,
			want:   []Entry{},
		},
		{
			name:   "empty_map",
			counts: map[string]int{},
			n:      5,
			want:   []Entry{},
		},
		{
			name:   "nil_map",
			counts: nil,
			n:      5,
			want:   []Entry{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TopN(tt.counts, tt.n)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TopN(%v, %d) = %v, want %v", tt.counts, tt.n, got, tt.want)
			}
		})
	}
}

func TestTopNTieBreakFullOrder(t *testing.T) {
	counts := map[string]int{"z": 2, "m": 2, "a": 2, "k": 2}
	got := TopN(counts, 4)
	want := []Entry{{"a", 2}, {"k", 2}, {"m", 2}, {"z", 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TopN tie-break order = %v, want %v", got, want)
	}
}
