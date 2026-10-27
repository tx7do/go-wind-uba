package server

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/tx7do/go-utils/trans"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-wind-uba/app/admin/service/internal/service"
)

// 服务端全量导出（:export 手动锚点）的路由装配。
//
// 为什么手动注册：kratos 生成 Handler 的响应一律走 JSON 编码器（transport/http 没有
// HttpBody/原始字节透传），导出文件必须手写 Response——与文件下载
// （i_file_transfer_http.pb.go，rest_server.go 里的 TODO）同因同解，沿用同一先例：
// 路由手动挂，鉴权/审计照走 ctx.Middleware 全链。契约以管理台
// api/composables/server-export.ts 为准（?format=xlsx&query=<列表同款查询 JSON>）。
// 各资源的权限在各自 service 方法里闸，与 List 同语义。

// 与生成代码的 Operation 命名同风格，让审计日志能对上"谁在什么时候导了什么"。
const (
	OperationNotificationServiceExportNotificationDelivery = "admin.service.v1.NotificationService/ExportNotificationDelivery"
	OperationApiAuditLogServiceExport                      = "admin.service.v1.ApiAuditLogService/ExportApiAuditLog"
	OperationLoginAuditLogServiceExport                    = "admin.service.v1.LoginAuditLogService/ExportLoginAuditLog"
	OperationOperationAuditLogServiceExport                = "admin.service.v1.OperationAuditLogService/ExportOperationAuditLog"
	OperationDataAccessAuditLogServiceExport               = "admin.service.v1.DataAccessAuditLogService/ExportDataAccessAuditLog"
	OperationPermissionAuditLogServiceExport               = "admin.service.v1.PermissionAuditLogService/ExportPermissionAuditLog"
	OperationAiUsageLogServiceExport                       = "admin.service.v1.AiUsageLogService/ExportAiUsageLog"
)

// bindExportRequest 解析导出锚点的查询参数：?query（go-crud 查询 JSON）/ ?orderBy / ?format。
func bindExportRequest(ctx http.Context) (*paginationV1.PagingRequest, string) {
	q := ctx.Query()
	req := &paginationV1.PagingRequest{}
	if v := q.Get("query"); v != "" {
		req.FilteringType = &paginationV1.PagingRequest_Query{Query: v}
	}
	if v := q.Get("orderBy"); v != "" {
		req.OrderBy = trans.Ptr(v)
	}
	format := q.Get("format")
	if format == "" {
		format = "xlsx"
	}
	return req, format
}

// writeExport 把导出产物写回裸 HTTP 响应。
func writeExport(ctx http.Context, payload *service.ExportPayload) error {
	rw := ctx.Response()
	if rw == nil {
		return ctx.Result(500, "response writer not available")
	}
	rw.Header().Set("Content-Type", payload.Mime)
	rw.Header().Set("Content-Disposition", `attachment; filename="`+payload.Filename+`"`)
	if payload.Truncated {
		rw.Header().Set("X-Export-Truncated", "true")
	}
	rw.Header().Set("Content-Length", strconv.Itoa(len(payload.Data)))
	rw.WriteHeader(200)
	_, err := rw.Write(payload.Data)
	return err
}

// handleExport :export 路由的公共骨架：绑参 → 中间件链（鉴权/审计） → service 组装 → 裸写响应。
func handleExport(ctx http.Context, operation string, do func(context.Context, *paginationV1.PagingRequest, string) (*service.ExportPayload, error)) error {
	http.SetOperation(ctx, operation)
	in, format := bindExportRequest(ctx)

	h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
		return do(ctx, req.(*paginationV1.PagingRequest), format)
	})
	out, err := h(ctx, in)
	if err != nil {
		return err
	}
	return writeExport(ctx, out.(*service.ExportPayload))
}

// registerExportRoutes 手动挂载各资源的 :export 锚点（新资源在此追加同款一行）。
func registerExportRoutes(
	srv *http.Server,
	notificationSvc *service.NotificationService,
	apiAuditLogSvc *service.ApiAuditLogService,
	loginAuditLogSvc *service.LoginAuditLogService,
	operationAuditLogSvc *service.OperationAuditLogService,
	dataAccessAuditLogSvc *service.DataAccessAuditLogService,
	permissionAuditLogSvc *service.PermissionAuditLogService,
	aiUsageLogSvc *service.AiUsageLogService,
) {
	r := srv.Route("/")
	r.GET("admin/v1/notification-deliveries:export", func(ctx http.Context) error {
		return handleExport(ctx, OperationNotificationServiceExportNotificationDelivery, notificationSvc.ExportNotificationDelivery)
	})
	r.GET("admin/v1/api-audit-logs:export", func(ctx http.Context) error {
		return handleExport(ctx, OperationApiAuditLogServiceExport, apiAuditLogSvc.ExportApiAuditLog)
	})
	r.GET("admin/v1/login-audit-logs:export", func(ctx http.Context) error {
		return handleExport(ctx, OperationLoginAuditLogServiceExport, loginAuditLogSvc.ExportLoginAuditLog)
	})
	r.GET("admin/v1/operation-audit-logs:export", func(ctx http.Context) error {
		return handleExport(ctx, OperationOperationAuditLogServiceExport, operationAuditLogSvc.ExportOperationAuditLog)
	})
	r.GET("admin/v1/data-access-audit-logs:export", func(ctx http.Context) error {
		return handleExport(ctx, OperationDataAccessAuditLogServiceExport, dataAccessAuditLogSvc.ExportDataAccessAuditLog)
	})
	r.GET("admin/v1/permission-audit-logs:export", func(ctx http.Context) error {
		return handleExport(ctx, OperationPermissionAuditLogServiceExport, permissionAuditLogSvc.ExportPermissionAuditLog)
	})
	r.GET("admin/v1/ai/usage-logs:export", func(ctx http.Context) error {
		return handleExport(ctx, OperationAiUsageLogServiceExport, aiUsageLogSvc.ExportAiUsageLog)
	})
}
