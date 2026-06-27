-- "Look and feel": per-theme panel CHROME colors (GH #435). Unlike the
-- single-value brand/semantic colors (000193+), chrome colors need separate
-- light + dark values because one custom bg/text can't read in both themes.
-- Empty string = use the built-in default for that color+mode.
ALTER TABLE server_settings
  ADD COLUMN panel_light_topbar_color    VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_dark_topbar_color     VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_light_bg_color        VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_dark_bg_color         VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_light_container_color VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_dark_container_color  VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_light_text_color      VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_dark_text_color       VARCHAR(9) NOT NULL DEFAULT '';
