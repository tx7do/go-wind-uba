// Package sso 是外部单点登录(OIDC/SAML/LDAP 等)的校验器接缝。
//
// 开源构建不配置任何实现:相关入口一律返回"不可用"。
// 企业版构建(go-wind-uba-enterprise)在进程装配时注入具体实现
// (如 OIDC 校验器),平台侧协议逻辑保持通用。
package sso

import (
	"context"
	"sync"
)

// Verifier 外部身份校验器。
type Verifier interface {
	// AuthorizeURL 生成 IdP 授权地址与配套 state。
	// state 必须由实现自行防伪(签名+时效),回调时原样带回校验。
	AuthorizeURL(ctx context.Context) (authorizeURL string, state string, err error)

	// Verify 消费 IdP 回调授权码:校验 state、换取并验证 IdP 令牌,
	// 返回已验证身份(提供方、标识类型如 email、标识值)。
	Verify(ctx context.Context, code, state string) (provider, identityType, identifier string, err error)
}

var (
	mu      sync.RWMutex
	current Verifier
)

// SetVerifier 注入校验器;企业版入口进程在启动装配时调用一次,传 nil 清除。
func SetVerifier(v Verifier) {
	mu.Lock()
	defer mu.Unlock()
	current = v
}

// Current 返回当前校验器;开源构建返回 nil。
func Current() Verifier {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Available 是否已配置校验器(企业构建)。
func Available() bool { return Current() != nil }
