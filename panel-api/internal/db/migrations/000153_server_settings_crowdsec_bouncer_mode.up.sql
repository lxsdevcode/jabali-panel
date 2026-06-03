-- Server-wide CrowdSec nginx-bouncer mode. Two values:
--   live   — per-request LAPI lookup. Instant ban application but
--            constant SQLite cgo load (~23% one core on a small VM).
--   stream — bouncer caches the full decision set in nginx Lua
--            shared_dict and refreshes every UPDATE_FREQUENCY
--            (60s). Lookups become O(1) in-process; LAPI hits drop
--            to one poll/minute. Cuts crowdsec CPU to ~10% sustained
--            on the same load. Up to 60s lag for newly-issued bans;
--            firewall bouncer already polls at 60s so net new
--            exposure is L7-only and bounded.
--
-- Applied via agent verb security.crowdsec.bouncer.mode.apply which
-- rewrites MODE= in /etc/crowdsec/bouncers/crowdsec-nginx-bouncer.conf
-- and reloads nginx. Default 'stream' matches what install.sh ships
-- on fresh hosts after PR2026-06-03.
ALTER TABLE server_settings
  ADD COLUMN crowdsec_bouncer_mode ENUM('live','stream')
    NOT NULL DEFAULT 'stream'
  AFTER crowdsec_sensitivity;
