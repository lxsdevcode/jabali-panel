-- M48 auto-update poller: track the upstream digest the panel last
-- observed so the UI can show "update available" and the reconciler
-- can act on it when update_mode='auto'. last_check_at gates the
-- per-app poll cadence (default ~6h between checks).
ALTER TABLE docker_apps
  ADD COLUMN available_digest VARCHAR(128) NULL,
  ADD COLUMN last_check_at    TIMESTAMP NULL;
