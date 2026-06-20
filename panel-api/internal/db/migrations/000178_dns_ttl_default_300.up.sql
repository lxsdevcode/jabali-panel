-- GH #244: lower the default DNS record TTL from 3600s (1h) to 300s (5m).
-- New installs get 300 via the column default; move installs still on the
-- old seeded default (3600) to 300 so the change applies to them too.
-- Operators who want a longer TTL can set it in Server Settings → DNS.
ALTER TABLE server_settings ALTER COLUMN default_dns_ttl SET DEFAULT 300;
UPDATE server_settings SET default_dns_ttl = 300 WHERE default_dns_ttl = 3600;
