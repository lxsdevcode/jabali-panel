package models

import "time"

// ImpersonationGrant is a server-side admin act-as grant (GH #183, ADR-0128).
//
// A grant lets an authenticated admin operate on a target (non-admin) user's
// resources via an effective-user override at the authorization layer — NOT a
// Kratos session swap, so the admin's own session is untouched. The ID is the
// token carried in the X-Jabali-Act-As header; the override middleware honors
// it only when the *real* Kratos cookie belongs to AdminUserID. Server-side
// (vs a bare header) so impersonation is revocable (EndedAt) and time-boxed
// (ExpiresAt).
type ImpersonationGrant struct {
	ID           string     `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	AdminUserID  string     `gorm:"column:admin_user_id;type:char(26);not null;index:idx_impersonation_grants_admin" json:"admin_user_id"`
	TargetUserID string     `gorm:"column:target_user_id;type:char(26);not null" json:"target_user_id"`
	CreatedAt    time.Time  `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
	ExpiresAt    time.Time  `gorm:"column:expires_at;type:datetime(6);not null;index:idx_impersonation_grants_expires" json:"expires_at"`
	EndedAt      *time.Time `gorm:"column:ended_at;type:datetime(6)" json:"ended_at,omitempty"`
}

func (ImpersonationGrant) TableName() string { return "impersonation_grants" }
