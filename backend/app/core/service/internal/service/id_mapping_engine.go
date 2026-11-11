package service

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data"
	clickhouseData "go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

const (
	// idMappingBuildInterval 每轮构建的间隔。
	idMappingBuildInterval = 30 * time.Minute
	idMappingBuildLockKey  = "uba:lock:id-mapping"
	idMappingBuildLockTTL  = 25 * time.Minute

	// idMappingLookback 每轮回扫的事件窗口；窗口内有重复对也没关系，PG 侧按唯一键 upsert。
	idMappingLookback = 24 * time.Hour
)

// idMappingDeviceConfidence 算法关联（同设备同账号共现）的置信度。
// 用变量而非常量：落库时 DTO 字段需要可取地址的指针。
var idMappingDeviceConfidence = float32(0.9)

// idMappingOLAPRepo ID-Mapping 引擎所需的 OLAP 能力（Doris/ClickHouse 双实现共用签名）。
type idMappingOLAPRepo interface {
	ListUserDevicePairs(ctx context.Context, startMs, endMs int64, limit int) ([]data.IdentityPair, error)
	BatchCreate(ctx context.Context, dtos []*ubaV1.IDMapping) error
}

// IDMappingEngine 构建跨端身份关联（登录用户 ↔ 设备）：定期回扫 events_fact 里
// user_id>0 且带 device_id 的事件，把共现对以 link_source=algorithm upsert 进
// PG 的 id_mapping，并同步一份到 OLAP 供分析 join（UNIQUE KEY 幂等）。
type IDMappingEngine struct {
	log *log.Helper

	pgRepo *data.IDMappingRepo
	doris  *dorisData.IDMappingRepo
	ch     *clickhouseData.IDMappingRepo
	redis  *redis.Client

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewIDMappingEngine(
	ctx *bootstrap.Context,
	pgRepo *data.IDMappingRepo,
	dorisRepo *dorisData.IDMappingRepo,
	chRepo *clickhouseData.IDMappingRepo,
	redisClient *redis.Client,
) *IDMappingEngine {
	return &IDMappingEngine{
		log:    ctx.NewLoggerHelper("id-mapping-engine/service/core-service"),
		pgRepo: pgRepo,
		doris:  dorisRepo,
		ch:     chRepo,
		redis:  redisClient,
	}
}

// olapImpl 按编译期引擎开关路由（与 alertEvaluator/reportEngine 同一约定）。
func (s *IDMappingEngine) olapImpl() idMappingOLAPRepo {
	if data.UseClickHouse && s.ch != nil {
		return s.ch
	}
	return s.doris
}

// Start 实现 transport.Server：启动构建循环。
func (s *IDMappingEngine) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.wg.Add(1)
	go s.loop(runCtx)

	return nil
}

// Stop 实现 transport.Server：停止构建循环。
func (s *IDMappingEngine) Stop(_ context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

func (s *IDMappingEngine) loop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(idMappingBuildInterval)
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

func (s *IDMappingEngine) tick(ctx context.Context) {
	if s.redis != nil {
		acquired, err := s.redis.SetNX(ctx, idMappingBuildLockKey, time.Now().UnixMilli(), idMappingBuildLockTTL).Result()
		if err != nil {
			s.log.Errorf("acquire id-mapping lock failed: %s", err.Error())
			return
		}
		if !acquired {
			return
		}
	}

	now := time.Now()
	pairs, err := s.olapImpl().ListUserDevicePairs(
		ctx,
		now.Add(-idMappingLookback).UnixMilli(),
		now.UnixMilli(),
		20000,
	)
	if err != nil {
		s.log.Errorf("list user device pairs failed: %s", err.Error())
		return
	}
	if len(pairs) == 0 {
		return
	}

	olap := s.olapImpl()
	var built int
	for _, pair := range pairs {
		globalUserID := strconv.FormatInt(int64(pair.UserID), 10)

		if err = s.pgRepo.UpsertDeviceMapping(ctx, pair.TenantID, globalUserID, pair.DeviceID, now); err != nil {
			s.log.Errorf("upsert id mapping failed (tenant=%d user=%d device=%s): %s",
				pair.TenantID, pair.UserID, pair.DeviceID, err.Error())
			continue
		}

		// OLAP 侧 UNIQUE KEY(global_user_id, tenant_id, id_type, id_value)，
		// 重复插入按最后写入生效，天然幂等；失败不影响 PG 主数据。
		idType := "device_id"
		linkSource := "algorithm"
		isActive := true
		if err = olap.BatchCreate(ctx, []*ubaV1.IDMapping{{
			TenantId:     &pair.TenantID,
			GlobalUserId: &globalUserID,
			IdType:       &idType,
			IdValue:      &pair.DeviceID,
			Confidence:   &idMappingDeviceConfidence,
			LinkSource:   &linkSource,
			IsActive:     &isActive,
		}}); err != nil {
			s.log.Errorf("olap id-mapping sync failed: %s", err.Error())
		}

		built++
	}

	s.log.Infof("id-mapping build done: %d pairs scanned, %d upserted", len(pairs), built)
}
