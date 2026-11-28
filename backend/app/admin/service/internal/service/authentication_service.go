package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"
	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"

	"go-wind-uba/pkg/license"
	"go-wind-uba/pkg/middleware/auth"
	applogging "go-wind-uba/pkg/middleware/logging"
	"go-wind-uba/pkg/sso"
)

type AuthenticationService struct {
	adminV1.AuthenticationServiceHTTPServer

	log *log.Helper

	authenticationServiceClient authenticationV1.AuthenticationServiceClient
	userServiceClient           identityV1.UserServiceClient
}

func NewAuthenticationService(
	ctx *bootstrap.Context,
	authenticationServiceClient authenticationV1.AuthenticationServiceClient,
	userServiceClient identityV1.UserServiceClient,
) *AuthenticationService {
	return &AuthenticationService{
		log:                         log.NewHelper(log.With(ctx.GetLogger(), "module", "user/service/admin-service")),
		authenticationServiceClient: authenticationServiceClient,
		userServiceClient:           userServiceClient,
	}
}

// Login 登录
func (s *AuthenticationService) Login(ctx context.Context, req *authenticationV1.LoginRequest) (*authenticationV1.LoginResponse, error) {
	if req == nil {
		return nil, authenticationV1.ErrorBadRequest("invalid request")
	}

	req.ClientType = trans.Ptr(authenticationV1.ClientType_admin)

	if req.GetGrantType() == authenticationV1.GrantType_refresh_token {
		operator, err := auth.FromContext(ctx)
		if err != nil {
			return nil, err
		}

		req.Jti = operator.Jti
		req.UserId = trans.Ptr(operator.GetUserId())
	}

	resp, err := s.authenticationServiceClient.Login(ctx, req)
	if err != nil {
		return nil, err
	}

	// 登录成功记录最后登录时间与 IP（用户列表/个人中心展示用）；失败不阻断登录。
	// 网关层才有原始 HTTP 请求可取真实 IP，故在此记录而非 core 服务。
	if req.GetGrantType() == authenticationV1.GrantType_password {
		s.recordUserLastLogin(ctx, req.GetUserId(), resp)
	}

	return resp, nil
}

// recordUserLastLogin 记录用户最后登录时间与 IP。
func (s *AuthenticationService) recordUserLastLogin(ctx context.Context, userID int64, resp *authenticationV1.LoginResponse) {
	if userID == 0 || s.userServiceClient == nil {
		return
	}

	var clientIP string
	if htr, ok := kratosHttp.RequestFromServerContext(ctx); ok && htr != nil {
		clientIP = applogging.ClientRealIP(htr)
	}

	mask, err := fieldmaskpb.New(&identityV1.User{}, "last_login_at", "last_login_ip")
	if err != nil {
		s.log.Errorf("build last-login fieldmask failed: %s", err.Error())
		return
	}
	if _, uerr := s.userServiceClient.Update(ctx, &identityV1.UpdateUserRequest{
		Id: userID,
		Data: &identityV1.User{
			LastLoginAt: timeutil.TimeToTimestamppb(trans.Ptr(time.Now())),
			LastLoginIp: trans.Ptr(clientIP),
		},
		UpdateMask: mask,
	}); uerr != nil {
		s.log.Errorf("record last login for user [%d] failed: %s", userID, uerr.Error())
	}
}

// Capabilities 查询平台已授权的企业能力(公开接口;开源构建恒为空)。
func (s *AuthenticationService) Capabilities(ctx context.Context, _ *emptypb.Empty) (*adminV1.CapabilitiesResponse, error) {
	return &adminV1.CapabilitiesResponse{
		Capabilities: license.Capabilities(),
	}, nil
}

// ssoUnavailable 单点登录不可用的统一报错:
// 开源构建未配置校验器,或企业构建未启用 sso 能力(校验器仅在能力开启时装配)。
func ssoUnavailable() error {
	return authenticationV1.ErrorBadRequest("single sign-on is not available on this deployment")
}

// ssoReady 单点登录入口的可用性判定(开源侧协议通用,不感知具体能力名)。
func ssoReady() bool {
	return license.IsConfigured() && sso.Available()
}

// SsoAuthorize 生成 IdP 授权地址与防 CSRF 的 state(公开接口)。
func (s *AuthenticationService) SsoAuthorize(ctx context.Context, _ *emptypb.Empty) (*adminV1.SsoAuthorizeResponse, error) {
	if !ssoReady() {
		return nil, ssoUnavailable()
	}

	authorizeURL, state, err := sso.Current().AuthorizeURL(ctx)
	if err != nil {
		return nil, err
	}
	return &adminV1.SsoAuthorizeResponse{
		AuthorizeUrl: authorizeURL,
		State:        state,
	}, nil
}

// SsoCallback 授权码换平台令牌(公开接口):校验 state 与 IdP 令牌后,
// 以已验证身份经 core 换取与密码登录等价的平台令牌。
func (s *AuthenticationService) SsoCallback(ctx context.Context, req *adminV1.SsoCallbackRequest) (*authenticationV1.LoginResponse, error) {
	if !ssoReady() {
		return nil, ssoUnavailable()
	}
	if req == nil || req.GetCode() == "" || req.GetState() == "" {
		return nil, authenticationV1.ErrorBadRequest("invalid request")
	}

	provider, identityType, identifier, err := sso.Current().Verify(ctx, req.GetCode(), req.GetState())
	if err != nil {
		return nil, err
	}

	resp, err := s.authenticationServiceClient.SsoLogin(ctx, &authenticationV1.SsoLoginRequest{
		Provider:     provider,
		IdentityType: identityType,
		Identifier:   identifier,
		ClientType:   authenticationV1.ClientType_admin,
	})
	if err != nil {
		return nil, err
	}

	// 记录最后登录(失败不阻断登录);用户可能按邮箱映射,先取回用户 ID。
	if user, gerr := s.userServiceClient.Get(ctx, &identityV1.GetUserRequest{
		QueryBy: &identityV1.GetUserRequest_Email{Email: identifier},
	}); gerr == nil {
		s.recordUserLastLogin(ctx, user.GetId(), resp)
	}

	return resp, nil
}

// Logout 登出
func (s *AuthenticationService) Logout(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	return s.authenticationServiceClient.Logout(ctx, &authenticationV1.LogoutRequest{
		ClientType: authenticationV1.ClientType_admin,
		UserId:     operator.GetUserId(),
	})
}

// RefreshToken 刷新令牌
func (s *AuthenticationService) RefreshToken(ctx context.Context, req *authenticationV1.LoginRequest) (*authenticationV1.LoginResponse, error) {
	if req == nil {
		return nil, authenticationV1.ErrorBadRequest("invalid request")
	}

	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	req.ClientType = trans.Ptr(authenticationV1.ClientType_admin)
	req.UserId = trans.Ptr(operator.GetUserId())
	req.Jti = operator.Jti

	return s.authenticationServiceClient.RefreshToken(ctx, req)
}

func (s *AuthenticationService) WhoAmI(ctx context.Context, _ *emptypb.Empty) (*authenticationV1.WhoAmIResponse, error) {
	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	return &authenticationV1.WhoAmIResponse{
		UserId:   operator.GetUserId(),
		Username: operator.GetUsername(),
	}, nil
}
