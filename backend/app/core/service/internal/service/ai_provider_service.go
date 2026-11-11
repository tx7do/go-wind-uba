package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
	"go-wind-uba/app/core/service/internal/data"
)

// AiProviderService AI 模型提供商管理：core 侧纯持久化透传，
// 操作人（created_by/updated_by）与密钥加密由 admin BFF 层负责。
type AiProviderService struct {
	aiV1.UnimplementedAiProviderServiceServer
	log  *log.Helper
	repo *data.AiProviderRepo
}

func NewAiProviderService(ctx *bootstrap.Context, repo *data.AiProviderRepo) *AiProviderService {
	return &AiProviderService{
		log:  ctx.NewLoggerHelper("ai_provider/service/core-service"),
		repo: repo,
	}
}

func (s *AiProviderService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*aiV1.CountAiProviderResponse, error) {
	return s.repo.Count(ctx, req)
}

func (s *AiProviderService) List(ctx context.Context, req *paginationV1.PagingRequest) (*aiV1.ListAiProviderResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *AiProviderService) Get(ctx context.Context, req *aiV1.GetAiProviderRequest) (*aiV1.AiProvider, error) {
	return s.repo.Get(ctx, req)
}

func (s *AiProviderService) Create(ctx context.Context, req *aiV1.CreateAiProviderRequest) (*emptypb.Empty, error) {
	if err := s.repo.Create(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AiProviderService) Update(ctx context.Context, req *aiV1.UpdateAiProviderRequest) (*emptypb.Empty, error) {
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AiProviderService) Delete(ctx context.Context, req *aiV1.DeleteAiProviderRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// aiProviderTestTimeout 连通性测试的单次请求兜底时长。
const aiProviderTestTimeout = 15 * time.Second

// TestConnection 连通性测试：对已保存的提供商配置发一次只读请求（拉模型列表）。
// 云端校验端点+密钥，本地 Ollama 校验端点可达；不消耗对话 token。
func (s *AiProviderService) TestConnection(ctx context.Context, req *aiV1.TestAiProviderConnectionRequest) (*aiV1.TestAiProviderConnectionResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, aiV1.ErrorBadRequest("id is required")
	}

	provider, err := s.repo.GetEntityByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	client, err := newOpenAIClientForProvider(provider)
	if err != nil {
		return &aiV1.TestAiProviderConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("构造客户端失败: %v", err),
		}, nil
	}

	testCtx, cancel := context.WithTimeout(ctx, aiProviderTestTimeout)
	defer cancel()

	start := time.Now()
	models, err := client.ListModels(testCtx)
	latencyMs := int64(time.Since(start).Milliseconds())
	if err != nil {
		s.log.Warnf("ai provider connection test failed: provider=%d: %v", req.GetId(), err)
		return &aiV1.TestAiProviderConnectionResponse{
			Success:   false,
			Message:   fmt.Sprintf("端点不可达或密钥无效: %v", err),
			LatencyMs: uint32(latencyMs),
		}, nil
	}

	modelsResp := models.Models
	summary := fmt.Sprintf("连接成功，端点返回 %d 个模型", len(modelsResp))
	if len(modelsResp) > 0 {
		const maxShow = 5
		names := make([]string, 0, maxShow)
		for i, m := range modelsResp {
			if i >= maxShow {
				break
			}
			names = append(names, m.ID)
		}
		summary += fmt.Sprintf("（%s）", strings.Join(names, ", "))
	}
	return &aiV1.TestAiProviderConnectionResponse{
		Success:   true,
		Message:   summary,
		LatencyMs: uint32(latencyMs),
	}, nil
}
