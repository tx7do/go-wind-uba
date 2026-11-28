package service

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
	"go-wind-uba/pkg/topic"
)

// 复用 report_service_test 的脚手架:fakeBroker / testEnabledApp(tenant=42)。
const (
	testAppID     = "app-001"
	testAppSecret = "secret-abc"
)

func TestPostAttributionValidation(t *testing.T) {
	svc := newTestReportService(t, &fakeBroker{}, testEnabledApp())
	ctx := context.Background()

	// 空请求
	if _, err := svc.PostAttribution(ctx, nil); err == nil {
		t.Fatal("PostAttribution(nil) = nil, want error")
	}

	// 鉴权失败
	if _, err := svc.PostAttribution(ctx, &ubaV1.PostAttributionRequest{
		AppId: "ghost", AppSecret: "x",
		Records: []*ubaV1.AttributionRecord{{AttributionId: "a1"}},
	}); err == nil {
		t.Fatal("PostAttribution(bad auth) = nil, want error")
	}

	// 逐条校验:缺 media_source 的记录被拒,合法记录被受理
	resp, err := svc.PostAttribution(ctx, &ubaV1.PostAttributionRequest{
		AppId: testAppID, AppSecret: testAppSecret,
		Records: []*ubaV1.AttributionRecord{
			{AttributionId: "ok-1", DeviceId: "dev-1", MediaSource: "tiktok",
				AttributionTime: timestamppb.New(testFixedTime())},
			{AttributionId: "bad-1", DeviceId: "dev-2",
				AttributionTime: timestamppb.New(testFixedTime())}, // 缺 media_source
		},
	})
	if err != nil {
		t.Fatalf("PostAttribution() = %v", err)
	}
	if resp.GetAccepted() != 1 || resp.GetRejected() != 1 {
		t.Fatalf("accepted/rejected = %d/%d, want 1/1", resp.GetAccepted(), resp.GetRejected())
	}
	if !strings.Contains(resp.GetMessage(), "media_source") {
		t.Fatalf("message = %q, want media_source reason", resp.GetMessage())
	}
}

func TestPostAttributionUnspecifiedTypeDefaultsToInstall(t *testing.T) {
	fb := &fakeBroker{}
	svc := newTestReportService(t, fb, testEnabledApp())

	resp, err := svc.PostAttribution(context.Background(), &ubaV1.PostAttributionRequest{
		AppId: testAppID, AppSecret: testAppSecret,
		Records: []*ubaV1.AttributionRecord{{
			AttributionId: "a-1", DeviceId: "dev-1", MediaSource: "organic",
			AttributionTime: timestamppb.New(testFixedTime()),
		}},
	})
	if err != nil {
		t.Fatalf("PostAttribution() = %v", err)
	}
	if resp.GetAccepted() != 1 || len(fb.bodies) != 1 {
		t.Fatalf("accepted/published = %d/%d, want 1/1", resp.GetAccepted(), len(fb.bodies))
	}
	if fb.topics[0] != topic.UbaAttribution {
		t.Fatalf("topic = %q, want %q", fb.topics[0], topic.UbaAttribution)
	}

	rec, ok := fb.bodies[0].(*ubaV1.AttributionRecord)
	if !ok {
		t.Fatalf("published type = %T", fb.bodies[0])
	}
	if rec.GetType() != ubaV1.AttributionType_INSTALL {
		t.Fatalf("type = %v, want INSTALL default", rec.GetType())
	}
	if rec.GetTenantId() != 42 {
		t.Fatalf("tenantId = %d, want 42(服务端覆盖)", rec.GetTenantId())
	}
}

func TestPostChannelCostValidation(t *testing.T) {
	fb := &fakeBroker{}
	svc := newTestReportService(t, fb, testEnabledApp())
	ctx := context.Background()

	resp, err := svc.PostChannelCost(ctx, &ubaV1.PostChannelCostRequest{
		AppId: testAppID, AppSecret: testAppSecret,
		Records: []*ubaV1.ChannelCostRecord{
			{Date: "2026-11-28", MediaSource: "tiktok", Currency: "USD", Cost: 100.5},
			{Date: "2026-11-3", MediaSource: "tiktok", Currency: "USD", Cost: 1}, // 日期格式错
			{Date: "2026-11-28", MediaSource: "tiktok", Cost: 1},                 // 缺 currency
			{Date: "2026-11-28", MediaSource: "tiktok", Currency: "USD", Cost: -1},
		},
	})
	if err != nil {
		t.Fatalf("PostChannelCost() = %v", err)
	}
	if resp.GetAccepted() != 1 || resp.GetRejected() != 3 {
		t.Fatalf("accepted/rejected = %d/%d, want 1/3", resp.GetAccepted(), resp.GetRejected())
	}
	if len(fb.topics) != 1 || fb.topics[0] != topic.UbaChannelCost {
		t.Fatalf("topic = %v, want %q", fb.topics, topic.UbaChannelCost)
	}
}
