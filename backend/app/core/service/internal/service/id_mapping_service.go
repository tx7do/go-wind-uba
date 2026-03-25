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

type IDMappingService struct {
	ubaV1.UnimplementedIDMappingServiceServer

	log *log.Helper

	idMappingRepo *data.IDMappingRepo
}

func NewIDMappingService(
	ctx *bootstrap.Context,
	idMappingRepo *data.IDMappingRepo,
) *IDMappingService {
	svc := &IDMappingService{
		log:           ctx.NewLoggerHelper("id-mapping/service/core-service"),
		idMappingRepo: idMappingRepo,
	}

	svc.init()

	return svc
}

func (s *IDMappingService) init() {
}

func (s *IDMappingService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListIDMappingResponse, error) {
	resp, err := s.idMappingRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *IDMappingService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountIDMappingResponse, error) {
	return s.idMappingRepo.Count(ctx, req)
}

func (s *IDMappingService) Get(ctx context.Context, req *ubaV1.GetIDMappingRequest) (*ubaV1.IDMapping, error) {
	resp, err := s.idMappingRepo.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *IDMappingService) Create(ctx context.Context, req *ubaV1.CreateIDMappingRequest) (*ubaV1.IDMapping, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.idMappingRepo.Create(ctx, req)
}

func (s *IDMappingService) Update(ctx context.Context, req *ubaV1.UpdateIDMappingRequest) (*ubaV1.IDMapping, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.idMappingRepo.Update(ctx, req)
}

func (s *IDMappingService) Delete(ctx context.Context, req *ubaV1.DeleteIDMappingRequest) (*emptypb.Empty, error) {
	if err := s.idMappingRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
