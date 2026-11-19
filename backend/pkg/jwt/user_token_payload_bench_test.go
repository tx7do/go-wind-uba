package jwt

// 本文件覆盖鉴权每请求路径的 benchmark：
//   - payload 构造（登录/刷新发令牌路径）；
//   - payload → AuthClaims 转换（签发路径）；
//   - MapClaims → payload 解析（请求鉴权路径）；
//   - HS256 签名 + 验签完整回环（authn 中间件逐请求成本上限参考）。
//
// 统一 b.ResetTimer() + b.ReportAllocs()。

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tx7do/go-utils/trans"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"
)

// benchPayload 构造一个"全字段"payload，代表最坏情况的每请求开销。
func benchPayload() *authenticationV1.UserTokenPayload {
	ou := int64(66)
	client := "client-bench"
	device := "device-bench"
	ds := identityV1.DataScope_UNIT_AND_CHILD
	p := NewUserTokenPayload("bench-user", 1001, 2002, &ou,
		[]string{"editor", "auditor"}, &ds, &client, &device)
	p.Jti = trans.Ptr("jti-bench-0001")
	return p
}

// buildBenchMapClaims 预生成一份模拟真实 JWT 解析产物（float64 数值 +
// []interface{} 角色列表）的 MapClaims，供解析路径 benchmark 使用。
func buildBenchMapClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"sub":               "bench-user",
		ClaimFieldUserID:    float64(1001),
		ClaimFieldTenantID:  float64(2002),
		ClaimFieldClientID:  "client-bench",
		ClaimFieldDeviceID:  "device-bench",
		ClaimFieldDataScope: identityV1.DataScope_UNIT_AND_CHILD.String(),
		ClaimFieldOrgUnitID: float64(66),
		ClaimFieldRoleCodes: []interface{}{"editor", "auditor"},
	}
}

func BenchmarkNewUserTokenPayload(b *testing.B) {
	ou := int64(66)
	client := "client-bench"
	device := "device-bench"
	ds := identityV1.DataScope_UNIT_AND_CHILD
	roles := []string{"editor", "auditor"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewUserTokenPayload("bench-user", 1001, 2002, &ou, roles, &ds, &client, &device)
	}
}

func BenchmarkNewUserTokenAuthClaims(b *testing.B) {
	p := benchPayload()
	exp := time.Now().Add(DefaultTokenExpiration)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewUserTokenAuthClaims(p, &exp)
	}
}

func BenchmarkNewUserTokenPayloadWithJwtMapClaims(b *testing.B) {
	mc := buildBenchMapClaims()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := NewUserTokenPayloadWithJwtMapClaims(mc); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewUserTokenPayloadWithClaims(b *testing.B) {
	p := benchPayload()
	exp := time.Now().Add(DefaultTokenExpiration)
	claims := NewUserTokenAuthClaims(p, &exp)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := NewUserTokenPayloadWithClaims(claims); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJWTSignAndParse 完整签名+验签回环：authn 中间件每请求的实际成本量级。
func BenchmarkJWTSignAndParse(b *testing.B) {
	p := benchPayload()
	exp := time.Now().Add(DefaultTokenExpiration)
	claims := NewUserTokenAuthClaims(p, &exp)

	mc := make(jwt.MapClaims, len(*claims))
	for k, v := range *claims {
		mc[k] = v
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, mc).SignedString(testSigningKey)
		if err != nil {
			b.Fatal(err)
		}
		parsed, err := jwt.Parse(signed, func(tk *jwt.Token) (interface{}, error) {
			return testSigningKey, nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !parsed.Valid {
			b.Fatalf("parse failed: %v", err)
		}
	}
}
