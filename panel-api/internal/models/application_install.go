package models

import (
	"encoding/json"
	"time"
)

// ApplicationInstall is one installed app on a domain (or subdirectory of
// one). The (DomainID, Subdirectory, AppType) triplet is the install's
// identity on disk and is enforced unique by both the GORM tags below
// and migration 000046's composite unique key. WordPress is one
// AppType value among many; DokuWiki, MediaWiki, etc. share this
// table once their installer descriptors land in `internal/apps`.
type ApplicationInstall struct {
	ID     string `gorm:"type:char(26);primaryKey" json:"id"`
	UserID string `gorm:"type:char(26);not null"   json:"user_id"`

	// Composite unique on (domain_id, subdirectory, app_type). GORM
	// requires contiguous priorities (1, 2, 3) for a multi-column
	// uniqueIndex to materialise — get one wrong and the index silently
	// degrades to per-column uniques. Field declaration order matches
	// the pre-M19 model on purpose so GORM's INSERT column ordering
	// stays stable for the existing repository sqlmock tests; AppType
	// is appended last for the same reason.
	DomainID      string  `gorm:"type:char(26);not null;uniqueIndex:uniq_app_installs_domain_subdir_apptype,priority:1" json:"domain_id"`
	// DBID is nullable to support RequiresDB=false apps (DokuWiki,
	// Grav, Backdrop, ...). Migration 000048 relaxed the column;
	// callers writing this row must pass nil for flat-file apps.
	// Reads tolerate either NULL or "" — see DBIDValue() helper.
	DBID          *string `gorm:"type:char(26);column:db_id" json:"db_id"`
	Version       *string `gorm:"type:varchar(32)" json:"version"`
	AdminUsername string  `gorm:"type:varchar(60);not null" json:"admin_username"`
	AdminEmail    string  `gorm:"type:varchar(320);not null" json:"admin_email"`
	Locale        string  `gorm:"type:varchar(16);not null;default:'en_US'" json:"locale"`
	UseWWW        bool    `gorm:"type:boolean;not null;default:false" json:"use_www"`
	Subdirectory  string  `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uniq_app_installs_domain_subdir_apptype,priority:2" json:"subdirectory"`

	Status    string    `gorm:"type:varchar(16);not null;default:'pending'" json:"status"`
	LastError string    `gorm:"type:varchar(1024);not null;default:''" json:"last_error"`
	CreatedAt time.Time `gorm:"type:datetime(6);not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:datetime(6);not null" json:"updated_at"`

	// AppType picks which app's installer the agent runs. Default
	// 'wordpress' so existing rows back-fill without a code change in
	// callers that still pre-date M19. Validated against the registry
	// (`internal/apps`) at API boundaries. Declared LAST so the column
	// is appended to existing INSERT/SELECT orderings rather than
	// reshuffling them.
	AppType string `gorm:"type:varchar(32);not null;default:'wordpress';uniqueIndex:uniq_app_installs_domain_subdir_apptype,priority:3" json:"app_type"`
	// CacheEnabled (GH #406): jabali-cache plugin + nginx page cache toggle.
	CacheEnabled bool `gorm:"column:cache_enabled;type:tinyint(1);not null;default:0" json:"cache_enabled"`
	// CacheSettings (GH #612/#616/#618): per-install cache tuning as JSON —
	// object/page split, TTL, profile, URL exclusions, cookie bypass. NULL =
	// jabali defaults (existing rows need no backfill). Declared LAST so the
	// column appends to existing INSERT/SELECT orderings. Parse with
	// ParseCacheSettings; the zero CacheSettingsData is the default profile.
	CacheSettings json.RawMessage `gorm:"column:cache_settings;type:json" json:"cache_settings,omitempty"`
}

// CacheSettingsData is the decoded shape of ApplicationInstall.CacheSettings.
// Pointer bools distinguish "unset (inherit default)" from an explicit false.
// URLExclusions is the only field applied today (GH #616, nginx bypass); the
// object/page split + TTL are forward capacity for #612 (not yet consumed). A
// profile preset and a cookie bypass/allowlist were intentionally dropped — the
// built-in bypass set + fail-closed cookie gate already cover the site types,
// and a tenant cookie allowlist would re-open the #416/#419 fail-open class.
type CacheSettingsData struct {
	// URLExclusions are extra path prefixes that must always bypass the page
	// cache (GH #616), on top of the built-in wp-admin/cart/etc set.
	URLExclusions []string `json:"url_exclusions,omitempty"`
}

// ParseCacheSettings decodes the JSON column; a nil/empty/invalid value yields
// the zero CacheSettingsData (jabali defaults) with ok=false so callers can tell
// "never configured" from "configured to defaults".
func (a ApplicationInstall) ParseCacheSettings() (CacheSettingsData, bool) {
	var d CacheSettingsData
	if len(a.CacheSettings) == 0 {
		return d, false
	}
	if err := json.Unmarshal(a.CacheSettings, &d); err != nil {
		return CacheSettingsData{}, false
	}
	return d, true
}

// TableName pins the GORM mapping to the renamed table. Keep it explicit
// even though the default convention would resolve correctly — the name
// changed in 000046 and a typo here would silently route writes to the
// wrong (now non-existent) table.
func (ApplicationInstall) TableName() string { return "application_installs" }

// DBIDOr returns the string value of DBID, or "" when nil. Existing
// readers compare to "" (RequiresDB=false sentinel) and pass DBID as a
// string parameter; this accessor lets them keep doing both without
// every call site sprouting a nil-check after migration 000048 made
// the column nullable.
func (a *ApplicationInstall) DBIDOr() string {
	if a == nil || a.DBID == nil {
		return ""
	}
	return *a.DBID
}

// DBIDPtr converts a string DBID to the *string the model expects on
// write. "" maps to nil so RequiresDB=false apps insert NULL (the
// only value the FK accepts).
func DBIDPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// WordPressInstall is a transitional alias kept so the WordPress-specific
// API + agent paths compile during the M19 window without a sweeping
// rename. Step 3 introduces the generic /applications routes; subsequent
// steps replace the old WordPressInstall references one site at a time
// until M19.1 deletes this alias.
type WordPressInstall = ApplicationInstall
