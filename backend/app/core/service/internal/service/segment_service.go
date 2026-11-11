package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"

	"go-wind-uba/app/core/service/internal/data/ent"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// segmentAnalytics 分群圈选所需的 OLAP 查询能力（Doris/ClickHouse 双实现共用签名）。
type segmentAnalytics interface {
	Segmentation(ctx context.Context, req *ubaV1.SegmentationRequest) (*ubaV1.SegmentationResponse, error)
}

type SegmentService struct {
	ubaV1.UnimplementedSegmentServiceServer

	log *log.Helper

	segmentRepo     *data.SegmentRepo
	segmentUserRepo *data.SegmentUserRepo

	analytics   segmentAnalytics
	chAnalytics segmentAnalytics
}

func NewSegmentService(
	ctx *bootstrap.Context,
	segmentRepo *data.SegmentRepo,
	segmentUserRepo *data.SegmentUserRepo,
	analytics *dorisData.AnalyticsRepo,
	chAnalytics *clickhouse.AnalyticsRepo,
) *SegmentService {
	return &SegmentService{
		log:             ctx.NewLoggerHelper("segment/service/core-service"),
		segmentRepo:     segmentRepo,
		segmentUserRepo: segmentUserRepo,
		analytics:       analytics,
		chAnalytics:     chAnalytics,
	}
}

// analyticsImpl 按 OLAP 引擎选择返回可用的分群查询实现。
func (s *SegmentService) analyticsImpl() segmentAnalytics {
	if data.UseClickHouse {
		return s.chAnalytics
	}
	return s.analytics
}

func (s *SegmentService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListSegmentResponse, error) {
	return s.segmentRepo.List(ctx, req)
}

func (s *SegmentService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountSegmentResponse, error) {
	return s.segmentRepo.Count(ctx, req)
}

func (s *SegmentService) Get(ctx context.Context, req *ubaV1.GetSegmentRequest) (*ubaV1.Segment, error) {
	return s.segmentRepo.Get(ctx, req)
}

func (s *SegmentService) Create(ctx context.Context, req *ubaV1.CreateSegmentRequest) (*ubaV1.Segment, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	created, err := s.segmentRepo.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	if req.GetRefreshOnCreate() {
		return s.Refresh(ctx, &ubaV1.RefreshSegmentRequest{Id: created.GetId()})
	}

	return created, nil
}

func (s *SegmentService) Update(ctx context.Context, req *ubaV1.UpdateSegmentRequest) (*ubaV1.Segment, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.segmentRepo.Update(ctx, req)
}

func (s *SegmentService) Delete(ctx context.Context, req *ubaV1.DeleteSegmentRequest) (*emptypb.Empty, error) {
	if err := s.segmentRepo.Delete(ctx, req); err != nil {
		return nil, err
	}
	// 成员表无软删除外键，随分群一并清理。
	if err := s.segmentUserRepo.DeleteBySegment(ctx, req.GetId()); err != nil {
		s.log.Errorf("cleanup segment users failed: %s", err.Error())
	}

	return &emptypb.Empty{}, nil
}

// Refresh 立即按圈选定义重算人群：OLAP 圈选 → 全量替换成员 → 回写人数与刷新时间。
func (s *SegmentService) Refresh(ctx context.Context, req *ubaV1.RefreshSegmentRequest) (*ubaV1.Segment, error) {
	if req == nil || req.GetId() == 0 {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	entity, err := s.segmentRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	return s.RefreshEntity(ctx, entity)
}

// RefreshEntity 对给定分群实体执行刷新；评估器定时刷新与手动刷新共用。
func (s *SegmentService) RefreshEntity(ctx context.Context, entity *ent.Segment) (*ubaV1.Segment, error) {
	def := data.SegmentDefinitionToDTO(entity.Definition)
	if def == nil || len(def.GetInclude()) == 0 {
		return nil, ubaV1.ErrorBadRequest("segment definition is empty or invalid")
	}

	req := &ubaV1.SegmentationRequest{
		TimeRange: def.TimeRange,
		Include:   def.GetInclude(),
		Exclude:   def.GetExclude(),
		Limit:     def.Limit,
	}
	if entity.AppID != nil {
		req.AppId = entity.AppID
	}
	// 定义未显式给上限时按分群容量截断。
	if req.GetLimit() == 0 || req.GetLimit() > entity.MaxUsers {
		limit := entity.MaxUsers
		req.Limit = &limit
	}

	resp, err := s.analyticsImpl().Segmentation(ctx, req)
	if err != nil {
		s.log.Errorf("segment [%d] refresh query failed: %s", entity.ID, err.Error())
		return nil, err
	}

	userIDs := resp.GetUserIds()
	if err = s.segmentUserRepo.ReplaceUsers(ctx, entity.ID, userIDs); err != nil {
		return nil, err
	}

	now := time.Now()
	if err = s.segmentRepo.UpdateRefreshState(ctx, entity.ID, int64(resp.GetTotal()), now); err != nil {
		return nil, err
	}

	return s.segmentRepo.Get(ctx, &ubaV1.GetSegmentRequest{
		QueryBy: &ubaV1.GetSegmentRequest_Id{Id: entity.ID},
	})
}

// ListDueSegments 透出到期待刷新的分群（评估器定时刷新用）。
func (s *SegmentService) ListDueSegments(ctx context.Context, now time.Time) ([]*ent.Segment, error) {
	return s.segmentRepo.ListDueSegments(ctx, now)
}

// ListUsers 分页查询分群命中的用户。
func (s *SegmentService) ListUsers(ctx context.Context, req *ubaV1.ListSegmentUsersRequest) (*ubaV1.ListSegmentUsersResponse, error) {
	if req == nil || req.GetSegmentId() == 0 {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	entities, total, err := s.segmentUserRepo.ListBySegment(ctx, req.GetSegmentId(), int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}

	items := make([]*ubaV1.SegmentUser, 0, len(entities))
	for _, entity := range entities {
		item := &ubaV1.SegmentUser{UserId: entity.UserID}
		if entity.CreatedAt != nil {
			item.JoinedAt = timestamppb.New(*entity.CreatedAt)
		}
		items = append(items, item)
	}

	return &ubaV1.ListSegmentUsersResponse{
		Items: items,
		Total: int64(total),
	}, nil
}
