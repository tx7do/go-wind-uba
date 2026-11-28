// Package license 为企业版能力提供授权接缝(seam)。
//
// 开源构建使用默认的 NoopProvider:平台始终合法、所有企业能力关闭。
// 企业版构建(go-wind-uba-enterprise 私有仓库)在进程装配时通过
// SetProvider 注入真实实现(离线 license 文件 + ED25519 签名校验)。
//
// 约定:能力(capability)以字符串命名,具体能力词汇表由 enterprise 侧
// 定义;本包不感知任何具体能力名,避免企业特性反向牵引开源代码。
package license

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Provider 是授权状态的只读视图。实现必须保证并发安全。
type Provider interface {
	// Valid 返回 nil 表示当前授权有效;缺失/损坏/过期返回错误。
	// 实现方必须在每次调用时重新判定有效期,不得只在装载时判定一次。
	Valid() error
	// Allows 判断某项能力当前是否已授权且授权有效。
	Allows(capability string) bool
	// Capabilities 返回当前已授权且有效的能力清单(给前端展示用,如 /capabilities)。
	Capabilities() []string
	// Licensee 返回被授权方名称;开源构建返回空串。
	Licensee() string
	// ExpiresAt 返回授权到期时间;开源构建返回零值。
	ExpiresAt() time.Time
}

// NotLicensedError 表示企业能力未被授权,供业务守卫区分错误类型。
type NotLicensedError struct{ Capability string }

func (e *NotLicensedError) Error() string {
	return fmt.Sprintf("capability %q is not licensed (enterprise edition required)", e.Capability)
}

var (
	// ErrNoProvider 表示进程未装配任何授权提供者(异常状态)。
	ErrNoProvider = errors.New("license: no provider configured")

	mu      sync.RWMutex
	current Provider = NoopProvider{}
)

// SetProvider 注入授权提供者;企业版入口进程在启动装配时调用一次。
// 传 nil 重置回开源默认。
func SetProvider(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	if p == nil {
		current = NoopProvider{}
		return
	}
	current = p
}

// Current 返回当前授权提供者。
func Current() Provider {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Enabled 快捷判断某能力当前是否可用。
func Enabled(capability string) bool { return Current().Allows(capability) }

// Capabilities 返回当前已授权的能力清单(开源构建恒为空)。
func Capabilities() []string { return Current().Capabilities() }

// IsConfigured 是否已装配真实授权提供者(企业构建)。
// 用于区分"平台能力"与"仅企业构建可用的入口"(如单点登录回调)。
func IsConfigured() bool {
	_, isNoop := Current().(NoopProvider)
	return !isNoop
}

// Require 作为企业特性守卫:能力未授权时返回 *NotLicensedError。
func Require(capability string) error {
	if Enabled(capability) {
		return nil
	}
	return &NotLicensedError{Capability: capability}
}

// NoopProvider 开源构建的默认实现:平台始终合法,所有企业能力关闭。
type NoopProvider struct{}

func (NoopProvider) Valid() error            { return nil }
func (NoopProvider) Allows(string) bool      { return false }
func (NoopProvider) Capabilities() []string  { return nil }
func (NoopProvider) Licensee() string        { return "" }
func (NoopProvider) ExpiresAt() time.Time    { return time.Time{} }

var _ Provider = NoopProvider{}
