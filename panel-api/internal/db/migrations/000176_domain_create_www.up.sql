-- GH #225: per-domain opt-in for the bootstrap `www` CNAME.
-- DEFAULT 1 preserves the historic always-create-www behaviour for any
-- non-API insert (e.g. docker-app auto-created rows); the tenant/admin
-- create handler always sets this column explicitly from the form
-- checkbox (unchecked => 0 => no www record).
ALTER TABLE domains
  ADD COLUMN create_www TINYINT(1) NOT NULL DEFAULT 1;
