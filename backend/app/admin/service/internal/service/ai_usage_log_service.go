package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"
	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
)

// AiUsageLogService AI 用量流水查询（BFF 直通转发；
// 租户口径过滤在 core 侧执行：租户用户按本租户，平台用户见全量）。
type AiUsageLogService struct {
	adminV1.AiUsageLogServiceHTTPServer

	aiUsageLogServiceClient aiV1.AiUsageLogServiceClient
	log                     *log.Helper
}

func NewAiUsageLogService(ctx *bootstrap.Context, aiUsageLogServiceClient aiV1.AiUsageLogServiceClient) *AiUsageLogService {
	return &AiUsageLogService{
		log:                     ctx.NewLoggerHelper("ai_usage_log/service/admin-service"),
		aiUsageLogServiceClient: aiUsageLogServiceClient,
	}
}

func (s *AiUsageLogService) List(ctx context.Context, req *paginationV1.PagingRequest) (*aiV1.ListAiUsageLogResponse, error) {
	return s.aiUsageLogServiceClient.List(ctx, req)
}

// 用量流水导出列：标题与用量页列（zh-CN）一致。
var aiUsageLogExportColumns = []ExportColumn[*aiV1.AiUsageLog]{
	{"模型名称", func(l *aiV1.AiUsageLog) string { return exportStr(l.ModelName) }},
	{"输入 tokens", func(l *aiV1.AiUsageLog) string { return exportUint32(l.PromptTokens) }},
	{"输出 tokens", func(l *aiV1.AiUsageLog) string { return exportUint32(l.CompletionTokens) }},
	{"总 tokens", func(l *aiV1.AiUsageLog) string { return exportUint32(l.TotalTokens) }},
	{"耗时（毫秒）", func(l *aiV1.AiUsageLog) string { return exportUint32(l.DurationMs) }},
	{"会话ID", func(l *aiV1.AiUsageLog) string { return exportUint32(l.ConversationId) }},
	{"调用时间", func(l *aiV1.AiUsageLog) string { return exportTimestamp(l.CreatedAt) }},
}

// ExportAiUsageLog 用量流水服务端全量导出（:export 手动锚点，路由装配见
// server/export_http.go）。租户口径与 List 同语义（core 侧行级过滤），导出的即当前搜索看到的。
func (s *AiUsageLogService) ExportAiUsageLog(ctx context.Context, req *paginationV1.PagingRequest, format string) (*ExportPayload, error) {
	if req == nil {
		req = &paginationV1.PagingRequest{}
	}
	return BuildExport(ctx, ExportOptions[*aiV1.AiUsageLog]{
		FilenameBase: "ai-usage-logs",
		SheetName:    "AI用量流水",
		Format:       format,
		Columns:      aiUsageLogExportColumns,
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*aiV1.AiUsageLog, error) {
			resp, err := s.aiUsageLogServiceClient.List(ctx, &paginationV1.PagingRequest{
				Page:          trans.Ptr(page),
				PageSize:      trans.Ptr(pageSize),
				OrderBy:       trans.Ptr(exportOrderBy(req)),
				FilteringType: req.FilteringType,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetItems(), nil
		},
	})
}

func (s *AiUsageLogService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*aiV1.CountAiUsageLogResponse, error) {
	return s.aiUsageLogServiceClient.Count(ctx, req)
}

func (s *AiUsageLogService) GetUsageSummary(ctx context.Context, req *emptypb.Empty) (*aiV1.UsageSummaryResponse, error) {
	return s.aiUsageLogServiceClient.GetUsageSummary(ctx, req)
}

// GetTenantQuota 查询租户月度 token 配额（租户用户在 core 侧被强制看本租户）。
func (s *AiUsageLogService) GetTenantQuota(ctx context.Context, req *aiV1.GetTenantQuotaRequest) (*aiV1.TenantQuotaResponse, error) {
	return s.aiUsageLogServiceClient.GetTenantQuota(ctx, req)
}

// SetTenantQuota 设置租户月度 token 配额（平台管理员专属；limit=0 取消）。
func (s *AiUsageLogService) SetTenantQuota(ctx context.Context, req *aiV1.SetTenantQuotaRequest) (*emptypb.Empty, error) {
	return s.aiUsageLogServiceClient.SetTenantQuota(ctx, req)
}
