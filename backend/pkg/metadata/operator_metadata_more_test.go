package metadata

// 本文件补齐 operator_metadata_test.go 的缺口：
//   1. operator.go 的三个 Operator 构造器（SYSTEM / SERVICE / USER）；
//   2. FromClientContext 的"client 未命中回退 server context"分支
//      （gRPC handler 内直接 Publish 到 Kafka 的场景依赖该回退）；
//   3. SetOperatorToRequestHeader / GetOperatorFromRequestHeader 的
//      kratos transport.Header 读写回环与错误路径；
//   4. Encode/Decode 的空输入错误与错误语义（全部为 401 UNAUTHORIZED）；
//   5. 每请求元数据读写路径的 benchmark。

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	kratosmetadata "github.com/go-kratos/kratos/v2/metadata"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"
)

// mapHeader 实现 kratos transport.Header 的最小 map 后端，
// 供 request header 读写回环测试使用。
type mapHeader map[string][]string

func (h mapHeader) Get(key string) string {
	if vs, ok := h[key]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func (h mapHeader) Set(key, value string) { h[key] = []string{value} }
func (h mapHeader) Add(key, value string) { h[key] = append(h[key], value) }
func (h mapHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
func (h mapHeader) Values(key string) []string { return h[key] }

// 编译期断言：桩实现了 transport.Header。
var _ transport.Header = mapHeader{}

// ---------------------------------------------------------------------------
// operator.go 构造器
// ---------------------------------------------------------------------------

// TestNewSystemOperator 系统操作员：类型 SYSTEM、UserId 恒为 0、租户透传、
// 数据范围必须是 ALL（系统操作员不受数据权限约束）。
func TestNewSystemOperator(t *testing.T) {
	op := NewSystemOperator(42)
	require.NotNil(t, op)
	assert.Equal(t, authenticationV1.OperatorMetadata_SYSTEM, op.GetType())
	assert.Equal(t, int64(0), op.GetUserId())
	assert.Equal(t, int64(42), op.GetTenantId())
	assert.Equal(t, identityV1.DataScope_ALL, op.GetDataScope())
}

// TestNewServiceOperator 服务操作员：类型 SERVICE，显式 userId，租户透传。
func TestNewServiceOperator(t *testing.T) {
	op := NewServiceOperator(7, 8)
	require.NotNil(t, op)
	assert.Equal(t, authenticationV1.OperatorMetadata_SERVICE, op.GetType())
	assert.Equal(t, int64(7), op.GetUserId())
	assert.Equal(t, int64(8), op.GetTenantId())
	assert.Equal(t, identityV1.DataScope_ALL, op.GetDataScope())
}

// TestNewUserOperator 用户操作员：类型 USER，四个入参逐一透传，
// dataScope 由调用方给定（不隐式放大权限）。
func TestNewUserOperator(t *testing.T) {
	op := NewUserOperator(1, 2, 3, identityV1.DataScope_UNIT_ONLY)
	require.NotNil(t, op)
	assert.Equal(t, authenticationV1.OperatorMetadata_USER, op.GetType())
	assert.Equal(t, int64(1), op.GetUserId())
	assert.Equal(t, int64(2), op.GetTenantId())
	assert.Equal(t, int64(3), op.GetOrgUnitId())
	assert.Equal(t, identityV1.DataScope_UNIT_ONLY, op.GetDataScope())
}

// TestOperatorCtors_SurviveProtoRoundtrip 三种构造器产出的元数据经
// Encode/Decode 回环后语义不变（元数据是跨服务传输的信任载体）。
func TestOperatorCtors_SurviveProtoRoundtrip(t *testing.T) {
	ops := []*authenticationV1.OperatorMetadata{
		NewSystemOperator(10),
		NewServiceOperator(11, 10),
		NewUserOperator(12, 10, 13, identityV1.DataScope_SELF),
	}
	for i, op := range ops {
		str, err := EncodeOperatorMetadata(op)
		require.NoError(t, err, "case %d", i)

		got, err := DecodeOperatorMetadata(str)
		require.NoError(t, err, "case %d", i)
		assert.Equal(t, op.GetType(), got.GetType(), "case %d", i)
		assert.Equal(t, op.GetUserId(), got.GetUserId(), "case %d", i)
		assert.Equal(t, op.GetTenantId(), got.GetTenantId(), "case %d", i)
		assert.Equal(t, op.GetOrgUnitId(), got.GetOrgUnitId(), "case %d", i)
		assert.Equal(t, op.GetDataScope(), got.GetDataScope(), "case %d", i)
	}
}

// ---------------------------------------------------------------------------
// context 读取：回退分支与缺失分支
// ---------------------------------------------------------------------------

// TestFromClientContext_FallbackToServerContext client context 未命中时必须
// 回退到 server context：gRPC server handler 内直接 Publish 到 Kafka 时，
// 操作员元数据只存在于 server context。回退是这条业务路径的命门。
func TestFromClientContext_FallbackToServerContext(t *testing.T) {
	op := NewUserOperator(5, 6, 7, identityV1.DataScope_UNIT_AND_CHILD)
	str, err := EncodeOperatorMetadata(op)
	require.NoError(t, err)

	// 只挂 server context（无 client context）。
	ctx := kratosmetadata.NewServerContext(context.Background(), kratosmetadata.Metadata{
		mdOperatorKey: []string{str},
	})

	got, err := FromClientContext(ctx)
	require.NoError(t, err, "client 未命中必须回退 server context")
	require.NotNil(t, got)
	assert.Equal(t, int64(5), got.GetUserId())

	// 反向：server=false 的 FromContext 也走同一回退。
	got, err = FromContext(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, int64(5), got.GetUserId())
}

// TestFromServerContext_ClientContextOnly server=true 时只认 server context，
// 元数据仅存在于 client context 时不得串读（server 视角必须来自请求头）。
func TestFromServerContext_ClientContextOnly(t *testing.T) {
	str, err := EncodeOperatorMetadata(NewUserOperator(1, 2, 3, identityV1.DataScope_ALL))
	require.NoError(t, err)

	ctx := kratosmetadata.AppendToClientContext(context.Background(), mdOperatorKey, str)

	got, err := FromServerContext(ctx)
	assert.Error(t, err, "server 视角不得读取 client context")
	assert.Nil(t, got)
}

// TestFromContext_NoMetadataAtAll 两种 context 都不存在 → ErrNoMetadata。
func TestFromContext_NoMetadataAtAll(t *testing.T) {
	got, err := FromContext(context.Background(), true)
	assert.ErrorIs(t, err, ErrNoMetadata)
	assert.Nil(t, got)

	got, err = FromContext(context.Background(), false)
	assert.ErrorIs(t, err, ErrNoMetadata)
	assert.Nil(t, got)
}

// TestFromContext_EmptyOperatorValue context 存在但 operator header 为空 →
// ErrNoOperatorHeader（区别于完全没有 context）。
func TestFromContext_EmptyOperatorValue(t *testing.T) {
	ctx := kratosmetadata.NewServerContext(context.Background(), kratosmetadata.Metadata{})
	got, err := FromServerContext(ctx)
	assert.ErrorIs(t, err, ErrNoOperatorHeader)
	assert.Nil(t, got)
}

// TestNewContext_NilInfo nil 元数据编码失败时必须原样返回 ctx 且带错误。
func TestNewContext_NilInfo(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "marker")
	got, err := NewContext(ctx, nil)
	assert.ErrorIs(t, err, ErrNilOperatorMetadata)
	assert.NotNil(t, got)
}

// ---------------------------------------------------------------------------
// Encode / Decode 错误路径
// ---------------------------------------------------------------------------

// TestEncodeOperatorMetadata_Nil 输入 nil → ErrNilOperatorMetadata。
func TestEncodeOperatorMetadata_Nil(t *testing.T) {
	str, err := EncodeOperatorMetadata(nil)
	assert.ErrorIs(t, err, ErrNilOperatorMetadata)
	assert.Empty(t, str)
}

// TestDecodeOperatorMetadata_EmptyAndBroken 空串与坏 base64 必须报错且不 panic。
func TestDecodeOperatorMetadata_EmptyAndBroken(t *testing.T) {
	got, err := DecodeOperatorMetadata("")
	assert.ErrorIs(t, err, ErrEmptyOperatorMetadataString)
	assert.Nil(t, got)

	got, err = DecodeOperatorMetadata("not-base64!!!")
	assert.Error(t, err)
	assert.Nil(t, got)
}

// TestMetadataErrors_AreUnauthorized 本包错误语义约定：所有错误都是
// 401 UNAUTHORIZED 且 reason 固定（上层按此映射 HTTP/gRPC 状态码）。
func TestMetadataErrors_AreUnauthorized(t *testing.T) {
	all := []error{
		ErrNoMetadata, ErrCodecNotInitialized, ErrNoOperatorHeader,
		ErrInvalidOperator, ErrNilOperatorMetadata, ErrEmptyOperatorMetadataString,
		ErrNilRequestHeader, ErrSignatureMismatch,
	}
	for _, e := range all {
		se := errors.FromError(e)
		assert.Equal(t, 401, int(se.Code), "错误 %v 应为 401", e)
		assert.Equal(t, reason, se.Reason, "错误 %v reason 应为 %s", e, reason)
	}
}

// ---------------------------------------------------------------------------
// request header 读写
// ---------------------------------------------------------------------------

// TestRequestHeader_OperatorRoundtrip 写入→读回必须还原同一操作员元数据。
func TestRequestHeader_OperatorRoundtrip(t *testing.T) {
	op := NewUserOperator(101, 202, 303, identityV1.DataScope_SELECTED_UNITS)

	h := mapHeader{}
	require.NoError(t, SetOperatorToRequestHeader(h, op))

	got, err := GetOperatorFromRequestHeader(h)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, op.GetUserId(), got.GetUserId())
	assert.Equal(t, op.GetTenantId(), got.GetTenantId())
	assert.Equal(t, op.GetOrgUnitId(), got.GetOrgUnitId())
	assert.Equal(t, op.GetDataScope(), got.GetDataScope())
}

// TestRequestHeader_NilHeader nil header 必须报 ErrNilRequestHeader 而非 panic。
func TestRequestHeader_NilHeader(t *testing.T) {
	err := SetOperatorToRequestHeader(nil, NewSystemOperator(1))
	assert.ErrorIs(t, err, ErrNilRequestHeader)

	got, err := GetOperatorFromRequestHeader(nil)
	assert.ErrorIs(t, err, ErrNilRequestHeader)
	assert.Nil(t, got)
}

// TestRequestHeader_MissingOrInvalid 无 header 值 → ErrNoOperatorHeader；
// 值损坏 → ErrInvalidOperator（不是底层解码错误，避免向上层泄漏细节）。
func TestRequestHeader_MissingOrInvalid(t *testing.T) {
	h := mapHeader{}
	got, err := GetOperatorFromRequestHeader(h)
	assert.ErrorIs(t, err, ErrNoOperatorHeader)
	assert.Nil(t, got)

	h2 := mapHeader{mdOperatorKey: []string{"not-base64!!!"}}
	got, err = GetOperatorFromRequestHeader(h2)
	assert.ErrorIs(t, err, ErrInvalidOperator)
	assert.Nil(t, got)

	bad := base64.RawStdEncoding.EncodeToString([]byte("junk-proto"))
	h3 := mapHeader{mdOperatorKey: []string{bad}}
	got, err = GetOperatorFromRequestHeader(h3)
	assert.ErrorIs(t, err, ErrInvalidOperator)
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// benchmark：每请求元数据读写路径
// ---------------------------------------------------------------------------

// BenchmarkOperatorMetadataContextRoundtrip 每请求链路的完整成本：
// 构造（Encode + 挂 context）→ 读取（取 md + Decode）。
func BenchmarkOperatorMetadataContextRoundtrip(b *testing.B) {
	op := NewUserOperator(101, 202, 303, identityV1.DataScope_UNIT_AND_CHILD)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, err := NewContext(context.Background(), op)
		if err != nil {
			b.Fatal(err)
		}
		got, err := FromClientContext(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if got.GetUserId() != op.GetUserId() {
			b.Fatal("roundtrip mismatch")
		}
	}
}

// BenchmarkEncodeDecodeOperatorMetadata 纯编码/解码成本（不含 context 开销）。
func BenchmarkEncodeDecodeOperatorMetadata(b *testing.B) {
	op := NewUserOperator(101, 202, 303, identityV1.DataScope_UNIT_AND_CHILD)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		str, err := EncodeOperatorMetadata(op)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := DecodeOperatorMetadata(str); err != nil {
			b.Fatal(err)
		}
	}
}
