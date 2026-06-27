-- Migration source SSH host-key fingerprint (GH #461, ADR-0151). Optional
-- admin-supplied SHA256 fingerprint; when set, the connector verifies the
-- source host key against it before sending any credential. Empty = TOFU.
ALTER TABLE migration_jobs
  ADD COLUMN expected_host_key VARCHAR(128) NOT NULL DEFAULT '';
