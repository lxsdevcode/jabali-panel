ALTER TABLE dns_records MODIFY COLUMN managed_by VARCHAR(16) NULL;
ALTER TABLE domains
  DROP COLUMN mail_provider,
  DROP COLUMN m365_onmicrosoft,
  DROP COLUMN google_dkim;
