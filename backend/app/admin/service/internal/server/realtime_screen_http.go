package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-uba/app/admin/service/internal/service"
)

// 实时大屏 SSE 的订阅控制面端点（数据面走 SSE 服务器,见 service/realtime_screen_service.go）。
// 与 :export 锚点同一手动注册先例：控制面为 SSE 功能的私有配套端点，契约以
// realtime-screen 页面与 service 层注释为准。

const (
	OperationRealtimeScreenSubscribe   = "admin.service.v1.RealtimeScreenService/Subscribe"
	OperationRealtimeScreenUnsubscribe = "admin.service.v1.RealtimeScreenService/Unsubscribe"
)

func registerRealtimeScreenRoutes(srv *http.Server, svc *service.RealtimeScreenService) {
	r := srv.Route("/")

	r.POST("admin/v1/realtime-screen/subscribe", func(ctx http.Context) error {
		http.SetOperation(ctx, OperationRealtimeScreenSubscribe)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return svc.Subscribe(ctx)
		})
		out, err := h(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}
		return ctx.Result(200, out)
	})

	r.POST("admin/v1/realtime-screen/unsubscribe", func(ctx http.Context) error {
		http.SetOperation(ctx, OperationRealtimeScreenUnsubscribe)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return svc.Unsubscribe(ctx)
		})
		out, err := h(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}
		return ctx.Result(200, out)
	})
}
