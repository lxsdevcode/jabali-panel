-- "Look and feel": more operator-customizable panel colors (empty = default).
ALTER TABLE server_settings
  ADD COLUMN panel_success_color VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_warning_color VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_error_color   VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_info_color    VARCHAR(9) NOT NULL DEFAULT '',
  ADD COLUMN panel_link_color    VARCHAR(9) NOT NULL DEFAULT '';
