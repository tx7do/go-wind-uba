package server

import (
	"context"
	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
	"go-wind-uba/app/collector/service/internal/service"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/rpc"

	swaggerUI "github.com/tx7do/kratos-swagger-ui"

	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
	"go-wind-uba/app/collector/service/cmd/server/assets"

	"go-wind-uba/pkg/middleware/auth"
	applogging "go-wind-uba/pkg/middleware/logging"
)

// NewRestMiddleware 创建中间件
func NewRestMiddleware(
	ctx *bootstrap.Context,
	// accessTokenChecker auth.AccessTokenChecker,
	// authorizer authzEngine.Engine,
) []middleware.Middleware {
	var ms []middleware.Middleware
	ms = append(ms, logging.Server(ctx.GetLogger()))

	// add white list for authentication.
	rpc.AddWhiteList(
		collectorV1.OperationReportServiceHealthCheck,
		collectorV1.OperationReportServicePostReport,
		collectorV1.OperationReportServiceListVisualEvents,
		collectorV1.OperationReportServiceUploadReplayChunk,
		collectorV1.OperationReportServicePostFlags,
	)

	ms = append(ms, applogging.Server(
		applogging.WithWriteApiLogFunc(func(ctx context.Context, data *auditV1.ApiAuditLog) error {
			return nil
		}),
		applogging.WithWriteLoginLogFunc(func(ctx context.Context, data *auditV1.LoginAuditLog) error {
			return nil
		}),
	))

	ms = append(ms, selector.Server(
		auth.Server(
			//auth.WithAccessTokenChecker(accessTokenChecker),
			auth.WithInjectMetadata(true),
			auth.WithInjectEnt(true),
		),
		//authz.Server(authorizer),
	).Match(rpc.NewRestWhiteListMatcher()).Build())

	return ms
}

// NewRestServer new an REST server.
func NewRestServer(
	ctx *bootstrap.Context,

	middlewares []middleware.Middleware,
	accessTokenChecker auth.AccessTokenChecker,

	reportService *service.ReportService,

	// register:rest-param ── 新模块服务形参在此行后追加(make register 工具锚点,勿删)
) *http.Server {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Rest == nil {
		return nil
	}

	srv, err := rpc.CreateRestServer(cfg, middlewares...)
	if err != nil {
		panic(err)
	}

	// 全站请求体护栏包在最外层（CORS/pprof 过滤器与路由都在其内层），
	// 使 401/413 的判定发生在任何 body 读取之前。
	srv.Handler = requestBodyGuard(accessTokenChecker)(srv.Handler)

	collectorV1.RegisterReportServiceHTTPServer(srv, reportService)

	// register:rest-route ── 新模块服务路由在此行后注册(make register 工具锚点,勿删)

	if cfg.GetServer().GetRest().GetEnableSwagger() {
		swaggerUI.RegisterSwaggerUIServerWithOption(
			srv,
			swaggerUI.WithTitle("GoWind UBA Collector API"),
			swaggerUI.WithMemoryData(assets.OpenApiData, "yaml"),
		)
	}

	return srv
}
