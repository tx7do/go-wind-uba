// Package masking 是字段脱敏接缝:开源构建不配置策略时恒不脱敏;
// 企业版构建(go-wind-uba-enterprise)按 field_permission 能力注入策略实现。
//
// 分工:本包定义"掩码形态"与执行器(开源、通用);策略解析——哪些资源/字段、
// 对哪些角色豁免——由企业版 Strategy 实现决定(闭源)。平台侧调用点只需
// Available() + Maybe(),不感知具体策略。
package masking

import (
	"context"
	"strings"
	"sync"
)

// Kind 掩码形态。
type Kind string

const (
	KindNone  Kind = "none"  // 不脱敏
	KindFull  Kind = "full"  // 全遮蔽:******
	KindEmail Kind = "email" // a***@domain.com
	KindPhone Kind = "phone" // 138****5678
)

// 资源命名约定(企业策略按此对齐)。
const (
	ResourceUser = "user" // 用户:email / mobile / telephone
)

// Strategy 字段掩码策略(企业版实现):返回 resource.field 对当前操作者的
// 掩码形态。ctx 携带操作者身份(pkg/middleware/auth 的令牌负载,含角色);
// targetOwnerID 是被查看记录的归属者 ID(如用户 ID),供"查看自己不脱敏"
// 之类的目标豁免判定,不适用时传 0。
type Strategy interface {
	FieldMask(ctx context.Context, resource, field string, targetOwnerID int64) Kind
}

var (
	mu      sync.RWMutex
	current Strategy
)

// SetStrategy 注入策略;企业版入口进程在启动装配时调用一次,传 nil 清除。
func SetStrategy(s Strategy) {
	mu.Lock()
	defer mu.Unlock()
	current = s
}

// Current 返回当前策略;开源构建返回 nil。
func Current() Strategy {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Available 是否已配置策略(企业构建)。
func Available() bool { return Current() != nil }

// Maybe 对单个可选字符串字段应用掩码:无策略、值为空或形态为 none 时原样返回。
// 不适用目标豁免场景(等价 targetOwnerID=0)。
func Maybe(ctx context.Context, resource, field string, value *string) *string {
	return MaybeFor(ctx, resource, field, value, 0)
}

// MaybeFor 同 Maybe,targetOwnerID 供策略做"看自己不脱敏"等目标豁免。
func MaybeFor(ctx context.Context, resource, field string, value *string, targetOwnerID int64) *string {
	if value == nil || *value == "" || !Available() {
		return value
	}
	kind := Current().FieldMask(ctx, resource, field, targetOwnerID)
	if kind == KindNone || kind == "" {
		return value
	}
	masked := Apply(kind, *value)
	return &masked
}

// Apply 按形态执行掩码;未知形态视为全遮蔽(fail-closed)。
func Apply(kind Kind, value string) string {
	switch kind {
	case KindNone:
		return value
	case KindEmail:
		return maskEmail(value)
	case KindPhone:
		return maskPhone(value)
	default: // KindFull 与未知形态
		return "***"
	}
}

func maskEmail(v string) string {
	at := strings.Index(v, "@")
	if at <= 0 {
		return "***"
	}
	return v[:1] + "***" + v[at:]
}

func maskPhone(v string) string {
	if len(v) < 7 {
		return "***"
	}
	return v[:3] + "****" + v[len(v)-4:]
}
