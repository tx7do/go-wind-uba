package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	authV1 "go-wind-uba/api/gen/go/authentication/service/v1"

	"go-wind-uba/pkg/constants"

	"go-wind-uba/pkg/middleware/auth"
)

// requirePlatformAdmin 把通知域的平台级配置与台账挡在平台管理员之内。
//
// 渠道/规则/模板/台账四张表无租户列（平台全局资源），租户侧可读即全平台泄漏。
// 鉴权发生在 admin BFF 边缘（JWT 中间件已将载荷写入 context），
// core 侧 gRPC 是内网信任模型、不再重复校验——与 uba 其余管理域一致。
//
// 移植自上游 go-wind-admin da2b3a5f：上游以 token 的 IsPlatformAdmin 旗标判定
// （登录链路 fillAdminFlags 按角色码填充）。本仓登录链路尚未填充该旗标，
// 故在旗标之外按角色码（platform:admin，经 JWT "roc" claim 往返）回退判定，
// 语义与上游同源；旗标填充补齐后此判定自动收敛为上游形态。
func requirePlatformAdmin(ctx context.Context, log *log.Helper, resource string) error {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return authV1.ErrorUnauthorized("authentication required: %v", err)
	}

	if operator.GetIsPlatformAdmin() {
		return nil
	}

	// 平台管理员判定回退走角色码（platform:admin）
	for _, role := range operator.GetRoles() {
		if role == constants.PlatformAdminRoleCode {
			return nil
		}
	}

	log.Warnf("operator [%d] (tenant %d) is not a platform administrator, %s access denied",
		operator.GetUserId(), operator.GetTenantId(), resource)

	return authV1.ErrorForbidden("%s is platform-level configuration, only platform administrators may access it", resource)
}
