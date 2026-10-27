package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"

	authzEngine "github.com/tx7do/kratos-authz/engine"
	authz "github.com/tx7do/kratos-authz/middleware"

	swaggerUI "github.com/tx7do/kratos-swagger-ui"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/rpc"

	"go-wind-uba/app/admin/service/cmd/server/assets"
	"go-wind-uba/app/admin/service/internal/service"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"
	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"

	"go-wind-uba/pkg/metadata"
	"go-wind-uba/pkg/middleware/auth"
	applogging "go-wind-uba/pkg/middleware/logging"
)

// NewRestMiddleware 创建中间件
func NewRestMiddleware(
	ctx *bootstrap.Context,
	accessTokenChecker auth.AccessTokenChecker,
	authorizer authzEngine.Engine,
	apiAuditLogServiceClient auditV1.ApiAuditLogServiceClient,
	loginAuditLogServiceClient auditV1.LoginAuditLogServiceClient,
) []middleware.Middleware {
	var ms []middleware.Middleware
	// 链首 panic 兜底：任何 handler panic 转 500，而非崩掉请求 goroutine
	ms = append(ms, recovery.Recovery())
	ms = append(ms, logging.Server(ctx.GetLogger()))

	// add white list for authentication.
	rpc.AddWhiteList(
		adminV1.OperationAuthenticationServiceLogin,
	)

	ms = append(ms, applogging.Server(
		applogging.WithWriteApiLogFunc(func(ctx context.Context, data *auditV1.ApiAuditLog) error {
			ctx, _ = metadata.NewContext(ctx, metadata.NewUserOperator(0, 0, 0, identityV1.DataScope_ALL))
			// TODO 如果系统的负载比较小，可以同步写入数据库，否则，建议使用异步方式，即投递进队列。
			_, err := apiAuditLogServiceClient.Create(ctx, &auditV1.CreateApiAuditLogRequest{Data: data})
			return err
		}),
		applogging.WithWriteLoginLogFunc(func(ctx context.Context, data *auditV1.LoginAuditLog) error {
			// TODO 如果系统的负载比较小，可以同步写入数据库，否则，建议使用异步方式，即投递进队列。
			ctx, _ = metadata.NewContext(ctx, metadata.NewUserOperator(0, 0, 0, identityV1.DataScope_ALL))
			_, err := loginAuditLogServiceClient.Create(ctx, &auditV1.CreateLoginAuditLogRequest{Data: data})
			return err
		}),
	))

	ms = append(ms, selector.Server(
		auth.Server(
			auth.WithAccessTokenChecker(accessTokenChecker),
			auth.WithInjectMetadata(true),
			auth.WithInjectEnt(true),
		),
		authz.Server(authorizer),
	).Match(rpc.NewRestWhiteListMatcher()).Build())

	return ms
}

// NewRestServer new an REST server.
func NewRestServer(
	ctx *bootstrap.Context,

	middlewares []middleware.Middleware,

	userService *service.UserService,
	userProfileService *service.UserProfileService,
	roleService *service.RoleService,
	tenantService *service.TenantService,
	orgUnitService *service.OrgUnitService,
	positionService *service.PositionService,

	menuSvc *service.MenuService,
	apiService *service.ApiService,
	permissionGroupService *service.PermissionGroupService,
	permissionService *service.PermissionService,

	adminPortalService *service.AdminPortalService,
	taskService *service.TaskService,

	authenticationService *service.AuthenticationService,
	loginPolicyService *service.LoginPolicyService,

	dictTypeService *service.DictTypeService,
	dictEntryService *service.DictEntryService,
	languageService *service.LanguageService,

	fileSvc *service.FileService,
	fileTransferService *service.FileTransferService,

	internalMessageService *service.InternalMessageService,
	internalMessageCategoryService *service.InternalMessageCategoryService,
	internalMessageRecipientService *service.InternalMessageRecipientService,

	apiAuditLogService *service.ApiAuditLogService,
	dataAccessAuditLogService *service.DataAccessAuditLogService,
	loginAuditLogService *service.LoginAuditLogService,
	policyEvaluationLogService *service.PolicyEvaluationLogService,
	operationAuditLogService *service.OperationAuditLogService,
	permissionAuditLogService *service.PermissionAuditLogService,

	applicationService *service.ApplicationService,
	idMappingService *service.IDMappingService,
	riskRuleService *service.RiskRuleService,
	tagDefinitionService *service.TagDefinitionService,
	userTagService *service.UserTagService,
	webhookService *service.WebhookService,
	alertRuleService *service.AlertRuleService,
	segmentService *service.SegmentService,
	dashboardService *service.DashboardService,
	reportSubscriptionService *service.ReportSubscriptionService,
	featureFlagService *service.FeatureFlagService,
	sqlQueryService *service.SqlQueryService,
	notificationBffService *service.NotificationService,
	notificationRuleBffService *service.NotificationRuleService,
	notificationTemplateBffService *service.NotificationTemplateService,
	notificationPreferenceBffService *service.NotificationPreferenceService,
	notificationChannelBffService *service.NotificationChannelService,
	campaignBffService *service.CampaignService,
	visualEventBffService *service.VisualEventService,
	replayBffService *service.ReplayService,
	importBffService *service.ImportService,
	eventPathService *service.EventPathService,
	objectService *service.ObjectService,
	riskEventService *service.RiskEventService,
	sessionService *service.SessionService,
	userBehaviorProfileService *service.UserBehaviorProfileService,
	behaviorEventService *service.BehaviorEventService,
	analyticsService *service.AnalyticsService,
	eventSchemaService *service.EventSchemaService,

	aiProviderBffService *service.AiProviderService,
	aiConversationBffService *service.AiConversationService,
	aiMessageBffService *service.AiMessageService,
	aiChatBffService *service.AiChatService,
	aiKnowledgeBffService *service.AiKnowledgeBaseService,
	aiUsageLogBffService *service.AiUsageLogService,
	aiQueryBffService *service.AiQueryService,

	realtimeScreenService *service.RealtimeScreenService,

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

	adminV1.RegisterAuthenticationServiceHTTPServer(srv, authenticationService)
	adminV1.RegisterLoginPolicyServiceHTTPServer(srv, loginPolicyService)

	adminV1.RegisterUserProfileServiceHTTPServer(srv, userProfileService)
	adminV1.RegisterUserServiceHTTPServer(srv, userService)
	adminV1.RegisterRoleServiceHTTPServer(srv, roleService)
	adminV1.RegisterTenantServiceHTTPServer(srv, tenantService)
	adminV1.RegisterOrgUnitServiceHTTPServer(srv, orgUnitService)
	adminV1.RegisterPositionServiceHTTPServer(srv, positionService)

	adminV1.RegisterAdminPortalServiceHTTPServer(srv, adminPortalService)
	adminV1.RegisterTaskServiceHTTPServer(srv, taskService)

	adminV1.RegisterDictTypeServiceHTTPServer(srv, dictTypeService)
	adminV1.RegisterDictEntryServiceHTTPServer(srv, dictEntryService)
	adminV1.RegisterLanguageServiceHTTPServer(srv, languageService)

	adminV1.RegisterApiServiceHTTPServer(srv, apiService)
	adminV1.RegisterMenuServiceHTTPServer(srv, menuSvc)
	adminV1.RegisterPermissionGroupServiceHTTPServer(srv, permissionGroupService)
	adminV1.RegisterPermissionServiceHTTPServer(srv, permissionService)

	adminV1.RegisterApiAuditLogServiceHTTPServer(srv, apiAuditLogService)
	adminV1.RegisterDataAccessAuditLogServiceHTTPServer(srv, dataAccessAuditLogService)
	adminV1.RegisterLoginAuditLogServiceHTTPServer(srv, loginAuditLogService)
	adminV1.RegisterOperationAuditLogServiceHTTPServer(srv, operationAuditLogService)
	adminV1.RegisterPermissionAuditLogServiceHTTPServer(srv, permissionAuditLogService)
	adminV1.RegisterPolicyEvaluationLogServiceHTTPServer(srv, policyEvaluationLogService)

	adminV1.RegisterInternalMessageServiceHTTPServer(srv, internalMessageService)
	adminV1.RegisterInternalMessageCategoryServiceHTTPServer(srv, internalMessageCategoryService)
	adminV1.RegisterInternalMessageRecipientServiceHTTPServer(srv, internalMessageRecipientService)

	adminV1.RegisterApplicationServiceHTTPServer(srv, applicationService)
	adminV1.RegisterIDMappingServiceHTTPServer(srv, idMappingService)
	adminV1.RegisterRiskRuleServiceHTTPServer(srv, riskRuleService)
	adminV1.RegisterTagDefinitionServiceHTTPServer(srv, tagDefinitionService)
	adminV1.RegisterUserTagServiceHTTPServer(srv, userTagService)
	adminV1.RegisterWebhookServiceHTTPServer(srv, webhookService)
	adminV1.RegisterAlertRuleServiceHTTPServer(srv, alertRuleService)
	adminV1.RegisterSegmentServiceHTTPServer(srv, segmentService)
	adminV1.RegisterDashboardServiceHTTPServer(srv, dashboardService)
	adminV1.RegisterReportSubscriptionServiceHTTPServer(srv, reportSubscriptionService)
	adminV1.RegisterFeatureFlagServiceHTTPServer(srv, featureFlagService)
	adminV1.RegisterSqlQueryServiceHTTPServer(srv, sqlQueryService)
	adminV1.RegisterNotificationServiceHTTPServer(srv, notificationBffService)
	// 服务端全量导出（:export 手动锚点）：生成 Handler 走 JSON 编码器无法输出文件
	// 字节流，与文件下载同因同解，手动注册，鉴权/审计照走中间件链（见 export_http.go）。
	registerExportRoutes(srv, notificationBffService,
		apiAuditLogService,
		loginAuditLogService,
		operationAuditLogService,
		dataAccessAuditLogService,
		permissionAuditLogService,
		aiUsageLogBffService,
	)
	// 实时大屏 SSE 订阅控制面（数据面走 SSE 服务器）。
	registerRealtimeScreenRoutes(srv, realtimeScreenService)
	adminV1.RegisterNotificationRuleServiceHTTPServer(srv, notificationRuleBffService)
	adminV1.RegisterNotificationTemplateServiceHTTPServer(srv, notificationTemplateBffService)
	adminV1.RegisterNotificationPreferenceServiceHTTPServer(srv, notificationPreferenceBffService)
	adminV1.RegisterNotificationChannelServiceHTTPServer(srv, notificationChannelBffService)
	adminV1.RegisterCampaignServiceHTTPServer(srv, campaignBffService)
	adminV1.RegisterVisualEventServiceHTTPServer(srv, visualEventBffService)
	adminV1.RegisterReplayServiceHTTPServer(srv, replayBffService)
	adminV1.RegisterImportServiceHTTPServer(srv, importBffService)
	adminV1.RegisterEventPathServiceHTTPServer(srv, eventPathService)
	adminV1.RegisterObjectServiceHTTPServer(srv, objectService)
	adminV1.RegisterRiskEventServiceHTTPServer(srv, riskEventService)
	adminV1.RegisterSessionServiceHTTPServer(srv, sessionService)
	adminV1.RegisterUserBehaviorProfileServiceHTTPServer(srv, userBehaviorProfileService)
	adminV1.RegisterBehaviorEventServiceHTTPServer(srv, behaviorEventService)
	adminV1.RegisterAnalyticsServiceHTTPServer(srv, analyticsService)
	adminV1.RegisterEventSchemaServiceHTTPServer(srv, eventSchemaService)

	adminV1.RegisterAiProviderServiceHTTPServer(srv, aiProviderBffService)
	adminV1.RegisterAiConversationServiceHTTPServer(srv, aiConversationBffService)
	adminV1.RegisterAiMessageServiceHTTPServer(srv, aiMessageBffService)
	adminV1.RegisterAiChatServiceHTTPServer(srv, aiChatBffService)
	adminV1.RegisterAiKnowledgeBaseServiceHTTPServer(srv, aiKnowledgeBffService)
	adminV1.RegisterAiUsageLogServiceHTTPServer(srv, aiUsageLogBffService)
	adminV1.RegisterAiQueryServiceHTTPServer(srv, aiQueryBffService)

	// 注册文件传输服务，用于处理文件上传下载等功能
	// TODO 它不能够使用代码生成器生成的Handler，需要手动注册。代码生成器生成的Handler无法处理文件上传下载的请求。
	// 但，代码生成器生成代码可以提供给OpenAPI使用。
	registerFileTransferServiceHandler(srv, fileTransferService)
	adminV1.RegisterFileServiceHTTPServer(srv, fileSvc)

	// register:rest-route ── 新模块服务路由在此行后注册(make register 工具锚点,勿删)

	if cfg.GetServer().GetRest().GetEnableSwagger() {
		swaggerUI.RegisterSwaggerUIServerWithOption(
			srv,
			swaggerUI.WithTitle("GoWind UBA Admin API"),
			swaggerUI.WithMemoryData(assets.OpenApiData, "yaml"),
		)
	}

	return srv
}
