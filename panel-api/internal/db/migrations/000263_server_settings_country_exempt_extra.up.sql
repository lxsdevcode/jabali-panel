-- Supplemental CIDRs for the country ban exemption (ADR-0166 amendment
-- 2026-08-14). Operator-supplied IPs/CIDRs that must never be blocked,
-- merged into the jabali-country-allowlist LAPI AllowList tagged
-- "country:extra" so they are managed (removed when removed here).
-- TEXT for the same row-size reason as country_exempt_countries (000262).

ALTER TABLE server_settings
    ADD COLUMN country_exempt_extra_cidrs TEXT NOT NULL DEFAULT ('');
