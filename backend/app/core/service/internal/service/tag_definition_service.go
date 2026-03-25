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

type TagDefinitionService struct {
	ubaV1.UnimplementedTagDefinitionServiceServer

	log *log.Helper

	tagDefinitionRepo *data.TagDefinitionRepo
}

func NewTagDefinitionService(
	ctx *bootstrap.Context,
	riskRuleRepo *data.TagDefinitionRepo,
) *TagDefinitionService {
	svc := &TagDefinitionService{
		log:               ctx.NewLoggerHelper("tag-definition/service/core-service"),
		tagDefinitionRepo: riskRuleRepo,
	}

	svc.init()

	return svc
}

func (s *TagDefinitionService) init() {
}

func (s *TagDefinitionService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListTagDefinitionResponse, error) {
	resp, err := s.tagDefinitionRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *TagDefinitionService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountTagDefinitionResponse, error) {
	return s.tagDefinitionRepo.Count(ctx, req)
}

func (s *TagDefinitionService) Get(ctx context.Context, req *ubaV1.GetTagDefinitionRequest) (*ubaV1.TagDefinition, error) {
	resp, err := s.tagDefinitionRepo.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *TagDefinitionService) Create(ctx context.Context, req *ubaV1.CreateTagDefinitionRequest) (*ubaV1.TagDefinition, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.tagDefinitionRepo.Create(ctx, req)
}

func (s *TagDefinitionService) Update(ctx context.Context, req *ubaV1.UpdateTagDefinitionRequest) (*ubaV1.TagDefinition, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.tagDefinitionRepo.Update(ctx, req)
}

func (s *TagDefinitionService) Delete(ctx context.Context, req *ubaV1.DeleteTagDefinitionRequest) (*emptypb.Empty, error) {
	if err := s.tagDefinitionRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
