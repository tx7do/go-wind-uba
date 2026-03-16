package schema

import "time"

// 路径特征表
// 对应表：gw_uba.path_features

type PathFeatures struct {
	PathID          string    `ch:"path_id"`
	TenantID        uint32    `ch:"tenant_id"`
	UserID          uint32    `ch:"user_id"`
	SessionID       uint32    `ch:"session_id"`
	PathHash        string    `ch:"path_hash"`
	FirstEvent      string    `ch:"first_event"`
	LastEvent       string    `ch:"last_event"`
	PathLength      uint8     `ch:"path_length"`
	First3Events    []string  `ch:"first_3_events"`
	Last3Events     []string  `ch:"last_3_events"`
	IsConverted     uint8     `ch:"is_converted"`
	ConversionEvent string    `ch:"conversion_event"`
	ConversionTime  time.Time `ch:"conversion_time"`
	StartTime       time.Time `ch:"start_time"`
	EndTime         time.Time `ch:"end_time"`
	EventDate       time.Time `ch:"event_date"`
	TotalDurationMs uint64    `ch:"total_duration_ms"`
	StepCount       uint8     `ch:"step_count"`
	CreatedAt       time.Time `ch:"created_at"`
	UpdatedAt       time.Time `ch:"updated_at"`
}
