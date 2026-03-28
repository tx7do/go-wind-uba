package schema

import "time"

type PathFeatures struct {
	PathID          *string     `db:"path_id"`
	TenantID        *uint32     `db:"tenant_id"`
	UserID          *uint32     `db:"user_id"`
	SessionID       *string     `db:"session_id"`
	PathHash        *string     `db:"path_hash"`
	FirstEvent      *string     `db:"first_event"`
	LastEvent       *string     `db:"last_event"`
	PathLength      *uint8      `db:"path_length"`
	First3Events    StringArray `db:"first_3_events"`
	Last3Events     StringArray `db:"last_3_events"`
	IsConverted     *uint8      `db:"is_converted"`
	ConversionEvent *string     `db:"conversion_event"`
	ConversionTime  *time.Time  `db:"conversion_time"`
	StartTime       *time.Time  `db:"start_time"`
	EndTime         *time.Time  `db:"end_time"`
	EventDate       *time.Time  `db:"event_date"`
	TotalDurationMs *uint64     `db:"total_duration_ms"`
	StepCount       *uint8      `db:"step_count"`
}
