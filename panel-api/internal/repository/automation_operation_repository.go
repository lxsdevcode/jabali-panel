package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// AutomationOperationRepository tracks async write-automation operations +
// serves as the idempotency ledger for non-idempotent creates (JAB-140).
type AutomationOperationRepository interface {
	Create(ctx context.Context, op *models.AutomationOperation) error
	FindByID(ctx context.Context, id string) (*models.AutomationOperation, error)
	// FindByIdempotencyKey returns the prior op for (tokenID, key) or ErrNotFound.
	// Used to short-circuit a retried create so it never double-fires.
	FindByIdempotencyKey(ctx context.Context, tokenID, key string) (*models.AutomationOperation, error)
	// SetStatus transitions an op and stamps updated_at (pending->running->done/error).
	SetStatus(ctx context.Context, id, status, message string) error
}

type automationOperationRepo struct {
	db *gorm.DB
}

func NewAutomationOperationRepository(db *gorm.DB) AutomationOperationRepository {
	return &automationOperationRepo{db: db}
}

func (r *automationOperationRepo) Create(ctx context.Context, op *models.AutomationOperation) error {
	now := time.Now().UTC()
	if op.CreatedAt.IsZero() {
		op.CreatedAt = now
	}
	op.UpdatedAt = now
	if op.Status == "" {
		op.Status = "pending"
	}
	return r.db.WithContext(ctx).Create(op).Error
}

func (r *automationOperationRepo) FindByID(ctx context.Context, id string) (*models.AutomationOperation, error) {
	var op models.AutomationOperation
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&op).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &op, nil
}

func (r *automationOperationRepo) FindByIdempotencyKey(ctx context.Context, tokenID, key string) (*models.AutomationOperation, error) {
	var op models.AutomationOperation
	if err := r.db.WithContext(ctx).
		Where("token_id = ? AND idempotency_key = ?", tokenID, key).
		First(&op).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &op, nil
}

func (r *automationOperationRepo) SetStatus(ctx context.Context, id, status, message string) error {
	res := r.db.WithContext(ctx).
		Model(&models.AutomationOperation{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     status,
			"message":    message,
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
