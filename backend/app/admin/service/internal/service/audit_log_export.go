package service

import (
	"context"
	"strings"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"

	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
)

// 五类审计日志的服务端全量导出（:export 手动锚点，路由装配见 server/export_http.go）。
//
// 权限语义与各 List 完全一致（不额外加闸）：租户隔离由 core 侧 TenantPrivacy 在
// 行级生效，导出走的是同一个 List。列与页面可见列对齐；枚举导出原名（与客户端
// 聚合导出同口径），地理位置/操作平台按页面槽位的拼接方式合成一个单元格。

// exportDevice 拼页面"操作平台"槽位：OS + 浏览器。
func exportDevice(d *auditV1.DeviceInfo) string {
	if d == nil {
		return ""
	}
	return strings.TrimSpace(exportStr(d.OsName) + " " + exportStr(d.BrowserName))
}

// exportGeo 拼页面"地理位置"槽位：省 + 市。
func exportGeo(g *auditV1.GeoLocation) string {
	if g == nil {
		return ""
	}
	return strings.TrimSpace(exportStr(g.Province) + " " + exportStr(g.City))
}

func exportOrderBy(req *paginationV1.PagingRequest) string {
	if req != nil && req.OrderBy != nil && *req.OrderBy != "" {
		return *req.OrderBy
	}
	return "-created_at"
}

// ── API 审计日志 ──

var apiAuditLogExportColumns = []ExportColumn[*auditV1.ApiAuditLog]{
	{"日志时间", func(l *auditV1.ApiAuditLog) string { return exportTimestamp(l.CreatedAt) }},
	{"操作状态", func(l *auditV1.ApiAuditLog) string { return exportBool(l.Success) }},
	{"状态码", func(l *auditV1.ApiAuditLog) string { return exportUint32(l.StatusCode) }},
	{"操作者名", func(l *auditV1.ApiAuditLog) string { return exportStr(l.Username) }},
	{"请求方法", func(l *auditV1.ApiAuditLog) string { return exportStr(l.HttpMethod) }},
	{"请求路径", func(l *auditV1.ApiAuditLog) string { return exportStr(l.Path) }},
	{"耗时（毫秒）", func(l *auditV1.ApiAuditLog) string { return exportUint32(l.LatencyMs) }},
	{"操作平台", func(l *auditV1.ApiAuditLog) string { return exportDevice(l.DeviceInfo) }},
	{"地理位置", func(l *auditV1.ApiAuditLog) string { return exportGeo(l.GeoLocation) }},
	{"IP地址", func(l *auditV1.ApiAuditLog) string { return exportStr(l.IpAddress) }},
}

// ExportApiAuditLog 走 BFF 自己的 List（保留 API 名称/模块回填），导出即页面所见。
func (s *ApiAuditLogService) ExportApiAuditLog(ctx context.Context, req *paginationV1.PagingRequest, format string) (*ExportPayload, error) {
	if req == nil {
		req = &paginationV1.PagingRequest{}
	}
	return BuildExport(ctx, ExportOptions[*auditV1.ApiAuditLog]{
		FilenameBase: "api-audit-logs",
		SheetName:    "API审计日志",
		Format:       format,
		Columns:      apiAuditLogExportColumns,
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*auditV1.ApiAuditLog, error) {
			resp, err := s.List(ctx, &paginationV1.PagingRequest{
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

// ── 登录审计日志 ──

var loginAuditLogExportColumns = []ExportColumn[*auditV1.LoginAuditLog]{
	{"日志时间", func(l *auditV1.LoginAuditLog) string { return exportTimestamp(l.CreatedAt) }},
	{"登录状态", func(l *auditV1.LoginAuditLog) string { return exportEnumText(l.Status) }},
	{"登录名", func(l *auditV1.LoginAuditLog) string { return exportStr(l.Username) }},
	{"事件动作", func(l *auditV1.LoginAuditLog) string { return exportEnumText(l.ActionType) }},
	{"风险等级", func(l *auditV1.LoginAuditLog) string { return exportEnumText(l.RiskLevel) }},
	{"登录平台", func(l *auditV1.LoginAuditLog) string { return exportDevice(l.DeviceInfo) }},
	{"地理位置", func(l *auditV1.LoginAuditLog) string { return exportGeo(l.GeoLocation) }},
	{"IP地址", func(l *auditV1.LoginAuditLog) string { return exportStr(l.IpAddress) }},
}

func (s *LoginAuditLogService) ExportLoginAuditLog(ctx context.Context, req *paginationV1.PagingRequest, format string) (*ExportPayload, error) {
	if req == nil {
		req = &paginationV1.PagingRequest{}
	}
	return BuildExport(ctx, ExportOptions[*auditV1.LoginAuditLog]{
		FilenameBase: "login-audit-logs",
		SheetName:    "登录审计日志",
		Format:       format,
		Columns:      loginAuditLogExportColumns,
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*auditV1.LoginAuditLog, error) {
			resp, err := s.loginAuditLogServiceClient.List(ctx, &paginationV1.PagingRequest{
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

// ── 操作审计日志 ──

var operationAuditLogExportColumns = []ExportColumn[*auditV1.OperationAuditLog]{
	{"日志时间", func(l *auditV1.OperationAuditLog) string { return exportTimestamp(l.CreatedAt) }},
	{"操作状态", func(l *auditV1.OperationAuditLog) string { return exportBool(l.Success) }},
	{"动作", func(l *auditV1.OperationAuditLog) string { return exportEnumText(l.Action) }},
	{"资源类型", func(l *auditV1.OperationAuditLog) string { return exportStr(l.ResourceType) }},
	{"资源ID", func(l *auditV1.OperationAuditLog) string { return exportStr(l.ResourceId) }},
	{"请求ID", func(l *auditV1.OperationAuditLog) string { return exportStr(l.RequestId) }},
	{"操作者名", func(l *auditV1.OperationAuditLog) string { return exportStr(l.Username) }},
	{"地理位置", func(l *auditV1.OperationAuditLog) string { return exportGeo(l.GeoLocation) }},
	{"IP地址", func(l *auditV1.OperationAuditLog) string { return exportStr(l.IpAddress) }},
}

func (s *OperationAuditLogService) ExportOperationAuditLog(ctx context.Context, req *paginationV1.PagingRequest, format string) (*ExportPayload, error) {
	if req == nil {
		req = &paginationV1.PagingRequest{}
	}
	return BuildExport(ctx, ExportOptions[*auditV1.OperationAuditLog]{
		FilenameBase: "operation-audit-logs",
		SheetName:    "操作审计日志",
		Format:       format,
		Columns:      operationAuditLogExportColumns,
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*auditV1.OperationAuditLog, error) {
			resp, err := s.operationAuditLogServiceClient.List(ctx, &paginationV1.PagingRequest{
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

// ── 数据访问审计日志 ──

var dataAccessAuditLogExportColumns = []ExportColumn[*auditV1.DataAccessAuditLog]{
	{"日志时间", func(l *auditV1.DataAccessAuditLog) string { return exportTimestamp(l.CreatedAt) }},
	{"操作状态", func(l *auditV1.DataAccessAuditLog) string { return exportBool(l.Success) }},
	{"访问类型", func(l *auditV1.DataAccessAuditLog) string { return exportEnumText(l.AccessType) }},
	{"数据表名", func(l *auditV1.DataAccessAuditLog) string { return exportStr(l.TableName) }},
	{"数据分类", func(l *auditV1.DataAccessAuditLog) string { return exportStr(l.DataCategory) }},
	{"耗时（毫秒）", func(l *auditV1.DataAccessAuditLog) string { return exportUint32(l.LatencyMs) }},
	{"操作者名", func(l *auditV1.DataAccessAuditLog) string { return exportStr(l.Username) }},
	{"IP地址", func(l *auditV1.DataAccessAuditLog) string { return exportStr(l.IpAddress) }},
}

func (s *DataAccessAuditLogService) ExportDataAccessAuditLog(ctx context.Context, req *paginationV1.PagingRequest, format string) (*ExportPayload, error) {
	if req == nil {
		req = &paginationV1.PagingRequest{}
	}
	return BuildExport(ctx, ExportOptions[*auditV1.DataAccessAuditLog]{
		FilenameBase: "data-access-audit-logs",
		SheetName:    "数据访问审计日志",
		Format:       format,
		Columns:      dataAccessAuditLogExportColumns,
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*auditV1.DataAccessAuditLog, error) {
			resp, err := s.dataAccessAuditLogServiceClient.List(ctx, &paginationV1.PagingRequest{
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

// ── 权限审计日志 ──

var permissionAuditLogExportColumns = []ExportColumn[*auditV1.PermissionAuditLog]{
	{"日志时间", func(l *auditV1.PermissionAuditLog) string { return exportTimestamp(l.CreatedAt) }},
	{"动作", func(l *auditV1.PermissionAuditLog) string { return exportEnumText(l.Action) }},
	{"目标类型", func(l *auditV1.PermissionAuditLog) string { return exportStr(l.TargetType) }},
	{"目标名称", func(l *auditV1.PermissionAuditLog) string { return exportStr(l.TargetName) }},
	{"变更原因", func(l *auditV1.PermissionAuditLog) string { return exportStr(l.Reason) }},
	{"操作者用户名", func(l *auditV1.PermissionAuditLog) string { return exportStr(l.OperatorName) }},
	{"IP地址", func(l *auditV1.PermissionAuditLog) string { return exportStr(l.IpAddress) }},
}

func (s *PermissionAuditLogService) ExportPermissionAuditLog(ctx context.Context, req *paginationV1.PagingRequest, format string) (*ExportPayload, error) {
	if req == nil {
		req = &paginationV1.PagingRequest{}
	}
	return BuildExport(ctx, ExportOptions[*auditV1.PermissionAuditLog]{
		FilenameBase: "permission-audit-logs",
		SheetName:    "权限审计日志",
		Format:       format,
		Columns:      permissionAuditLogExportColumns,
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*auditV1.PermissionAuditLog, error) {
			resp, err := s.permissionAuditLogServiceClient.List(ctx, &paginationV1.PagingRequest{
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
