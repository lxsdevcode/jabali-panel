package models

import "time"

// CacheTokenSalt is the per-tenant (per-user) random salt mixed into the
// WP-cache Redis ACL token derivation (Gitea #415). One row per user, created
// lazily on first cache-enable; regenerating it rotates ONLY that tenant's
// token without touching the global secret or any other tenant.
type CacheTokenSalt struct {
	UserID    string    `gorm:"column:user_id;type:char(26);primaryKey" json:"user_id"`
	Salt      string    `gorm:"column:salt;type:char(64);not null" json:"-"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
}

func (CacheTokenSalt) TableName() string { return "cache_token_salts" }
