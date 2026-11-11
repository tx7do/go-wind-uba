package task

// CampaignSendTaskType 是营销触达群发任务的类型常量。
const CampaignSendTaskType = "campaign_send"

// CampaignSendTaskData 是营销触达群发任务的载荷。
type CampaignSendTaskData struct {
	CampaignID int64 `json:"campaignId"`
}

// ReplayRetentionSweepTaskType 是会话回放保留期清扫任务的类型常量。
const ReplayRetentionSweepTaskType = "replay_retention_sweep"

// ReplayRetentionSweepCronSpec 是保留期清扫的系统级 cron（每天 03:30）。
const ReplayRetentionSweepCronSpec = "30 3 * * *"

// ReplayRetentionSweepTaskData 是清扫任务载荷。
type ReplayRetentionSweepTaskData struct{}
