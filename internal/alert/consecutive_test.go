package alert

import (
	"reflect"
	"testing"
)

// run 驱动 Detector:seq 中 'E' 表示 ERROR,其它字符表示非 ERROR。
func run(threshold int, seq string) []Span {
	d := NewDetector(threshold)
	for _, c := range seq {
		d.Observe(c == 'E')
	}
	return d.Spans()
}

func TestDetectorBasic(t *testing.T) {
	tests := []struct {
		name      string
		threshold int
		seq       string
		want       []Span
	}{
		{
			name:      "exactly_threshold_five",
			threshold: 5,
			seq:       "EEEEE",
			want:      []Span{{StartLine: 1, EndLine: 5, Count: 5}},
		},
		{
			name:      "below_threshold_no_alert",
			threshold: 5,
			seq:       "EEEE",
			want:      []Span{},
		},
		{
			name:      "threshold_three",
			threshold: 3,
			seq:       "EEE",
			want:      []Span{{StartLine: 1, EndLine: 3, Count: 3}},
		},
		{
			name:      "two_intervals_separated",
			threshold: 3,
			seq:       "EEE.EEEE",
			want: []Span{
				{StartLine: 1, EndLine: 3, Count: 3},
				{StartLine: 5, EndLine: 8, Count: 4},
			},
		},
		{
			name:      "error_at_end_no_trailing_break",
			threshold: 3,
			seq:       "..EEE",
			want:      []Span{{StartLine: 3, EndLine: 5, Count: 3}},
		},
		{
			name:      "interspersed_non_error_resets",
			threshold: 3,
			seq:       "EE.EE.EEE",
			want:      []Span{{StartLine: 7, EndLine: 9, Count: 3}},
		},
		{
			name:      "no_error_at_all",
			threshold: 3,
			seq:       ".....",
			want:      []Span{},
		},
		{
			name:      "empty_input",
			threshold: 3,
			seq:       "",
			want:      []Span{},
		},
		{
			name:      "threshold_one_every_error",
			threshold: 1,
			seq:       "E.E",
			want: []Span{
				{StartLine: 1, EndLine: 1, Count: 1},
				{StartLine: 3, EndLine: 3, Count: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(tt.threshold, tt.seq)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("run(%d, %q) = %v, want %v", tt.threshold, tt.seq, got, tt.want)
			}
		})
	}
}

func TestNewDetectorClampsThreshold(t *testing.T) {
	d := NewDetector(0)
	d.Observe(true)
	if got := d.Spans(); len(got) != 1 {
		t.Errorf("threshold 0 clamped to 1; got %d spans, want 1", len(got))
	}
	dn := NewDetector(-5)
	dn.Observe(true)
	if got := dn.Spans(); len(got) != 1 {
		t.Errorf("threshold -5 clamped to 1; got %d spans, want 1", len(got))
	}
}

func TestSpansReturnsNonNilEmpty(t *testing.T) {
	d := NewDetector(5)
	got := d.Spans()
	if got == nil {
		t.Error("Spans() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Spans() len = %d, want 0", len(got))
	}
}

func TestLongRunExceedsThreshold(t *testing.T) {
	d := NewDetector(3)
	for i := 0; i < 10; i++ {
		d.Observe(true)
	}
	got := d.Spans()
	if len(got) != 1 {
		t.Fatalf("got %d spans, want 1", len(got))
	}
	want := Span{StartLine: 1, EndLine: 10, Count: 10}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("span = %v, want %v", got[0], want)
	}
}

func TestPeekNoAlerts(t *testing.T) {
	d := NewDetector(3)
	d.Observe(false)
	d.Observe(false)
	got := d.Peek()
	if len(got) != 0 {
		t.Errorf("Peek() = %v, want empty", got)
	}
	if got == nil {
		t.Error("Peek() returned nil, want non-nil empty slice")
	}
}

func TestPeekReturnsCompletedExcludesInProgress(t *testing.T) {
	d := NewDetector(3)
	// E E E . → flush at '.', completed span {1,3,3} in d.spans
	d.Observe(true)
	d.Observe(true)
	d.Observe(true)
	d.Observe(false)
	// 新的 in-progress 区间 E E(未结束,不应出现在 Peek 结果中)
	d.Observe(true)
	d.Observe(true)
	got := d.Peek()
	want := []Span{{StartLine: 1, EndLine: 3, Count: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Peek() = %v, want %v (in-progress excluded)", got, want)
	}
}

func TestPeekRepeatedConsistent(t *testing.T) {
	d := NewDetector(3)
	d.Observe(true)
	d.Observe(true)
	d.Observe(true)
	d.Observe(false)
	first := d.Peek()
	second := d.Peek()
	if !reflect.DeepEqual(first, second) {
		t.Errorf("repeated Peek inconsistent: %v vs %v", first, second)
	}
}

func TestPeekDoesNotBreakSpans(t *testing.T) {
	d := NewDetector(3)
	// in-progress 区间 E E E E(count=4,未结束)
	d.Observe(true)
	d.Observe(true)
	d.Observe(true)
	d.Observe(true)
	_ = d.Peek() // 不应破坏状态
	got := d.Spans()
	want := []Span{{StartLine: 1, EndLine: 4, Count: 4}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Spans() after Peek() = %v, want %v", got, want)
	}
}

func TestPeekReturnsCopy(t *testing.T) {
	d := NewDetector(3)
	d.Observe(true)
	d.Observe(true)
	d.Observe(true)
	d.Observe(false)
	got := d.Peek()
	if len(got) != 1 {
		t.Fatalf("Peek() len = %d, want 1", len(got))
	}
	got[0].Count = 999 // 修改返回的切片
	again := d.Peek()
	if again[0].Count != 3 {
		t.Errorf("internal state mutated by caller: Peek() Count = %d, want 3", again[0].Count)
	}
}
