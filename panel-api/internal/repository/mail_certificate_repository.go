package repository

import (
	"context"
	"errors"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// MailCertificateRepository owns mail_certificate rows. One row per
// domain that has opted in to per-domain mail TLS. See M6.6 blueprint
// (plans/m6.6-per-domain-mail-tls.md) + ADR-0091.
type MailCertificateRepository interface {
	GetByDomain(ctx context.Context, domainID string) (*models.MailCertificate, error)
	// ListWithDomain / ListWithDomainByUser project non-disabled mail certs as
	// SSLCertificateWithDomain rows so they render in the SSL Manager next to
	// website certs (id="mail-cert:<domainID>", domain_name="mail.<domain>").
	ListWithDomain(ctx context.Context) ([]SSLCertificateWithDomain, error)
	ListWithDomainByUser(ctx context.Context, userID string) ([]SSLCertificateWithDomain, error)
	List(ctx context.Context) ([]*models.MailCertificate, error)
	EnsureForDomain(ctx context.Context, domainID string) (*models.MailCertificate, error)
	UpdateStatus(ctx context.Context, id, status string, lastError *string) error
	MarkIssued(ctx context.Context, id, lineagePath string, issuedAt, expiresAt time.Time) error
	MarkFailed(ctx context.Context, id, errMsg string, retryAfter time.Duration) error
	MarkDNSMissing(ctx context.Context, id, errMsg string) error
	Delete(ctx context.Context, id string) error
	// CountFirstIssueAttemptsLastWeek returns the number of distinct
	// domains that have a first-issuance attempt recorded in the last
	// 7 days. Renewals (rows with issued_at NOT NULL) don't count
	// against LE's new-order rate limit and are excluded.
	CountFirstIssueAttemptsLastWeek(ctx context.Context) (int64, error)
}

type mailCertRepo struct{ db *gorm.DB }

func NewMailCertificateRepository(db *gorm.DB) MailCertificateRepository {
	return &mailCertRepo{db: db}
}

func (r *mailCertRepo) GetByDomain(ctx context.Context, domainID string) (*models.MailCertificate, error) {
	var c models.MailCertificate
	err := r.db.WithContext(ctx).Where("domain_id = ?", domainID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, translate(err)
	}
	return &c, nil
}

// mailCertSSLCols projects mail_certificate (joined with domains + users) into
// the SSLCertificateWithDomain shape used by the SSL Manager. Synthetic id +
// "mail." hostname mark it as a per-domain mail cert; renewal_count/staging/
// last_renewed_at have no mail-cert analogue and stay zero.
const mailCertSSLCols = `CONCAT('mail-cert:', mc.domain_id) as id, mc.domain_id, ` +
	`CONCAT('mail.', d.name) as domain_name, d.user_id, u.username as user_username, ` +
	`mc.status, mc.issued_at, mc.expires_at, mc.last_error, mc.updated_at as last_attempt_at`

func (r *mailCertRepo) ListWithDomain(ctx context.Context) ([]SSLCertificateWithDomain, error) {
	var out []SSLCertificateWithDomain
	err := r.db.WithContext(ctx).
		Select(mailCertSSLCols).
		Table("mail_certificate mc").
		Joins("JOIN domains d ON mc.domain_id = d.id").
		Joins("JOIN users u ON d.user_id = u.id").
		Where("mc.status <> ?", models.MailCertStatusDisabled).
		Order("mc.updated_at DESC").
		Scan(&out).Error
	if err != nil {
		return nil, translate(err)
	}
	return out, nil
}

func (r *mailCertRepo) ListWithDomainByUser(ctx context.Context, userID string) ([]SSLCertificateWithDomain, error) {
	var out []SSLCertificateWithDomain
	err := r.db.WithContext(ctx).
		Select(mailCertSSLCols).
		Table("mail_certificate mc").
		Joins("JOIN domains d ON mc.domain_id = d.id").
		Joins("JOIN users u ON d.user_id = u.id").
		Where("d.user_id = ? AND mc.status <> ?", userID, models.MailCertStatusDisabled).
		Order("mc.updated_at DESC").
		Scan(&out).Error
	if err != nil {
		return nil, translate(err)
	}
	return out, nil
}

func (r *mailCertRepo) List(ctx context.Context) ([]*models.MailCertificate, error) {
	var rows []*models.MailCertificate
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, translate(err)
	}
	return rows, nil
}

// EnsureForDomain returns the row for this domain, creating a fresh
// `pending` row if none exists. Idempotent — repeat calls return
// the existing row.
func (r *mailCertRepo) EnsureForDomain(ctx context.Context, domainID string) (*models.MailCertificate, error) {
	existing, err := r.GetByDomain(ctx, domainID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	row := &models.MailCertificate{
		ID:       ulid.Make().String(),
		DomainID: domainID,
		Status:   models.MailCertStatusPending,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, translate(err)
	}
	return row, nil
}

func (r *mailCertRepo) UpdateStatus(ctx context.Context, id, status string, lastError *string) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}
	if lastError != nil {
		updates["last_error"] = *lastError
	} else {
		updates["last_error"] = nil
	}
	return translate(r.db.WithContext(ctx).Model(&models.MailCertificate{}).Where("id = ?", id).Updates(updates).Error)
}

func (r *mailCertRepo) MarkIssued(ctx context.Context, id, lineagePath string, issuedAt, expiresAt time.Time) error {
	updates := map[string]any{
		"status":        models.MailCertStatusIssued,
		"lineage_path":  lineagePath,
		"issued_at":     issuedAt,
		"expires_at":    expiresAt,
		"last_error":    nil,
		"next_retry_at": nil,
		"attempt_count": 0,
		"updated_at":    time.Now(),
	}
	return translate(r.db.WithContext(ctx).Model(&models.MailCertificate{}).Where("id = ?", id).Updates(updates).Error)
}

func (r *mailCertRepo) MarkFailed(ctx context.Context, id, errMsg string, retryAfter time.Duration) error {
	retryAt := time.Now().Add(retryAfter)
	updates := map[string]any{
		"status":        models.MailCertStatusFailed,
		"last_error":    errMsg,
		"next_retry_at": retryAt,
		"updated_at":    time.Now(),
	}
	return translate(r.db.WithContext(ctx).Model(&models.MailCertificate{}).Where("id = ?", id).
		Updates(updates).
		Error)
}

func (r *mailCertRepo) MarkDNSMissing(ctx context.Context, id, errMsg string) error {
	// 1h backoff for DNS recheck. Cheap query so we don't need long.
	retryAt := time.Now().Add(time.Hour)
	updates := map[string]any{
		"status":        models.MailCertStatusDNSMissing,
		"last_error":    errMsg,
		"next_retry_at": retryAt,
		"updated_at":    time.Now(),
	}
	return translate(r.db.WithContext(ctx).Model(&models.MailCertificate{}).Where("id = ?", id).Updates(updates).Error)
}

func (r *mailCertRepo) Delete(ctx context.Context, id string) error {
	return translate(r.db.WithContext(ctx).Delete(&models.MailCertificate{}, "id = ?", id).Error)
}

func (r *mailCertRepo) CountFirstIssueAttemptsLastWeek(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.MailCertificate{}).
		Where("attempt_count >= ? AND updated_at > ? AND issued_at IS NULL", 1, cutoff).
		Count(&count).Error
	return count, translate(err)
}
