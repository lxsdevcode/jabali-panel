package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// MailboxSendDelegationRepository is data access for send-as delegations (GH #347).
// Jabali is truth (ADR-0042); the converger materialises ListAllPairs into
// Stalwart's mustMatchSender expression (Mechanism C).
type MailboxSendDelegationRepository interface {
	Create(ctx context.Context, d *models.MailboxSendDelegation) error
	Delete(ctx context.Context, id string) error
	DeleteByPair(ctx context.Context, delegateMailboxID, grantorMailboxID string) error
	// ListByDelegate returns the grantors a delegate mailbox may send as.
	ListByDelegate(ctx context.Context, delegateMailboxID string) ([]models.MailboxSendDelegation, error)
	// ListByGrantor returns the delegates allowed to send as a grantor mailbox.
	ListByGrantor(ctx context.Context, grantorMailboxID string) ([]models.MailboxSendDelegation, error)
	// ExistsPair reports whether a (delegate, grantor) delegation already exists.
	ExistsPair(ctx context.Context, delegateMailboxID, grantorMailboxID string) (bool, error)
	// ListAllPairs returns every ENABLED delegation as resolved (delegate email,
	// grantor email) — both mailboxes joined and required to be non-disabled.
	// This is the authoritative input to the mustMatchSender converger; a
	// disabled mailbox on either side drops the pair (it can neither send nor be
	// sent-as). Ordered deterministically so the built expression is stable.
	ListAllPairs(ctx context.Context) ([]models.SendAsPair, error)
}

type mailboxSendDelegationRepo struct {
	db *gorm.DB
}

func NewMailboxSendDelegationRepository(db *gorm.DB) MailboxSendDelegationRepository {
	return &mailboxSendDelegationRepo{db: db}
}

func (r *mailboxSendDelegationRepo) Create(ctx context.Context, d *models.MailboxSendDelegation) error {
	now := time.Now().UTC()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	d.UpdatedAt = now
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *mailboxSendDelegationRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.MailboxSendDelegation{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *mailboxSendDelegationRepo) DeleteByPair(ctx context.Context, delegateMailboxID, grantorMailboxID string) error {
	res := r.db.WithContext(ctx).
		Where("delegate_mailbox_id = ? AND grantor_mailbox_id = ?", delegateMailboxID, grantorMailboxID).
		Delete(&models.MailboxSendDelegation{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *mailboxSendDelegationRepo) ListByDelegate(ctx context.Context, delegateMailboxID string) ([]models.MailboxSendDelegation, error) {
	return r.listByField(ctx, "delegate_mailbox_id", delegateMailboxID)
}

func (r *mailboxSendDelegationRepo) ListByGrantor(ctx context.Context, grantorMailboxID string) ([]models.MailboxSendDelegation, error) {
	return r.listByField(ctx, "grantor_mailbox_id", grantorMailboxID)
}

func (r *mailboxSendDelegationRepo) listByField(ctx context.Context, field, value string) ([]models.MailboxSendDelegation, error) {
	var rows []models.MailboxSendDelegation
	if err := r.db.WithContext(ctx).
		Where(field+" = ?", value).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *mailboxSendDelegationRepo) ExistsPair(ctx context.Context, delegateMailboxID, grantorMailboxID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.MailboxSendDelegation{}).
		Where("delegate_mailbox_id = ? AND grantor_mailbox_id = ?", delegateMailboxID, grantorMailboxID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *mailboxSendDelegationRepo) ListAllPairs(ctx context.Context) ([]models.SendAsPair, error) {
	var pairs []models.SendAsPair
	err := r.db.WithContext(ctx).
		Model(&models.MailboxSendDelegation{}).
		Select("dm.email_cached AS delegate_email, gm.email_cached AS grantor_email").
		Joins("JOIN mailboxes dm ON dm.id = mailbox_send_delegations.delegate_mailbox_id AND dm.is_disabled = 0").
		Joins("JOIN mailboxes gm ON gm.id = mailbox_send_delegations.grantor_mailbox_id AND gm.is_disabled = 0").
		Order("dm.email_cached ASC, gm.email_cached ASC").
		Scan(&pairs).Error
	if err != nil {
		return nil, err
	}
	return pairs, nil
}
