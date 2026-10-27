package service

import (
	"context"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	notificationV1 "go-wind-uba/api/gen/go/notification/service/v1"
	notificationChannelV1 "go-wind-uba/api/gen/go/notification_channel/service/v1"
)

// ── NotificationService（投递台账只读 + SendDirect 不开放 HTTP） ──

type NotificationService struct {
	adminV1.NotificationServiceHTTPServer

	log *log.Helper

	notificationServiceClient notificationV1.NotificationServiceClient
}

func NewNotificationService(
	ctx *bootstrap.Context,
	notificationServiceClient notificationV1.NotificationServiceClient,
) *NotificationService {
	return &NotificationService{
		log:                       ctx.NewLoggerHelper("notification/service/admin-service"),
		notificationServiceClient: notificationServiceClient,
	}
}

func (s *NotificationService) ListNotificationDelivery(ctx context.Context, req *paginationV1.PagingRequest) (*notificationV1.ListNotificationDeliveryResponse, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification delivery ledger"); err != nil {
		return nil, err
	}
	return s.notificationServiceClient.ListDelivery(ctx, req)
}

func (s *NotificationService) GetNotificationDelivery(ctx context.Context, req *notificationV1.GetNotificationDeliveryRequest) (*notificationV1.NotificationDelivery, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification delivery ledger"); err != nil {
		return nil, err
	}
	return s.notificationServiceClient.GetDelivery(ctx, req)
}

// 投递台账导出列：标题与页面列（zh-CN）一致，值与客户端聚合导出同口径（枚举导出原名）。
// target 已是服务端脱敏值，导出不二次遮蔽（与页面一致，见 NotificationDelivery.target 注释）。
var notificationDeliveryExportColumns = []ExportColumn[*notificationV1.NotificationDelivery]{
	{"投递时间", func(d *notificationV1.NotificationDelivery) string { return exportTimestamp(d.CreatedAt) }},
	{"业务事件", func(d *notificationV1.NotificationDelivery) string { return exportEnumText(d.EventType) }},
	{"渠道", func(d *notificationV1.NotificationDelivery) string { return exportEnumText(d.Channel) }},
	{"投递目标", func(d *notificationV1.NotificationDelivery) string { return exportStr(d.Target) }},
	{"结果", func(d *notificationV1.NotificationDelivery) string { return exportEnumText(d.Status) }},
	{"尝试次数", func(d *notificationV1.NotificationDelivery) string { return exportUint32(d.Attempts) }},
	{"收件用户", func(d *notificationV1.NotificationDelivery) string { return exportUint32(d.RecipientUserId) }},
	{"渠道配置", func(d *notificationV1.NotificationDelivery) string { return exportUint32(d.ChannelId) }},
	{"关联对象", func(d *notificationV1.NotificationDelivery) string { return exportUint32(d.RelatedId) }},
	{"派发请求", func(d *notificationV1.NotificationDelivery) string { return exportStr(d.RequestId) }},
	{"完成时间", func(d *notificationV1.NotificationDelivery) string { return exportTimestamp(d.SentAt) }},
	{"失败原因", func(d *notificationV1.NotificationDelivery) string { return exportStr(d.LastError) }},
}

// ExportNotificationDelivery 投递台账服务端全量导出（:export 手动锚点，路由装配见
// server/export_http.go）。与 List 同权限语义（平台管理员）、同 query 透传——导出的即
// 当前搜索看到的；行序锚在创建时间倒序（前端导出锚点只透传 query），页内拉满直到取完
// 或行数上限（截断以 X-Export-Truncated 响应头标记）。
func (s *NotificationService) ExportNotificationDelivery(ctx context.Context, req *paginationV1.PagingRequest, format string) (*ExportPayload, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification delivery ledger"); err != nil {
		return nil, err
	}
	if req == nil {
		req = &paginationV1.PagingRequest{}
	}
	orderBy := req.OrderBy
	if orderBy == nil || *orderBy == "" {
		orderBy = trans.Ptr("-created_at")
	}
	return BuildExport(ctx, ExportOptions[*notificationV1.NotificationDelivery]{
		FilenameBase: "notification-deliveries",
		SheetName:    "投递台账",
		Format:       format,
		Columns:      notificationDeliveryExportColumns,
		FetchPage: func(ctx context.Context, page, pageSize uint32) ([]*notificationV1.NotificationDelivery, error) {
			resp, err := s.notificationServiceClient.ListDelivery(ctx, &paginationV1.PagingRequest{
				Page:          trans.Ptr(page),
				PageSize:      trans.Ptr(pageSize),
				OrderBy:       orderBy,
				FilteringType: req.FilteringType,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetItems(), nil
		},
	})
}

// ── NotificationRuleService ──

type NotificationRuleService struct {
	adminV1.NotificationRuleServiceHTTPServer

	log *log.Helper

	notificationRuleServiceClient notificationV1.NotificationRuleServiceClient
}

func NewNotificationRuleService(
	ctx *bootstrap.Context,
	notificationRuleServiceClient notificationV1.NotificationRuleServiceClient,
) *NotificationRuleService {
	return &NotificationRuleService{
		log:                           ctx.NewLoggerHelper("notification-rule/service/admin-service"),
		notificationRuleServiceClient: notificationRuleServiceClient,
	}
}

func (s *NotificationRuleService) ListNotificationRule(ctx context.Context, req *paginationV1.PagingRequest) (*notificationV1.ListNotificationRuleResponse, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification routing rules"); err != nil {
		return nil, err
	}
	return s.notificationRuleServiceClient.List(ctx, req)
}

func (s *NotificationRuleService) GetNotificationRule(ctx context.Context, req *notificationV1.GetNotificationRuleRequest) (*notificationV1.NotificationRule, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification routing rules"); err != nil {
		return nil, err
	}
	return s.notificationRuleServiceClient.Get(ctx, req)
}

func (s *NotificationRuleService) CreateNotificationRule(ctx context.Context, req *notificationV1.CreateNotificationRuleRequest) (*notificationV1.NotificationRule, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification routing rules"); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, notificationV1.ErrorNotificationBadRequest("invalid parameter")
	}

	return s.notificationRuleServiceClient.Create(ctx, req)
}

func (s *NotificationRuleService) UpdateNotificationRule(ctx context.Context, req *notificationV1.UpdateNotificationRuleRequest) (*emptypb.Empty, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification routing rules"); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, notificationV1.ErrorNotificationBadRequest("invalid parameter")
	}

	return s.notificationRuleServiceClient.Update(ctx, req)
}

func (s *NotificationRuleService) DeleteNotificationRule(ctx context.Context, req *notificationV1.DeleteNotificationRuleRequest) (*emptypb.Empty, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification routing rules"); err != nil {
		return nil, err
	}
	return s.notificationRuleServiceClient.Delete(ctx, req)
}

func (s *NotificationRuleService) TestDispatchNotification(ctx context.Context, req *notificationV1.TestDispatchNotificationRequest) (*notificationV1.TestDispatchNotificationResponse, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification routing rules"); err != nil {
		return nil, err
	}
	return s.notificationRuleServiceClient.TestDispatch(ctx, req)
}

// ── NotificationTemplateService ──

type NotificationTemplateService struct {
	adminV1.NotificationTemplateServiceHTTPServer

	log *log.Helper

	notificationTemplateServiceClient notificationV1.NotificationTemplateServiceClient
}

func NewNotificationTemplateService(
	ctx *bootstrap.Context,
	notificationTemplateServiceClient notificationV1.NotificationTemplateServiceClient,
) *NotificationTemplateService {
	return &NotificationTemplateService{
		log:                               ctx.NewLoggerHelper("notification-template/service/admin-service"),
		notificationTemplateServiceClient: notificationTemplateServiceClient,
	}
}

func (s *NotificationTemplateService) ListNotificationTemplate(ctx context.Context, req *paginationV1.PagingRequest) (*notificationV1.ListNotificationTemplateResponse, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification templates"); err != nil {
		return nil, err
	}
	return s.notificationTemplateServiceClient.ListNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) GetNotificationTemplate(ctx context.Context, req *notificationV1.GetNotificationTemplateRequest) (*notificationV1.NotificationTemplate, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification templates"); err != nil {
		return nil, err
	}
	return s.notificationTemplateServiceClient.GetNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) CreateNotificationTemplate(ctx context.Context, req *notificationV1.CreateNotificationTemplateRequest) (*notificationV1.NotificationTemplate, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification templates"); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, notificationV1.ErrorNotificationBadRequest("invalid parameter")
	}

	return s.notificationTemplateServiceClient.CreateNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) UpdateNotificationTemplate(ctx context.Context, req *notificationV1.UpdateNotificationTemplateRequest) (*emptypb.Empty, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification templates"); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, notificationV1.ErrorNotificationBadRequest("invalid parameter")
	}

	return s.notificationTemplateServiceClient.UpdateNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) DeleteNotificationTemplate(ctx context.Context, req *notificationV1.DeleteNotificationTemplateRequest) (*emptypb.Empty, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification templates"); err != nil {
		return nil, err
	}
	return s.notificationTemplateServiceClient.DeleteNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) RenderNotificationTemplate(ctx context.Context, req *notificationV1.RenderNotificationTemplateRequest) (*notificationV1.RenderNotificationTemplateResponse, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification templates"); err != nil {
		return nil, err
	}
	return s.notificationTemplateServiceClient.RenderNotificationTemplate(ctx, req)
}

// ── NotificationChannelService ──

type NotificationChannelService struct {
	adminV1.NotificationChannelServiceHTTPServer

	log *log.Helper

	notificationChannelServiceClient notificationChannelV1.NotificationChannelServiceClient
}

func NewNotificationChannelService(
	ctx *bootstrap.Context,
	notificationChannelServiceClient notificationChannelV1.NotificationChannelServiceClient,
) *NotificationChannelService {
	return &NotificationChannelService{
		log:                              ctx.NewLoggerHelper("notification-channel/service/admin-service"),
		notificationChannelServiceClient: notificationChannelServiceClient,
	}
}

func (s *NotificationChannelService) ListNotificationChannel(ctx context.Context, req *paginationV1.PagingRequest) (*notificationChannelV1.ListNotificationChannelResponse, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification channels"); err != nil {
		return nil, err
	}
	return s.notificationChannelServiceClient.List(ctx, req)
}

func (s *NotificationChannelService) GetNotificationChannel(ctx context.Context, req *notificationChannelV1.GetNotificationChannelRequest) (*notificationChannelV1.NotificationChannel, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification channels"); err != nil {
		return nil, err
	}
	return s.notificationChannelServiceClient.Get(ctx, req)
}

func (s *NotificationChannelService) CreateNotificationChannel(ctx context.Context, req *notificationChannelV1.CreateNotificationChannelRequest) (*notificationChannelV1.NotificationChannel, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification channels"); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, notificationV1.ErrorNotificationBadRequest("invalid parameter")
	}

	return s.notificationChannelServiceClient.Create(ctx, req)
}

func (s *NotificationChannelService) UpdateNotificationChannel(ctx context.Context, req *notificationChannelV1.UpdateNotificationChannelRequest) (*emptypb.Empty, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification channels"); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, notificationV1.ErrorNotificationBadRequest("invalid parameter")
	}

	return s.notificationChannelServiceClient.Update(ctx, req)
}

func (s *NotificationChannelService) DeleteNotificationChannel(ctx context.Context, req *notificationChannelV1.DeleteNotificationChannelRequest) (*emptypb.Empty, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification channels"); err != nil {
		return nil, err
	}
	return s.notificationChannelServiceClient.Delete(ctx, req)
}

func (s *NotificationChannelService) SendTestEmail(ctx context.Context, req *notificationChannelV1.SendTestEmailRequest) (*emptypb.Empty, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification channels"); err != nil {
		return nil, err
	}
	return s.notificationChannelServiceClient.SendTestEmail(ctx, req)
}

func (s *NotificationChannelService) SendTest(ctx context.Context, req *notificationChannelV1.SendTestRequest) (*emptypb.Empty, error) {
	if err := requirePlatformAdmin(ctx, s.log, "notification channels"); err != nil {
		return nil, err
	}
	return s.notificationChannelServiceClient.SendTest(ctx, req)
}

// ── NotificationPreferenceService（自助：服务端从操作人钉定 user） ──

type NotificationPreferenceService struct {
	adminV1.NotificationPreferenceServiceHTTPServer

	log *log.Helper

	notificationPreferenceServiceClient notificationV1.NotificationPreferenceServiceClient
}

func NewNotificationPreferenceService(
	ctx *bootstrap.Context,
	notificationPreferenceServiceClient notificationV1.NotificationPreferenceServiceClient,
) *NotificationPreferenceService {
	return &NotificationPreferenceService{
		log:                                 ctx.NewLoggerHelper("notification-preference/service/admin-service"),
		notificationPreferenceServiceClient: notificationPreferenceServiceClient,
	}
}

func (s *NotificationPreferenceService) GetMyNotificationPreference(ctx context.Context, req *emptypb.Empty) (*notificationV1.NotificationPreference, error) {
	return s.notificationPreferenceServiceClient.GetMyNotificationPreference(ctx, req)
}

func (s *NotificationPreferenceService) UpdateMyNotificationPreference(ctx context.Context, req *notificationV1.UpdateNotificationPreferenceRequest) (*notificationV1.NotificationPreference, error) {
	return s.notificationPreferenceServiceClient.UpdateMyNotificationPreference(ctx, req)
}

func (s *NotificationPreferenceService) ListMyNotificationCategories(ctx context.Context, req *emptypb.Empty) (*notificationV1.ListMyNotificationCategoriesResponse, error) {
	return s.notificationPreferenceServiceClient.ListMyNotificationCategories(ctx, req)
}
