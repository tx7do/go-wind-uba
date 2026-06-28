package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"

	"github.com/tx7do/kratos-bootstrap/bootstrap"

	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

const (
	// appAuthCacheKeyPrefix 应用鉴权缓存的 Redis key 前缀。
	appAuthCacheKeyPrefix = "uba:collector:app:"
	// appAuthCacheTTL 应用鉴权缓存有效期。
	appAuthCacheTTL = 10 * time.Minute
)

// cachedApp 缓存到 Redis 的应用鉴权快照，只保留鉴权与租户覆盖所需的必要字段。
type cachedApp struct {
	AppID    string                 `json:"app_id"`
	AppSecret string                `json:"app_secret"`
	Status   ubaV1.Application_Status `json:"status"`
	TenantID uint32                 `json:"tenant_id"`
}

// AppAuthenticator 应用级鉴权器：用请求体中的 app_id 反查应用、比对 app_secret、
// 校验启用状态，并返回携带权威 tenant_id 的应用信息。结果按 app_id 缓存到 Redis，
// 避免每条上报请求都打 gRPC。
type AppAuthenticator struct {
	log                      *log.Helper
	applicationServiceClient ubaV1.ApplicationServiceClient
	redisClient              *redis.Client
}

// NewAppAuthenticator 创建应用鉴权器。
func NewAppAuthenticator(
	ctx *bootstrap.Context,
	applicationServiceClient ubaV1.ApplicationServiceClient,
	redisClient *redis.Client,
) *AppAuthenticator {
	return &AppAuthenticator{
		log:                      ctx.NewLoggerHelper("app-auth/service/collector-service"),
		applicationServiceClient: applicationServiceClient,
		redisClient:              redisClient,
	}
}

// Authenticate 校验上报凭证，返回通过鉴权的应用信息（含权威 tenant_id）。
// 校验顺序：参数非空 -> Redis 命中 -> 未命中则 gRPC 反查并回写缓存 -> 比对密钥与状态。
func (a *AppAuthenticator) Authenticate(ctx context.Context, appID, appSecret string) (*cachedApp, error) {
	if appID == "" || appSecret == "" {
		return nil, collectorV1.ErrorUnauthorized("app_id and app_secret are required")
	}

	app, err := a.loadApp(ctx, appID)
	if err != nil {
		return nil, err
	}

	// 应用状态校验：仅放行启用状态。
	if app.Status != ubaV1.Application_ON {
		a.log.Warnf("app [%s] is not enabled, status=%s", appID, app.Status.String())
		return nil, collectorV1.ErrorUnauthorized("application is disabled")
	}

	// 密钥校验。
	if app.AppSecret != appSecret {
		return nil, collectorV1.ErrorIncorrectAppSecret("app_secret mismatch for app [%s]", appID)
	}

	return app, nil
}

// loadApp 优先从 Redis 读取应用快照，缓存未命中时通过 gRPC 反查并回写。
func (a *AppAuthenticator) loadApp(ctx context.Context, appID string) (*cachedApp, error) {
	// 命中缓存则直接返回。
	if app, ok := a.getAppFromCache(ctx, appID); ok {
		return app, nil
	}

	// 缓存未命中，走 gRPC 反查。
	resp, err := a.applicationServiceClient.Get(ctx, &ubaV1.GetApplicationRequest{
		QueryBy: &ubaV1.GetApplicationRequest_AppId{AppId: appID},
	})
	if err != nil {
		a.log.Errorf("query application by app_id [%s] failed: %s", appID, err.Error())
		return nil, collectorV1.ErrorUnauthorized("invalid app_id")
	}
	if resp == nil {
		return nil, collectorV1.ErrorUnauthorized("invalid app_id")
	}

	app := &cachedApp{
		AppID:     resp.GetAppId(),
		AppSecret: resp.GetAppSecret(),
		Status:    resp.GetStatus(),
		TenantID:  resp.GetTenantId(),
	}

	// 回写缓存；失败不阻断主流程，仅记录日志。
	a.setAppToCache(ctx, app)

	return app, nil
}

// getAppFromCache 从 Redis 读取应用快照。命中返回 (app, true)，否则 (nil, false)。
func (a *AppAuthenticator) getAppFromCache(ctx context.Context, appID string) (*cachedApp, bool) {
	if a.redisClient == nil {
		return nil, false
	}

	val, err := a.redisClient.Get(ctx, appAuthCacheKey(appID)).Bytes()
	if err != nil || len(val) == 0 {
		return nil, false
	}

	app := &cachedApp{}
	if err = json.Unmarshal(val, app); err != nil {
		a.log.Warnf("unmarshal cached app [%s] failed: %s", appID, err.Error())
		return nil, false
	}
	return app, true
}

// setAppToCache 将应用快照写入 Redis。
func (a *AppAuthenticator) setAppToCache(ctx context.Context, app *cachedApp) {
	if a.redisClient == nil || app == nil || app.AppID == "" {
		return
	}

	data, err := json.Marshal(app)
	if err != nil {
		a.log.Warnf("marshal app [%s] failed: %s", app.AppID, err.Error())
		return
	}

	if err = a.redisClient.Set(ctx, appAuthCacheKey(app.AppID), data, appAuthCacheTTL).Err(); err != nil {
		a.log.Warnf("cache app [%s] failed: %s", app.AppID, err.Error())
	}
}

// appAuthCacheKey 拼接应用鉴权缓存的 Redis key。
func appAuthCacheKey(appID string) string {
	return fmt.Sprintf("%s%s", appAuthCacheKeyPrefix, appID)
}
