package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type WebhookService struct {
	ubaV1.UnimplementedWebhookServiceServer

	log *log.Helper

	webhookRepo *data.WebhookRepo
}

func NewWebhookService(
	ctx *bootstrap.Context,
	webhookRepo *data.WebhookRepo,
) *WebhookService {
	svc := &WebhookService{
		log:         ctx.NewLoggerHelper("webhook/service/core-service"),
		webhookRepo: webhookRepo,
	}

	svc.init()

	return svc
}

func (s *WebhookService) init() {
}

func (s *WebhookService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListWebhookResponse, error) {
	resp, err := s.webhookRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *WebhookService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountWebhookResponse, error) {
	return s.webhookRepo.Count(ctx, req)
}

func (s *WebhookService) Get(ctx context.Context, req *ubaV1.GetWebhookRequest) (*ubaV1.Webhook, error) {
	resp, err := s.webhookRepo.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *WebhookService) Create(ctx context.Context, req *ubaV1.CreateWebhookRequest) (*ubaV1.Webhook, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.webhookRepo.Create(ctx, req)
}

func (s *WebhookService) Update(ctx context.Context, req *ubaV1.UpdateWebhookRequest) (*ubaV1.Webhook, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.webhookRepo.Update(ctx, req)
}

func (s *WebhookService) Delete(ctx context.Context, req *ubaV1.DeleteWebhookRequest) (*emptypb.Empty, error) {
	if err := s.webhookRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
