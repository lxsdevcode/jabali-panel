ALTER TABLE server_settings ALTER COLUMN default_dns_ttl SET DEFAULT 3600;
UPDATE server_settings SET default_dns_ttl = 3600 WHERE default_dns_ttl = 300;
