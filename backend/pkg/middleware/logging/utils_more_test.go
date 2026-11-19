package logging

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"
	"google.golang.org/protobuf/proto"

	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
)

// newHeaderReq 构造带头部/远端地址的测试请求。
func newHeaderReq(headers map[string]string, remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	return req
}

func TestClientRealIP(t *testing.T) {
	// XFF 链首(与 auth 中间件同口径:最外层代理覆写,链首为真实客户端)。
	req := newHeaderReq(map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.1"}, "10.0.0.5:1")
	assert.Equal(t, "203.0.113.7", ClientRealIP(req))

	// XFF 全部非法时跳过,回退 X-Real-IP。
	req = newHeaderReq(map[string]string{
		"X-Forwarded-For": "garbage",
		"X-Real-IP":       "198.51.100.2",
	}, "10.0.0.5:1")
	assert.Equal(t, "198.51.100.2", ClientRealIP(req))

	// 无任何代理头时取 RemoteAddr 并去端口。
	req = newHeaderReq(nil, "198.51.100.77:5678")
	assert.Equal(t, "198.51.100.77", ClientRealIP(req))

	// IPv6 RemoteAddr 去端口。
	req = newHeaderReq(nil, "[2001:db8::68]:443")
	assert.Equal(t, "2001:db8::68", ClientRealIP(req))

	// nil 请求防御。
	assert.Equal(t, "", ClientRealIP(nil))
}

func TestGetRequestId(t *testing.T) {
	// nil 请求返回空串。
	assert.Equal(t, "", getRequestId(nil))

	// 优先 X-Request-ID。
	req := newHeaderReq(map[string]string{
		"X-Request-ID":     "rid-1",
		"X-Correlation-ID": "cid-1",
		"x-fc-request-id":  "fc-1",
	}, "")
	assert.Equal(t, "rid-1", getRequestId(req))

	// 其次 X-Correlation-ID。
	req = newHeaderReq(map[string]string{"X-Correlation-ID": "cid-1", "x-fc-request-id": "fc-1"}, "")
	assert.Equal(t, "cid-1", getRequestId(req))

	// 再次函数计算请求 ID。
	req = newHeaderReq(map[string]string{"x-fc-request-id": "fc-1"}, "")
	assert.Equal(t, "fc-1", getRequestId(req))

	// 全缺失时生成非空 GUID。
	req = newHeaderReq(nil, "")
	got := getRequestId(req)
	assert.NotEmpty(t, got)
	assert.NotEqual(t, got, getRequestId(newHeaderReq(nil, "")), "两次生成应互不相同")
}

func TestGetClientID(t *testing.T) {
	// 优先自定义 X-Client-IP 头。
	req := newHeaderReq(map[string]string{"X-Client-IP": "cli-1"}, "")
	assert.Equal(t, "cli-1", getClientID(req, nil))

	// 其次取 JWT 载荷的 ClientId。
	req = newHeaderReq(nil, "")
	token := &authenticationV1.UserTokenPayload{ClientId: trans.Ptr("cli-from-token")}
	assert.Equal(t, "cli-from-token", getClientID(req, token))

	// 头优先于 token。
	req = newHeaderReq(map[string]string{"X-Client-IP": "cli-header"}, "")
	assert.Equal(t, "cli-header", getClientID(req, token))

	// 全缺失返回空串。
	assert.Equal(t, "", getClientID(newHeaderReq(nil, ""), nil))
	assert.Equal(t, "", getClientID(nil, token))
}

func TestGetStatusCode(t *testing.T) {
	// 无错误 → 200 成功。
	code, reason, success := getStatusCode(nil)
	assert.Equal(t, int64(200), code)
	assert.Equal(t, "", reason)
	assert.True(t, success)

	// kratos 401 错误 → 401 失败,reason 透传。
	kerr := errors.Unauthorized("WRONG_PASSWORD", "wrong password")
	code, reason, success = getStatusCode(kerr)
	assert.Equal(t, int64(401), code)
	assert.Equal(t, "WRONG_PASSWORD", reason)
	assert.False(t, success)

	// 非 kratos 错误包装为 5xx 失败。
	code, _, success = getStatusCode(errors.New(503, "UNAVAILABLE", "unavailable"))
	assert.Equal(t, int64(503), code)
	assert.False(t, success)
}

func TestParseUsernameFromBytes(t *testing.T) {
	// JSON 标准形态。
	u, err := parseUsernameFromBytes([]byte(`{"username":"alice","password":"x"}`))
	require.NoError(t, err)
	assert.Equal(t, "alice", u)

	// JSON 冒号后带空格的形态(正则容忍 \s*)。
	u, err = parseUsernameFromBytes([]byte(`{"username" : "bob"}`))
	require.NoError(t, err)
	assert.Equal(t, "bob", u)

	// 表单形态。
	u, err = parseUsernameFromBytes([]byte("username=carol&password=y"))
	require.NoError(t, err)
	assert.Equal(t, "carol", u)

	// 均无 → 报错。
	_, err = parseUsernameFromBytes([]byte(`{"foo":1}`))
	assert.Error(t, err)
	_, err = parseUsernameFromBytes([]byte(""))
	assert.Error(t, err)
}

func TestExtractUsernameFromRequest(t *testing.T) {
	// JSON 体。
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"alice"}`))
	u, err := extractUsernameFromRequest(req)
	require.NoError(t, err)
	assert.Equal(t, "alice", u)

	// 表单体。
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString("username=carol"))
	u, err = extractUsernameFromRequest(req)
	require.NoError(t, err)
	assert.Equal(t, "carol", u)

	// 提取后 Body 必须可重读(中间件不能消费掉业务 handler 的请求体)。
	body2, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, "username=carol", string(body2))

	// 空 body:第二分支 url.ParseQuery 成功但无 username → 返回空串且无错误
	// 【现状钉死】调用方以 username=="" 判定缺失。
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(""))
	u, err = extractUsernameFromRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, "", u)

	// nil 请求防御。
	_, err = extractUsernameFromRequest(nil)
	assert.Error(t, err)
}

func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"10.1.2.3", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false}, // 172.32 已出私网段
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"169.254.1.1", true},
		{"::1", true},
		{"fc00::1", true},
		{"fd12::1", true}, // fc00::/7 覆盖 fd00::/8
		{"203.0.113.7", false},
		{"8.8.8.8", false},
		{"2606:4700::1111", false},
		{"not-an-ip", false},
		{"", false},
		{" 10.0.0.1 ", true}, // 首尾空白应裁剪后判定
	}
	for _, c := range cases {
		assert.Equal(t, c.want, isPrivateIP(c.ip), "isPrivateIP(%q)", c.ip)
	}
}

func TestTokenHash(t *testing.T) {
	// 已知输入的 SHA-256(固定预期,防止算法被误改)。
	assert.Equal(t,
		"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		tokenHash("test"),
	)

	// 形态:64 位小写十六进制。
	h := tokenHash("another-token")
	assert.Len(t, h, 64)
	assert.Equal(t, strings.ToLower(h), h)
	assert.Equal(t, h, tokenHash("another-token"), "同输入必须得到同摘要")

	// 空串也要有确定性摘要。
	assert.Len(t, tokenHash(""), 64)
}

func TestGenerateECDSAKeyPair(t *testing.T) {
	priv, pub, err := generateECDSAKeyPair()
	require.NoError(t, err)
	require.NotNil(t, priv)
	require.NotNil(t, pub)
	assert.True(t, priv.PublicKey.Equal(pub))
	assert.True(t, pub.Curve == elliptic.P256(), "应为 secp256r1 曲线")

	// 密钥可用:对固定摘要签名后可用公钥验签。
	digest := sha256.Sum256([]byte("audit-log-content"))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	require.NoError(t, err)
	assert.True(t, ecdsa.Verify(pub, digest[:], r, s))
}

func TestEncodeDER(t *testing.T) {
	// 小值:r=1,s=2 → DER 结构 0x30 len(0x02 0x01 0x01)(0x02 0x01 0x02)。
	der, err := encodeDER(bigIntFromInt64(1), bigIntFromInt64(2))
	require.NoError(t, err)
	assert.Equal(t, []byte{0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02}, der)

	// 高位为 1 的 r 需补 0x00 前缀(否则会被解析为负数)。
	// r=[0x00,0xff,0x01](len 3)、s=[0x02](len 1),总长 = 2+3+2+1 = 8。
	rVal := bigIntFromBytes([]byte{0xFF, 0x01})
	sVal := bigIntFromBytes([]byte{0x02})
	der, err = encodeDER(rVal, sVal)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x30, 0x08, 0x02, 0x03, 0x00, 0xff, 0x01, 0x02, 0x01, 0x02}, der)
}

func TestEncodeDERWithRealSignature(t *testing.T) {
	// 端到端:真实密钥签名 → encodeDER → ecdsa.VerifyASN1 验签成功。
	priv, pub, err := generateECDSAKeyPair()
	require.NoError(t, err)

	digest := sha256.Sum256([]byte("verify-der-roundtrip"))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	require.NoError(t, err)

	der, err := encodeDER(r, s)
	require.NoError(t, err)
	assert.True(t, ecdsa.VerifyASN1(pub, digest[:], der), "DER 编码必须能被标准库验签")
}

func TestDetectPlatformFromUA(t *testing.T) {
	cases := []struct {
		ua   string
		want string
	}{
		{"", PlatformOther},
		{"okhttp/4.9.0", PlatformAndroidApp},
		{"Dalvik/2.1.0 (Linux; U; Android 12)", PlatformAndroidApp},
		{"Mozilla/5.0 (Linux; Android 12; SM-G991B) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36 build/HMMA", PlatformAndroidApp},
		{"Android com.example.app/1.0", PlatformAndroidApp},
		{"CFNetwork/1410 Darwin/22.4.0", PlatformiOSApp},
		{"MyApp/1.0 CFNetwork Darwin", PlatformiOSApp},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1", PlatformiOSApp},
		{"Electron/1.0 Windows_NT 10.0", PlatformDesktopWindows},
		{"appname=MyApp macintosh", PlatformDesktopMac},
		{"MyDesktopApp node.js linux x11", PlatformDesktopLinux},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36", PlatformWeb},
		{"Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0", PlatformWeb},
		{"curl/8.0", PlatformOther},
		{"   ", PlatformOther},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, detectPlatformFromUA(c.ua), "detectPlatformFromUA(%q)", c.ua)
	}

	// 【现状钉死】iPhone/iPad UA 命中 iOS 原生指示器分支,排在 Web 浏览器
	// 分支之前 —— iPhone Safari 网页请求会被判成 iOSApp。若后续调整判定
	// 顺序,本用例失败属预期,请同步更新。
	assert.Equal(t, PlatformiOSApp, detectPlatformFromUA(
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Safari/604.1"))
}

// ── 审计中间件公共部分 ──

func TestHashLogDeterministic(t *testing.T) {
	lm := NewLoginAuditLogMiddleware(&options{})

	log1 := &auditLoginLog{Username: trans.Ptr("alice"), UserId: trans.Ptr(int64(42))}

	// 同字段两次哈希一致(确定性序列化)。
	h1 := lm.hashLog(cloneLoginLog(log1))
	h2 := lm.hashLog(cloneLoginLog(log1))
	assert.NotEmpty(t, h1)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64)

	// 字段不同 → 哈希不同。
	log2 := cloneLoginLog(log1)
	log2.Username = trans.Ptr("bob")
	assert.NotEqual(t, h1, lm.hashLog(log2))

	// log_hash/signature 字段本身必须排除在哈希之外。
	log3 := cloneLoginLog(log1)
	log3.LogHash = trans.Ptr("pre-existing-hash")
	log3.Signature = []byte{0x01, 0x02}
	assert.Equal(t, h1, lm.hashLog(log3), "hash 计算必须忽略 log_hash 与 signature")

	// nil 防御。
	assert.Equal(t, "", lm.hashLog(nil))
}

func TestSignatureRoundtrip(t *testing.T) {
	priv, pub, err := generateECDSAKeyPair()
	require.NoError(t, err)

	lm := NewLoginAuditLogMiddleware(&options{ecPrivateKey: priv})

	la := &auditLoginLog{
		UserId:   trans.Ptr(int64(1001)),
		TenantId: trans.Ptr(int64(42)),
		Username: trans.Ptr("alice"),
		LogHash:  trans.Ptr("deadbeef"),
	}
	sig := lm.signature(la)
	require.NotEmpty(t, sig)

	// 用公钥按标准 ASN.1 验签:签名内容为 tenant_id+user_id+username+log_hash。
	type signContent struct {
		TenantID int64  `json:"tenant_id"`
		UserID   int64  `json:"user_id"`
		Username string `json:"username"`
		Sec      int64  `json:"sec"`
		Nanos    int32  `json:"nanos"`
		LogHash  string `json:"log_hash"`
	}
	scBytes, err := jsonMarshal(signContent{
		TenantID: 42, UserID: 1001, Username: "alice", LogHash: "deadbeef",
	})
	require.NoError(t, err)
	digest := sha256.Sum256(scBytes)
	assert.True(t, ecdsa.VerifyASN1(pub, digest[:], sig), "签名必须可用公钥按 ASN.1 验证")

	// 无私钥 → 不签名。
	lmNoKey := NewLoginAuditLogMiddleware(&options{})
	assert.Nil(t, lmNoKey.signature(la))

	// nil 防御。
	assert.Nil(t, lm.signature(nil))
}

func TestSignatureDERVerifiableForApiLog(t *testing.T) {
	// API 审计签名同样走 DER + ASN.1,内容不含 username(与登录审计的差异)。
	priv, pub, err := generateECDSAKeyPair()
	require.NoError(t, err)

	am := NewApiAuditLogMiddleware(&options{ecPrivateKey: priv})
	al := &auditApiLog{UserId: trans.Ptr(int64(1001)), LogHash: trans.Ptr("cafe")}
	sig := am.signature(al)
	require.NotEmpty(t, sig)

	type signContent struct {
		TenantID int64  `json:"tenant_id"`
		UserID   int64  `json:"user_id"`
		Sec      int64  `json:"sec"`
		Nanos    int32  `json:"nanos"`
		LogHash  string `json:"log_hash"`
	}
	scBytes, err := jsonMarshal(signContent{UserID: 1001, LogHash: "cafe"})
	require.NoError(t, err)
	digest := sha256.Sum256(scBytes)
	assert.True(t, ecdsa.VerifyASN1(pub, digest[:], sig))
}

func TestMiddlewareName(t *testing.T) {
	// 中间件名用于链路追踪与排序,必须稳定。
	assert.Equal(t, "LoginAuditLogMiddleware", NewLoginAuditLogMiddleware(&options{}).Name())
	assert.Equal(t, "ApiAuditLogMiddleware", NewApiAuditLogMiddleware(&options{}).Name())
}

// ── 构造真实 kratos HTTP 传输上下文的辅助(进程内直连 ServeHTTP) ──

// withKratosTransport 直连真实 kratos HTTP server 拿到生产一致的 *khttp.Transport。
func withKratosTransport(
	t testing.TB,
	method, path string,
	headers map[string]string,
	body string,
	run func(htr *khttp.Transport),
) {
	t.Helper()

	srv := khttp.NewServer()
	srv.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		tr, ok := kratosTransportFromContext(r.Context())
		require.True(t, ok)
		htr, ok := tr.(*khttp.Transport)
		require.True(t, ok)
		run(htr)
	})

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	srv.ServeHTTP(httptest.NewRecorder(), req)
}

func TestExtractAuthToken(t *testing.T) {
	// 无 Authorization 头 → nil。
	withKratosTransport(t, http.MethodGet, "/t", nil, "", func(htr *khttp.Transport) {
		assert.Nil(t, extractAuthToken(htr))
	})

	// 无效 token → nil(解析失败不得 panic)。
	withKratosTransport(t, http.MethodGet, "/t", map[string]string{
		"Authorization": "Bearer not-a-jwt",
	}, "", func(htr *khttp.Transport) {
		assert.Nil(t, extractAuthToken(htr))
	})
}

func TestFillDeviceInfo(t *testing.T) {
	// 空请求头:UA 解析为 OTHER 设备类型,client id 为空。
	withKratosTransport(t, http.MethodGet, "/t", nil, "", func(htr *khttp.Transport) {
		info := fillDeviceInfo(htr, nil)
		require.NotNil(t, info)
		assert.Equal(t, auditDeviceInfo_OTHER, *info.DeviceType)
		assert.Empty(t, *info.ClientName)
	})

	// Chrome 桌面 UA:各字段应正确解析。
	withKratosTransport(t, http.MethodGet, "/t", map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36",
	}, "", func(htr *khttp.Transport) {
		info := fillDeviceInfo(htr, nil)
		require.NotNil(t, info)
		assert.Equal(t, auditDeviceInfo_DESKTOP, *info.DeviceType)
		assert.Equal(t, "PC", *info.ClientName)
		assert.Equal(t, "Chrome", *info.BrowserName)
		assert.Equal(t, "Chrome", *info.BrowserName)
		assert.Equal(t, "Windows", *info.OsName)
		assert.NotEmpty(t, *info.UserAgent)
		assert.Equal(t, PlatformWeb, *info.Platform)
	})
}

// ── 测试辅助:类型别名与最小构造(避免断言处出现冗长的生成类型名) ──

type auditLoginLog = auditV1.LoginAuditLog
type auditApiLog = auditV1.ApiAuditLog

const (
	auditDeviceInfo_OTHER   = auditV1.DeviceInfo_OTHER
	auditDeviceInfo_DESKTOP = auditV1.DeviceInfo_DESKTOP
	auditDeviceInfo_TABLET  = auditV1.DeviceInfo_TABLET
	auditDeviceInfo_MOBILE  = auditV1.DeviceInfo_MOBILE
	auditDeviceInfo_BOT     = auditV1.DeviceInfo_BOT
)

func bigIntFromInt64(v int64) *big.Int  { return big.NewInt(v) }
func bigIntFromBytes(b []byte) *big.Int { return new(big.Int).SetBytes(b) }
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func cloneLoginLog(src *auditLoginLog) *auditLoginLog {
	return proto.Clone(src).(*auditLoginLog)
}

func kratosTransportFromContext(ctx context.Context) (kratosTransport.Transporter, bool) {
	return kratosTransport.FromServerContext(ctx)
}
