-- Country ban exemption (ADR-0166). Countries whose IPs must never be
-- blocked by CrowdSec from any decision source. Applied two ways by the
-- agent: a GeoIP parser whitelist at
-- /etc/crowdsec/parsers/s02-enrich/zz-jabali-country-allowlist.yaml
-- (verb security.crowdsec.country_exempt.set) and the LAPI AllowList
-- jabali-country-allowlist seeded with per-country CIDR sets (verb
-- security.crowdsec.country_allowlist.sync, fed by panel-api's
-- internal/countryexempt syncer). DB is the source of truth;
-- empty = feature off.

ALTER TABLE server_settings
    ADD COLUMN country_exempt_countries VARCHAR(1000) NOT NULL DEFAULT '';
ALTER TABLE server_settings
    DROP COLUMN country_exempt_countries;
