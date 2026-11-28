package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tx7do/go-utils/trans"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"

	"go-wind-uba/pkg/topic"
)

// 本文件实现买量归因的两个服务端对接入口:
//
//	PostAttribution  —— MMP(AppsFlyer / Adjust 等)回传的渠道归因记录
//	PostChannelCost  —— MMP / 广告平台报表的渠道日花费
//
// 数据面与 SDK 上报同构:app 鉴权 → 逐条校验 → Kafka(uba_attribution /
// uba_channel_cost)→ OLAP 引擎自拉入库(attribution_records / channel_cost_daily,
// 见 sql/{doris,clickhouse}/07_attribution.sql)。渠道 ROI 由分析模型消费这两张表。

// PostAttribution 买量归因回传。
func (s *ReportService) PostAttribution(ctx context.Context, req *ubaV1.PostAttributionRequest) (*ubaV1.PostAttributionResponse, error) {
	if req == nil || len(req.GetRecords()) == 0 {
		return nil, ubaV1.ErrorBadRequest("request data is required")
	}

	app, err := s.appAuth.Authenticate(ctx, req.GetAppId(), req.GetAppSecret())
	if err != nil {
		return nil, err
	}

	var accepted, rejected int32
	reasons := make([]string, 0, 4)
	for _, r := range req.GetRecords() {
		if err := validateAttribution(r); err != nil {
			rejected++
			reasons = appendLimited(reasons, fmt.Sprintf("%s: %v", r.GetAttributionId(), err))
			continue
		}

		// 租户以 app 鉴权结果为准,杜绝客户端伪造跨租户归因
		r.TenantId = trans.Ptr(app.TenantID)
		// 类型缺省视为自然量安装(MMP 回传遗漏时不至于丢数据)
		if r.GetType() == ubaV1.AttributionType_ATTRIBUTION_TYPE_UNSPECIFIED {
			r.Type = ubaV1.AttributionType_INSTALL
		}
		if err := s.publish(ctx, topic.UbaAttribution, r); err != nil {
			rejected++
			reasons = appendLimited(reasons, fmt.Sprintf("%s: %v", r.GetAttributionId(), err))
			continue
		}
		accepted++
	}

	return &ubaV1.PostAttributionResponse{
		Accepted: accepted,
		Rejected: rejected,
		Message:  strings.Join(reasons, "; "),
	}, nil
}

// validateAttribution 归因记录的必填项校验。
func validateAttribution(r *ubaV1.AttributionRecord) error {
	switch {
	case r.GetAttributionId() == "":
		return fmt.Errorf("attribution_id is required")
	case r.GetDeviceId() == "" && r.GetUserId() == 0:
		return fmt.Errorf("device_id or user_id is required")
	case r.GetMediaSource() == "":
		return fmt.Errorf("media_source is required")
	case r.GetAttributionTime() == nil || r.GetAttributionTime().AsTime().IsZero():
		return fmt.Errorf("attribution_time is required")
	}
	return nil
}

// PostChannelCost 渠道日花费回传。
func (s *ReportService) PostChannelCost(ctx context.Context, req *ubaV1.PostChannelCostRequest) (*ubaV1.PostChannelCostResponse, error) {
	if req == nil || len(req.GetRecords()) == 0 {
		return nil, ubaV1.ErrorBadRequest("request data is required")
	}

	app, err := s.appAuth.Authenticate(ctx, req.GetAppId(), req.GetAppSecret())
	if err != nil {
		return nil, err
	}

	var accepted, rejected int32
	reasons := make([]string, 0, 4)
	for _, r := range req.GetRecords() {
		if err := validateChannelCost(r); err != nil {
			rejected++
			reasons = appendLimited(reasons, fmt.Sprintf("%s/%s: %v", r.GetDate(), r.GetMediaSource(), err))
			continue
		}

		r.TenantId = trans.Ptr(app.TenantID)
		if err := s.publish(ctx, topic.UbaChannelCost, r); err != nil {
			rejected++
			reasons = appendLimited(reasons, fmt.Sprintf("%s/%s: %v", r.GetDate(), r.GetMediaSource(), err))
			continue
		}
		accepted++
	}

	return &ubaV1.PostChannelCostResponse{
		Accepted: accepted,
		Rejected: rejected,
		Message:  strings.Join(reasons, "; "),
	}, nil
}

// validateChannelCost 渠道花费的必填项与取值校验。
func validateChannelCost(r *ubaV1.ChannelCostRecord) error {
	switch {
	case len(r.GetDate()) != 10:
		return fmt.Errorf("date must be YYYY-MM-DD")
	default:
		if _, err := time.Parse("2006-01-02", r.GetDate()); err != nil {
			return fmt.Errorf("date must be YYYY-MM-DD")
		}
	}
	switch {
	case r.GetMediaSource() == "":
		return fmt.Errorf("media_source is required")
	case r.GetCurrency() == "":
		return fmt.Errorf("currency is required")
	case r.GetCost() < 0:
		return fmt.Errorf("cost must be >= 0")
	}
	return nil
}

// appendLimited 追加原因,最多保留 max 条(响应体积不随批量线性膨胀)。
func appendLimited(reasons []string, reason string) []string {
	const max = 3
	if len(reasons) < max {
		return append(reasons, reason)
	}
	if len(reasons) == max {
		return append(reasons, "...")
	}
	return reasons
}
