package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// UserAPITokenRepository handles per-user API token persistence.
// Token authentication takes the hot path FindByHash on every API
// request, so it MUST be O(1) via the secret_hash unique index.
type UserAPITokenRepository interface {
	// Create persists a new token. SecretHash must be set by the
	// caller (sha256 of the plaintext); the repo never sees the
	// plaintext token.
	Create(ctx context.Context, t *models.UserAPIToken) error
	// ListByUser returns every token (active + revoked) for the
	// given user, newest first. Used by the user-facing UI page.
	ListByUser(ctx context.Context, userID string) ([]models.UserAPIToken, error)
	// FindByHash is the auth hot path. Returns the token row when
	// the hash matches AND the token isn't revoked. ErrNotFound
	// otherwise. The middleware does its own expiry check on the
	// returned row.
	FindByHash(ctx context.Context, hashHex string) (*models.UserAPIToken, error)
	// FindByID is for revoke/inspect flows. Returns the row
	// regardless of revoked_at so the UI can still show revoked
	// tokens in the history.
	FindByID(ctx context.Context, id string) (*models.UserAPIToken, error)
	// Revoke flips revoked_at to NOW(). Idempotent — re-revoking a
	// revoked token doesn't update the timestamp.
	Revoke(ctx context.Context, id string, at time.Time) error
	// BumpLastUsed updates last_used_at + last_used_ip. Called
	// async (fire-and-forget) from the auth middleware so it
	// never blocks the API request itself.
	BumpLastUsed(ctx context.Context, id, ip string, at time.Time) error
}

type userAPITokenRepo struct {
	db *gorm.DB
}

func NewUserAPITokenRepository(db *gorm.DB) UserAPITokenRepository {
	return &userAPITokenRepo{db: db}
}

func (r *userAPITokenRepo) Create(ctx context.Context, t *models.UserAPIToken) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *userAPITokenRepo) ListByUser(ctx context.Context, userID string) ([]models.UserAPIToken, error) {
	var rows []models.UserAPIToken
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *userAPITokenRepo) FindByHash(ctx context.Context, hashHex string) (*models.UserAPIToken, error) {
	var t models.UserAPIToken
	err := r.db.WithContext(ctx).
		Where("secret_hash = ? AND revoked_at IS NULL", hashHex).
		First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *userAPITokenRepo) FindByID(ctx context.Context, id string) (*models.UserAPIToken, error) {
	var t models.UserAPIToken
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *userAPITokenRepo) Revoke(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.UserAPIToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at.UTC()).Error
}

func (r *userAPITokenRepo) BumpLastUsed(ctx context.Context, id, ip string, at time.Time) error {
	upd := map[string]any{
		"last_used_at": at.UTC(),
	}
	if ip != "" {
		upd["last_used_ip"] = ip
	}
	return r.db.WithContext(ctx).
		Model(&models.UserAPIToken{}).
		Where("id = ?", id).
		Updates(upd).Error
}
