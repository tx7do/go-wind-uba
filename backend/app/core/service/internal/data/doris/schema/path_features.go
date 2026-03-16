package schema

import "time"

type PathFeatures struct {
	PathID          *string    `json:"path_id"`
	TenantID        *uint32    `json:"tenant_id"`
	UserID          *uint32    `json:"user_id"`
	SessionID       *uint32    `json:"session_id"`
	PathHash        *string    `json:"path_hash"`
	FirstEvent      *string    `json:"first_event"`
	LastEvent       *string    `json:"last_event"`
	PathLength      *uint8     `json:"path_length"`
	First3Events    []string   `json:"first_3_events"`
	Last3Events     []string   `json:"last_3_events"`
	IsConverted     *uint8     `json:"is_converted"`
	ConversionEvent *string    `json:"conversion_event"`
	ConversionTime  *time.Time `json:"conversion_time"`
	StartTime       *time.Time `json:"start_time"`
	EndTime         *time.Time `json:"end_time"`
	EventDate       *time.Time `json:"event_date"`
	TotalDurationMs *uint64    `json:"total_duration_ms"`
	StepCount       *uint8     `json:"step_count"`
	CreatedAt       *time.Time `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at"`
}
