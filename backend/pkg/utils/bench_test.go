package utils

import "testing"

// BenchmarkFilterBlacklist 黑名单过滤(1e4 数据 × 1e3 黑名单,原地算法)。
func BenchmarkFilterBlacklist(b *testing.B) {
	data := make([]string, 10000)
	for i := range data {
		data[i] = "item-" + string(rune('a'+i%26)) + string(rune('0'+i%10))
	}
	blacklist := make([]string, 1000)
	for i := range blacklist {
		blacklist[i] = "item-a" + string(rune('0'+i%10))
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		work := append([]string(nil), data...)
		b.StartTimer()
		if got := FilterBlacklist(work, blacklist); len(got) == 0 {
			b.Fatal("unexpected empty result")
		}
	}
}

// BenchmarkNumberSliceToString ID 列表 → CSV 串(权限/分群查询拼接用)。
func BenchmarkNumberSliceToString(b *testing.B) {
	ids := make([]int64, 1000)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if NumberSliceToString(ids) == "" {
			b.Fatal("unexpected empty string")
		}
	}
}
