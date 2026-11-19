package task

// 本文件钉死 asynq 任务类型常量与 payload 的 JSON 序列化契约。
//
// 为什么只测 JSON 往返与字段标签：payload 会以 JSON 形态写进 Redis（asynq
// 队列），handler 按同名结构体反序列化。字段标签一旦漂移，队列里在途任务
// 会被静默丢字段（默认零值）而非报错——因此用"期望 JSON 逐字节相等"钉死
// 标签，任何无意的 tag 改动都会测试失败。
//
// 注意到本包存在两套命名风格并存：snake_case（backup/base_id/delivery_id）
// 与 camelCase（campaignId）。风格不统一但各自稳定，测试按现状钉死。

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaskTypeConstants 任务类型常量契约：asynq 按 type 字符串路由到 handler，
// 类型名漂移 = 任务入队后没有消费者。
func TestTaskTypeConstants(t *testing.T) {
	assert.Equal(t, "backup", BackupTaskType)
	assert.Equal(t, "campaign_send", CampaignSendTaskType)
	assert.Equal(t, "notification_dispatch", NotificationDispatchTaskType)
	assert.Equal(t, "notification_delivery_sweep", NotificationDeliverySweepTaskType)
	assert.Equal(t, "replay_retention_sweep", ReplayRetentionSweepTaskType)
	assert.Equal(t, "ai.doc.reindex", AiDocReindexTaskType)

	// 任务类型彼此互异（同类型不同 payload 结构会在路由上互相踩踏）。
	names := []string{
		BackupTaskType, CampaignSendTaskType, NotificationDispatchTaskType,
		NotificationDeliverySweepTaskType, ReplayRetentionSweepTaskType,
		AiDocReindexTaskType,
	}
	seen := map[string]bool{}
	for _, n := range names {
		assert.False(t, seen[n], "任务类型重复: %s", n)
		seen[n] = true
	}
}

// TestCronSpecConstants 系统级 cron 表达式契约：调度器按固定表达式建调度项。
func TestCronSpecConstants(t *testing.T) {
	assert.Equal(t, "30 3 * * *", ReplayRetentionSweepCronSpec, "回放保留期清扫：每天 03:30")
	assert.Equal(t, "*/5 * * * *", NotificationDeliverySweepCronSpec, "台账清扫：每 5 分钟")
}

// TestCreateBackupTaskID 任务 ID 格式：backup:<业务ID>。
// 前缀语义钉死——上层按前缀区分任务种类。
func TestCreateBackupTaskID(t *testing.T) {
	assert.Equal(t, "backup:1", CreateBackupTaskID(1))
	assert.Equal(t, "backup:123456", CreateBackupTaskID(123456))
	assert.Equal(t, "backup:0", CreateBackupTaskID(0))
	// ID 可回推（前缀剥离后是原 ID 的十进制形式）。
	assert.Equal(t, int64(42), mustParseBackupID(t, CreateBackupTaskID(42)))
}

// mustParseBackupID 从任务 ID 剥离前缀并回推数值（辅助断言，不入生产代码）。
func mustParseBackupID(t *testing.T, taskID string) int64 {
	t.Helper()
	require.Equal(t, BackupTaskType, taskID[:len(BackupTaskType)])
	require.Greater(t, len(taskID), len(BackupTaskType)+1)
	var id int64
	for _, c := range taskID[len(BackupTaskType)+1:] {
		require.True(t, c >= '0' && c <= '9', "任务 ID 数值段含非数字: %s", taskID)
		id = id*10 + int64(c-'0')
	}
	return id
}

// ---------------------------------------------------------------------------
// payload JSON 序列化契约（roundtrip + 标签稳定性）
// ---------------------------------------------------------------------------

// roundtripJSON 序列化 → 反序列化 → 再序列化，返回首末两段 JSON。
func roundtripJSON[T any](t *testing.T, v T) (first, second string) {
	t.Helper()
	b1, err := json.Marshal(v)
	require.NoError(t, err)

	var back T
	require.NoError(t, json.Unmarshal(b1, &back))

	b2, err := json.Marshal(back)
	require.NoError(t, err)
	return string(b1), string(b2)
}

// TestBackupTaskDataJSON backup 载荷：name 字段（snake_case）。
func TestBackupTaskDataJSON(t *testing.T) {
	in := BackupTaskData{Name: "daily-full-backup"}

	first, second := roundtripJSON(t, in)
	assert.Equal(t, first, second, "序列化必须幂等")
	assert.JSONEq(t, `{"name":"daily-full-backup"}`, first,
		"字段标签漂移会破坏 Redis 队列里在途任务的兼容性")
}

// TestCampaignSendTaskDataJSON 群发载荷：campaignId 为 camelCase（历史标签）。
func TestCampaignSendTaskDataJSON(t *testing.T) {
	in := CampaignSendTaskData{CampaignID: 9001}

	first, second := roundtripJSON(t, in)
	assert.Equal(t, first, second)
	assert.JSONEq(t, `{"campaignId":9001}`, first)
}

// TestReplayRetentionSweepTaskDataJSON 回放清扫载荷：空结构体序列化为 {}，
// 反序列化后仍为空（系统级周期任务，无参数）。
func TestReplayRetentionSweepTaskDataJSON(t *testing.T) {
	first, second := roundtripJSON(t, ReplayRetentionSweepTaskData{})
	assert.Equal(t, "{}", first)
	assert.Equal(t, first, second)
}

// TestNotificationDeliverySweepTaskDataJSON 台账清扫载荷：同上，空对象契约。
func TestNotificationDeliverySweepTaskDataJSON(t *testing.T) {
	first, second := roundtripJSON(t, NotificationDeliverySweepTaskData{})
	assert.Equal(t, "{}", first)
	assert.Equal(t, first, second)
}

// TestAiDocReindexPayloadJSON 重索引载荷：base_id 为 snake_case；
// BaseID=0 表示全量重索引，0 必须真实出现在 JSON 中（不可 omitempty，
// 否则全量语义丢失）。
func TestAiDocReindexPayloadJSON(t *testing.T) {
	in := AiDocReindexPayload{BaseID: 7}
	first, second := roundtripJSON(t, in)
	assert.Equal(t, first, second)
	assert.JSONEq(t, `{"base_id":7}`, first)

	zero := AiDocReindexPayload{BaseID: 0}
	b, err := json.Marshal(zero)
	require.NoError(t, err)
	assert.JSONEq(t, `{"base_id":0}`, string(b), "BaseID=0（全量重索引）必须序列化出来，禁止 omitempty")
}

// TestNotificationDispatchTaskDataJSON 派发载荷：三元组 + 可选 params。
// 这是"验证码明文在 Redis 停留"的唯一载体，字段必须与注释承诺的形态一致。
func TestNotificationDispatchTaskDataJSON(t *testing.T) {
	in := NotificationDispatchTaskData{
		DeliveryId: 77,
		Target:     "user@example.com",
		Title:      "找回密码",
		Content:    "您的验证码是 123456",
		Params:     map[string]string{"code": "123456"},
	}
	first, second := roundtripJSON(t, in)
	assert.Equal(t, first, second)
	assert.JSONEq(t, `{
		"delivery_id": 77,
		"target": "user@example.com",
		"title": "找回密码",
		"content": "您的验证码是 123456",
		"params": {"code": "123456"}
	}`, first)

	// params 为 nil 时应被 omitempty 省略（载荷最小化，减少 Redis 里的可关联信息）。
	noParams := NotificationDispatchTaskData{
		DeliveryId: 1, Target: "a***@x.com", Title: "t", Content: "c",
	}
	b, err := json.Marshal(noParams)
	require.NoError(t, err)
	assert.JSONEq(t, `{"delivery_id":1,"target":"a***@x.com","title":"t","content":"c"}`, string(b))
}
