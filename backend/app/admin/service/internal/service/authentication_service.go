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

	"go-wind-uba/pkg/middleware/auth"
	applogging "go-wind-uba/pkg/middleware/logging"
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
