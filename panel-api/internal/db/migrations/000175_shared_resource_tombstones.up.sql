-- M52 (ADR-0133): durable teardown for shared resources. When a shared
-- resource is deleted, the panel records a tombstone of its host address; the
-- reconciler destroys the Stalwart host principal and clears the tombstone.
-- This closes the orphan gap where an agent-down delete would leave the host
-- behind (the REST/CLI synchronous destroy was best-effort only).
CREATE TABLE shared_resource_tombstones (
  email      VARCHAR(320) NOT NULL PRIMARY KEY,
  created_at DATETIME(6)  NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
