package models

import "time"

// M53 Updates Center models. See ADR-0118.

// Update kinds + actions + statuses. Kept as constants so handlers, repos and
// the reconciler all agree on the wire spelling (the columns are plain
// VARCHARs, not DB enums).
const (
	UpdateKindJabali = "jabali"
	UpdateKindApt    = "apt"

	UpdateActionCheck = "check"
	UpdateActionRun   = "run"

	UpdateStatusRunning = "running"
	UpdateStatusSuccess = "success"
	UpdateStatusFailed  = "failed"
)

// UpdateState is the singleton (id=1) snapshot of the last check results so the
// Updates page renders instantly without an agent round-trip. Refreshed by
// panel-api whenever a check/run completes.
type UpdateState struct {
	ID               uint8      `gorm:"type:tinyint;not null;default:1;primaryKey" json:"id"`
	JabaliBehind     int        `gorm:"type:int;not null;default:0"                json:"jabali_behind"`
	JabaliCurrentSHA string     `gorm:"column:jabali_current_sha;type:char(40)"    json:"jabali_current_sha,omitempty"`
	JabaliCheckedAt  *time.Time `                                                  json:"jabali_checked_at,omitempty"`
	AptTotal         int        `gorm:"type:int;not null;default:0"                json:"apt_total"`
	AptSecurity      int        `gorm:"type:int;not null;default:0"                json:"apt_security"`
	AptCheckedAt     *time.Time `                                                  json:"apt_checked_at,omitempty"`
}

// TableName pins GORM to the migration's spelling (no pluralisation).
func (UpdateState) TableName() string { return "update_state" }

// UpdateHistory is one check/run record. `Unit` holds the transient systemd
// unit name for a run so the run reconciler can match a still-running row back
// to its host unit and mark it finished independently of any UI poll.
type UpdateHistory struct {
	ID            string     `gorm:"type:char(26);primaryKey"            json:"id"`
	Kind          string     `gorm:"type:varchar(16);not null"           json:"kind"`
	Action        string     `gorm:"type:varchar(16);not null"           json:"action"`
	Status        string     `gorm:"type:varchar(16);not null"           json:"status"`
	SecurityCount int        `gorm:"type:int;not null;default:0"         json:"security_count"`
	Summary       string     `gorm:"type:varchar(255);not null;default:''" json:"summary"`
	LogExcerpt    string     `gorm:"type:text"                           json:"log_excerpt,omitempty"`
	Unit          string     `gorm:"type:varchar(128)"                   json:"unit,omitempty"`
	StartedAt     time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP"  json:"started_at"`
	FinishedAt    *time.Time `                                           json:"finished_at,omitempty"`
}

// TableName pins GORM to the migration's spelling.
func (UpdateHistory) TableName() string { return "update_history" }

// UpdateAutoupdateConfig is the singleton (id=1) desired auto-update state. The
// autoupdate reconciler converges it onto the host. apt patches via
// unattended-upgrades; jabali self-update is opt-in and OFF by default because
// a bad self-update can brick the panel (ADR-0118 decision 1).
type UpdateAutoupdateConfig struct {
	ID            uint8     `gorm:"type:tinyint;not null;default:1;primaryKey" json:"id"`
	AptEnabled    bool      `gorm:"type:boolean;not null;default:false"        json:"apt_enabled"`
	AptTime       string    `gorm:"type:varchar(5);not null;default:'03:30'"   json:"apt_time"`
	JabaliEnabled bool      `gorm:"type:boolean;not null;default:false"        json:"jabali_enabled"`
	JabaliTime    string    `gorm:"type:varchar(5);not null;default:'04:30'"   json:"jabali_time"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"                              json:"updated_at"`
}

// TableName pins GORM to the migration's spelling.
func (UpdateAutoupdateConfig) TableName() string { return "update_autoupdate_config" }
