package stat

import "sort"

// Entry 表示一个键及其出现次数,用于 Top N 统计输出。
type Entry struct {
	Key   string
	Count int
}

// TopN 返回 counts 中计数最高的 n 个条目,按计数降序排列;
// 计数相同的按键名字典序升序排列以保证输出稳定。
// n <= 0 时返回空切片;n 大于元素数量时返回全部。
func TopN(counts map[string]int, n int) []Entry {
	entries := make([]Entry, 0, len(counts))
	for k, v := range counts {
		entries = append(entries, Entry{Key: k, Count: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Key < entries[j].Key
	})
	if n <= 0 {
		return []Entry{}
	}
	if n > len(entries) {
		n = len(entries)
	}
	return entries[:n]
}
