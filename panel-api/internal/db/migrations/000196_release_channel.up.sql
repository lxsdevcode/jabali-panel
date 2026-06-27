-- Release channel (GH #445): "stable" tracks a promoted, reviewed build
-- (the movable `stable` git tag); "development" tracks main (today's behavior).
-- Default stable = conservative (a stable host only moves to promoted builds;
-- with no stable tag yet it simply reports up-to-date).
ALTER TABLE server_settings
  ADD COLUMN release_channel VARCHAR(16) NOT NULL DEFAULT 'stable';
