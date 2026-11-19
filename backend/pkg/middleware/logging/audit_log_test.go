package logging

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"

	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
)

const (
	testLoginOperation  = "/uba/v1/login"
	testLogoutOperation = "/uba/v1/logout"
)

func TestComputeRiskScore(t *testing.T) {
	lm := NewLoginAuditLogMiddleware(&options{})

	cases := []struct {
		name string
		log  *auditLoginLog
		want int64
	}{
		{
			// 成功登录、用户/设备/IP 齐全、内网 IP → 0 - 10 = 0(下限截断)。
			name: "全齐内网成功",
			log: &auditLoginLog{
				Status:     trans.Ptr(auditV1.LoginAuditLog_SUCCESS),
				UserId:     trans.Ptr(int64(1001)),
				Username:   trans.Ptr("alice"),
				IpAddress:  trans.Ptr("10.0.0.1"),
				DeviceInfo: &auditV1.DeviceInfo{ClientId: trans.Ptr("cli")},
			},
			want: 0,
		},
		{
			// 失败登录 +50。
			name: "失败登录",
			log: &auditLoginLog{
				Status:     trans.Ptr(auditV1.LoginAuditLog_FAILED),
				UserId:     trans.Ptr(int64(1001)),
				IpAddress:  trans.Ptr("203.0.113.7"),
				DeviceInfo: &auditV1.DeviceInfo{ClientId: trans.Ptr("cli")},
			},
			want: 50,
		},
		{
			// 无 userId 且有 username → +10(疑似账号不存在)。
			name: "未知用户",
			log: &auditLoginLog{
				Status:     trans.Ptr(auditV1.LoginAuditLog_SUCCESS),
				Username:   trans.Ptr("ghost"),
				IpAddress:  trans.Ptr("203.0.113.7"),
				DeviceInfo: &auditV1.DeviceInfo{ClientId: trans.Ptr("cli")},
			},
			want: 10,
		},
		{
			// 无 userId 无 username → +20(匿名请求)。
			name: "匿名",
			log: &auditLoginLog{
				Status:     trans.Ptr(auditV1.LoginAuditLog_SUCCESS),
				IpAddress:  trans.Ptr("203.0.113.7"),
				DeviceInfo: &auditV1.DeviceInfo{ClientId: trans.Ptr("cli")},
			},
			want: 20,
		},
		{
			// 缺 clientId +10、缺 IP +5。
			name: "缺设备缺IP",
			log: &auditLoginLog{
				Status:   trans.Ptr(auditV1.LoginAuditLog_SUCCESS),
				UserId:   trans.Ptr(int64(1001)),
				Username: trans.Ptr("alice"),
			},
			want: 15,
		},
		{
			// 失败 + 匿名 + 缺设备缺 IP + 空 IP:50+20+10+5 = 85。
			name: "多重风险叠加",
			log: &auditLoginLog{
				Status: trans.Ptr(auditV1.LoginAuditLog_FAILED),
			},
			want: 85,
		},
		{
			name: "nil防御",
			log:  nil,
			want: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, lm.computeRiskScore(c.log))
		})
	}
}

func TestLevelFromScore(t *testing.T) {
	lm := NewLoginAuditLogMiddleware(&options{})
	cases := []struct {
		score int64
		want  auditV1.LoginAuditLog_RiskLevel
	}{
		{0, auditV1.LoginAuditLog_LOW},
		{30, auditV1.LoginAuditLog_LOW},
		{31, auditV1.LoginAuditLog_MEDIUM},
		{70, auditV1.LoginAuditLog_MEDIUM},
		{71, auditV1.LoginAuditLog_HIGH},
		{100, auditV1.LoginAuditLog_HIGH},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, lm.levelFromScore(c.score), "levelFromScore(%d)", c.score)
	}
}

func TestComputeRiskFactors(t *testing.T) {
	lm := NewLoginAuditLogMiddleware(&options{})

	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, lm.computeRiskFactors(nil))
	})

	t.Run("失败匿名登录", func(t *testing.T) {
		la := &auditLoginLog{
			Status:        trans.Ptr(auditV1.LoginAuditLog_FAILED),
			RiskScore:     trans.Ptr(uint32(85)),
			FailureReason: trans.Ptr("incorrect password"),
		}
		got := lm.computeRiskFactors(la)
		// 返回必须去重且按字典序稳定排序(便于比对与展示)。
		assert.Contains(t, got, RiskFactorFailedLogin)
		assert.Contains(t, got, RiskFactorAnonymousLogin)
		assert.Contains(t, got, RiskFactorUnknownDevice)
		assert.Contains(t, got, RiskFactorIpMissing)
		assert.Contains(t, got, RiskFactorPasswordFailure)
		assert.Contains(t, got, RiskFactorNoSession)
		assert.Contains(t, got, RiskFactorHighRiskScore)
		for i := 1; i < len(got); i++ {
			assert.True(t, got[i-1] < got[i], "必须升序排序: %v", got)
		}
	})

	t.Run("MFA与内网", func(t *testing.T) {
		la := &auditLoginLog{
			Status:    trans.Ptr(auditV1.LoginAuditLog_SUCCESS),
			UserId:    trans.Ptr(int64(1001)),
			Username:  trans.Ptr("alice"),
			MfaStatus: trans.Ptr("MFA_FAILED"),
			IpAddress: trans.Ptr("192.168.1.1"),
			RequestId: trans.Ptr("rid"),
		}
		got := lm.computeRiskFactors(la)
		assert.Contains(t, got, RiskFactorMfaFailed)
		assert.Contains(t, got, RiskFactorInternalIP)
		assert.NotContains(t, got, RiskFactorNoRequestID)
	})

	t.Run("成功公网低分", func(t *testing.T) {
		la := &auditLoginLog{
			Status:    trans.Ptr(auditV1.LoginAuditLog_SUCCESS),
			UserId:    trans.Ptr(int64(1001)),
			Username:  trans.Ptr("alice"),
			IpAddress: trans.Ptr("203.0.113.7"),
			RequestId: trans.Ptr("rid"),
			RiskScore: trans.Ptr(uint32(0)),
		}
		got := lm.computeRiskFactors(la)
		assert.Contains(t, got, RiskFactorExternalIP)
		assert.Contains(t, got, RiskFactorUnknownDevice)
		assert.NotContains(t, got, RiskFactorLowRiskScore, "score=0 不产生低分因子")
	})
}

func TestLoginAuditHandleFullFlow(t *testing.T) {
	// 完整流:登录操作 + 表单体 → Handle 应写出台账,
	// 字段(身份/IP/UA/请求ID/状态/风险/哈希/签名)全部就位。
	var captured *auditLoginLog
	lm := NewLoginAuditLogMiddleware(&options{
		loginOperation:  testLoginOperation,
		logoutOperation: testLogoutOperation,
		writeLoginLogFunc: func(_ context.Context, data *auditLoginLog) error {
			captured = data
			return nil
		},
	})

	withKratosTransport(t, http.MethodPost, testLoginOperation, map[string]string{
		"X-Forwarded-For": "203.0.113.7, 10.0.0.1",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0",
		"X-Request-ID":    "rid-login-1",
	}, "username=alice&password=secret", func(htr *khttp.Transport) {
		lm.Handle(context.Background(), htr, errors.Unauthorized("WRONG_PASSWORD", "wrong password"))
	})

	require.NotNil(t, captured, "登录操作必须产出审计台账")
	assert.Equal(t, auditV1.LoginAuditLog_LOGIN, *captured.ActionType)
	assert.Equal(t, "alice", *captured.Username)
	assert.Equal(t, "203.0.113.7", *captured.IpAddress)
	assert.Equal(t, "rid-login-1", *captured.RequestId)
	assert.Equal(t, auditV1.LoginAuditLog_FAILED, *captured.Status)
	assert.Equal(t, "WRONG_PASSWORD", *captured.FailureReason)
	assert.Greater(t, *captured.RiskScore, uint32(0))
	assert.NotEmpty(t, captured.RiskFactors)
	assert.NotEmpty(t, *captured.LogHash, "必须计算日志哈希")
	assert.Nil(t, captured.Signature, "未配置密钥时不签名(Handle 不自动生成)")
	require.NotNil(t, captured.DeviceInfo)
	require.NotNil(t, captured.GeoLocation, "地理位置结构必须存在")
	// 离线测试环境无 GeoLite 库:国家/城市应为 nil(查询失败静默),不得 panic。

	// 非登录/登出操作直接忽略。
	var otherCalls int
	lmOther := NewLoginAuditLogMiddleware(&options{
		loginOperation:  testLoginOperation,
		logoutOperation: testLogoutOperation,
		writeLoginLogFunc: func(_ context.Context, _ *auditLoginLog) error {
			otherCalls++
			return nil
		},
	})
	withKratosTransport(t, http.MethodGet, "/uba/v1/other", nil, "", func(htr *khttp.Transport) {
		lmOther.Handle(context.Background(), htr, nil)
	})
	assert.Zero(t, otherCalls, "非登录操作不得写台账")

	// 登出操作 → ActionType=LOGOUT。
	var capturedLogout *auditLoginLog
	lm2 := NewLoginAuditLogMiddleware(&options{
		logoutOperation: testLogoutOperation,
		writeLoginLogFunc: func(_ context.Context, data *auditLoginLog) error {
			capturedLogout = data
			return nil
		},
	})
	withKratosTransport(t, http.MethodPost, testLogoutOperation, nil, "", func(htr *khttp.Transport) {
		lm2.Handle(context.Background(), htr, nil)
	})
	require.NotNil(t, capturedLogout)
	assert.Equal(t, auditV1.LoginAuditLog_LOGOUT, *capturedLogout.ActionType)
	assert.Equal(t, auditV1.LoginAuditLog_SUCCESS, *capturedLogout.Status)
}

func TestApiAuditHandleFullFlow(t *testing.T) {
	// API 审计:非登录操作应产出完整台账(方法/路径/URI/Body/时延/状态码/哈希)。
	var captured *auditApiLog
	am := NewApiAuditLogMiddleware(&options{
		loginOperation: testLoginOperation,
		writeApiLogFunc: func(_ context.Context, data *auditApiLog) error {
			captured = data
			return nil
		},
	})

	withKratosTransport(t, http.MethodPost, "/uba/v1/event-trend", map[string]string{
		"X-Forwarded-For": "203.0.113.7",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0",
		"Referer":         "https://example.com/report",
	}, `{"granularity":"day"}`, func(htr *khttp.Transport) {
		am.Handle(context.Background(), htr, nil, 123)
	})

	require.NotNil(t, captured, "非登录操作必须产出 API 审计台账")
	assert.Equal(t, http.MethodPost, *captured.HttpMethod)
	assert.Equal(t, "/uba/v1/event-trend", *captured.ApiOperation)
	assert.Equal(t, "203.0.113.7", *captured.IpAddress)
	assert.Equal(t, `{"granularity":"day"}`, *captured.RequestBody)
	assert.Equal(t, uint32(123), *captured.LatencyMs)
	assert.Equal(t, uint32(200), *captured.StatusCode)
	assert.True(t, *captured.Success)
	assert.NotEmpty(t, *captured.LogHash)
	assert.Nil(t, captured.Signature, "默认构造无密钥时不签名")
	require.NotNil(t, captured.DeviceInfo)

	// 登录操作必须被 API 审计跳过(由登录审计接管)。
	captured = nil
	withKratosTransport(t, http.MethodPost, testLoginOperation, nil, "", func(htr *khttp.Transport) {
		am.Handle(context.Background(), htr, nil, 1)
	})
	assert.Nil(t, captured, "登录操作不得重复记录 API 审计")

	// 业务失败:状态码与 Success 反映 middleErr。
	captured = nil
	withKratosTransport(t, http.MethodGet, "/uba/v1/event-trend", nil, "", func(htr *khttp.Transport) {
		am.Handle(context.Background(), htr, errors.Unauthorized("UNAUTHORIZED", "no token"), 5)
	})
	require.NotNil(t, captured)
	assert.Equal(t, uint32(401), *captured.StatusCode)
	assert.False(t, *captured.Success)
	assert.Equal(t, "UNAUTHORIZED", *captured.Reason)
}

func TestLoginAuditHandleNilTransport(t *testing.T) {
	// htr/操作不匹配的防御路径:不写台账、不 panic。
	var called bool
	lm := NewLoginAuditLogMiddleware(&options{
		loginOperation: testLoginOperation,
		writeLoginLogFunc: func(_ context.Context, data *auditLoginLog) error {
			called = true
			return nil
		},
	})
	lm.Handle(context.Background(), nil, nil)
	assert.False(t, called)

	lm.Handle(context.Background(), nil, nil)
	assert.False(t, called, "nil transport 不得写台账")
}
