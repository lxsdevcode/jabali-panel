-- Country ban exemption (ADR-0166). Countries whose IPs must never be
-- blocked by CrowdSec from any decision source. Applied two ways by the
-- agent: a GeoIP parser whitelist at
-- /etc/crowdsec/parsers/s02-enrich/zz-jabali-country-allowlist.yaml
-- (verb security.crowdsec.country_exempt.set) and the LAPI AllowList
-- jabali-country-allowlist seeded with per-country CIDR sets (verb
-- security.crowdsec.country_allowlist.sync, fed by panel-api's
-- internal/countryexempt syncer). DB is the source of truth;
-- empty = feature off.

-- TEXT (not VARCHAR) on purpose: server_settings is close to InnoDB's
-- 65535-byte row-size ceiling — a VARCHAR(1000) trips Error 1118 on
-- hosts carrying the full column set (hit on testserver 2026-08-13).

ALTER TABLE server_settings
    ADD COLUMN country_exempt_countries TEXT NOT NULL DEFAULT ('');
