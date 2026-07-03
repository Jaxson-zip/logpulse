package alert

// Span 表示一个连续 ERROR 告警区间。
// StartLine/EndLine 为被观察(过滤后)序列中的 1-based 行号。
type Span struct {
	StartLine int
	EndLine   int
	Count     int
}

// Detector 跟踪连续 ERROR 行,在区间结束且长度达到阈值时生成告警。
// 零值 Detector 阈值为 0,不会产生任何告警。
type Detector struct {
	threshold int
	start     int // 当前连续区间起始行号;0 表示未在区间内
	end       int // 当前区间最后一条 ERROR 行号
	count     int // 当前连续 ERROR 计数
	line      int // 已观察行数
	spans     []Span
}

// NewDetector 创建阈值检测器;threshold < 1 时 clamp 为 1。
func NewDetector(threshold int) *Detector {
	if threshold < 1 {
		threshold = 1
	}
	return &Detector{threshold: threshold}
}

// Observe 处理一行;isError 为 true 表示该行为 ERROR 级别。
// 非 ERROR 行会结束当前区间(若满足阈值则记录告警)。
func (d *Detector) Observe(isError bool) {
	d.line++
	if isError {
		if d.count == 0 {
			d.start = d.line
		}
		d.end = d.line
		d.count++
		return
	}
	d.flush()
}

// flush 结束当前连续区间,满足阈值则记录一条告警并重置状态。
func (d *Detector) flush() {
	if d.count >= d.threshold && d.threshold > 0 {
		d.spans = append(d.spans, Span{
			StartLine: d.start,
			EndLine:   d.end,
			Count:     d.count,
		})
	}
	d.start = 0
	d.end = 0
	d.count = 0
}

// Spans 返回所有告警区间;会先 flush 当前未结束的区间。
// 返回值始终为非 nil 切片(无告警时为空切片)。
func (d *Detector) Spans() []Span {
	d.flush()
	if d.spans == nil {
		return []Span{}
	}
	return d.spans
}
