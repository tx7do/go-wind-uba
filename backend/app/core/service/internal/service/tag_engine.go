package service

import (
	"context"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-wind-uba/app/core/service/internal/data"
	clickhouseData "go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"
	"go-wind-uba/app/core/service/internal/data/ent"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

const (
	// tagRefreshScanInterval 轮询到期标签的间隔；标签粒度的定时以
	// refresh_interval_seconds 为准（最小 60s），扫描只是触发器。
	tagRefreshScanInterval = time.Minute
	tagRefreshLockKey      = "uba:lock:tag-engine"
	tagRefreshLockTTL      = 55 * time.Second

	// tagPredicateLimit 单个标签单轮命中的用户上限。
	tagPredicateLimit = 50000
)

// tagOLAPRepo 标签引擎所需的 OLAP 能力（Doris/ClickHouse 双实现共用签名）。
type tagOLAPRepo interface {
	SelectUsersByPredicate(ctx context.Context, predicate string, limit int) ([]data.TagUserHit, error)
	BatchCreate(ctx context.Context, dtos []*ubaV1.UserTag) error
}

// TagEngine 动态标签执行器：把 TagDefinition.rule.expression 当作 users_dim 上的
// 布尔谓词周期性执行，命中人群全量替换进 PG 的 user_tags，并同步一份到 OLAP。
// 到期判定按每个标签的 refresh_interval_seconds；表达式黑名单校验在数据层。
type TagEngine struct {
	log *log.Helper

	tagDefRepo  *data.TagDefinitionRepo
	userTagRepo *data.UserTagRepo
	doris       *dorisData.UserTagsRepo
	ch          *clickhouseData.UserTagsRepo
	redis       *redis.Client

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewTagEngine(
	ctx *bootstrap.Context,
	tagDefRepo *data.TagDefinitionRepo,
	userTagRepo *data.UserTagRepo,
	dorisRepo *dorisData.UserTagsRepo,
	chRepo *clickhouseData.UserTagsRepo,
	redisClient *redis.Client,
) *TagEngine {
	return &TagEngine{
		log:         ctx.NewLoggerHelper("tag-engine/service/core-service"),
		tagDefRepo:  tagDefRepo,
		userTagRepo: userTagRepo,
		doris:       dorisRepo,
		ch:          chRepo,
		redis:       redisClient,
	}
}

// olapImpl 按编译期引擎开关路由（与 alertEvaluator/reportEngine 同一约定）。
func (s *TagEngine) olapImpl() tagOLAPRepo {
	if data.UseClickHouse && s.ch != nil {
		return s.ch
	}
	return s.doris
}

// Start 实现 transport.Server：启动刷新循环。
func (s *TagEngine) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.wg.Add(1)
	go s.loop(runCtx)

	return nil
}

// Stop 实现 transport.Server：停止刷新循环。
func (s *TagEngine) Stop(_ context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

func (s *TagEngine) loop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(tagRefreshScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *TagEngine) tick(ctx context.Context) {
	if s.redis != nil {
		acquired, err := s.redis.SetNX(ctx, tagRefreshLockKey, time.Now().UnixMilli(), tagRefreshLockTTL).Result()
		if err != nil {
			s.log.Errorf("acquire tag-engine lock failed: %s", err.Error())
			return
		}
		if !acquired {
			return
		}
	}

	defs, err := s.tagDefRepo.ListDueDynamicTags(ctx, time.Now())
	if err != nil {
		s.log.Errorf("list due dynamic tags failed: %s", err.Error())
		return
	}

	now := time.Now()
	for _, def := range defs {
		if err = s.refreshTag(ctx, def, now); err != nil {
			s.log.Errorf("refresh tag [%d] failed: %s", def.ID, err.Error())
			continue
		}
	}
}

// refreshTag 计算单个动态标签的人群并全量替换。
func (s *TagEngine) refreshTag(ctx context.Context, def *ent.TagDefinition, now time.Time) error {
	rule := def.Rule
	if rule == nil {
		return s.markRefreshed(ctx, def.ID, now)
	}

	rawRule, err := protojson.Marshal(rule)
	if err != nil {
		return err
	}
	parsed := &ubaV1.TagRule{}
	popts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := popts.Unmarshal(rawRule, parsed); err != nil {
		return err
	}

	predicate := parsed.GetExpression()
	if predicate == "" {
		return s.markRefreshed(ctx, def.ID, now)
	}

	hits, err := s.olapImpl().SelectUsersByPredicate(ctx, predicate, tagPredicateLimit)
	if err != nil {
		return err
	}

	pgHits := make([]data.TagUserHit, 0, len(hits))
	dtos := make([]*ubaV1.UserTag, 0, len(hits))
	for _, hit := range hits {
		pgHits = append(pgHits, data.TagUserHit{TenantID: hit.TenantID, UserID: hit.UserID})

		tenantID := hit.TenantID
		userID := hit.UserID
		confidence := float64(1.0)
		source := "algorithm"
		value := "true"
		dtos = append(dtos, &ubaV1.UserTag{
			TenantId:      &tenantID,
			UserId:        &userID,
			TagId:         &def.ID,
			Value:         &value,
			ValueLabel:    def.Name,
			Confidence:    &confidence,
			Source:        &source,
			SourceRuleId:  &def.ID,
			EffectiveTime: timestamppb.New(now),
		})
	}

	if err = s.userTagRepo.ReplaceTagUsers(ctx, def.ID, "true", derefStr(def.Name), pgHits, now); err != nil {
		return err
	}

	// OLAP 侧 user_tags 为 UNIQUE KEY(tenant_id, user_id, tag_id, expire_date)，
	// 只增不删；这里仅追加最新命中，历史由 expire_date 自然过期。
	if err = s.olapImpl().BatchCreate(ctx, dtos); err != nil {
		s.log.Errorf("olap user-tag sync failed: %s", err.Error())
	}

	return s.markRefreshed(ctx, def.ID, now)
}

func (s *TagEngine) markRefreshed(ctx context.Context, id int64, now time.Time) error {
	return s.tagDefRepo.UpdateRefreshState(ctx, id, now)
}
