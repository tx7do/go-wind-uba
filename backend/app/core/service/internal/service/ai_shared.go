package service

import (
	"context"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/go-utils/crypto"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
	"go-wind-uba/app/core/service/internal/data/ent"
	aiprovider "go-wind-uba/app/core/service/internal/data/ent/aiprovider"

	pkgAi "go-wind-uba/pkg/ai"
)

// viewerUserIDFromContext 从 viewer context 提取操作人的用户 ID。
// core 服务经由 ent middleware 注入 viewer context（来源为 gRPC metadata 中的 OperatorMetadata）。
// 系统上下文（初始化、定时任务等）不存在 viewer，返回 (0, false)。
func viewerUserIDFromContext(ctx context.Context) (int64, bool) {
	vc, exist := viewer.FromContext(ctx)
	if !exist || vc == nil {
		return 0, false
	}
	uid := vc.UserID()
	if uid == 0 {
		return 0, false
	}
	return int64(uid), true
}

// newOpenAIClientForProvider 从 provider 表行构造客户端（api_key 密文就地解密，
// 不出 service 层）。
func newOpenAIClientForProvider(provider *ent.AiProvider) (*openai.Client, error) {
	cfg := &pkgAi.ClientConfig{
		ModelType:      modelTypeToProtoInt(provider.ModelType),
		ModelName:      ptrStrOr(provider.ModelName, ""),
		BaseUrl:        ptrStrOr(provider.BaseURL, ""),
		Organization:   ptrStrOr(provider.Organization, ""),
		LocalHost:      ptrStrOr(provider.LocalHost, ""),
		LocalPort:      int32(ptrIntOr(provider.LocalPort, 0)),
		TimeoutSeconds: int32(ptrIntOr(provider.TimeoutSeconds, 0)),
	}

	var apiKey string
	if pkgAi.ModelType(cfg.ModelType) == pkgAi.ModelTypeCloud {
		encrypted := ptrStrOr(provider.APIKey, "")
		if encrypted == "" {
			return nil, aiV1.ErrorBadRequest("ai provider api key is not configured")
		}
		plain, err := crypto.DecryptIfNeeded(encrypted)
		if err != nil {
			return nil, aiV1.ErrorInternalServerError("decrypt api key failed")
		}
		apiKey = plain
	}

	client, err := pkgAi.NewClient(cfg, apiKey)
	if err != nil {
		return nil, aiV1.ErrorInternalServerError("create ai client failed")
	}
	return client, nil
}

// modelTypeToProtoInt ent 枚举（string 底层）→ proto ModelType 数值。
// 两端枚举命名逐字一致（LOCAL/CLOUD），这里只做值域对齐。
func modelTypeToProtoInt(mt *aiprovider.ModelType) int32 {
	if mt == nil {
		return int32(aiV1.AiProvider_MODEL_TYPE_UNSPECIFIED)
	}
	switch *mt {
	case aiprovider.ModelTypeLOCAL:
		return int32(aiV1.AiProvider_LOCAL)
	case aiprovider.ModelTypeCLOUD:
		return int32(aiV1.AiProvider_CLOUD)
	default:
		return int32(aiV1.AiProvider_MODEL_TYPE_UNSPECIFIED)
	}
}

// langName BCP-47 → 提示词里的语言名。
func langName(lang string) string {
	if strings.HasPrefix(lang, "en") {
		return "English"
	}
	return "中文"
}

func ptrStrOr(p *string, def string) string {
	if p == nil || *p == "" {
		return def
	}
	return *p
}

func ptrIntOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}
