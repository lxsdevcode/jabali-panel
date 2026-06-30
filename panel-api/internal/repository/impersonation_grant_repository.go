package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// ImpersonationGrantRepository persists admin act-as grants (ADR-0128). A
// grant is "active" while ended_at IS NULL and expires_at > now; FindActive
// enforces that at read time so an expired grant can never override claims
// even before the reaper deletes it.
type ImpersonationGrantRepository interface {
	Create(ctx context.Context, g *models.ImpersonationGrant) error
	// FindActive returns the grant only if it is unexpired and not ended.
	// Returns ErrNotFound otherwise.
	FindActive(ctx context.Context, id string, now time.Time) (*models.ImpersonationGrant, error)
	// End marks a grant ended (idempotent: a no-op if already ended).
	End(ctx context.Context, id string, endedAt time.Time) error
	ListActiveByAdmin(ctx context.Context, adminUserID string, now time.Time) ([]models.ImpersonationGrant, error)
	// ListAllActive returns every active grant (all admins) — operator/CLI view.
	ListAllActive(ctx context.Context, now time.Time) ([]models.ImpersonationGrant, error)
	// ReapExpired hard-deletes grants whose expires_at is before `before`.
	ReapExpired(ctx context.Context, before time.Time) (int64, error)
}

type impersonationGrantRepo struct{ db *gorm.DB }

func NewImpersonationGrantRepository(db *gorm.DB) ImpersonationGrantRepository {
	return &impersonationGrantRepo{db: db}
}

func (r *impersonationGrantRepo) Create(ctx context.Context, g *models.ImpersonationGrant) error {
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	return r.db.WithContext(ctx).Create(g).Error
}

func (r *impersonationGrantRepo) FindActive(ctx context.Context, id string, now time.Time) (*models.ImpersonationGrant, error) {
	var g models.ImpersonationGrant
	err := r.db.WithContext(ctx).
		Where("id = ? AND ended_at IS NULL AND expires_at > ?", id, now).
		First(&g).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *impersonationGrantRepo) End(ctx context.Context, id string, endedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.ImpersonationGrant{}).
		Where("id = ? AND ended_at IS NULL", id).
		Update("ended_at", endedAt).Error
}

func (r *impersonationGrantRepo) ListActiveByAdmin(ctx context.Context, adminUserID string, now time.Time) ([]models.ImpersonationGrant, error) {
	var out []models.ImpersonationGrant
	err := r.db.WithContext(ctx).
		Where("admin_user_id = ? AND ended_at IS NULL AND expires_at > ?", adminUserID, now).
		Order("created_at DESC").
		Find(&out).Error
	return out, err
}

func (r *impersonationGrantRepo) ListAllActive(ctx context.Context, now time.Time) ([]models.ImpersonationGrant, error) {
	var out []models.ImpersonationGrant
	err := r.db.WithContext(ctx).
		Where("ended_at IS NULL AND expires_at > ?", now).
		Order("created_at DESC").
		Find(&out).Error
	return out, err
}

func (r *impersonationGrantRepo) ReapExpired(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("expires_at < ?", before).
		Delete(&models.ImpersonationGrant{})
	return res.RowsAffected, res.Error
}
