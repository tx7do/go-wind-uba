package service

// 通知域的 gRPC 服务端适配层。
//
// 上游（go-wind-admin 单服务）这些方法以 HTTP 长名暴露；go-wind-uba 的 core/admin
// 分层走"领域 proto gRPC + admin BFF REST"，方法名对齐领域 proto 的短名，
// 这里用薄包装委托到既有的长名实现，保持移植代码零改动。

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	notificationV1 "go-wind-uba/api/gen/go/notification/service/v1"
	notificationChannelV1 "go-wind-uba/api/gen/go/notification_channel/service/v1"
)

// ── NotificationService（SendDirect / 台账查询） ──

var _ notificationV1.NotificationServiceServer = (*NotificationService)(nil)

func (s *NotificationService) ListDelivery(ctx context.Context, req *paginationV1.PagingRequest) (*notificationV1.ListNotificationDeliveryResponse, error) {
	return s.ListNotificationDelivery(ctx, req)
}

func (s *NotificationService) GetDelivery(ctx context.Context, req *notificationV1.GetNotificationDeliveryRequest) (*notificationV1.NotificationDelivery, error) {
	return s.GetNotificationDelivery(ctx, req)
}

// ── NotificationRuleService ──

var _ notificationV1.NotificationRuleServiceServer = (*NotificationRuleService)(nil)

func (s *NotificationRuleService) List(ctx context.Context, req *paginationV1.PagingRequest) (*notificationV1.ListNotificationRuleResponse, error) {
	return s.ListNotificationRule(ctx, req)
}

func (s *NotificationRuleService) Get(ctx context.Context, req *notificationV1.GetNotificationRuleRequest) (*notificationV1.NotificationRule, error) {
	return s.GetNotificationRule(ctx, req)
}

func (s *NotificationRuleService) Create(ctx context.Context, req *notificationV1.CreateNotificationRuleRequest) (*notificationV1.NotificationRule, error) {
	return s.CreateNotificationRule(ctx, req)
}

func (s *NotificationRuleService) Update(ctx context.Context, req *notificationV1.UpdateNotificationRuleRequest) (*emptypb.Empty, error) {
	return s.UpdateNotificationRule(ctx, req)
}

func (s *NotificationRuleService) Delete(ctx context.Context, req *notificationV1.DeleteNotificationRuleRequest) (*emptypb.Empty, error) {
	return s.DeleteNotificationRule(ctx, req)
}

func (s *NotificationRuleService) TestDispatch(ctx context.Context, req *notificationV1.TestDispatchNotificationRequest) (*notificationV1.TestDispatchNotificationResponse, error) {
	return s.TestDispatchNotification(ctx, req)
}

// ── NotificationTemplateService ──

var _ notificationV1.NotificationTemplateServiceServer = (*NotificationTemplateService)(nil)

func (s *NotificationTemplateService) List(ctx context.Context, req *paginationV1.PagingRequest) (*notificationV1.ListNotificationTemplateResponse, error) {
	return s.ListNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) Get(ctx context.Context, req *notificationV1.GetNotificationTemplateRequest) (*notificationV1.NotificationTemplate, error) {
	return s.GetNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) Create(ctx context.Context, req *notificationV1.CreateNotificationTemplateRequest) (*notificationV1.NotificationTemplate, error) {
	return s.CreateNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) Update(ctx context.Context, req *notificationV1.UpdateNotificationTemplateRequest) (*emptypb.Empty, error) {
	return s.UpdateNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) Delete(ctx context.Context, req *notificationV1.DeleteNotificationTemplateRequest) (*emptypb.Empty, error) {
	return s.DeleteNotificationTemplate(ctx, req)
}

func (s *NotificationTemplateService) Render(ctx context.Context, req *notificationV1.RenderNotificationTemplateRequest) (*notificationV1.RenderNotificationTemplateResponse, error) {
	return s.RenderNotificationTemplate(ctx, req)
}

// ── NotificationPreferenceService（方法名与领域 proto 一致，仅需接口合规声明） ──

var _ notificationV1.NotificationPreferenceServiceServer = (*NotificationPreferenceService)(nil)

// ── NotificationChannelService ──

var _ notificationChannelV1.NotificationChannelServiceServer = (*NotificationChannelService)(nil)

func (s *NotificationChannelService) List(ctx context.Context, req *paginationV1.PagingRequest) (*notificationChannelV1.ListNotificationChannelResponse, error) {
	return s.ListNotificationChannel(ctx, req)
}

func (s *NotificationChannelService) Get(ctx context.Context, req *notificationChannelV1.GetNotificationChannelRequest) (*notificationChannelV1.NotificationChannel, error) {
	return s.GetNotificationChannel(ctx, req)
}

func (s *NotificationChannelService) Create(ctx context.Context, req *notificationChannelV1.CreateNotificationChannelRequest) (*notificationChannelV1.NotificationChannel, error) {
	return s.CreateNotificationChannel(ctx, req)
}

func (s *NotificationChannelService) Update(ctx context.Context, req *notificationChannelV1.UpdateNotificationChannelRequest) (*emptypb.Empty, error) {
	return s.UpdateNotificationChannel(ctx, req)
}

func (s *NotificationChannelService) Delete(ctx context.Context, req *notificationChannelV1.DeleteNotificationChannelRequest) (*emptypb.Empty, error) {
	return s.DeleteNotificationChannel(ctx, req)
}
