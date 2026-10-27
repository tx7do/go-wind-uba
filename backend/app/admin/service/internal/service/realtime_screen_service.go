package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/tx7do/go-utils/id"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	sse "github.com/tx7do/kratos-transport/transport/sse"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"
	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	pkgmetadata "go-wind-uba/pkg/metadata"
)

// 实时大屏 SSE 推送：替代前端 5 秒 REST 轮询（realtime-screen 页 TODO 的落地）。
//
// 架构：复用既有 SSE 服务器（streamID=访问令牌，auto_stream，见 server/sse_server.go），
// 本服务做"谁订阅给谁推"的服务端轮询搬运：
//   - 订阅时捕获操作者上下文（pkg/metadata），后台循环用它调用 core 的事件/风险列表——
//     每个订阅者永远只看到自己权限内的数据，与原 REST 轮询完全同源，无跨租户扩散面；
//   - 推送为全量快照（两次 List 的原载荷），与页面现轮询的请求一致，前端派生 KPI 不变；
//     快照语义对断线/丢包自愈，不需要游标管理；
//   - 订阅由前端心跳续期，TTL 兜底回收，防止页面非正常退出后循环泄漏。
//
// 后续若引入 Go 侧 Kafka tail 消费（当前架构刻意不为入库写消费者），只需把
// pushOnce 的两次 List 换成消费订阅，前端协议不变。

const (
	realtimeScreenEventName     = "realtime_lists"
	realtimeScreenPollInterval  = 5 * time.Second  // 与原前端轮询节奏一致
	realtimeScreenPollTimeout   = 4 * time.Second  // 单轮两次 List 的总预算（gRPC 客户端默认超时 5s）
	realtimeScreenIdleTTL       = 10 * time.Minute // 心跳间隔 4 分钟，10 分钟无续期即回收
	realtimeScreenMaxFailStreak = 5                // 连续失败 N 轮后放弃订阅（心跳可重建）
)

// RealtimeScreenService 大屏订阅注册表 + 每订阅一个推送循环。
type RealtimeScreenService struct {
	log *log.Helper

	behaviorEventService *BehaviorEventService
	riskEventService     *RiskEventService

	sseServer *sse.Server

	mu   sync.Mutex
	subs map[string]*realtimeScreenSubscription
}

// realtimeScreenSubscription 一个大屏订阅：streamID=访问令牌。
type realtimeScreenSubscription struct {
	cancel    context.CancelFunc
	lastRenew atomic.Int64 // 最近一次 Subscribe 续期的 unix 秒
}

func NewRealtimeScreenService(
	ctx *bootstrap.Context,
	behaviorEventService *BehaviorEventService,
	riskEventService *RiskEventService,
	sseServer *sse.Server,
) *RealtimeScreenService {
	s := &RealtimeScreenService{
		log:                  ctx.NewLoggerHelper("realtime-screen/service/admin-service"),
		behaviorEventService: behaviorEventService,
		riskEventService:     riskEventService,
		sseServer:            sseServer,
		subs:                 make(map[string]*realtimeScreenSubscription),
	}
	go s.reapLoop()
	return s
}

// Subscribe 注册/续期大屏订阅（幂等：已存在只续期，不重复起循环）。
func (s *RealtimeScreenService) Subscribe(ctx context.Context) (*emptypb.Empty, error) {
	token := realtimeStreamToken(ctx)
	if token == "" {
		return nil, adminV1.ErrorUnauthorized("missing access token")
	}
	operator, err := pkgmetadata.FromServerContext(ctx)
	if err != nil || operator == nil {
		return nil, adminV1.ErrorUnauthorized("operator context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.subs[token]; ok {
		sub.lastRenew.Store(time.Now().Unix())
		return &emptypb.Empty{}, nil
	}
	runCtx, cancel := context.WithCancel(context.Background())
	sub := &realtimeScreenSubscription{cancel: cancel}
	sub.lastRenew.Store(time.Now().Unix())
	s.subs[token] = sub

	go s.pollLoop(runCtx, token, operator)
	return &emptypb.Empty{}, nil
}

// Unsubscribe 取消大屏订阅（页面卸载时调用；TTL 兜底覆盖未调用的场景）。
func (s *RealtimeScreenService) Unsubscribe(ctx context.Context) (*emptypb.Empty, error) {
	token := realtimeStreamToken(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.subs[token]; ok {
		sub.cancel()
		delete(s.subs, token)
	}
	return &emptypb.Empty{}, nil
}

func (s *RealtimeScreenService) remove(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.subs[token]; ok {
		sub.cancel()
		delete(s.subs, token)
	}
}

func (s *RealtimeScreenService) pollLoop(ctx context.Context, token string, operator *authenticationV1.OperatorMetadata) {
	ticker := time.NewTicker(realtimeScreenPollInterval)
	defer ticker.Stop()

	failStreak := 0
	if s.pushOnce(ctx, token, operator) {
		failStreak = 0
	} else {
		failStreak++
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if s.pushOnce(ctx, token, operator) {
			failStreak = 0
		} else {
			failStreak++
		}
		if failStreak >= realtimeScreenMaxFailStreak {
			s.log.Errorf("realtime screen subscription [%s] failed %d rounds in a row, dropping it",
				realtimeScreenTokenBrief(token), failStreak)
			s.remove(token)
			return
		}
	}
}

// pushOnce 拉一轮两个列表并整体推给订阅者。返回 false 表示该轮失败。
func (s *RealtimeScreenService) pushOnce(ctx context.Context, token string, operator *authenticationV1.OperatorMetadata) bool {
	callCtx, err := pkgmetadata.NewContext(ctx, operator)
	if err != nil {
		s.log.Errorf("realtime screen: build operator context failed: %s", err)
		return false
	}
	callCtx, cancel := context.WithTimeout(callCtx, realtimeScreenPollTimeout)
	defer cancel()

	events, evErr := s.behaviorEventService.List(callCtx, &paginationV1.PagingRequest{
		Page:     trans.Ptr(uint32(1)),
		PageSize: trans.Ptr(uint32(50)),
		OrderBy:  trans.Ptr("-event_ts"),
	})
	risks, riskErr := s.riskEventService.List(callCtx, &paginationV1.PagingRequest{
		Page:     trans.Ptr(uint32(1)),
		PageSize: trans.Ptr(uint32(50)),
		OrderBy:  trans.Ptr("-occur_time"),
	})
	if evErr != nil || riskErr != nil {
		s.log.Errorf("realtime screen: poll failed (events=%v risks=%v)", evErr, riskErr)
		return false
	}

	evJson, err := protojson.Marshal(events)
	if err != nil {
		s.log.Errorf("realtime screen: marshal events failed: %s", err)
		return false
	}
	riskJson, err := protojson.Marshal(risks)
	if err != nil {
		s.log.Errorf("realtime screen: marshal risks failed: %s", err)
		return false
	}
	payload, err := json.Marshal(map[string]json.RawMessage{
		"events": json.RawMessage(evJson),
		"risks":  json.RawMessage(riskJson),
	})
	if err != nil {
		s.log.Errorf("realtime screen: marshal snapshot failed: %s", err)
		return false
	}

	// 流未连接（无浏览器挂着）时 Publish 是 no-op，循环照常跑——订阅由 TTL/心跳管理，
	// 不依赖流的存在。
	s.sseServer.Publish(ctx, sse.StreamID(token), &sse.Event{
		ID:    []byte(id.NewGUIDv7(false)),
		Data:  payload,
		Event: []byte(realtimeScreenEventName),
	})
	return true
}

// reapLoop TTL 兜底：页面非正常退出（直接关屏/断网）不会调 Unsubscribe，
// 靠前端心跳（4 分钟）续期，超期回收推送循环。
func (s *RealtimeScreenService) reapLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		s.mu.Lock()
		for token, sub := range s.subs {
			if now.Unix()-sub.lastRenew.Load() > int64(realtimeScreenIdleTTL/time.Second) {
				sub.cancel()
				delete(s.subs, token)
				s.log.Infof("realtime screen subscription [%s] idle timeout, dropped",
					realtimeScreenTokenBrief(token))
			}
		}
		s.mu.Unlock()
	}
}

// realtimeStreamToken 取订阅者的访问令牌（即前端 ?stream= 用的同一个值，就是 StreamID）。
func realtimeStreamToken(ctx context.Context) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	return strings.TrimPrefix(tr.RequestHeader().Get("Authorization"), "Bearer ")
}

// realtimeScreenTokenBrief 日志里只带令牌首尾，不打全文。
func realtimeScreenTokenBrief(token string) string {
	if len(token) <= 12 {
		return "short-token"
	}
	return token[:6] + "..." + token[len(token)-6:]
}
