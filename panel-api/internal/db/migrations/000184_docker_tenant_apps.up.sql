-- Admin-curated subset of tenant-installable Docker apps exposed to users.
-- CSV of catalog slugs; empty = expose ALL tenant-eligible apps (the curated
-- default). Lets the admin narrow which Docker apps tenants can install.
ALTER TABLE server_settings
  ADD COLUMN docker_tenant_apps VARCHAR(4096) NOT NULL DEFAULT '';
