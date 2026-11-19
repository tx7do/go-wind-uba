package jwt

// 本文件补齐 user_token_payload.go 既有测试（user_token_payload_test.go）的缺口：
//   1. IsTokenExpired / IsTokenNotValidYet —— 鉴权信任根的过期/未生效判定；
//   2. NewUserTokenAuthClaims 的 jti / exp 分支与"最小 payload"形态；
//   3. NewUserTokenPayloadWithClaims / WithJwtMapClaims 对客户端可伪造 claims
//      的异常类型防御（类型不符必须跳过字段而非 panic）；
//   4. 真实 JWT 自签自验回环：AuthClaims → HS256 签名 → 解析 → MapClaims →
//      payload，并验证篡改令牌/换钥验证必须失败。
//
// 硬性约束：签名密钥为测试内嵌密钥，自签自验，不依赖任何基础设施。

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"
	authn "github.com/tx7do/kratos-authn/engine"

	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"
)

// testSigningKey 单元测试签名密钥（HS256 自签自验，非生产密钥）。
var testSigningKey = []byte("unit-test-secret-do-not-use-in-prod")

// ---------------------------------------------------------------------------
// IsTokenExpired / IsTokenNotValidYet
// ---------------------------------------------------------------------------

// TestIsTokenExpired 过期判定：exp 按 JWT 解析惯例必须是 float64
// （authn.AuthClaims.parseNumericDate 只认 float64/json.Number）。
func TestIsTokenExpired(t *testing.T) {
	now := time.Now().UTC()

	cases := []struct {
		name string
		exp  interface{} // nil 表示不携带 exp 声明
		want bool
	}{
		{"nil claims", nil, true},
		{"no exp claim", "NO_EXP", false},
		{"expired beyond leeway(60s)", float64(now.Add(-2 * time.Minute).Unix()), true},
		{"expired within leeway", float64(now.Add(-30 * time.Second).Unix()), false},
		{"future exp", float64(now.Add(time.Hour).Unix()), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "nil claims" {
				assert.True(t, IsTokenExpired(nil))
				return
			}
			claims := &authn.AuthClaims{}
			if tc.exp != "NO_EXP" {
				(*claims)[authn.ClaimFieldExpirationTime] = tc.exp
			}
			assert.Equal(t, tc.want, IsTokenExpired(claims))
		})
	}
}

// TestIsTokenExpired_Int64ExpIsSilentlyIgnored 记录当前行为的回归测试。
//
// NewUserTokenAuthClaims 写入的 exp 是 int64（expirationTime.Unix()），
// 而 authn.AuthClaims.parseNumericDate 只接受 float64/json.Number，
// 因此对这批手工构造的 claims 调 IsTokenExpired 时 exp 会被当作"不存在"，
// 一律返回未过期。生产链路（authenticator.go）的 claims 来自真实 JWT 解析
// （float64），不受影响；仅手工构造 claims 再自查时才有此偏差。
//
// 可疑点：IsTokenExpired 忽略 GetExpirationTime 的 error，类型不符被静默
// 当成"无 exp"。若未来修复（int64 也应识别，或出错时按过期处理），本用例
// 需要同步更新断言。
func TestIsTokenExpired_Int64ExpIsSilentlyIgnored(t *testing.T) {
	claims := &authn.AuthClaims{
		authn.ClaimFieldExpirationTime: time.Now().Add(-time.Hour).Unix(), // int64，已过期
	}
	assert.False(t, IsTokenExpired(claims),
		"当前实现：int64 exp 解析失败被忽略 → 判定为未过期（与 float64 行为不一致）")
}

// TestIsTokenNotValidYet 未生效判定：nbf 在容差（60s）之外的未来才算未生效。
func TestIsTokenNotValidYet(t *testing.T) {
	now := time.Now().UTC()

	cases := []struct {
		name string
		nbf  interface{} // nil 表示不携带 nbf 声明
		want bool
	}{
		{"nil claims", nil, true},
		{"no nbf claim", "NO_NBF", false},
		{"nbf far future", float64(now.Add(time.Hour).Unix()), true},
		{"nbf within leeway", float64(now.Add(30 * time.Second).Unix()), false},
		{"nbf past", float64(now.Add(-time.Hour).Unix()), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "nil claims" {
				assert.True(t, IsTokenNotValidYet(nil))
				return
			}
			claims := &authn.AuthClaims{}
			if tc.nbf != "NO_NBF" {
				(*claims)[authn.ClaimFieldNotBefore] = tc.nbf
			}
			assert.Equal(t, tc.want, IsTokenNotValidYet(claims))
		})
	}
}

// ---------------------------------------------------------------------------
// NewUserTokenAuthClaims 补充分支
// ---------------------------------------------------------------------------

// TestNewUserTokenAuthClaims_WithJtiAndExpiry jti 与 exp 声明的写入分支。
func TestNewUserTokenAuthClaims_WithJtiAndExpiry(t *testing.T) {
	payload := NewUserTokenPayload("carol", 8, 9, nil, nil, nil, nil, nil)
	payload.Jti = trans.Ptr("jwt-id-1234")

	exp := time.Now().Add(DefaultTokenExpiration)
	claims := NewUserTokenAuthClaims(payload, &exp)

	assert.Equal(t, "jwt-id-1234", (*claims)[authn.ClaimFieldJwtID])
	assert.Equal(t, exp.Unix(), (*claims)[authn.ClaimFieldExpirationTime])
	assert.NotEmpty(t, (*claims)[authn.ClaimFieldIssuedAt], "iat 应总是写入")
}

// TestNewUserTokenAuthClaims_MinimalPayload 全可选字段为 nil 时，claims 只含
// 基础四件套（sub/uid/tid/iat），可选字段不得以零值形式出现。
func TestNewUserTokenAuthClaims_MinimalPayload(t *testing.T) {
	payload := NewUserTokenPayload("dan", 1, 2, nil, nil, nil, nil, nil)
	claims := NewUserTokenAuthClaims(payload, nil)

	assert.Equal(t, "dan", (*claims)[ClaimFieldUserName])
	assert.Equal(t, int64(1), (*claims)[ClaimFieldUserID])
	assert.Equal(t, int64(2), (*claims)[ClaimFieldTenantID])
	for _, k := range []string{ClaimFieldRoleCodes, ClaimFieldDeviceID, ClaimFieldClientID, ClaimFieldDataScope, ClaimFieldOrgUnitID, authn.ClaimFieldJwtID, authn.ClaimFieldExpirationTime} {
		_, exists := (*claims)[k]
		assert.False(t, exists, "可选字段 %s 在 payload 未携带时不应写入 claims", k)
	}
}

// ---------------------------------------------------------------------------
// claims → payload 的异常类型防御（claims 可能来自客户端伪造头）
// ---------------------------------------------------------------------------

// TestNewUserTokenPayloadWithClaims_MissingFields 空 claims 应产出零值 payload
// 且不报错（缺字段是常态而非异常）。
func TestNewUserTokenPayloadWithClaims_MissingFields(t *testing.T) {
	payload, err := NewUserTokenPayloadWithClaims(&authn.AuthClaims{})
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Empty(t, payload.GetUsername())
	assert.Zero(t, payload.GetUserId())
	assert.Nil(t, payload.TenantId)
	assert.Nil(t, payload.GetRoles())
	assert.Nil(t, payload.DataScope)
}

// TestNewUserTokenPayloadWithClaims_InvalidClaimTypes 类型不符的 claim 必须
// 被跳过而不是让请求 goroutine panic（GetSubject/GetInt64 等返回 error 时
// 源码只记日志继续）。
func TestNewUserTokenPayloadWithClaims_InvalidClaimTypes(t *testing.T) {
	claims := &authn.AuthClaims{
		authn.ClaimFieldSubject: 123,           // sub 应为 string
		ClaimFieldUserID:        "not-a-num",   // uid 应为数值
		ClaimFieldTenantID:      "999",         // tid 应为数值
		ClaimFieldClientID:      []string{"x"}, // cid 应为 string
		ClaimFieldOrgUnitID:     "42",          // ouid 应为数值
	}
	payload, err := NewUserTokenPayloadWithClaims(claims)
	require.NoError(t, err, "类型不符只跳过字段，不返回错误")
	assert.Empty(t, payload.GetUsername())
	assert.Zero(t, payload.GetUserId())
	assert.Nil(t, payload.TenantId, "类型不符的 tid 应保持指针为 nil")
	assert.Nil(t, payload.ClientId)
	assert.Nil(t, payload.OrgUnitId)
}

// TestNewUserTokenPayloadWithClaims_RolesAndDataScope roleCodes 与 dataScope
// 的还原：roleCodes 支持 []string；dataScope 必须是合法枚举字符串。
func TestNewUserTokenPayloadWithClaims_RolesAndDataScope(t *testing.T) {
	claims := &authn.AuthClaims{
		ClaimFieldRoleCodes: []string{"a", "b"},
		ClaimFieldDataScope: identityV1.DataScope_ALL.String(),
	}
	payload, err := NewUserTokenPayloadWithClaims(claims)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, payload.GetRoles())
	if assert.NotNil(t, payload.DataScope) {
		assert.Equal(t, identityV1.DataScope_ALL, payload.GetDataScope())
	}

	// 非法 dataScope 字符串：跳过该字段（不 panic、不报错）。
	claims2 := &authn.AuthClaims{ClaimFieldDataScope: "not-a-scope"}
	payload2, err := NewUserTokenPayloadWithClaims(claims2)
	require.NoError(t, err)
	assert.Nil(t, payload2.DataScope)
}

// TestNewUserTokenPayloadWithJwtMapClaims_DefenseAgainstForgedClaims 模拟
// 客户端伪造 Authorization 头（甚至不验签 ParseUnverified）时的各种畸形
// claims：全部必须安静跳过，绝不能 panic。
func TestNewUserTokenPayloadWithJwtMapClaims_DefenseAgainstForgedClaims(t *testing.T) {
	mapClaims := jwt.MapClaims{
		ClaimFieldUserID:    "hack",       // 数字字段被塞成字符串
		ClaimFieldTenantID:  true,         // 数字字段被塞成 bool
		ClaimFieldClientID:  12345,        // 字符串字段被塞成数字
		ClaimFieldDeviceID:  9.9,          // 字符串字段被塞成浮点
		ClaimFieldDataScope: 3,            // 枚举字段被塞成数字
		ClaimFieldOrgUnitID: []string{""}, // 数字字段被塞成数组
		// roleCodes 单独测：伪造为数字类型时会走显式 error 分支（见下一条用例）。
	}
	payload, err := NewUserTokenPayloadWithJwtMapClaims(mapClaims)
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Zero(t, payload.GetUserId())
	assert.Zero(t, payload.GetTenantId())
	assert.Empty(t, payload.GetClientId())
	assert.Empty(t, payload.GetDeviceId())
	assert.Nil(t, payload.DataScope)
	assert.Zero(t, payload.GetOrgUnitId())
	assert.Nil(t, payload.GetRoles())

	// roleCodes 被伪造为数字：返回 error（拒绝校验）而非 panic/跳过。
	payload, err = NewUserTokenPayloadWithJwtMapClaims(jwt.MapClaims{ClaimFieldRoleCodes: 777})
	assert.Error(t, err)
	assert.Nil(t, payload)
}

// TestNewUserTokenPayloadWithJwtMapClaims_InvalidRoleCodesType roleCodes 为
// 无法处理的类型时必须显式报错（源码唯一返回 error 的分支）。
func TestNewUserTokenPayloadWithJwtMapClaims_InvalidRoleCodesType(t *testing.T) {
	mapClaims := jwt.MapClaims{ClaimFieldRoleCodes: 123}
	payload, err := NewUserTokenPayloadWithJwtMapClaims(mapClaims)
	assert.Error(t, err)
	assert.Nil(t, payload)
	assert.Equal(t, "invalid roleCodes type", err.Error())
}

// TestNewUserTokenPayloadWithJwtMapClaims_RolesVariants roleCodes 的三种形态：
// []interface{}（JSON 反序列化产物）、[]string（进程内构造产物）、
// 混入非字符串元素时只取字符串部分。
func TestNewUserTokenPayloadWithJwtMapClaims_RolesVariants(t *testing.T) {
	// []interface{} 形态，混入非字符串元素（只保留字符串）。
	payload, err := NewUserTokenPayloadWithJwtMapClaims(jwt.MapClaims{
		ClaimFieldRoleCodes: []interface{}{"admin", 1, nil, "viewer"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"admin", "viewer"}, payload.GetRoles())

	// []string 形态（不经 JSON 往返的进程内 claims）。
	payload, err = NewUserTokenPayloadWithJwtMapClaims(jwt.MapClaims{
		ClaimFieldRoleCodes: []string{"r1"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"r1"}, payload.GetRoles())

	// 空列表 → payload.Roles 保持 nil 而非空切片。
	payload, err = NewUserTokenPayloadWithJwtMapClaims(jwt.MapClaims{
		ClaimFieldRoleCodes: []interface{}{},
	})
	require.NoError(t, err)
	assert.Nil(t, payload.GetRoles())
}

// ---------------------------------------------------------------------------
// 真实 JWT 自签自验回环（签名密钥为测试密钥，无基础设施依赖）
// ---------------------------------------------------------------------------

// signTestClaims 用测试密钥把 AuthClaims 签成 HS256 JWT。
func signTestClaims(t *testing.T, claims *authn.AuthClaims, key []byte) string {
	t.Helper()
	mc := make(jwt.MapClaims, len(*claims))
	for k, v := range *claims {
		mc[k] = v
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, mc).SignedString(key)
	require.NoError(t, err)
	return signed
}

// parseTestToken 用测试密钥解析并校验 JWT（限定 HS256，防算法混淆）。
func parseTestToken(t *testing.T, tokenStr string, key []byte) jwt.MapClaims {
	t.Helper()
	parsed, err := jwt.Parse(tokenStr, func(tk *jwt.Token) (interface{}, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	require.NoError(t, err, "合法签名令牌必须能通过验签")
	require.True(t, parsed.Valid)
	mc, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	return mc
}

// TestJWTRoundtrip_SignVerifyPayload 端到端信任根回环：
// payload → AuthClaims → HS256 签名 → 解析（自动验签）→ MapClaims → payload。
// 全字段在两次转换后必须无损还原（除源码即不还原的 jti，见断言内说明）。
func TestJWTRoundtrip_SignVerifyPayload(t *testing.T) {
	ou := int64(66)
	client := "client-r"
	device := "device-r"
	ds := identityV1.DataScope_UNIT_AND_CHILD

	original := NewUserTokenPayload("roundtrip-user", 1001, 2002, &ou,
		[]string{"editor", "auditor"}, &ds, &client, &device)
	original.Jti = trans.Ptr("jti-abc")

	exp := time.Now().Add(DefaultTokenExpiration)
	claims := NewUserTokenAuthClaims(original, &exp)

	tokenStr := signTestClaims(t, claims, testSigningKey)
	mc := parseTestToken(t, tokenStr, testSigningKey)

	// 自动验签触发的标准声明检查。
	sub, err := mc.GetSubject()
	require.NoError(t, err)
	assert.Equal(t, "roundtrip-user", sub)

	got, err := NewUserTokenPayloadWithJwtMapClaims(mc)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, original.GetUsername(), got.GetUsername())
	assert.Equal(t, original.GetUserId(), got.GetUserId())
	assert.Equal(t, original.GetTenantId(), got.GetTenantId())
	assert.Equal(t, original.GetOrgUnitId(), got.GetOrgUnitId())
	assert.Equal(t, original.GetRoles(), got.GetRoles())
	assert.Equal(t, original.GetDataScope(), got.GetDataScope())
	assert.Equal(t, original.GetClientId(), got.GetClientId())
	assert.Equal(t, original.GetDeviceId(), got.GetDeviceId())

	// 可疑点（只报告不修）：NewUserTokenPayloadWithJwtMapClaims 不读 jti 声明，
	// MapClaims → payload 路径会丢 jti；authn.AuthClaims 路径
	// （NewUserTokenPayloadWithClaims）则正常还原。
	assert.Nil(t, got.Jti, "当前实现从 MapClaims 还原 payload 时丢失 jti")
}

// TestJWTRoundtrip_TamperedTokenRejected 篡改令牌（翻转签名末字符）必须验签失败。
func TestJWTRoundtrip_TamperedTokenRejected(t *testing.T) {
	claims := NewUserTokenAuthClaims(NewUserTokenPayload("tamper", 1, 2, nil, nil, nil, nil, nil), nil)
	tokenStr := signTestClaims(t, claims, testSigningKey)

	last := tokenStr[len(tokenStr)-1]
	tampered := tokenStr[:len(tokenStr)-1] + string(('a'+((last-'a'+1)%26))%26+'a')
	if tampered == tokenStr { // 保险：确保真的改动了
		tampered = tokenStr[:len(tokenStr)-1] + "Z"
	}

	_, err := jwt.Parse(tampered, func(tk *jwt.Token) (interface{}, error) {
		return testSigningKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	assert.Error(t, err, "篡改签名必须验签失败")
}

// TestJWTRoundtrip_WrongKeyRejected 换钥验证必须失败（伪造者拿别的密钥签发
// 的令牌不能通过验签）。
func TestJWTRoundtrip_WrongKeyRejected(t *testing.T) {
	claims := NewUserTokenAuthClaims(NewUserTokenPayload("forged", 1, 2, nil, nil, nil, nil, nil), nil)
	tokenStr := signTestClaims(t, claims, []byte("attacker-key"))

	_, err := jwt.Parse(tokenStr, func(tk *jwt.Token) (interface{}, error) {
		return testSigningKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	assert.Error(t, err, "其他密钥签发的令牌必须验签失败")
}

// TestJWTRoundtrip_ExpiredTokenRejected 过期令牌在 jwt.Parse（自动校验 exp）时
// 必须失败，双保险验证 IsTokenExpired 之外的标准校验链路。
func TestJWTRoundtrip_ExpiredTokenRejected(t *testing.T) {
	payload := NewUserTokenPayload("stale", 1, 2, nil, nil, nil, nil, nil)
	exp := time.Now().Add(-DefaultTokenExpiration) // 已过期 2 小时
	claims := NewUserTokenAuthClaims(payload, &exp)

	tokenStr := signTestClaims(t, claims, testSigningKey)
	_, err := jwt.Parse(tokenStr, func(tk *jwt.Token) (interface{}, error) {
		return testSigningKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	require.Error(t, err, "过期令牌必须被标准 exp 校验拒绝")
	assert.True(t, strings.Contains(fmt.Sprintf("%v", err), "expired"), "错误信息应指向过期: %v", err)
}
