package main

import (
	"os"

	"github.com/go-kratos/kratos/v2"
	appCrypto "github.com/tx7do/go-utils/crypto"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"go-wind-uba/app/admin/service/internal/data"
	"go-wind-uba/app/admin/service/internal/server"
	"go-wind-uba/app/admin/service/internal/service"
	"go-wind-uba/pkg/middleware/auth"
)

// initApp 手写装配整个应用,是本服务的依赖注入点(替代原 google/wire 生成)。
// initApp assembles the whole application by hand — the dependency injection point.
//
// 装配严格单向分层,自上而下的阅读顺序即依赖方向:
// The wiring is strictly layered; reading top-down follows the dependency direction:
//
//	基础设施 → 服务客户端(data) → 服务层(service) → 传输层(server)
//
// 本服务是 BFF 网关:不直连数据库,仓储层由对 core-service 的 gRPC 服务客户端替代。
// 约定 / Conventions:
//   - 新增模块:先加服务客户端行,再加服务构造行,并传给 NewRestServer;漏接由编译器在调用处报错。
//     To add a module: append a service-client line, then a service constructor, then pass it to
//     NewRestServer; a missing connection is a compile error at the call site.
//   - 持有 cleanup 的资源创建成功后立即注册进 cleanups;任何一步失败,rollback 逆序执行已注册
//     的清理(shutdown 与中途失败共用同一条 LIFO 路径)。
//     Resources owning a cleanup register it immediately; rollback runs them LIFO both on
//     mid-way failure and on shutdown.
//   - 本文件只做构造与传参,不写业务逻辑。
//     Construction and parameter passing only; no business logic in this file.
func initApp(ctx *bootstrap.Context) (*kratos.App, func(), error) {
	// cleanup 注册表:本服务无持有 cleanup 的资源,rollback 为空实现,保持各服务同构的返回签名。
	// No cleanup-owning resources in this service; rollback is a no-op kept for
	// signature parity across the wiring files.
	var cleanups []func()
	rollback := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	// ═══════════════════════ 一、基础设施 ═══════════════════════

	// 全局凭据加密器（AES-256-GCM）：AI 提供商 API Key、SMTP 密码等
	// 凭据类字段落库加密的总开关。key 从 ENCRYPTION_KEY 环境变量读取；
	// 未配置时降级明文并告警（生产必须配置）。
	if encKey := os.Getenv("ENCRYPTION_KEY"); encKey != "" {
		if err := appCrypto.InitGlobalEncryptor(encKey, true); err != nil {
			rollback()
			return nil, nil, err
		}
	} else {
		ctx.NewLoggerHelper("crypto/wiring").Warnf(
			"ENCRYPTION_KEY not set: credential encryption DISABLED (secrets stored in plaintext)")
	}

	discovery := data.NewDiscovery(ctx)
	authenticationServiceClient := data.NewAuthenticationServiceClient(ctx, discovery)
	clientType := data.NewClientType()
	accessTokenChecker := auth.NewTokenChecker(ctx, authenticationServiceClient, clientType)
	engine := data.NewAuthorizer()
	minIOClient := data.NewMinIoClient(ctx)
	sseServer := server.NewSseServer(ctx)

	// ═══════════════════════ 二、服务客户端(internal/data) ═══════════════════════

	apiAuditLogServiceClient := data.NewApiAuditLogServiceClient(ctx, discovery)
	loginAuditLogServiceClient := data.NewLoginAuditLogServiceClient(ctx, discovery)
	userServiceClient := data.NewUserServiceClient(ctx, discovery)
	tenantServiceClient := data.NewTenantServiceClient(ctx, discovery)
	orgUnitServiceClient := data.NewOrgUnitServiceClient(ctx, discovery)
	positionServiceClient := data.NewPositionServiceClient(ctx, discovery)
	roleServiceClient := data.NewRoleServiceClient(ctx, discovery)
	userCredentialServiceClient := data.NewUserCredentialServiceClient(ctx, discovery)
	menuServiceClient := data.NewMenuServiceClient(ctx, discovery)
	apiServiceClient := data.NewApiServiceClient(ctx, discovery)
	permissionServiceClient := data.NewPermissionServiceClient(ctx, discovery)
	permissionGroupServiceClient := data.NewPermissionGroupServiceClient(ctx, discovery)
	taskServiceClient := data.NewTaskServiceClient(ctx, discovery)
	loginPolicyServiceClient := data.NewLoginPolicyServiceClient(ctx, discovery)
	dictTypeServiceClient := data.NewDictTypeServiceClient(ctx, discovery)
	dictEntryServiceClient := data.NewDictEntryServiceClient(ctx, discovery)
	languageServiceClient := data.NewLanguageServiceClient(ctx, discovery)
	fileServiceClient := data.NewFileServiceClient(ctx, discovery)
	internalMessageServiceClient := data.NewInternalMessageServiceClient(ctx, discovery)
	internalMessageCategoryServiceClient := data.NewInternalMessageCategoryServiceClient(ctx, discovery)
	internalMessageRecipientServiceClient := data.NewInternalMessageRecipientServiceClient(ctx, discovery)
	dataAccessAuditLogServiceClient := data.NewDataAccessAuditLogServiceClient(ctx, discovery)
	policyEvaluationLogServiceClient := data.NewPolicyEvaluationLogServiceClient(ctx, discovery)
	operationAuditLogServiceClient := data.NewOperationAuditLogServiceClient(ctx, discovery)
	permissionAuditLogServiceClient := data.NewPermissionAuditLogServiceClient(ctx, discovery)
	applicationServiceClient := data.NewApplicationServiceClient(ctx, discovery)
	idMappingServiceClient := data.NewIDMappingServiceClient(ctx, discovery)
	riskRuleServiceClient := data.NewRiskRuleServiceClient(ctx, discovery)
	tagDefinitionServiceClient := data.NewTagDefinitionServiceClient(ctx, discovery)
	userTagServiceClient := data.NewUserTagServiceClient(ctx, discovery)
	webhookServiceClient := data.NewWebhookServiceClient(ctx, discovery)
	alertRuleServiceClient := data.NewAlertRuleServiceClient(ctx, discovery)
	segmentServiceClient := data.NewSegmentServiceClient(ctx, discovery)
	dashboardServiceClient := data.NewDashboardServiceClient(ctx, discovery)
	reportSubscriptionServiceClient := data.NewReportSubscriptionServiceClient(ctx, discovery)
	notificationServiceClient := data.NewNotificationServiceClient(ctx, discovery)
	notificationRuleServiceClient := data.NewNotificationRuleServiceClient(ctx, discovery)
	notificationTemplateServiceClient := data.NewNotificationTemplateServiceClient(ctx, discovery)
	notificationPreferenceServiceClient := data.NewNotificationPreferenceServiceClient(ctx, discovery)
	notificationChannelServiceClient := data.NewNotificationChannelServiceClient(ctx, discovery)
	campaignServiceClient := data.NewCampaignServiceClient(ctx, discovery)
	visualEventServiceClient := data.NewVisualEventServiceClient(ctx, discovery)
	replayServiceClient := data.NewReplayServiceClient(ctx, discovery)
	importServiceClient := data.NewImportServiceClient(ctx, discovery)
	featureFlagServiceClient := data.NewFeatureFlagServiceClient(ctx, discovery)
	sqlQueryServiceClient := data.NewSqlQueryServiceClient(ctx, discovery)
	eventPathServiceClient := data.NewEventPathServiceClient(ctx, discovery)
	objectServiceClient := data.NewObjectServiceClient(ctx, discovery)
	riskEventServiceClient := data.NewRiskEventServiceClient(ctx, discovery)
	sessionServiceClient := data.NewSessionServiceClient(ctx, discovery)
	userBehaviorProfileServiceClient := data.NewUserBehaviorProfileServiceClient(ctx, discovery)
	behaviorEventServiceClient := data.NewBehaviorEventServiceClient(ctx, discovery)
	analyticsServiceClient := data.NewAnalyticsServiceClient(ctx, discovery)
	eventSchemaServiceClient := data.NewEventSchemaServiceClient(ctx, discovery)

	aiProviderServiceClient := data.NewAiProviderServiceClient(ctx, discovery)
	aiConversationServiceClient := data.NewAiConversationServiceClient(ctx, discovery)
	aiMessageServiceClient := data.NewAiMessageServiceClient(ctx, discovery)
	aiChatServiceClient := data.NewAiChatServiceClient(ctx, discovery)
	aiKnowledgeBaseServiceClient := data.NewAiKnowledgeBaseServiceClient(ctx, discovery)
	aiUsageLogServiceClient := data.NewAiUsageLogServiceClient(ctx, discovery)
	aiQueryServiceClient := data.NewAiQueryServiceClient(ctx, discovery)

	// ── register:client ── 新模块服务客户端在此行后注册(make register 工具锚点,勿删)

	// ═══════════════════════ 三、服务层(internal/service) ═══════════════════════

	userService := service.NewUserService(ctx, userServiceClient, tenantServiceClient, orgUnitServiceClient, positionServiceClient, roleServiceClient, userCredentialServiceClient)
	userProfileService := service.NewUserProfileService(ctx, userServiceClient, tenantServiceClient, orgUnitServiceClient, positionServiceClient, roleServiceClient, userCredentialServiceClient, authenticationServiceClient)
	roleService := service.NewRoleService(ctx, roleServiceClient, tenantServiceClient)
	tenantService := service.NewTenantService(ctx, userServiceClient, userCredentialServiceClient, tenantServiceClient, roleServiceClient)
	orgUnitService := service.NewOrgUnitService(ctx, orgUnitServiceClient, userServiceClient)
	positionService := service.NewPositionService(ctx, positionServiceClient, orgUnitServiceClient)
	menuService := service.NewMenuService(ctx, menuServiceClient)
	apiService := service.NewApiService(ctx, apiServiceClient)
	permissionGroupService := service.NewPermissionGroupService(ctx, permissionServiceClient, permissionGroupServiceClient)
	permissionService := service.NewPermissionService(ctx, permissionServiceClient, permissionGroupServiceClient, roleServiceClient, apiServiceClient, menuServiceClient)
	adminPortalService := service.NewRouterService(ctx, menuServiceClient, permissionServiceClient, roleServiceClient, userServiceClient)
	taskService := service.NewTaskService(ctx, taskServiceClient)
	authenticationService := service.NewAuthenticationService(ctx, authenticationServiceClient, userServiceClient)
	loginPolicyService := service.NewLoginPolicyService(ctx, loginPolicyServiceClient)
	dictTypeService := service.NewDictTypeService(ctx, dictTypeServiceClient)
	dictEntryService := service.NewDictEntryService(ctx, dictEntryServiceClient)
	languageService := service.NewLanguageService(ctx, languageServiceClient)
	fileService := service.NewFileService(ctx, fileServiceClient)
	fileTransferService := service.NewFileTransferService(ctx, minIOClient, fileServiceClient)
	internalMessageService := service.NewInternalMessageService(ctx, internalMessageServiceClient, internalMessageCategoryServiceClient, internalMessageRecipientServiceClient, authenticationServiceClient, userServiceClient, sseServer)
	internalMessageCategoryService := service.NewInternalMessageCategoryService(ctx, internalMessageCategoryServiceClient)
	internalMessageRecipientService := service.NewInternalMessageRecipientService(ctx, internalMessageServiceClient, internalMessageRecipientServiceClient)
	apiAuditLogService := service.NewApiAuditLogService(ctx, apiAuditLogServiceClient, apiServiceClient)
	dataAccessAuditLogService := service.NewDataAccessAuditLogService(ctx, dataAccessAuditLogServiceClient)
	loginAuditLogService := service.NewLoginAuditLogService(ctx, loginAuditLogServiceClient)
	policyEvaluationLogService := service.NewPolicyEvaluationLogService(ctx, policyEvaluationLogServiceClient)
	operationAuditLogService := service.NewOperationAuditLogService(ctx, operationAuditLogServiceClient)
	permissionAuditLogService := service.NewPermissionAuditLogService(ctx, permissionAuditLogServiceClient)
	applicationService := service.NewApplicationService(ctx, applicationServiceClient)
	idMappingService := service.NewIDMappingService(ctx, idMappingServiceClient)
	riskRuleService := service.NewRiskRuleService(ctx, riskRuleServiceClient)
	tagDefinitionService := service.NewTagDefinitionService(ctx, tagDefinitionServiceClient)
	userTagService := service.NewUserTagService(ctx, userTagServiceClient)
	webhookService := service.NewWebhookService(ctx, webhookServiceClient)
	alertRuleService := service.NewAlertRuleService(ctx, alertRuleServiceClient)
	segmentService := service.NewSegmentService(ctx, segmentServiceClient)
	dashboardService := service.NewDashboardService(ctx, dashboardServiceClient)
	reportSubscriptionService := service.NewReportSubscriptionService(ctx, reportSubscriptionServiceClient)
	featureFlagService := service.NewFeatureFlagService(ctx, featureFlagServiceClient)
	notificationBffService := service.NewNotificationService(ctx, notificationServiceClient)
	notificationRuleBffService := service.NewNotificationRuleService(ctx, notificationRuleServiceClient)
	notificationTemplateBffService := service.NewNotificationTemplateService(ctx, notificationTemplateServiceClient)
	notificationPreferenceBffService := service.NewNotificationPreferenceService(ctx, notificationPreferenceServiceClient)
	notificationChannelBffService := service.NewNotificationChannelService(ctx, notificationChannelServiceClient)
	campaignBffService := service.NewCampaignService(ctx, campaignServiceClient)
	visualEventBffService := service.NewVisualEventService(ctx, visualEventServiceClient)
	replayBffService := service.NewReplayService(ctx, replayServiceClient)
	importBffService := service.NewImportService(ctx, importServiceClient)
	sqlQueryService := service.NewSqlQueryService(ctx, sqlQueryServiceClient)
	eventPathService := service.NewEventPathService(ctx, eventPathServiceClient)
	objectService := service.NewObjectService(ctx, objectServiceClient)
	riskEventService := service.NewRiskEventService(ctx, riskEventServiceClient)
	sessionService := service.NewSessionService(ctx, sessionServiceClient)
	userBehaviorProfileService := service.NewUserBehaviorProfileService(ctx, userBehaviorProfileServiceClient)
	behaviorEventService := service.NewBehaviorEventService(ctx, behaviorEventServiceClient)
	analyticsService := service.NewAnalyticsService(ctx, analyticsServiceClient)
	eventSchemaService := service.NewEventSchemaService(ctx, eventSchemaServiceClient)

	aiProviderBffService := service.NewAiProviderService(ctx, aiProviderServiceClient)
	aiConversationBffService := service.NewAiConversationService(ctx, aiConversationServiceClient)
	aiMessageBffService := service.NewAiMessageService(ctx, aiMessageServiceClient)
	aiChatBffService := service.NewAiChatService(ctx, aiChatServiceClient, authenticationServiceClient, sseServer)
	aiKnowledgeBffService := service.NewAiKnowledgeBaseService(ctx, aiKnowledgeBaseServiceClient)
	aiUsageLogBffService := service.NewAiUsageLogService(ctx, aiUsageLogServiceClient)
	aiQueryBffService := service.NewAiQueryService(ctx, aiQueryServiceClient)

	// 实时大屏 SSE 订阅（复用既有 SSE 服务器,推送循环在 service 层,见 realtime_screen_service.go）
	realtimeScreenService := service.NewRealtimeScreenService(ctx, behaviorEventService, riskEventService, sseServer)

	// ── register:service ── 新模块服务在此行后注册(make register 工具锚点,勿删)

	// ═══════════════════════ 四、传输层(internal/server) ═══════════════════════

	restMiddlewares := server.NewRestMiddleware(ctx, accessTokenChecker, engine, apiAuditLogServiceClient, loginAuditLogServiceClient)

	httpServer := server.NewRestServer(ctx, restMiddlewares,
		userService,
		userProfileService,
		roleService,
		tenantService,
		orgUnitService,
		positionService,
		menuService,
		apiService,
		permissionGroupService,
		permissionService,
		adminPortalService,
		taskService,
		authenticationService,
		loginPolicyService,
		dictTypeService,
		dictEntryService,
		languageService,
		fileService,
		fileTransferService,
		internalMessageService,
		internalMessageCategoryService,
		internalMessageRecipientService,
		apiAuditLogService,
		dataAccessAuditLogService,
		loginAuditLogService,
		policyEvaluationLogService,
		operationAuditLogService,
		permissionAuditLogService,
		applicationService,
		idMappingService,
		riskRuleService,
		tagDefinitionService,
		userTagService,
		webhookService,
		alertRuleService,
		segmentService,
		dashboardService,
		reportSubscriptionService,
		featureFlagService,
		sqlQueryService,
		notificationBffService,
		notificationRuleBffService,
		notificationTemplateBffService,
		notificationPreferenceBffService,
		notificationChannelBffService,
		campaignBffService,
		visualEventBffService,
		replayBffService,
		importBffService,
		eventPathService,
		objectService,
		riskEventService,
		sessionService,
		userBehaviorProfileService,
		behaviorEventService,
		analyticsService,
		eventSchemaService,
		aiProviderBffService,
		aiConversationBffService,
		aiMessageBffService,
		aiChatBffService,
		aiKnowledgeBffService,
		aiUsageLogBffService,
		aiQueryBffService,
		realtimeScreenService,
		// register:rest-arg ── 新模块服务实参在此行后追加(make register 工具锚点,勿删)
	)

	return newApp(ctx, httpServer, sseServer), rollback, nil
}
