package service

import (
	"context"
	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type WebhookService struct {
	adminV1.WebhookServiceHTTPServer

	log *log.Helper

	webhookServiceClient ubaV1.WebhookServiceClient
}

func NewWebhookService(
	ctx *bootstrap.Context,
	webhookServiceClient ubaV1.WebhookServiceClient,
) *WebhookService {
	svc := &WebhookService{
		log:                  ctx.NewLoggerHelper("webhook/service/admin-service"),
		webhookServiceClient: webhookServiceClient,
	}

	svc.init()

	return svc
}

func (s *WebhookService) init() {
}

func (s *WebhookService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListWebhookResponse, error) {
	resp, err := s.webhookServiceClient.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *WebhookService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountWebhookResponse, error) {
	return s.webhookServiceClient.Count(ctx, req)
}

func (s *WebhookService) Get(ctx context.Context, req *ubaV1.GetWebhookRequest) (*ubaV1.Webhook, error) {
	resp, err := s.webhookServiceClient.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *WebhookService) Create(ctx context.Context, req *ubaV1.CreateWebhookRequest) (*ubaV1.Webhook, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.webhookServiceClient.Create(ctx, req)
}

func (s *WebhookService) Update(ctx context.Context, req *ubaV1.UpdateWebhookRequest) (*ubaV1.Webhook, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.webhookServiceClient.Update(ctx, req)
}

func (s *WebhookService) Delete(ctx context.Context, req *ubaV1.DeleteWebhookRequest) (*emptypb.Empty, error) {
	return s.webhookServiceClient.Delete(ctx, req)
}
