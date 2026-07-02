package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// CacheTokenSaltRepository owns the per-tenant WP-cache token salt (Gitea #415).
type CacheTokenSaltRepository interface {
	// GetOrCreate returns the user's salt, generating + persisting a fresh
	// random 32-byte (64-hex) salt on first call.
	GetOrCreate(ctx context.Context, userID string) (string, error)
	// Rotate generates a new salt for the user (next cache-enable re-derives a
	// fresh token), invalidating the old one without touching other tenants.
	Rotate(ctx context.Context, userID string) (string, error)
}

type cacheTokenSaltRepo struct{ db *gorm.DB }

func NewCacheTokenSaltRepository(db *gorm.DB) CacheTokenSaltRepository {
	return &cacheTokenSaltRepo{db: db}
}

func newSalt() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (r *cacheTokenSaltRepo) GetOrCreate(ctx context.Context, userID string) (string, error) {
	var row models.CacheTokenSalt
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error
	if err == nil {
		return row.Salt, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	salt, gerr := newSalt()
	if gerr != nil {
		return "", gerr
	}
	now := time.Now()
	row = models.CacheTokenSalt{UserID: userID, Salt: salt, CreatedAt: now, UpdatedAt: now}
	// Race-safe insert: if another request created it first, read it back.
	if cerr := r.db.WithContext(ctx).Create(&row).Error; cerr != nil {
		var existing models.CacheTokenSalt
		if rerr := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&existing).Error; rerr == nil {
			return existing.Salt, nil
		}
		return "", cerr
	}
	return salt, nil
}

func (r *cacheTokenSaltRepo) Rotate(ctx context.Context, userID string) (string, error) {
	salt, err := newSalt()
	if err != nil {
		return "", err
	}
	now := time.Now()
	row := models.CacheTokenSalt{UserID: userID, Salt: salt, CreatedAt: now, UpdatedAt: now}
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		return "", err
	}
	return salt, nil
}
