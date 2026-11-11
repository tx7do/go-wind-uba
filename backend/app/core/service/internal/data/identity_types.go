package data

// IdentityPair 事件表里聚合出的 登录用户↔设备 身份对。
// ID-Mapping 引擎的回扫结果，Doris/ClickHouse 两个 OLAP 仓库共用同一返回类型。
type IdentityPair struct {
	TenantID int64
	UserID   int64
	DeviceID string
}
