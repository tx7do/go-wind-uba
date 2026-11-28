// Package app 是 admin-service 的启动装配:RunApp 组装 Context 并拉起服务,
// wiring.go(initApp)是手写依赖注入点。独立成可导入包,使企业版等外部
// 入口能复用同一份装配(先注入 license,再调用 RunApp)。
package app

import (
	"context"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport/http"

	"github.com/tx7do/kratos-transport/transport/sse"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	_ "github.com/tx7do/kratos-bootstrap/registry/etcd"
	_ "github.com/tx7do/kratos-bootstrap/tracer"

	"go-wind-uba/pkg/serviceid"
)

func newApp(
	ctx *bootstrap.Context,
	hs *http.Server,
	ss *sse.Server,
) *kratos.App {
	return bootstrap.NewApp(ctx,
		hs,
		ss,
	)
}

// RunApp 以给定版本号组装并运行 admin-service(原 cmd/server/main.go 的 runApp)。
func RunApp(version string) error {
	ctx := bootstrap.NewContext(
		context.Background(),
		&conf.AppInfo{
			Project: serviceid.ProjectName,
			AppId:   serviceid.AdminService,
			Version: version,
		},
	)
	return bootstrap.RunApp(ctx, initApp)
}
