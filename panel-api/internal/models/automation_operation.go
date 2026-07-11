package models

import "time"

// AutomationOperation tracks a long-running write-automation action (JAB-140).
// Fast writes (restart, suspend, purge) complete synchronously and never create a
// row; only async actions (backups) do, so Sounder can poll GET
// /automation/operations/:id. Also the idempotency ledger for non-idempotent
// creates: a repeated (token_id, idempotency_key) returns the first op instead of
// re-running the action.
//
// No FK to automation_tokens — tokens are revocable/deletable and we keep the op
// history regardless. Column tags pinned for the *ID initialisms (gorm_column_tags).
type AutomationOperation struct {
	ID             string    `gorm:"column:id;primaryKey;type:char(26)" json:"id"`
	TokenID        string    `gorm:"column:token_id;type:char(26);not null;index:ix_ao_token;uniqueIndex:uq_ao_idem,priority:1" json:"token_id"`
	IdempotencyKey *string   `gorm:"column:idempotency_key;type:varchar(80);uniqueIndex:uq_ao_idem,priority:2" json:"idempotency_key,omitempty"`
	Action         string    `gorm:"column:action;type:varchar(64);not null" json:"action"`
	Target         string    `gorm:"column:target;type:varchar(255);not null;default:''" json:"target"`
	Status         string    `gorm:"column:status;type:enum('pending','running','done','error');not null;default:'pending'" json:"status"`
	Message        string    `gorm:"column:message;type:varchar(512);not null;default:''" json:"message"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
}

func (AutomationOperation) TableName() string { return "automation_operations" }
