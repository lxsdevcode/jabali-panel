-- GH #259: cache the domain's registrar expiry (from a scheduled WHOIS fetch)
-- so the DNS Zones list can show an Expiration column without an on-demand
-- whois per row. registrar_checked_at gates the refresh cadence.
ALTER TABLE domains
  ADD COLUMN registrar_expires_at DATETIME(6) NULL,
  ADD COLUMN registrar_checked_at DATETIME(6) NULL;
