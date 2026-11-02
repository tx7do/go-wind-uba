package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tx7do/go-utils/trans"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/aiprovider"
)

func TestLangName(t *testing.T) {
	assert.Equal(t, "English", langName("en"))
	assert.Equal(t, "English", langName("en-US"))
	assert.Equal(t, "中文", langName("zh-CN"))
	assert.Equal(t, "中文", langName("ja"))
	assert.Equal(t, "中文", langName(""), "未知语言回退中文")
}

func TestPtrStrOrAndPtrIntOr(t *testing.T) {
	s := "v"
	assert.Equal(t, "v", ptrStrOr(&s, "def"))
	assert.Equal(t, "def", ptrStrOr(nil, "def"))
	assert.Equal(t, "def", ptrStrOr(trans.Ptr(""), "def"), "空串视为未配置回退默认")

	n := 3
	assert.Equal(t, 3, ptrIntOr(&n, 9))
	assert.Equal(t, 9, ptrIntOr(nil, 9))
}

func TestModelTypeToProtoInt(t *testing.T) {
	// ent 枚举是 string 底层，映射到 proto 数值。
	local := aiprovider.ModelTypeLOCAL
	cloud := aiprovider.ModelTypeCLOUD
	assert.Equal(t, int32(1), modelTypeToProtoInt(&local))
	assert.Equal(t, int32(2), modelTypeToProtoInt(&cloud))

	// 未知枚举回退 UNSPECIFIED(0)，不 panic。
	unknown := aiprovider.ModelType("weird")
	assert.Equal(t, int32(0), modelTypeToProtoInt(&unknown))
	assert.Equal(t, int32(0), modelTypeToProtoInt(nil), "nil 指针回退 UNSPECIFIED")
}

func TestNewOpenAIClientForProvider_CloudProviderRequiresApiKey(t *testing.T) {
	cloud := aiprovider.ModelTypeCLOUD
	_, err := newOpenAIClientForProvider(&ent.AiProvider{
		BaseURL:   trans.Ptr("https://api.example.com"),
		ModelType: &cloud,
	})
	assert.Error(t, err, "云模型缺 api_key 应报配置错误而非 panic")

	local := aiprovider.ModelTypeLOCAL
	_, err = newOpenAIClientForProvider(&ent.AiProvider{
		BaseURL:   trans.Ptr("http://127.0.0.1:11434"),
		ModelType: &local,
	})
	assert.NoError(t, err, "本地模型无需 api_key 应能构造客户端")
}
