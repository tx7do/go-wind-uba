package uba

// 上报契约数据模型。字段名与后端 proto（uba/service/v1/report.proto 等的 json_name）
// 逐一对齐，encoding/json 输出 camelCase 并用 omitempty 跳过空值。
//
// 铁律：
//   - tenantId/serverTime/ip 由服务端补全，SDK 不发送（结构体里就没有这些字段）；
//   - 数值标量（durationMs/quantity/score/sessionSeq/level）proto3 无法区分 unset 0，
//     只放 behavior oneof 内才被服务端信任，故顶层不设这些字段。
type envelope struct {
	AppID      string      `json:"appId"`
	AppSecret  string      `json:"appSecret"`
	ClientInfo *clientInfo `json:"clientInfo,omitempty"`
	Events     []*Event    `json:"events"`
}

type clientInfo struct {
	UserAgent string `json:"userAgent,omitempty"`
}

// Event 单条上报事件（proto ReportEvent）。
type Event struct {
	EventType string `json:"eventType"` // "BEHAVIOR" / "RISK"
	EventID   string `json:"eventId"`   // uuid v4，去重键
	EventName string `json:"eventName"`
	EventTime string `json:"eventTime"` // RFC3339 毫秒 UTC

	UserID    int64  `json:"userId,omitempty"`
	DeviceID  string `json:"deviceId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Platform  string `json:"platform,omitempty"`

	EventCategory string             `json:"eventCategory,omitempty"`
	Properties    map[string]string  `json:"properties,omitempty"`
	Metrics       map[string]float64 `json:"metrics,omitempty"`
	TraceID       string             `json:"traceId,omitempty"`

	// 顶层字符串镜像维度（服务端 oneof 优先，顶层兜底；对齐 Web/C#/Flutter SDK 写法）
	EventAction string `json:"eventAction,omitempty"`
	ObjectType  string `json:"objectType,omitempty"`
	ObjectID    string `json:"objectId,omitempty"`
	ObjectName  string `json:"objectName,omitempty"`
	Amount      string `json:"amount,omitempty"`
	OpResult    string `json:"opResult,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`

	// oneof payload：behavior 事件必须带 Behavior（服务端校验 payload 与 eventType 匹配）
	Behavior *behaviorPayload `json:"behavior,omitempty"`
	Risk     *riskPayload     `json:"risk,omitempty"`
}

// behaviorPayload behavior oneof 载荷：数值标量只在这里才有意义。
type behaviorPayload struct {
	EventAction string `json:"eventAction,omitempty"`
	ObjectType  string `json:"objectType,omitempty"`
	ObjectID    string `json:"objectId,omitempty"`
	ObjectName  string `json:"objectName,omitempty"`
	OS          string `json:"os,omitempty"`
	AppVersion  string `json:"appVersion,omitempty"`
	Channel     string `json:"channel,omitempty"`
	Network     string `json:"network,omitempty"`
	DurationMs  int64  `json:"durationMs,omitempty"`
	Amount      string `json:"amount,omitempty"`
	Quantity    int64  `json:"quantity,omitempty"`
	Score       int32  `json:"score,omitempty"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	OpResult    string `json:"opResult,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
	ServerID    string `json:"serverId,omitempty"`
	Level       int32  `json:"level,omitempty"`
}

// RiskEvent 风险事件载荷（risk oneof）。
// riskEventId/occurTime/reportTime 服务端补全，无需填写。
type RiskEvent struct {
	RiskType  string  `json:"riskType,omitempty"`
	RiskLevel string  `json:"riskLevel,omitempty"` // HIGH / MEDIUM / LOW
	RiskScore float64 `json:"riskScore,omitempty"` // 0-100
	RuleID    int64   `json:"ruleId,omitempty"`
	RuleName  string  `json:"ruleName,omitempty"`
	// RuleContext 规则上下文（任意 JSON 对象，对应 google.protobuf.Struct）。
	RuleContext     map[string]any    `json:"ruleContext,omitempty"`
	RelatedEventIDs []string          `json:"relatedEventIds,omitempty"`
	Description     string            `json:"description,omitempty"`
	Evidence        map[string]string `json:"evidence,omitempty"`
	Status          string            `json:"status,omitempty"`
}

type riskPayload RiskEvent

type reportResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	TotalCount   int    `json:"totalCount"`
	SuccessCount int    `json:"successCount"`
	FailedCount  int    `json:"failedCount"`
	RequestID    string `json:"requestId"`
}

type flagsResponse struct {
	Flags map[string]string `json:"flags"`
}

// FlushResult 一次 flush 的结果。
type FlushResult struct {
	// Success 本次 flush 是否成功（可重试失败为 false；4xx 确定性失败也是 false）。
	Success bool
	// Sent 成功送达的事件数。
	Sent int
	// Dropped 未送达的事件数（部分失败/丢弃）。
	Dropped int
	// Retryable 失败是否值得补发（网络错误/5xx/429 为 true，4xx 确定性失败为 false）。
	Retryable bool
	// Message 失败原因。
	Message string
}
