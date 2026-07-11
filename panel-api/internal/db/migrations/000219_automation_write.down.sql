DROP TABLE IF EXISTS automation_operations;
ALTER TABLE automation_tokens
  DROP COLUMN writes_enabled,
  DROP COLUMN ip_allowlist_json,
  DROP COLUMN expires_at;
