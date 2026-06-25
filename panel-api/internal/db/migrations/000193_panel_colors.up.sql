-- "Look and feel": operator-customizable panel colors (server-wide singleton).
-- Empty string = use the built-in default for that color.
ALTER TABLE server_settings
  ADD COLUMN panel_primary_color VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_accent_color  VARCHAR(9) NOT NULL DEFAULT '';
