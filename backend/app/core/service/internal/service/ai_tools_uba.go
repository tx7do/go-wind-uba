package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
	"go-wind-uba/app/core/service/internal/data"
)

// UBA 分析工具（Function Call）：把行为分析的两个高频问题（活跃度趋势、
// 热门事件）暴露给模型，让对话免写 SQL 直接回答。
//
// 语义与自助分析 UI 同水位：appId 留空 = 全部应用（前端看板同款约定），
// 时间窗最近 N 天（上限 90），上限条数与 aiQueryRowLimit 同量级。
// 实现直接复用 AnalyticsRepo 的 Doris/ClickHouse 双实现，路由与
// AnalyticsService.repo() 同一编译期开关。

const (
	// ubaToolMaxDays 工具查询的时间窗上限（天）。
	ubaToolMaxDays = 90

	// ubaToolMaxTopN 热门事件返回条数上限。
	ubaToolMaxTopN = 20
)

// ubaToolDefs UBA 分析工具清单。
func ubaToolDefs() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "query_active_users",
				Description: "查询活跃用户趋势（最近 N 天的每日 DAU/WAU/MAU）。回答\"最近活跃用户多少\"\"用户量走势\"类问题时调用。",
				Parameters: json.RawMessage(`{
  "type": "object",
  "properties": {
    "days": {"type": "integer", "description": "查询最近多少天，默认 7，最大 90"},
    "app_id": {"type": "integer", "description": "应用 ID，0 或缺省表示全部应用"}
  },
  "required": []
}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "query_top_events",
				Description: "查询最近 N 天的热门事件排行（按事件名分组）。回答\"哪些事件最多\"\"用户最常做什么\"类问题时调用。",
				Parameters: json.RawMessage(`{
  "type": "object",
  "properties": {
    "days": {"type": "integer", "description": "查询最近多少天，默认 7，最大 90"},
    "limit": {"type": "integer", "description": "返回条数，默认 10，最大 20"},
    "metric": {"type": "string", "enum": ["COUNT", "UNIQUE_USER"], "description": "指标：COUNT=触发次数（默认），UNIQUE_USER=触达人数"},
    "app_id": {"type": "integer", "description": "应用 ID，0 或缺省表示全部应用"}
  },
  "required": []
}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "get_event_list",
				Description: "获取平台已登记的事件定义清单（事件名/显示名/分类/描述）。回答\"平台有哪些事件\"\"xxx 事件是什么含义\"类问题前先调用本工具核对事件名。",
				Parameters: json.RawMessage(`{
  "type": "object",
  "properties": {
    "limit": {"type": "integer", "description": "返回条数上限，默认 50，最大 100"}
  },
  "required": []
}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "query_event_trend",
				Description: "查询单个事件最近 N 天的每日触发趋势。回答\"xxx 事件上周趋势如何\"\"某事件量有没有涨\"类问题时调用。event_name 应先用 get_event_list 核对。",
				Parameters: json.RawMessage(`{
  "type": "object",
  "properties": {
    "event_name": {"type": "string", "description": "事件名（平台登记的 event_name 原值）"},
    "days": {"type": "integer", "description": "查询最近多少天，默认 7，最大 90"},
    "app_id": {"type": "integer", "description": "应用 ID，0 或缺省表示全部应用"}
  },
  "required": ["event_name"]
}`),
			},
		},
	}
}

// ubaAnalytics 活跃度与分组聚合所需的分析能力（Doris/ClickHouse 双实现共用签名）。
type ubaAnalytics interface {
	ActiveUsers(ctx context.Context, req *ubaV1.ActiveUsersRequest) (*ubaV1.ActiveUsersResponse, error)
	GroupBy(ctx context.Context, req *ubaV1.GroupByRequest) (*ubaV1.GroupByResponse, error)
	EventTrend(ctx context.Context, req *ubaV1.EventTrendRequest) (*ubaV1.EventTrendResponse, error)
}

// analyticsImpl 按编译期引擎开关路由（与 AnalyticsService.repo() 同一约定）。
func (s *AiChatService) analyticsImpl() ubaAnalytics {
	if data.UseClickHouse && s.ckAnalyticsRepo != nil {
		return s.ckAnalyticsRepo
	}
	return s.dorisAnalyticsRepo
}

// execUBATool 执行一个 UBA 分析工具调用；handled=false 表示不是本组工具，
// 交回调用方走内置工具分支。
func (s *AiChatService) execUBATool(ctx context.Context, name, args string) (string, bool, error) {
	switch name {
	case "query_active_users":
		out, err := s.toolActiveUsers(ctx, args)
		return out, true, err
	case "query_top_events":
		out, err := s.toolTopEvents(ctx, args)
		return out, true, err
	case "get_event_list":
		out, err := s.toolEventList(ctx, args)
		return out, true, err
	case "query_event_trend":
		out, err := s.toolEventTrend(ctx, args)
		return out, true, err
	default:
		return "", false, nil
	}
}

// clampToolDays 钳制时间窗天数（非法/缺省回落 7）。
func clampToolDays(days int32) int32 {
	if days <= 0 {
		return 7
	}
	if days > ubaToolMaxDays {
		return ubaToolMaxDays
	}
	return days
}

func (s *AiChatService) toolActiveUsers(ctx context.Context, argsJson string) (string, error) {
	args := struct {
		Days  int32 `json:"days"`
		AppID int64 `json:"app_id"`
	}{Days: 7}
	if argsJson != "" {
		if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
	}
	days := clampToolDays(args.Days)

	now := time.Now()
	start := startOfDaysWindow(now, int(days))

	resp, err := s.analyticsImpl().ActiveUsers(ctx, &ubaV1.ActiveUsersRequest{
		TimeRange:   &ubaV1.TimeRange{StartMs: start.UnixMilli(), EndMs: now.UnixMilli()},
		Granularity: ubaV1.AnalyticsGranularity_DAY,
		AppId:       toolAppIdPtr(args.AppID),
	})
	if err != nil {
		return "", err
	}

	var out []byte
	out = fmt.Appendf(out, "最近 %d 天活跃用户趋势（appId=%d）：\n", days, args.AppID)
	out = fmt.Appendf(out, "日期,DAU,WAU,MAU\n")
	for _, p := range resp.GetPoints() {
		out = fmt.Appendf(out, "%s,%d,%d,%d\n",
			time.UnixMilli(p.GetTimestamp()).Format("2006-01-02"),
			p.GetDau(), p.GetWau(), p.GetMau())
	}
	out = fmt.Appendf(out, "最新一天 DAU：%d\n", resp.GetLatestDau())
	return string(out), nil
}

func (s *AiChatService) toolTopEvents(ctx context.Context, argsJson string) (string, error) {
	args := struct {
		Days   int32  `json:"days"`
		Limit  int32  `json:"limit"`
		Metric string `json:"metric"`
		AppID  int64  `json:"app_id"`
	}{Days: 7, Limit: 10}
	if argsJson != "" {
		if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
	}
	days := clampToolDays(args.Days)
	limit := int(args.Limit)
	if limit <= 0 {
		limit = 10
	}
	if limit > ubaToolMaxTopN {
		limit = ubaToolMaxTopN
	}
	metric := args.Metric
	if metric != "UNIQUE_USER" {
		metric = "COUNT"
	}

	now := time.Now()
	start := startOfDaysWindow(now, int(days))

	resp, err := s.analyticsImpl().GroupBy(ctx, &ubaV1.GroupByRequest{
		TimeRange: &ubaV1.TimeRange{StartMs: start.UnixMilli(), EndMs: now.UnixMilli()},
		Dimension: "event_name",
		Metric:    &metric,
		TopN:      uint32Ptr(uint32(limit)),
		AppId:     toolAppIdPtr(args.AppID),
	})
	if err != nil {
		return "", err
	}

	metricName := "触发次数"
	if metric == "UNIQUE_USER" {
		metricName = "触达人数"
	}
	var out []byte
	out = fmt.Appendf(out, "最近 %d 天热门事件（%s，appId=%d）：\n", days, metricName, args.AppID)
	out = fmt.Appendf(out, "事件名,%s,占比\n", metricName)
	for _, b := range resp.GetBuckets() {
		out = fmt.Appendf(out, "%s,%.0f,%.1f%%\n", b.GetLabel(), b.GetValue(), b.GetPercentage()*100)
	}
	return string(out), nil
}

// startOfDaysWindow 最近 N 天窗口的起点（今天 0 点往前推 N-1 天）。
func startOfDaysWindow(now time.Time, days int) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(days - 1))
}

// aiToolMaxEventList 事件清单工具的返回上限。
const aiToolMaxEventList = 100

// toolEventList 平台已登记的事件定义清单（sys_event_schemas 现取）。
// 与问数提示词的事件字典同源；取失败回传错误文本让模型自行调整话术。
func (s *AiChatService) toolEventList(ctx context.Context, argsJson string) (string, error) {
	args := struct {
		Limit int32 `json:"limit"`
	}{Limit: 50}
	if argsJson != "" {
		if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
	}
	limit := int(args.Limit)
	if limit <= 0 {
		limit = 50
	}
	if limit > aiToolMaxEventList {
		limit = aiToolMaxEventList
	}

	if s.eventSchemaRepo == nil {
		return "平台尚未接入事件定义查询能力", nil
	}
	resp, err := s.eventSchemaRepo.List(ctx, noPagingRequest())
	if err != nil {
		return "", err
	}
	items := resp.GetItems()
	if len(items) == 0 {
		return "平台尚未登记任何事件定义；接入 SDK 上报后请在「数据配置 → 事件接入」登记，再在知识库页同步事件字典。", nil
	}

	var out []byte
	out = fmt.Appendf(out, "平台已登记 %d 个事件：\n", len(items))
	shown := 0
	for _, item := range items {
		if shown >= limit {
			out = fmt.Appendf(out, "（其余 %d 个略，可让用户缩小范围）\n", len(items)-limit)
			break
		}
		line := "- " + item.GetEventName()
		if dn := item.GetDisplayName(); dn != "" {
			line += "（" + dn + "）"
		}
		if cat := item.GetCategory(); cat != "" {
			line += " [" + cat + "]"
		}
		if desc := item.GetDescription(); desc != "" {
			line += "：" + desc
		}
		out = fmt.Appendf(out, "%s\n", line)
		shown++
	}
	return string(out), nil
}

// toolEventTrend 单事件每日触发趋势（AnalyticsRepo.EventTrend，COUNT 口径）。
func (s *AiChatService) toolEventTrend(ctx context.Context, argsJson string) (string, error) {
	args := struct {
		EventName string `json:"event_name"`
		Days      int32  `json:"days"`
		AppID     int64  `json:"app_id"`
	}{Days: 7}
	if argsJson != "" {
		if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
	}
	if strings.TrimSpace(args.EventName) == "" {
		return "", fmt.Errorf("event_name is required")
	}
	days := clampToolDays(args.Days)

	now := time.Now()
	start := startOfDaysWindow(now, int(days))

	resp, err := s.analyticsImpl().EventTrend(ctx, &ubaV1.EventTrendRequest{
		TimeRange:   &ubaV1.TimeRange{StartMs: start.UnixMilli(), EndMs: now.UnixMilli()},
		Granularity: ubaV1.AnalyticsGranularity_DAY,
		EventName:   &args.EventName,
		AppId:       toolAppIdPtr(args.AppID),
	})
	if err != nil {
		return "", err
	}

	var out []byte
	out = fmt.Appendf(out, "事件 %s 最近 %d 天每日触发趋势（appId=%d，区间总量 %d）：\n",
		args.EventName, days, args.AppID, resp.GetTotal())
	out = fmt.Appendf(out, "日期,触发次数\n")
	for _, p := range resp.GetPoints() {
		out = fmt.Appendf(out, "%s,%.0f\n",
			time.UnixMilli(p.GetTimestamp()).Format("2006-01-02"), p.GetValue())
	}
	return string(out), nil
}

// toolAppIdPtr appId=0 视为未指定（全部应用），转 nil。
func toolAppIdPtr(appId int64) *int64 {
	if appId == 0 {
		return nil
	}
	return &appId
}

func uint32Ptr(v uint32) *uint32 { return &v }
