package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// UserAPIToken — per-user bearer token for the public API. Owner
// (user_id) is the only identity the token can act as; ownership
// checks on existing handlers (claims.UserID == resource.UserID)
// continue to work unmodified when a token-authed request flows
// through them.
//
// Wire format: `Authorization: Bearer jat_<43-char base64url>`
// (random 32 bytes). The plaintext is shown ONCE at mint and never
// persisted; only sha256(plaintext) goes in secret_hash. last4 is
// stored for UI display ("jat_…aB9c") and carries no security weight.
type UserAPIToken struct {
	ID           string         `gorm:"column:id;primaryKey;type:char(26)" json:"id"`
	UserID       string         `gorm:"column:user_id;type:char(26);not null;index:ix_user_api_tokens_user" json:"user_id"`
	Name         string         `gorm:"column:name;type:varchar(100);not null" json:"name"`
	SecretHash   string         `gorm:"column:secret_hash;type:char(64);not null;uniqueIndex:uq_user_api_tokens_secret_hash" json:"-"`
	SecretLast4  string         `gorm:"column:secret_last4;type:char(4);not null" json:"secret_last4"`
	Scopes       UserAPIScopes  `gorm:"column:scopes_json;type:json" json:"scopes"`
	LastUsedAt   *time.Time     `gorm:"column:last_used_at;type:datetime(6)" json:"last_used_at,omitempty"`
	LastUsedIP   *string        `gorm:"column:last_used_ip;type:varchar(45)" json:"last_used_ip,omitempty"`
	ExpiresAt    *time.Time     `gorm:"column:expires_at;type:datetime(6)" json:"expires_at,omitempty"`
	RevokedAt    *time.Time     `gorm:"column:revoked_at;type:datetime(6)" json:"revoked_at,omitempty"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
}

func (UserAPIToken) TableName() string { return "user_api_tokens" }

// IsActive reports whether the token can authenticate a request
// right now: not revoked, not expired. The DB query that pulls
// tokens for auth already filters revoked_at IS NULL, but a token
// fetched for an admin/listing path may need this client-side.
func (t *UserAPIToken) IsActive(now time.Time) bool {
	if t == nil {
		return false
	}
	if t.RevokedAt != nil {
		return false
	}
	if t.ExpiresAt != nil && !now.Before(*t.ExpiresAt) {
		return false
	}
	return true
}

// UserAPIScopes — same shape + semantics as AutomationScopes:
// JSON-marshalled []string with wildcard support ("dns:*" covers
// any "dns:..." capability). Nil/empty scopes => the token can
// do anything its owner can do (full per-user access). Scope
// strings are operator-defined for now; the well-known set in
// docs/api-tokens.md should be the only producer.
type UserAPIScopes []string

func (s *UserAPIScopes) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("user_api_scopes: unsupported scan type")
	}
	if len(b) == 0 {
		*s = nil
		return nil
	}
	return json.Unmarshal(b, s)
}

func (s UserAPIScopes) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// Has returns true when the scope grants the requested capability.
// Nil/empty scopes grant EVERYTHING (full account access). Wildcards
// follow the AutomationScopes convention: "dns:*" matches "dns:read",
// "dns:write", etc.
func (s UserAPIScopes) Has(want string) bool {
	if len(s) == 0 {
		return true
	}
	for _, granted := range s {
		if granted == want {
			return true
		}
		if len(granted) > 2 && granted[len(granted)-2:] == ":*" {
			prefix := granted[:len(granted)-1]
			if len(want) >= len(prefix) && want[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}
