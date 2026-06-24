-- #433: per-panel font-size branding option (small|medium|large).
-- Server-wide singleton setting, matches the rest of M28 branding.
ALTER TABLE server_settings
  ADD COLUMN panel_font_size VARCHAR(10) NOT NULL DEFAULT 'medium';
