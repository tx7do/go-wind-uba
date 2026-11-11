package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterBlacklist(t *testing.T) {
	// 原地过滤：返回值与底层数组共享，len 收缩 cap 不变。
	data := []string{"a", "b", "c", "b", "d"}
	got := FilterBlacklist(data, []string{"b"})
	assert.Equal(t, []string{"a", "c", "d"}, got)

	// 空黑名单：原样保留。
	assert.Equal(t, []string{"a", "b"}, FilterBlacklist([]string{"a", "b"}, nil))

	// 全部命中：返回空切片（非 nil）。
	got = FilterBlacklist([]string{"x"}, []string{"x"})
	assert.NotNil(t, got)
	assert.Empty(t, got)

	// 空 data：返回空结果。
	assert.Empty(t, FilterBlacklist(nil, []string{"x"}))

	// 重复黑名单项也是同一个键。
	assert.Equal(t, []string{"a"}, FilterBlacklist([]string{"a", "b"}, []string{"b", "b"}))
}

func TestNumberSliceToString(t *testing.T) {
	assert.Equal(t, "1,2,3", NumberSliceToString([]int64{1, 2, 3}))
	assert.Equal(t, "42", NumberSliceToString([]int64{42}))
	assert.Equal(t, "", NumberSliceToString(nil))
	assert.Equal(t, "", NumberSliceToString([]int64{}))
	assert.Equal(t, "4294967295", NumberSliceToString([]int64{4294967295}), "int64 上界不得溢出")
}
