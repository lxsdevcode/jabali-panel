package models

import "time"

// Mail certificate status state machine. Mirrors PanelCertificate
// (ADR-0066) on purpose so the same verbs work for the panel
// hostname cert AND per-domain mail certs.
//
// pending      → row just created (domain opted in). Reconciler will
//                dispatch ssl.mail.issue on the next tick.
// dns_missing  → agent reported the 4 SAN hostnames (mail/autoconfig/
//                mta-sts/autodiscover) do not all resolve to the panel
//                IP. Reconciler re-checks once per hour.
// issuing      → agent is mid-flight on certbot certonly. Should be
//                short-lived; watchdog re-flips to failed after a
//                stale window.
// issued       → certbot returned a fresh lineage; deploy-hook
//                copied the cert into Stalwart. expires_at carries
//                the LE notAfter.
// failed       → certbot returned a real error (rate-limit, HTTP-01
//                fail, etc). next_retry_at gates 3h backoff.
// disabled     → operator opted out; reconciler ignores.
const (
	MailCertStatusPending     = "pending"
	MailCertStatusDNSMissing  = "dns_missing"
	MailCertStatusIssuing     = "issuing"
	MailCertStatusIssued      = "issued"
	MailCertStatusFailed      = "failed"
	MailCertStatusDisabled    = "disabled"
)

// MailCertificate tracks per-domain mail TLS state. See M6.6
// blueprint (plans/m6.6-per-domain-mail-tls.md) + ADR-0091.
type MailCertificate struct {
	ID            string     `gorm:"type:char(26);primaryKey" json:"id"`
	DomainID      string     `gorm:"column:domain_id;type:char(26);not null;uniqueIndex" json:"domain_id"`
	LineagePath   string     `gorm:"column:lineage_path;type:varchar(255);not null;default:''" json:"lineage_path"`
	Status        string     `gorm:"type:varchar(32);not null;default:'pending'" json:"status"`
	LastError     *string    `gorm:"column:last_error;type:text;null" json:"last_error,omitempty"`
	AttemptCount  uint32     `gorm:"column:attempt_count;type:int unsigned;not null;default:0" json:"attempt_count"`
	NextRetryAt   *time.Time `gorm:"column:next_retry_at;type:timestamp;null" json:"next_retry_at,omitempty"`
	IssuedAt      *time.Time `gorm:"column:issued_at;type:timestamp;null" json:"issued_at,omitempty"`
	ExpiresAt     *time.Time `gorm:"column:expires_at;type:timestamp;null" json:"expires_at,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:timestamp;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:timestamp;not null;default:CURRENT_TIMESTAMP;onUpdate:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (MailCertificate) TableName() string { return "mail_certificate" }
