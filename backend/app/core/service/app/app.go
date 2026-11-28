// Package app 是 core-service 的启动装配:RunApp 组装 Context 并拉起服务,
// wiring.go(initApp)是手写依赖注入点。独立成可导入包,使企业版等外部
// 入口能复用同一份装配(先注入 license,再调用 RunApp)。
package app

import (
	"context"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/tx7do/kratos-transport/transport/asynq"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	_ "github.com/tx7do/kratos-bootstrap/registry/etcd"
	_ "github.com/tx7do/kratos-bootstrap/tracer"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"

	"go-wind-uba/pkg/serviceid"
)

func newApp(
	ctx *bootstrap.Context,
	gs *grpc.Server,
	as *asynq.Server,
	backgroundServers ...transport.Server,
) *kratos.App {
	servers := []transport.Server{gs, as}
	servers = append(servers, backgroundServers...)

	return bootstrap.NewApp(
		ctx,
		servers...,
	)
}

// RunApp 以给定版本号组装并运行 core-service(原 cmd/server/main.go 的 runApp)。
func RunApp(version string) error {
	ctx := bootstrap.NewContext(
		context.Background(),
		&conf.AppInfo{
			Project: serviceid.ProjectName,
			AppId:   serviceid.CoreService,
			Version: version,
		},
	)

	ctx.RegisterCustomConfig("Authenticator", &authenticationV1.AuthenticatorOptionWrapper{})

	return bootstrap.RunApp(ctx, initApp)
}
