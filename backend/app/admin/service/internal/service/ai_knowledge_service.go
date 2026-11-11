package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"
	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
)

// AiKnowledgeBaseService AI 知识库管理（BFF 直通转发；
// 文档入库/检索/重索引的业务编排全部在 core 侧执行）。
type AiKnowledgeBaseService struct {
	adminV1.AiKnowledgeBaseServiceHTTPServer

	aiKnowledgeBaseServiceClient aiV1.AiKnowledgeBaseServiceClient
	log                          *log.Helper
}

func NewAiKnowledgeBaseService(ctx *bootstrap.Context, aiKnowledgeBaseServiceClient aiV1.AiKnowledgeBaseServiceClient) *AiKnowledgeBaseService {
	return &AiKnowledgeBaseService{
		log:                          ctx.NewLoggerHelper("ai_knowledge/service/admin-service"),
		aiKnowledgeBaseServiceClient: aiKnowledgeBaseServiceClient,
	}
}

func (s *AiKnowledgeBaseService) List(ctx context.Context, req *paginationV1.PagingRequest) (*aiV1.ListAiKnowledgeBaseResponse, error) {
	return s.aiKnowledgeBaseServiceClient.List(ctx, req)
}

func (s *AiKnowledgeBaseService) Get(ctx context.Context, req *aiV1.GetAiKnowledgeBaseRequest) (*aiV1.AiKnowledgeBase, error) {
	return s.aiKnowledgeBaseServiceClient.Get(ctx, req)
}

func (s *AiKnowledgeBaseService) Create(ctx context.Context, req *aiV1.CreateAiKnowledgeBaseRequest) (*emptypb.Empty, error) {
	return s.aiKnowledgeBaseServiceClient.Create(ctx, req)
}

func (s *AiKnowledgeBaseService) Update(ctx context.Context, req *aiV1.UpdateAiKnowledgeBaseRequest) (*emptypb.Empty, error) {
	return s.aiKnowledgeBaseServiceClient.Update(ctx, req)
}

func (s *AiKnowledgeBaseService) Delete(ctx context.Context, req *aiV1.DeleteAiKnowledgeBaseRequest) (*emptypb.Empty, error) {
	return s.aiKnowledgeBaseServiceClient.Delete(ctx, req)
}

func (s *AiKnowledgeBaseService) UploadDoc(ctx context.Context, req *aiV1.UploadAiDocRequest) (*aiV1.UploadAiDocResponse, error) {
	return s.aiKnowledgeBaseServiceClient.UploadDoc(ctx, req)
}

func (s *AiKnowledgeBaseService) UploadDocFile(ctx context.Context, req *aiV1.UploadAiDocFileRequest) (*aiV1.UploadAiDocResponse, error) {
	return s.aiKnowledgeBaseServiceClient.UploadDocFile(ctx, req)
}

func (s *AiKnowledgeBaseService) ListDocs(ctx context.Context, req *aiV1.ListAiDocsRequest) (*aiV1.ListAiDocsResponse, error) {
	return s.aiKnowledgeBaseServiceClient.ListDocs(ctx, req)
}

func (s *AiKnowledgeBaseService) DeleteDoc(ctx context.Context, req *aiV1.DeleteAiDocRequest) (*emptypb.Empty, error) {
	return s.aiKnowledgeBaseServiceClient.DeleteDoc(ctx, req)
}

func (s *AiKnowledgeBaseService) Search(ctx context.Context, req *aiV1.SearchAiKnowledgeRequest) (*aiV1.SearchAiKnowledgeResponse, error) {
	return s.aiKnowledgeBaseServiceClient.Search(ctx, req)
}

func (s *AiKnowledgeBaseService) ReindexDocs(ctx context.Context, req *aiV1.ReindexDocsRequest) (*emptypb.Empty, error) {
	return s.aiKnowledgeBaseServiceClient.ReindexDocs(ctx, req)
}

func (s *AiKnowledgeBaseService) SyncOpsKnowledge(ctx context.Context, req *aiV1.SyncOpsKnowledgeRequest) (*aiV1.SyncOpsKnowledgeResponse, error) {
	return s.aiKnowledgeBaseServiceClient.SyncOpsKnowledge(ctx, req)
}
