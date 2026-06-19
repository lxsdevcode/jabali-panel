-- Server-side per-user UI preferences (GH #218). Replaces client-side
-- localStorage for things like the "don't show the welcome modal again"
-- flag, which was lost when a browser cleared storage on close. Keyed by
-- the authenticated principal's id (works for both admin and tenant
-- users); no FK so it tolerates any claims subject and never blocks a
-- user delete (orphan prefs are negligible).
CREATE TABLE user_ui_prefs (
  user_id    CHAR(26)    NOT NULL,
  pref_key   VARCHAR(64) NOT NULL,
  pref_value TEXT        NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (user_id, pref_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
