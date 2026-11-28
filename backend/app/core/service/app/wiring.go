package app

import (
	"os"

	"github.com/go-kratos/kratos/v2"
	appCrypto "github.com/tx7do/go-utils/crypto"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/channel"
	"go-wind-uba/app/core/service/internal/data/clickhouse"
	"go-wind-uba/app/core/service/internal/data/client"
	"go-wind-uba/app/core/service/internal/data/doris"
	"go-wind-uba/app/core/service/internal/server"
	"go-wind-uba/app/core/service/internal/service"
)

// initApp 手写装配整个应用,是本服务的依赖注入点(替代原 google/wire 生成)。
// initApp assembles the whole application by hand — the dependency injection point.
//
// 装配严格单向分层,自上而下的阅读顺序即依赖方向:
// The wiring is strictly layered; reading top-down follows the dependency direction:
//
//	基础设施 → 仓储层(data) → 服务层(service) → 传输层(server)
//
// 约定 / Conventions:
//   - 新增仓储/服务:在对应分层小节追加一行构造,并传给下游消费者;漏接由编译器在调用处报错。
//     To add a repo/service: append one constructor line in its layer section, then pass it to
//     downstream consumers; a missing connection is a compile error at the call site.
//   - 持有 cleanup 的资源创建成功后立即注册进 cleanups;任何一步失败,rollback 逆序执行已注册
//     的清理(shutdown 与中途失败共用同一条 LIFO 路径)。
//     Resources owning a cleanup register it immediately; rollback runs them LIFO both on
//     mid-way failure and on shutdown.
//   - 本文件只做构造与传参,不写业务逻辑。
//     Construction and parameter passing only; no business logic in this file.
func initApp(ctx *bootstrap.Context) (*kratos.App, func(), error) {
	// cleanup 注册表:rollback 时逆序执行。
	// Cleanup registry; rollback runs entries in reverse order.
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

	authenticatorOption := data.NewAuthenticatorConfig(ctx)
	redisClient, cleanupRedis, err := client.NewRedisClient(ctx)
	if err != nil {
		rollback()
		return nil, nil, err
	}
	cleanups = append(cleanups, cleanupRedis)

	userTokenCache := data.NewUserTokenCache(ctx, redisClient)
	authenticator := data.NewAuthenticator(ctx, authenticatorOption, userTokenCache)
	entClient, cleanupEnt, err := client.NewEntClient(ctx)
	if err != nil {
		rollback()
		return nil, nil, err
	}
	cleanups = append(cleanups, cleanupEnt)

	crypto := data.NewPasswordCrypto()
	minIOClient := client.NewMinIoClient(ctx)
	kafkaBroker := client.NewKafkaBroker(ctx)
	dorisClient, cleanupDoris, err := client.NewDorisClient(ctx)
	if err != nil {
		rollback()
		return nil, nil, err
	}
	cleanups = append(cleanups, cleanupDoris)

	clickhouseClient, clickhouseAdhocConn, cleanupClickHouse, err := client.NewClickHouseClient(ctx)
	if err != nil {
		rollback()
		return nil, nil, err
	}
	cleanups = append(cleanups, cleanupClickHouse)

	// ═══════════════════════ 二、仓储层(internal/data) ═══════════════════════

	userCredentialRepo := data.NewUserCredentialRepo(ctx, entClient, crypto)
	userRoleRepo := data.NewUserRoleRepo(ctx, entClient)
	userOrgUnitRepo := data.NewUserOrgUnitRepo(ctx, entClient)
	userPositionRepo := data.NewUserPositionRepo(ctx, entClient)
	userRepo := data.NewUserRepo(ctx, entClient, userRoleRepo, userOrgUnitRepo, userPositionRepo)
	rolePermissionRepo := data.NewRolePermissionRepo(ctx, entClient)
	permissionApiRepo := data.NewPermissionApiRepo(ctx, entClient)
	permissionMenuRepo := data.NewPermissionMenuRepo(ctx, entClient)
	permissionRepo := data.NewPermissionRepo(ctx, entClient, permissionApiRepo, permissionMenuRepo)
	roleMetadataRepo := data.NewRoleMetadataRepo(ctx, entClient)
	roleRepo := data.NewRoleRepo(ctx, entClient, rolePermissionRepo, permissionRepo, roleMetadataRepo, userRoleRepo)
	tenantRepo := data.NewTenantRepo(ctx, entClient)
	loginPolicyRepo := data.NewLoginPolicyRepo(ctx, entClient)
	taskRepo := data.NewTaskRepo(ctx, entClient)
	fileRepo := data.NewFileRepo(ctx, entClient)
	dictTypeRepo := data.NewDictTypeRepo(ctx, entClient)
	dictEntryI18nRepo := data.NewDictEntryI18nRepo(ctx, entClient)
	dictEntryRepo := data.NewDictEntryRepo(ctx, entClient, dictEntryI18nRepo)
	languageRepo := data.NewLanguageRepo(ctx, entClient)
	positionRepo := data.NewPositionRepo(ctx, entClient)
	orgUnitRepo := data.NewOrgUnitRepo(ctx, entClient)
	menuRepo := data.NewMenuRepo(ctx, entClient)
	apiRepo := data.NewApiRepo(ctx, entClient)
	permissionGroupRepo := data.NewPermissionGroupRepo(ctx, entClient)
	permissionAuditLogRepo := data.NewPermissionAuditLogRepo(ctx, entClient)
	policyEvaluationLogRepo := data.NewPolicyEvaluationLogRepo(ctx, entClient)
	loginAuditLogRepo := data.NewLoginAuditLogRepo(ctx, entClient)
	apiAuditLogRepo := data.NewApiAuditLogRepo(ctx, entClient)
	operationAuditLogRepo := data.NewOperationAuditLogRepo(ctx, entClient)
	dataAccessAuditLogRepo := data.NewDataAccessAuditLogRepo(ctx, entClient)
	internalMessageRepo := data.NewInternalMessageRepo(ctx, entClient)
	internalMessageCategoryRepo := data.NewInternalMessageCategoryRepo(ctx, entClient)
	internalMessageRecipientRepo := data.NewInternalMessageRecipientRepo(ctx, entClient)
	applicationRepo := data.NewApplicationRepo(ctx, entClient)
	idMappingRepo := data.NewIDMappingRepo(ctx, entClient)
	dorisIDMappingRepo := doris.NewIDMappingRepo(ctx, dorisClient)
	clickhouseIDMappingRepo := clickhouse.NewIDMappingRepo(ctx, clickhouseClient)
	riskRuleRepo := data.NewRiskRuleRepo(ctx, entClient)
	tagDefinitionRepo := data.NewTagDefinitionRepo(ctx, entClient)
	userTagRepo := data.NewUserTagRepo(ctx, entClient)
	userTagsRepo := doris.NewUserTagsRepo(ctx, dorisClient)
	clickhouseUserTagsRepo := clickhouse.NewUserTagsRepo(ctx, clickhouseClient)
	webhookRepo := data.NewWebhookRepo(ctx, entClient)
	eventsFactRepo := doris.NewEventsFactRepo(ctx, dorisClient)
	clickhouseEventsFactRepo := clickhouse.NewEventsFactRepo(ctx, clickhouseClient)
	pathFeaturesRepo := doris.NewPathFeaturesRepo(ctx, dorisClient)
	clickhousePathFeaturesRepo := clickhouse.NewPathFeaturesRepo(ctx, clickhouseClient)
	objectsDimRepo := doris.NewObjectsDimRepo(ctx, dorisClient)
	clickhouseObjectsDimRepo := clickhouse.NewObjectsDimRepo(ctx, clickhouseClient)
	riskEventsRepo := doris.NewRiskEventsRepo(ctx, dorisClient)
	clickhouseRiskEventsRepo := clickhouse.NewRiskEventsRepo(ctx, clickhouseClient)
	sessionsFactRepo := doris.NewSessionsFactRepo(ctx, dorisClient)
	clickhouseSessionsFactRepo := clickhouse.NewSessionsFactRepo(ctx, clickhouseClient)
	usersDimRepo := doris.NewUsersDimRepo(ctx, dorisClient)
	clickhouseUsersDimRepo := clickhouse.NewUsersDimRepo(ctx, clickhouseClient)
	analyticsRepo := doris.NewAnalyticsRepo(ctx, dorisClient)
	clickhouseAnalyticsRepo := clickhouse.NewAnalyticsRepo(ctx, clickhouseClient, clickhouseAdhocConn)
	eventSchemaRepo := data.NewEventSchemaRepo(ctx, entClient)

	webhookDeliveryLogRepo := data.NewWebhookDeliveryLogRepo(ctx, entClient)
	alertRuleRepo := data.NewAlertRuleRepo(ctx, entClient)
	segmentRepo := data.NewSegmentRepo(ctx, entClient)
	segmentUserRepo := data.NewSegmentUserRepo(ctx, entClient)
	dashboardRepo := data.NewDashboardRepo(ctx, entClient)
	reportSubscriptionRepo := data.NewReportSubscriptionRepo(ctx, entClient)
	featureFlagRepo := data.NewFeatureFlagRepo(ctx, entClient)
	notificationChannelRepo := data.NewNotificationChannelRepo(ctx, entClient)
	notificationDeliveryRepo := data.NewNotificationDeliveryRepo(ctx, entClient)
	notificationPreferenceRepo := data.NewNotificationPreferenceRepo(ctx, entClient)
	notificationRuleRepo := data.NewNotificationRuleRepo(ctx, entClient)
	notificationTemplateRepo := data.NewNotificationTemplateRepo(ctx, entClient)
	campaignRepo := data.NewCampaignRepo(ctx, entClient)
	visualEventRepo := data.NewVisualEventRepo(ctx, entClient)
	replayRepo := data.NewReplayRepo(ctx, entClient)
	importJobRepo := data.NewImportJobRepo(ctx, entClient)

	// AI 域：provider/会话/消息/知识库/用量流水（向量列由 knowledge 服务启动期补建）
	aiProviderRepo := data.NewAiProviderRepo(ctx, entClient)
	aiConversationRepo := data.NewAiConversationRepo(ctx, entClient)
	aiMessageRepo := data.NewAiMessageRepo(ctx, entClient)
	aiKnowledgeRepo := data.NewAiKnowledgeRepo(ctx, entClient)
	aiUsageLogRepo := data.NewAiUsageLogRepo(ctx, entClient)
	aiTenantQuotaRepo := data.NewAiTenantQuotaRepo(ctx, entClient)

	// ── register:repo ── 新模块仓储在此行后注册(make register 工具锚点,勿删)

	// ═══════════════════════ 三、服务层(internal/service) ═══════════════════════

	authenticationService := service.NewAuthenticationService(ctx, authenticator, userCredentialRepo, userRepo, roleRepo, tenantRepo, permissionRepo)
	loginPolicyService := service.NewLoginPolicyService(ctx, loginPolicyRepo)
	userCredentialService := service.NewUserCredentialService(ctx, userCredentialRepo)
	taskService := service.NewTaskService(ctx, taskRepo, userRepo)
	fileService := service.NewFileService(ctx, fileRepo, minIOClient)
	dictTypeService := service.NewDictTypeService(ctx, dictTypeRepo)
	dictEntryService := service.NewDictEntryService(ctx, dictEntryRepo)
	languageService := service.NewLanguageService(ctx, languageRepo)
	tenantService := service.NewTenantService(ctx, tenantRepo, userRepo, userCredentialRepo, roleRepo)
	userService := service.NewUserService(ctx, userRepo, roleRepo, userCredentialRepo, positionRepo, orgUnitRepo, tenantRepo)
	roleService := service.NewRoleService(ctx, roleRepo, tenantRepo, userRoleRepo)
	positionService := service.NewPositionService(ctx, positionRepo, orgUnitRepo)
	orgUnitService := service.NewOrgUnitService(ctx, orgUnitRepo, userRepo)
	menuService := service.NewMenuService(ctx, menuRepo)
	apiService := service.NewApiService(ctx, apiRepo)
	permissionService := service.NewPermissionService(ctx, permissionRepo, permissionGroupRepo, menuRepo, apiRepo, roleRepo)
	permissionGroupService := service.NewPermissionGroupService(ctx, permissionGroupRepo, permissionRepo)
	permissionAuditLogService := service.NewPermissionAuditLogService(ctx, permissionAuditLogRepo)
	policyEvaluationLogService := service.NewPolicyEvaluationLogService(ctx, policyEvaluationLogRepo)
	loginAuditLogService := service.NewLoginAuditLogService(ctx, loginAuditLogRepo)
	apiAuditLogService := service.NewApiAuditLogService(ctx, apiAuditLogRepo, apiRepo)
	operationAuditLogService := service.NewOperationAuditLogService(ctx, operationAuditLogRepo)
	dataAccessAuditLogService := service.NewDataAccessAuditLogService(ctx, dataAccessAuditLogRepo)
	internalMessageService := service.NewInternalMessageService(ctx, internalMessageRepo, internalMessageCategoryRepo, internalMessageRecipientRepo, userRepo)
	internalMessageCategoryService := service.NewInternalMessageCategoryService(ctx, internalMessageCategoryRepo)
	internalMessageRecipientService := service.NewInternalMessageRecipientService(ctx, internalMessageRepo, internalMessageRecipientRepo)
	applicationService := service.NewApplicationService(ctx, applicationRepo)
	idMappingService := service.NewIDMappingService(ctx, idMappingRepo, dorisIDMappingRepo, clickhouseIDMappingRepo)
	riskRuleService := service.NewRiskRuleService(ctx, riskRuleRepo)
	tagDefinitionService := service.NewTagDefinitionService(ctx, tagDefinitionRepo)
	userTagService := service.NewUserTagService(ctx, userTagRepo, userTagsRepo, clickhouseUserTagsRepo)
	webhookService := service.NewWebhookService(ctx, webhookRepo, webhookDeliveryLogRepo)
	webhookSender := service.NewWebhookSender(ctx, webhookRepo, webhookDeliveryLogRepo)
	behaviorEventService := service.NewBehaviorEventService(ctx, eventsFactRepo, clickhouseEventsFactRepo)
	eventPathService := service.NewEventPathService(ctx, pathFeaturesRepo, clickhousePathFeaturesRepo)
	objectService := service.NewObjectService(ctx, objectsDimRepo, clickhouseObjectsDimRepo)
	riskEventService := service.NewRiskEventService(ctx, riskEventsRepo, clickhouseRiskEventsRepo)
	sessionService := service.NewSessionService(ctx, sessionsFactRepo, clickhouseSessionsFactRepo)
	userBehaviorProfileService := service.NewUserBehaviorProfileService(ctx, usersDimRepo, clickhouseUsersDimRepo)
	analyticsService := service.NewAnalyticsService(ctx, analyticsRepo, clickhouseAnalyticsRepo)
	eventSchemaService := service.NewEventSchemaService(ctx, eventSchemaRepo)
	segmentService := service.NewSegmentService(ctx, segmentRepo, segmentUserRepo, analyticsRepo, clickhouseAnalyticsRepo)
	alertRuleService := service.NewAlertRuleService(ctx, alertRuleRepo)
	dashboardService := service.NewDashboardService(ctx, dashboardRepo)
	// ── 通知域：渠道花名册 → 发送引擎 → 规则/模板/偏好（与上游 go-wind-admin 同构） ──
	transactionalMailer := &service.TransactionMailer{TemplateRepo: notificationTemplateRepo}
	channelRegistry := channel.NewRegistry()
	channelRegistry.Register(channel.NewEmailSender(notificationChannelRepo))
	channelRegistry.Register(channel.NewWebhookSender(notificationChannelRepo))
	channelRegistry.Register(channel.NewSmsSender(notificationChannelRepo))
	notificationService := service.NewNotificationService(ctx, notificationDeliveryRepo, notificationRuleRepo, notificationTemplateRepo, channelRegistry)
	notificationTemplateService := service.NewNotificationTemplateService(ctx, notificationTemplateRepo)
	notificationRuleService := service.NewNotificationRuleService(ctx, notificationRuleRepo, notificationChannelRepo, notificationService)
	notificationChannelService := service.NewNotificationChannelService(ctx, notificationChannelRepo, notificationService, transactionalMailer)
	notificationPreferenceService := service.NewNotificationPreferenceService(ctx, notificationPreferenceRepo, internalMessageCategoryRepo)
	campaignService := service.NewCampaignService(ctx, campaignRepo, segmentRepo, segmentUserRepo)
	visualEventService := service.NewVisualEventService(ctx, visualEventRepo)
	replayService := service.NewReplayService(ctx, replayRepo)
	importService := service.NewImportService(ctx, importJobRepo, eventsFactRepo, clickhouseEventsFactRepo)
	campaignService.RegisterTaskEnqueuer(taskService)
	campaignService.RegisterNotifier(notificationService)

	// INTERNAL 渠道实现反向依赖服务层（收件行落库 + SSE 推送在 InternalMessageService 里）
	channelRegistry.Register(service.NewInternalMessageSender(internalMessageService))

	alertEvaluator := service.NewAlertEvaluator(ctx, alertRuleRepo, internalMessageRepo, internalMessageRecipientRepo, segmentService, webhookSender, redisClient, analyticsRepo, clickhouseAnalyticsRepo)
	riskEventNotifier := service.NewRiskEventNotifier(ctx, kafkaBroker, webhookSender)
	idMappingEngine := service.NewIDMappingEngine(ctx, idMappingRepo, dorisIDMappingRepo, clickhouseIDMappingRepo, redisClient)
	tagEngine := service.NewTagEngine(ctx, tagDefinitionRepo, userTagRepo, userTagsRepo, clickhouseUserTagsRepo, redisClient)
	reportSubscriptionService := service.NewReportSubscriptionService(ctx, reportSubscriptionRepo)
	featureFlagService := service.NewFeatureFlagService(ctx, featureFlagRepo, analyticsRepo, clickhouseAnalyticsRepo)
	sqlQueryService := service.NewSqlQueryService(ctx, analyticsRepo, clickhouseAnalyticsRepo, dataAccessAuditLogRepo)

	reportEngine := service.NewReportEngine(ctx, reportSubscriptionRepo, internalMessageRepo, internalMessageRecipientRepo, webhookSender, redisClient, analyticsRepo, clickhouseAnalyticsRepo, aiProviderRepo, aiUsageLogRepo)

	// AI 域：CRUD 透传 + 流式对话 + RAG 知识库 + OLAP 智能问数
	aiProviderService := service.NewAiProviderService(ctx, aiProviderRepo)
	aiConversationService := service.NewAiConversationService(ctx, aiConversationRepo)
	aiMessageService := service.NewAiMessageService(ctx, aiMessageRepo)
	aiUsageLogService := service.NewAiUsageLogService(ctx, aiUsageLogRepo, aiTenantQuotaRepo)
	aiKnowledgeService := service.NewAiKnowledgeService(ctx, aiKnowledgeRepo, aiProviderRepo, aiUsageLogRepo, eventSchemaRepo, taskService, aiTenantQuotaRepo)
	aiChatService := service.NewAiChatService(ctx, aiConversationRepo, aiMessageRepo, aiProviderRepo, aiUsageLogRepo, aiKnowledgeRepo, entClient, analyticsRepo, clickhouseAnalyticsRepo, eventSchemaRepo, aiTenantQuotaRepo)
	aiQueryService := service.NewAiQueryService(ctx, aiProviderRepo, aiUsageLogRepo, analyticsRepo, clickhouseAnalyticsRepo, dataAccessAuditLogRepo, eventSchemaRepo, aiTenantQuotaRepo)

	// ── register:service ── 新模块服务在此行后注册(make register 工具锚点,勿删)

	// ═══════════════════════ 四、传输层(internal/server) ═══════════════════════

	grpcMiddlewares := server.NewGrpcMiddleware(ctx)

	grpcServer, err := server.NewGrpcServer(ctx, grpcMiddlewares,
		authenticationService,
		loginPolicyService,
		userCredentialService,
		taskService,
		fileService,
		dictTypeService,
		dictEntryService,
		languageService,
		tenantService,
		userService,
		roleService,
		positionService,
		orgUnitService,
		menuService,
		apiService,
		permissionService,
		permissionGroupService,
		permissionAuditLogService,
		policyEvaluationLogService,
		loginAuditLogService,
		apiAuditLogService,
		operationAuditLogService,
		dataAccessAuditLogService,
		internalMessageService,
		internalMessageCategoryService,
		internalMessageRecipientService,
		applicationService,
		idMappingService,
		riskRuleService,
		tagDefinitionService,
		userTagService,
		webhookService,
		behaviorEventService,
		eventPathService,
		objectService,
		riskEventService,
		sessionService,
		userBehaviorProfileService,
		analyticsService,
		eventSchemaService,
		alertRuleService,
		segmentService,
		dashboardService,
		reportSubscriptionService,
		featureFlagService,
		sqlQueryService,
		notificationService,
		notificationRuleService,
		notificationTemplateService,
		notificationChannelService,
		notificationPreferenceService,
		campaignService,
		visualEventService,
		replayService,
		importService,
		aiProviderService,
		aiConversationService,
		aiMessageService,
		aiUsageLogService,
		aiKnowledgeService,
		aiChatService,
		aiQueryService,
		// register:grpc-arg ── 新模块服务实参在此行后追加(make register 工具锚点,勿删)
	)
	if err != nil {
		rollback()
		return nil, nil, err
	}

	asynqServer := server.NewAsynqServer(ctx, taskService, notificationService, campaignService, replayService, aiKnowledgeService)

	return newApp(ctx, grpcServer, asynqServer, alertEvaluator, riskEventNotifier, idMappingEngine, tagEngine, reportEngine), rollback, nil
}
