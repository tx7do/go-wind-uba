package clickhouse

import (
	"context"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"
	"github.com/tx7do/go-utils/trans"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newClickHouseTestClient() *clickhouseCrud.Client {
	cli, err := clickhouseCrud.NewClient(
		clickhouseCrud.WithAddresses("localhost:9000"),
		clickhouseCrud.WithDatabase("gw_uba"),
		clickhouseCrud.WithUsername("default"),
		clickhouseCrud.WithPassword("*Abcd123456"),
		clickhouseCrud.WithScheme("native"),
	)
	if err != nil {
		return nil
	}

	return cli
}

func TestEventsFactRepo(t *testing.T) {
	ctx := context.Background()

	db := newClickHouseTestClient()
	if db == nil {
		t.Fatal("failed to create clickhouse client")
	}

	cfg := &conf.Bootstrap{}

	bctx := bootstrap.NewContextWithParam(ctx, &conf.AppInfo{}, cfg, log.DefaultLogger)
	repo := NewEventsFactRepo(bctx, db)
	assert.NotNil(t, repo)

	event := &ubaV1.BehaviorEvent{
		EventId:       "test-event-uuid-001",
		TenantId:      1001,
		UserId:        1,
		DeviceId:      "device-001",
		AccountId:     "account-001",
		GlobalUserId:  "global-user-001",
		EventTime:     timestamppb.Now(),
		EventTs:       time.Now().UnixMilli(),
		ServerTime:    timestamppb.Now(),
		EventCategory: trans.Ptr(ubaV1.EventCategory_EVENT_CATEGORY_AUTH),
		EventName:     "login",
		EventAction:   "success",
		ObjectType:    "page",
		ObjectId:      "page-001",
		ObjectName:    "首页",
		SessionId:     "session-001",
		SessionSeq:    1,
		Platform:      trans.Ptr(ubaV1.Platform_PLATFORM_WEB),
		Os:            "Windows",
		AppVersion:    "1.0.0",
		Channel:       "official",
		Ip:            "127.0.0.1",
		IpCity:        "北京",
		Country:       "CN",
		Network:       "WiFi",
		Geo:           "39.9042,116.4074",
		UserAgent:     "Mozilla/5.0",
		Referer:       "https://example.com",
		Context: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
		Metrics: map[string]float64{
			"metric1": 1.23,
			"metric2": 4.56,
			"metric3": 7.89,
		},
		Properties: map[string]string{
			"prop1": "value1",
			"prop2": "value2",
			"prop3": "value3",
		},
		OpResult:  trans.Ptr(ubaV1.OpResult_OP_RESULT_SUCCESS),
		RiskLevel: trans.Ptr(ubaV1.RiskLevel_RISK_LEVEL_CRITICAL),
	}
	err := repo.Create(ctx, event)
	assert.Nil(t, err)
}
