package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationV1 "go-wind-uba/api/gen/go/notification/service/v1"
)

// defaultNotificationRules 是「空表才播」的初始事件→通道路由：宁可重复播不可漏播，
// 删行不复活，所以这张表的结构约定一旦被误改，所有新租户的默认通知路由都会错。
func TestDefaultNotificationRules_Coverage(t *testing.T) {
	require.NotEmpty(t, defaultNotificationRules)

	seen := make(map[notificationV1.EventType]notificationV1.Channel, len(defaultNotificationRules))
	for _, r := range defaultNotificationRules {
		assert.NotEqual(t, notificationV1.EventType_EVENT_TYPE_UNSPECIFIED, r.EventType,
			"路由表不得包含未指定事件类型")
		assert.NotEqual(t, notificationV1.Channel_CHANNEL_UNSPECIFIED, r.Channel,
			"路由表不得包含未指定渠道")

		// 同一事件类型不得出现重复路由（重复会造成双发）。
		_, dup := seen[r.EventType]
		assert.False(t, dup, "事件类型 %v 重复配置路由", r.EventType)
		seen[r.EventType] = r.Channel
	}

	// 关键路由锚定：站内信走台账、监控告警走站内信、风险告警异步走 Webhook。
	assert.Equal(t, notificationV1.Channel_INTERNAL, seen[notificationV1.EventType_INTERNAL_MESSAGE])
	assert.Equal(t, notificationV1.Channel_INTERNAL, seen[notificationV1.EventType_MONITOR_ALERT])
	assert.Equal(t, notificationV1.Channel_WEBHOOK, seen[notificationV1.EventType_RISK_ALERT])
	assert.Equal(t, notificationV1.Channel_INTERNAL, seen[notificationV1.EventType_REPORT_GENERATED])

	// 异步标记只允许出现在 WEBHOOK 渠道（站内信创建必须同步落台账）。
	for _, r := range defaultNotificationRules {
		if r.Channel == notificationV1.Channel_INTERNAL {
			assert.False(t, r.IsAsync, "站内信路由 %v 应同步投递", r.EventType)
		}
	}
	assert.True(t, isRiskAlertAsync(), "风险告警应异步投递")
}

func isRiskAlertAsync() bool {
	for _, r := range defaultNotificationRules {
		if r.EventType == notificationV1.EventType_RISK_ALERT {
			return r.IsAsync
		}
	}
	return false
}
