package data

import (
	notificationV1 "go-wind-uba/api/gen/go/notification/service/v1"
	notificationChannelV1 "go-wind-uba/api/gen/go/notification_channel/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"

	"github.com/redis/go-redis/v9"
	authzEngine "github.com/tx7do/kratos-authz/engine"
	"github.com/tx7do/kratos-authz/engine/noop"

	"github.com/go-kratos/kratos/v2/registry"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	redisClient "github.com/tx7do/kratos-bootstrap/cache/redis"
	bRegistry "github.com/tx7do/kratos-bootstrap/registry"
	"github.com/tx7do/kratos-bootstrap/rpc"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	dictV1 "go-wind-uba/api/gen/go/dict/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"
	internalMessageV1 "go-wind-uba/api/gen/go/internal_message/service/v1"
	permissionV1 "go-wind-uba/api/gen/go/permission/service/v1"
	resourceV1 "go-wind-uba/api/gen/go/resource/service/v1"
	storageV1 "go-wind-uba/api/gen/go/storage/service/v1"
	taskV1 "go-wind-uba/api/gen/go/task/service/v1"

	"go-wind-uba/pkg/oss"
	"go-wind-uba/pkg/serviceid"
)

func NewClientType() authenticationV1.ClientType {
	return authenticationV1.ClientType_admin
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(ctx *bootstrap.Context) (*redis.Client, func(), error) {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil, func() {}, nil
	}

	l := ctx.NewLoggerHelper("redis/data/admin-service")

	cli := redisClient.NewClient(cfg.Data, l)

	return cli, func() {
		if err := cli.Close(); err != nil {
			l.Error(err)
		}
	}, nil
}

// NewDiscovery 创建服务发现客户端
func NewDiscovery(ctx *bootstrap.Context) registry.Discovery {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil
	}

	discovery, err := bRegistry.NewDiscovery(cfg.Registry)
	if err != nil {
		return nil
	}

	return discovery
}

func NewMinIoClient(ctx *bootstrap.Context) *oss.MinIOClient {
	return oss.NewMinIoClient(ctx.GetConfig(), ctx.GetLogger())
}

// NewAuthorizer 创建权鉴器
func NewAuthorizer() authzEngine.Engine {
	return noop.State{}
}

func NewAuthenticationServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.AuthenticationServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return authenticationV1.NewAuthenticationServiceClient(cli)
}

func NewUserCredentialServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.UserCredentialServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return authenticationV1.NewUserCredentialServiceClient(cli)
}

func NewLoginPolicyServiceClient(ctx *bootstrap.Context, r registry.Discovery) authenticationV1.LoginPolicyServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return authenticationV1.NewLoginPolicyServiceClient(cli)
}

func NewUserServiceClient(ctx *bootstrap.Context, r registry.Discovery) identityV1.UserServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return identityV1.NewUserServiceClient(cli)
}

func NewTenantServiceClient(ctx *bootstrap.Context, r registry.Discovery) identityV1.TenantServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return identityV1.NewTenantServiceClient(cli)
}

func NewRoleServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.RoleServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return permissionV1.NewRoleServiceClient(cli)
}

func NewOrgUnitServiceClient(ctx *bootstrap.Context, r registry.Discovery) identityV1.OrgUnitServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return identityV1.NewOrgUnitServiceClient(cli)
}

func NewPositionServiceClient(ctx *bootstrap.Context, r registry.Discovery) identityV1.PositionServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return identityV1.NewPositionServiceClient(cli)
}

func NewInternalMessageCategoryServiceClient(ctx *bootstrap.Context, r registry.Discovery) internalMessageV1.InternalMessageCategoryServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return internalMessageV1.NewInternalMessageCategoryServiceClient(cli)
}

func NewInternalMessageServiceClient(ctx *bootstrap.Context, r registry.Discovery) internalMessageV1.InternalMessageServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return internalMessageV1.NewInternalMessageServiceClient(cli)
}

func NewInternalMessageRecipientServiceClient(ctx *bootstrap.Context, r registry.Discovery) internalMessageV1.InternalMessageRecipientServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return internalMessageV1.NewInternalMessageRecipientServiceClient(cli)
}

func NewOssServiceClient(ctx *bootstrap.Context, r registry.Discovery) storageV1.OssServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return storageV1.NewOssServiceClient(cli)
}

func NewFileServiceClient(ctx *bootstrap.Context, r registry.Discovery) storageV1.FileServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return storageV1.NewFileServiceClient(cli)
}

func NewPermissionGroupServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.PermissionGroupServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return permissionV1.NewPermissionGroupServiceClient(cli)
}

func NewPermissionServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.PermissionServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return permissionV1.NewPermissionServiceClient(cli)
}

func NewApiServiceClient(ctx *bootstrap.Context, r registry.Discovery) resourceV1.ApiServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return resourceV1.NewApiServiceClient(cli)
}

func NewMenuServiceClient(ctx *bootstrap.Context, r registry.Discovery) resourceV1.MenuServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return resourceV1.NewMenuServiceClient(cli)
}

func NewPermissionAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.PermissionAuditLogServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return auditV1.NewPermissionAuditLogServiceClient(cli)
}

func NewPolicyEvaluationLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) permissionV1.PolicyEvaluationLogServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return permissionV1.NewPolicyEvaluationLogServiceClient(cli)
}

func NewApiAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.ApiAuditLogServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return auditV1.NewApiAuditLogServiceClient(cli)
}

func NewDataAccessAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.DataAccessAuditLogServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return auditV1.NewDataAccessAuditLogServiceClient(cli)
}

func NewLoginAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.LoginAuditLogServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return auditV1.NewLoginAuditLogServiceClient(cli)
}

func NewOperationAuditLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) auditV1.OperationAuditLogServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return auditV1.NewOperationAuditLogServiceClient(cli)
}

func NewDictEntryServiceClient(ctx *bootstrap.Context, r registry.Discovery) dictV1.DictEntryServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return dictV1.NewDictEntryServiceClient(cli)
}

func NewDictTypeServiceClient(ctx *bootstrap.Context, r registry.Discovery) dictV1.DictTypeServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return dictV1.NewDictTypeServiceClient(cli)
}

func NewLanguageServiceClient(ctx *bootstrap.Context, r registry.Discovery) dictV1.LanguageServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return dictV1.NewLanguageServiceClient(cli)
}

func NewTaskServiceClient(ctx *bootstrap.Context, r registry.Discovery) taskV1.TaskServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return taskV1.NewTaskServiceClient(cli)
}

func NewApplicationServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.ApplicationServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewApplicationServiceClient(cli)
}

func NewIDMappingServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.IDMappingServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewIDMappingServiceClient(cli)
}

func NewRiskRuleServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.RiskRuleServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewRiskRuleServiceClient(cli)
}

func NewTagDefinitionServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.TagDefinitionServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewTagDefinitionServiceClient(cli)
}

func NewUserTagServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.UserTagServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewUserTagServiceClient(cli)
}

func NewWebhookServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.WebhookServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewWebhookServiceClient(cli)
}

func NewAlertRuleServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.AlertRuleServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewAlertRuleServiceClient(cli)
}

func NewSegmentServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.SegmentServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewSegmentServiceClient(cli)
}

func NewSqlQueryServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.SqlQueryServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewSqlQueryServiceClient(cli)
}

func NewCampaignServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.CampaignServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewCampaignServiceClient(cli)
}

func NewImportServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.ImportServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewImportServiceClient(cli)
}

func NewReplayServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.ReplayServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewReplayServiceClient(cli)
}

func NewVisualEventServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.VisualEventServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewVisualEventServiceClient(cli)
}

func NewNotificationServiceClient(ctx *bootstrap.Context, r registry.Discovery) notificationV1.NotificationServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return notificationV1.NewNotificationServiceClient(cli)
}

func NewNotificationRuleServiceClient(ctx *bootstrap.Context, r registry.Discovery) notificationV1.NotificationRuleServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return notificationV1.NewNotificationRuleServiceClient(cli)
}

func NewNotificationTemplateServiceClient(ctx *bootstrap.Context, r registry.Discovery) notificationV1.NotificationTemplateServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return notificationV1.NewNotificationTemplateServiceClient(cli)
}

func NewNotificationPreferenceServiceClient(ctx *bootstrap.Context, r registry.Discovery) notificationV1.NotificationPreferenceServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return notificationV1.NewNotificationPreferenceServiceClient(cli)
}

func NewNotificationChannelServiceClient(ctx *bootstrap.Context, r registry.Discovery) notificationChannelV1.NotificationChannelServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return notificationChannelV1.NewNotificationChannelServiceClient(cli)
}

func NewFeatureFlagServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.FeatureFlagServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewFeatureFlagServiceClient(cli)
}

func NewReportSubscriptionServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.ReportSubscriptionServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewReportSubscriptionServiceClient(cli)
}

func NewDashboardServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.DashboardServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewDashboardServiceClient(cli)
}

func NewBehaviorEventServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.BehaviorEventServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewBehaviorEventServiceClient(cli)
}

func NewAnalyticsServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.AnalyticsServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewAnalyticsServiceClient(cli)
}

func NewEventSchemaServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.EventSchemaServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewEventSchemaServiceClient(cli)
}

func NewEventPathServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.EventPathServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewEventPathServiceClient(cli)
}

func NewObjectServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.ObjectServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewObjectServiceClient(cli)
}

func NewRiskEventServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.RiskEventServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewRiskEventServiceClient(cli)
}

func NewSessionServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.SessionServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewSessionServiceClient(cli)
}

func NewUserBehaviorProfileServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.UserBehaviorProfileServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewUserBehaviorProfileServiceClient(cli)
}

func NewAiProviderServiceClient(ctx *bootstrap.Context, r registry.Discovery) aiV1.AiProviderServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return aiV1.NewAiProviderServiceClient(cli)
}

func NewAiConversationServiceClient(ctx *bootstrap.Context, r registry.Discovery) aiV1.AiConversationServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return aiV1.NewAiConversationServiceClient(cli)
}

func NewAiMessageServiceClient(ctx *bootstrap.Context, r registry.Discovery) aiV1.AiMessageServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return aiV1.NewAiMessageServiceClient(cli)
}

func NewAiChatServiceClient(ctx *bootstrap.Context, r registry.Discovery) aiV1.AiChatServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return aiV1.NewAiChatServiceClient(cli)
}

func NewAiKnowledgeBaseServiceClient(ctx *bootstrap.Context, r registry.Discovery) aiV1.AiKnowledgeBaseServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return aiV1.NewAiKnowledgeBaseServiceClient(cli)
}

func NewAiUsageLogServiceClient(ctx *bootstrap.Context, r registry.Discovery) aiV1.AiUsageLogServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return aiV1.NewAiUsageLogServiceClient(cli)
}

func NewAiQueryServiceClient(ctx *bootstrap.Context, r registry.Discovery) aiV1.AiQueryServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return aiV1.NewAiQueryServiceClient(cli)
}
