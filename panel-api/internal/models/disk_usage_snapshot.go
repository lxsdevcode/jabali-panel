package models

import "time"

// DiskUsageSnapshot is the last computed per-user disk-usage breakdown for the
// tenant Disk Usage page. One row per user (PK user_id), upserted on an
// explicit Refresh. Payload is the JSON-encoded diskUsageResponse so the page
// reads the cached snapshot without re-running quota/db.size/mailbox agent
// calls on every visit; recompute happens only on demand.
type DiskUsageSnapshot struct {
	UserID     string    `gorm:"column:user_id;type:char(26);primaryKey" json:"user_id"`
	Payload    string    `gorm:"column:payload;type:longtext;not null" json:"-"`
	ComputedAt time.Time `gorm:"column:computed_at;type:datetime(6);not null" json:"computed_at"`
	CreatedAt  time.Time `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
}

func (DiskUsageSnapshot) TableName() string { return "disk_usage_snapshots" }
