-- DNS record-type permissions for tenants (GH #466, ADR-0150). Server-wide
-- matrix of which record types non-admin users may create/edit/delete on their
-- own zones, stored as a JSON object on the singleton server_settings row.
-- Default = permissive (every user-manageable type fully allowed) so the
-- upgrade preserves today's behavior; admins opt into restriction via presets.
ALTER TABLE server_settings
  ADD COLUMN dns_user_record_policy LONGTEXT NOT NULL
  DEFAULT '{"A":{"create":true,"edit":true,"delete":true},"AAAA":{"create":true,"edit":true,"delete":true},"CNAME":{"create":true,"edit":true,"delete":true},"MX":{"create":true,"edit":true,"delete":true},"TXT":{"create":true,"edit":true,"delete":true},"SRV":{"create":true,"edit":true,"delete":true},"CAA":{"create":true,"edit":true,"delete":true}}';
