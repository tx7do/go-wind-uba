package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"
	"go-wind-uba/app/core/service/internal/data/ent"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

const (
	// featureFlagExposureEvent 曝光事件名：SDK 首次取值时上报，实验分析以它圈人。
	featureFlagExposureEvent = "$feature_flag"

	// flagExperimentDefaultWindowMs 实验结果缺省分析窗口：近 30 天（毫秒）。
	// 注意：不能写 int64(30*24*time.Hour)——time.Hour 是纳秒，会被误放大 1000 倍。
	flagExperimentDefaultWindowMs = int64(30 * 24 * 60 * 60 * 1000)
)

type FeatureFlagService struct {
	ubaV1.UnimplementedFeatureFlagServiceServer

	log *log.Helper

	flagRepo  *data.FeatureFlagRepo
	dorisRepo *dorisData.AnalyticsRepo
	chRepo    *clickhouse.AnalyticsRepo
}

func NewFeatureFlagService(
	ctx *bootstrap.Context,
	flagRepo *data.FeatureFlagRepo,
	analytics *dorisData.AnalyticsRepo,
	chAnalytics *clickhouse.AnalyticsRepo,
) *FeatureFlagService {
	return &FeatureFlagService{
		log:       ctx.NewLoggerHelper("feature-flag/service/core-service"),
		flagRepo:  flagRepo,
		dorisRepo: analytics,
		chRepo:    chAnalytics,
	}
}

func (s *FeatureFlagService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListFeatureFlagResponse, error) {
	return s.flagRepo.List(ctx, req)
}

func (s *FeatureFlagService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountFeatureFlagResponse, error) {
	return s.flagRepo.Count(ctx, req)
}

func (s *FeatureFlagService) Get(ctx context.Context, req *ubaV1.GetFeatureFlagRequest) (*ubaV1.FeatureFlag, error) {
	return s.flagRepo.Get(ctx, req)
}

func (s *FeatureFlagService) Create(ctx context.Context, req *ubaV1.CreateFeatureFlagRequest) (*ubaV1.FeatureFlag, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	if err := validateFlagData(req.Data); err != nil {
		return nil, err
	}

	return s.flagRepo.Create(ctx, req)
}

func (s *FeatureFlagService) Update(ctx context.Context, req *ubaV1.UpdateFeatureFlagRequest) (*ubaV1.FeatureFlag, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	if err := validateFlagData(req.Data); err != nil {
		return nil, err
	}

	return s.flagRepo.Update(ctx, req)
}

func (s *FeatureFlagService) Delete(ctx context.Context, req *ubaV1.DeleteFeatureFlagRequest) (*emptypb.Empty, error) {
	if err := s.flagRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// EvaluateFlags 为一个身份评估租户下全部启用的开关。
// collector 的 /uba/v1/flags 端点背后就是它；分配是一致性哈希，同一身份结果稳定。
func (s *FeatureFlagService) EvaluateFlags(ctx context.Context, req *ubaV1.EvaluateFlagsRequest) (*ubaV1.EvaluateFlagsResponse, error) {
	if req == nil || req.GetTenantId() == 0 {
		return nil, ubaV1.ErrorBadRequest("tenant_id is required")
	}

	distinctID := distinctIDOf(req.GetUserId(), req.GetDeviceId())
	if distinctID == "" {
		return nil, ubaV1.ErrorBadRequest("user_id or device_id is required")
	}

	flags, err := s.flagRepo.ListEnabledByTenant(ctx, req.GetTenantId(), req.GetAppId())
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(flags))
	for _, flag := range flags {
		if variant := AssignFlagVariant(flag, distinctID); variant != "" {
			result[derefStr(flag.Key)] = variant
		}
	}

	return &ubaV1.EvaluateFlagsResponse{Flags: result}, nil
}

// GetFlagResult 实验结果：各变体曝光人数与目标事件转化率。
func (s *FeatureFlagService) GetFlagResult(ctx context.Context, req *ubaV1.GetFlagResultRequest) (*ubaV1.FlagResultResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	flag, err := s.flagRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	endMs := time.Now().UnixMilli()
	startMs := endMs - flagExperimentDefaultWindowMs
	if tr := req.GetTimeRange(); tr != nil && tr.GetStartMs() != 0 {
		startMs, endMs = tr.GetStartMs(), tr.GetEndMs()
	}

	// events_fact.tenant_id 的过滤口径与分群一致：优先应用，其次租户。
	appFilter := derefU32(flag.AppID)
	if appFilter == 0 {
		appFilter = derefU32(flag.TenantID)
	}

	stats, err := s.resultImpl().FlagExperimentResult(
		ctx, derefStr(flag.Key), derefStr(flag.GoalEventName), appFilter, startMs, endMs,
	)
	if err != nil {
		return nil, err
	}

	byVariant := make(map[string]*ubaV1.FlagVariantResult, len(stats))
	for _, stat := range stats {
		byVariant[stat.Variant] = &ubaV1.FlagVariantResult{
			Variant:        stat.Variant,
			ExposedUsers:   uint64(stat.ExposedUsers),
			ConvertedUsers: uint64(stat.ConvertedUsers),
			ConversionRate: conversionRate(int64(stat.ConvertedUsers), int64(stat.ExposedUsers)),
		}
	}

	// 配置过的变体零值补齐，保证前端展示完整分组
	variants := make([]*ubaV1.FlagVariantResult, 0, len(flag.Variants))
	for _, v := range flag.Variants {
		if res, ok := byVariant[v.Key]; ok {
			variants = append(variants, res)
			delete(byVariant, v.Key)
			continue
		}
		variants = append(variants, &ubaV1.FlagVariantResult{Variant: v.Key})
	}
	// 未配置但出现在数据里的变体（如历史遗留）也返回
	for _, res := range byVariant {
		variants = append(variants, res)
	}

	return &ubaV1.FlagResultResponse{
		Flag:          s.flagRepo.ToDTO(flag),
		Variants:      variants,
		GoalEventName: flag.GoalEventName,
	}, nil
}

// flagResultAnalytics 实验结果聚合能力（Doris/ClickHouse 双实现共用签名）。
type flagResultAnalytics interface {
	FlagExperimentResult(ctx context.Context, flagKey, goalEvent string, appFilter int64, startMs, endMs int64) ([]data.FlagVariantStat, error)
}

func (s *FeatureFlagService) resultImpl() flagResultAnalytics {
	if data.UseClickHouse && s.chRepo != nil {
		return s.chRepo
	}
	return s.dorisRepo
}

// ──────────── 分配逻辑 ────────────

// AssignFlagVariant 一致性哈希分配：同一 flag+身份 永远得到同一变体。
// 返回空串表示未命中放量（rollout 未覆盖）或开关不可用。
func AssignFlagVariant(flag *ent.FeatureFlag, distinctID string) string {
	if !flag.Enabled || len(flag.Variants) == 0 || distinctID == "" {
		return ""
	}

	rollout := flag.RolloutPercentage
	if rollout <= 0 {
		return ""
	}
	if rollout > 100 {
		rollout = 100
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(derefStr(flag.Key)))
	_, _ = h.Write([]byte{'|'})
	_, _ = h.Write([]byte(distinctID))
	bucket := int(h.Sum32() % 10000)

	// 放量：前 rollout% 的桶进入实验
	if bucket >= int(rollout)*100 {
		return ""
	}

	// 变体：按权重千分比归一切分
	totalWeight := int32(0)
	for _, v := range flag.Variants {
		if v.Weight > 0 {
			totalWeight += v.Weight
		}
	}
	if totalWeight <= 0 {
		// 权重全 0：均分
		return flag.Variants[bucket%len(flag.Variants)].Key
	}

	cursor := bucket * int(totalWeight) / 10000
	cumulative := int32(0)
	for _, v := range flag.Variants {
		if v.Weight <= 0 {
			continue
		}
		cumulative += v.Weight
		if int32(cursor) < cumulative {
			return v.Key
		}
	}
	return flag.Variants[len(flag.Variants)-1].Key
}

// distinctIDOf 评估身份：登录用户优先，否则设备。
func distinctIDOf(userID int64, deviceID string) string {
	if userID > 0 {
		return fmt.Sprintf("u:%d", userID)
	}
	if deviceID != "" {
		return fmt.Sprintf("d:%s", deviceID)
	}
	return ""
}

// validateFlagData 创建/更新时的基础校验。
func validateFlagData(data *ubaV1.FeatureFlag) error {
	if data.GetKey() == "" {
		return ubaV1.ErrorBadRequest("flag key is required")
	}
	if data.GetRolloutPercentage() < 0 || data.GetRolloutPercentage() > 100 {
		return ubaV1.ErrorBadRequest("rollout_percentage must be 0~100")
	}
	seen := map[string]bool{}
	for _, v := range data.GetVariants() {
		if v.GetKey() == "" {
			return ubaV1.ErrorBadRequest("variant key is required")
		}
		if seen[v.GetKey()] {
			return ubaV1.ErrorBadRequest("duplicate variant key: %s", v.GetKey())
		}
		seen[v.GetKey()] = true
	}
	return nil
}

func conversionRate(converted, exposed int64) float64 {
	if exposed == 0 {
		return 0
	}
	return float64(converted) / float64(exposed)
}
