-- GH #612/#616/#618: per-install cache settings (object/page split, TTL,
-- profile, URL exclusions, cookie bypass). One additive nullable JSON column;
-- NULL = "use jabali defaults" so existing rows need no backfill. MariaDB JSON
-- is LONGTEXT+CHECK — safe, no reserved words, no data restructure.
ALTER TABLE application_installs ADD COLUMN cache_settings JSON NULL;
