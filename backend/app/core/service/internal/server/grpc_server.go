package server

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/rpc"

	"go-wind-uba/app/core/service/internal/service"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	dictV1 "go-wind-uba/api/gen/go/dict/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"
	internalMessageV1 "go-wind-uba/api/gen/go/internal_message/service/v1"
	notificationV1 "go-wind-uba/api/gen/go/notification/service/v1"
	notificationChannelV1 "go-wind-uba/api/gen/go/notification_channel/service/v1"
	permissionV1 "go-wind-uba/api/gen/go/permission/service/v1"
	resourceV1 "go-wind-uba/api/gen/go/resource/service/v1"
	storageV1 "go-wind-uba/api/gen/go/storage/service/v1"
	taskV1 "go-wind-uba/api/gen/go/task/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"

	"go-wind-uba/pkg/middleware/ent"
)

func NewGrpcMiddleware(ctx *bootstrap.Context) []middleware.Middleware {
	var ms []middleware.Middleware
	// 链首 panic 兜底：任何 handler panic 转 500，而非崩掉请求 goroutine
	ms = append(ms, recovery.Recovery())
	ms = append(ms, logging.Server(ctx.GetLogger()))
	ms = append(ms, ent.Server())
	return ms
}

// NewGrpcServer new a gRPC server.
func NewGrpcServer(
	ctx *bootstrap.Context,
	middlewares []middleware.Middleware,

	authenticationService *service.AuthenticationService,
	loginPolicyService *service.LoginPolicyService,
	userCredentialService *service.UserCredentialService,

	taskService *service.TaskService,

	fileService *service.FileService,

	dictTypeService *service.DictTypeService,
	dictEntryService *service.DictEntryService,
	languageService *service.LanguageService,

	tenantService *service.TenantService,
	userService *service.UserService,
	roleService *service.RoleService,
	positionService *service.PositionService,
	orgUnitService *service.OrgUnitService,

	menuService *service.MenuService,
	apiService *service.ApiService,
	permissionService *service.PermissionService,
	permissionGroupService *service.PermissionGroupService,
	permissionAuditLogService *service.PermissionAuditLogService,
	policyEvaluationLogService *service.PolicyEvaluationLogService,

	loginAuditLogService *service.LoginAuditLogService,
	apiAuditLogService *service.ApiAuditLogService,
	operationAuditLogService *service.OperationAuditLogService,
	dataAccessAuditLogService *service.DataAccessAuditLogService,

	internalMessageService *service.InternalMessageService,
	internalMessageCategoryService *service.InternalMessageCategoryService,
	internalMessageRecipientService *service.InternalMessageRecipientService,

	applicationService *service.ApplicationService,
	idMappingService *service.IDMappingService,
	riskRuleService *service.RiskRuleService,
	tagDefinitionService *service.TagDefinitionService,
	userTagService *service.UserTagService,
	webhookService *service.WebhookService,
	behaviorEventService *service.BehaviorEventService,
	eventPathService *service.EventPathService,
	objectService *service.ObjectService,
	riskEventService *service.RiskEventService,
	sessionService *service.SessionService,
	userBehaviorProfileService *service.UserBehaviorProfileService,
	analyticsService *service.AnalyticsService,
	eventSchemaService *service.EventSchemaService,

	alertRuleService *service.AlertRuleService,
	segmentService *service.SegmentService,
	dashboardService *service.DashboardService,
	reportSubscriptionService *service.ReportSubscriptionService,
	featureFlagService *service.FeatureFlagService,
	sqlQueryService *service.SqlQueryService,
	notificationService *service.NotificationService,
	notificationRuleService *service.NotificationRuleService,
	notificationTemplateService *service.NotificationTemplateService,
	notificationChannelService *service.NotificationChannelService,
	notificationPreferenceService *service.NotificationPreferenceService,
	campaignService *service.CampaignService,
	visualEventService *service.VisualEventService,
	replayService *service.ReplayService,
	importService *service.ImportService,

	aiProviderService *service.AiProviderService,
	aiConversationService *service.AiConversationService,
	aiMessageService *service.AiMessageService,
	aiUsageLogService *service.AiUsageLogService,
	aiKnowledgeService *service.AiKnowledgeService,
	aiChatService *service.AiChatService,
	aiQueryService *service.AiQueryService,

	// register:grpc-param ── 新模块服务形参在此行后追加(make register 工具锚点,勿删)
) (*grpc.Server, error) {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Server == nil || cfg.Server.Grpc == nil {
		return nil, nil
	}

	srv, err := rpc.CreateGrpcServer(cfg, middlewares...)
	if err != nil {
		return nil, err
	}

	taskV1.RegisterTaskServiceServer(srv, taskService)

	authenticationV1.RegisterLoginPolicyServiceServer(srv, loginPolicyService)
	authenticationV1.RegisterAuthenticationServiceServer(srv, authenticationService)
	authenticationV1.RegisterUserCredentialServiceServer(srv, userCredentialService)

	dictV1.RegisterDictTypeServiceServer(srv, dictTypeService)
	dictV1.RegisterDictEntryServiceServer(srv, dictEntryService)
	dictV1.RegisterLanguageServiceServer(srv, languageService)

	resourceV1.RegisterApiServiceServer(srv, apiService)
	resourceV1.RegisterMenuServiceServer(srv, menuService)

	permissionV1.RegisterPermissionServiceServer(srv, permissionService)
	permissionV1.RegisterPermissionGroupServiceServer(srv, permissionGroupService)
	permissionV1.RegisterPolicyEvaluationLogServiceServer(srv, policyEvaluationLogService)
	permissionV1.RegisterRoleServiceServer(srv, roleService)

	identityV1.RegisterUserServiceServer(srv, userService)
	identityV1.RegisterOrgUnitServiceServer(srv, orgUnitService)
	identityV1.RegisterPositionServiceServer(srv, positionService)
	identityV1.RegisterTenantServiceServer(srv, tenantService)

	auditV1.RegisterLoginAuditLogServiceServer(srv, loginAuditLogService)
	auditV1.RegisterApiAuditLogServiceServer(srv, apiAuditLogService)
	auditV1.RegisterOperationAuditLogServiceServer(srv, operationAuditLogService)
	auditV1.RegisterDataAccessAuditLogServiceServer(srv, dataAccessAuditLogService)
	auditV1.RegisterPermissionAuditLogServiceServer(srv, permissionAuditLogService)

	storageV1.RegisterFileServiceServer(srv, fileService)

	internalMessageV1.RegisterInternalMessageServiceServer(srv, internalMessageService)
	internalMessageV1.RegisterInternalMessageCategoryServiceServer(srv, internalMessageCategoryService)
	internalMessageV1.RegisterInternalMessageRecipientServiceServer(srv, internalMessageRecipientService)

	ubaV1.RegisterApplicationServiceServer(srv, applicationService)
	ubaV1.RegisterIDMappingServiceServer(srv, idMappingService)
	ubaV1.RegisterRiskRuleServiceServer(srv, riskRuleService)
	ubaV1.RegisterTagDefinitionServiceServer(srv, tagDefinitionService)
	ubaV1.RegisterUserTagServiceServer(srv, userTagService)
	ubaV1.RegisterWebhookServiceServer(srv, webhookService)
	ubaV1.RegisterBehaviorEventServiceServer(srv, behaviorEventService)
	ubaV1.RegisterEventPathServiceServer(srv, eventPathService)
	ubaV1.RegisterObjectServiceServer(srv, objectService)
	ubaV1.RegisterRiskEventServiceServer(srv, riskEventService)
	ubaV1.RegisterSessionServiceServer(srv, sessionService)
	ubaV1.RegisterUserBehaviorProfileServiceServer(srv, userBehaviorProfileService)
	ubaV1.RegisterAnalyticsServiceServer(srv, analyticsService)
	ubaV1.RegisterAlertRuleServiceServer(srv, alertRuleService)
	ubaV1.RegisterSegmentServiceServer(srv, segmentService)
	ubaV1.RegisterDashboardServiceServer(srv, dashboardService)
	ubaV1.RegisterReportSubscriptionServiceServer(srv, reportSubscriptionService)
	ubaV1.RegisterFeatureFlagServiceServer(srv, featureFlagService)
	ubaV1.RegisterSqlQueryServiceServer(srv, sqlQueryService)
	notificationV1.RegisterNotificationServiceServer(srv, notificationService)
	notificationV1.RegisterNotificationRuleServiceServer(srv, notificationRuleService)
	notificationV1.RegisterNotificationTemplateServiceServer(srv, notificationTemplateService)
	notificationV1.RegisterNotificationPreferenceServiceServer(srv, notificationPreferenceService)
	notificationChannelV1.RegisterNotificationChannelServiceServer(srv, notificationChannelService)
	ubaV1.RegisterCampaignServiceServer(srv, campaignService)
	ubaV1.RegisterVisualEventServiceServer(srv, visualEventService)
	ubaV1.RegisterReplayServiceServer(srv, replayService)
	ubaV1.RegisterImportServiceServer(srv, importService)
	ubaV1.RegisterEventSchemaServiceServer(srv, eventSchemaService)

	aiV1.RegisterAiProviderServiceServer(srv, aiProviderService)
	aiV1.RegisterAiConversationServiceServer(srv, aiConversationService)
	aiV1.RegisterAiMessageServiceServer(srv, aiMessageService)
	aiV1.RegisterAiUsageLogServiceServer(srv, aiUsageLogService)
	aiV1.RegisterAiKnowledgeBaseServiceServer(srv, aiKnowledgeService)
	aiV1.RegisterAiChatServiceServer(srv, aiChatService)
	aiV1.RegisterAiQueryServiceServer(srv, aiQueryService)

	// register:grpc-route ── 新模块服务路由在此行后注册(make register 工具锚点,勿删)

	return srv, nil
}
