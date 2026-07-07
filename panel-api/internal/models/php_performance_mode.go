package models

import "time"

// PHPPerformanceMode is one of the 4 admin-editable Performance Mode presets
// (GH #339 phase 2, Step 2). L1 users pick a mode; it resolves to these pm.*
// values, then gets re-clamped to the owner's package caps at apply time.
type PHPPerformanceMode struct {
	Mode                           string    `gorm:"column:mode;type:varchar(24);primaryKey" json:"mode"`
	Label                          string    `gorm:"column:label;type:varchar(64);not null;default:''" json:"label"`
	PmMode                         string    `gorm:"column:pm_mode;type:varchar(16);not null;default:'ondemand'" json:"pm_mode"`
	PmMaxChildren                  uint32    `gorm:"column:pm_max_children;type:int unsigned;not null;default:10" json:"pm_max_children"`
	PmStartServers                 uint32    `gorm:"column:pm_start_servers;type:int unsigned;not null;default:2" json:"pm_start_servers"`
	PmMinSpareServers              uint32    `gorm:"column:pm_min_spare_servers;type:int unsigned;not null;default:1" json:"pm_min_spare_servers"`
	PmMaxSpareServers              uint32    `gorm:"column:pm_max_spare_servers;type:int unsigned;not null;default:3" json:"pm_max_spare_servers"`
	PmMaxRequests                  uint32    `gorm:"column:pm_max_requests;type:int unsigned;not null;default:0" json:"pm_max_requests"`
	RequestTerminateTimeoutSeconds uint32    `gorm:"column:request_terminate_timeout_seconds;type:int unsigned;not null;default:0" json:"request_terminate_timeout_seconds"`
	ProcessIdleTimeoutSeconds      uint32    `gorm:"column:process_idle_timeout_seconds;type:int unsigned;not null;default:60" json:"process_idle_timeout_seconds"`
	UpdatedAt                      time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
}

// TableName pins the table (GORM would pluralize to php_performance_modes anyway,
// but be explicit).
func (PHPPerformanceMode) TableName() string { return "php_performance_modes" }
