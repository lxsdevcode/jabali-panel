package models

import "time"

// UserUIPref is a single server-side UI preference for one authenticated
// principal (admin or tenant). Keyed by (user_id, pref_key). See GH #218 —
// the welcome/quick-start modal dismissal moved here from localStorage so
// it survives a browser that clears storage on close.
type UserUIPref struct {
	UserID    string    `gorm:"column:user_id;type:char(26);primaryKey" json:"-"`
	PrefKey   string    `gorm:"column:pref_key;type:varchar(64);primaryKey" json:"key"`
	PrefValue string    `gorm:"column:pref_value;type:text;not null" json:"value"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
}

func (UserUIPref) TableName() string { return "user_ui_prefs" }
