package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// UserUIPrefRepository stores server-side per-user UI preferences (GH #218).
type UserUIPrefRepository interface {
	// GetAll returns every preference for a principal as a key→value map
	// (empty map when none).
	GetAll(ctx context.Context, userID string) (map[string]string, error)
	// Set upserts one preference value.
	Set(ctx context.Context, userID, key, value string) error
}

type userUIPrefRepo struct{ db *gorm.DB }

func NewUserUIPrefRepository(db *gorm.DB) UserUIPrefRepository {
	return &userUIPrefRepo{db: db}
}

func (r *userUIPrefRepo) GetAll(ctx context.Context, userID string) (map[string]string, error) {
	var rows []models.UserUIPref
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, p := range rows {
		out[p.PrefKey] = p.PrefValue
	}
	return out, nil
}

func (r *userUIPrefRepo) Set(ctx context.Context, userID, key, value string) error {
	row := models.UserUIPref{
		UserID:    userID,
		PrefKey:   key,
		PrefValue: value,
		UpdatedAt: time.Now().UTC(),
	}
	// Upsert on the (user_id, pref_key) primary key.
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "pref_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"pref_value", "updated_at"}),
	}).Create(&row).Error
}
