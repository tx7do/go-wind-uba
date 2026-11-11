// Package data 内的跨引擎共享类型：Doris 与 ClickHouse 的实验结果聚合
// 共用同一返回结构，供服务层以统一接口消费。
package data

// FlagVariantStat A/B 实验的单变体曝光/转化统计。
type FlagVariantStat struct {
	Variant        string
	ExposedUsers   int64
	ConvertedUsers int64
}
