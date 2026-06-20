-- M52 (ADR-0133): standalone shared resources (mailbox / calendar / address
-- book / file folder) shared selectively via Stalwart per-collection shareWith.
--
-- Model (validated on mx 2026-06-20): each shared resource is ONE collection
-- on its own host principal, so a member granted access gets only that resource
-- (selectivity is inherent — Stalwart "group membership grants ALL collections",
-- so we do NOT bundle resources on a group principal). Calendar/AddressBook/File
-- access is granted via shareWith; a shared mailbox's send-as comes from the
-- member being a member of that mailbox-only principal.
--
-- DB-as-truth: these rows are authoritative; the reconciler projects each
-- shared_resource into a Stalwart host principal + its single collection, and
-- each grant into that collection's shareWith map. host_account_id /
-- collection_id cache the projected Stalwart ids (filled by the reconciler;
-- empty until first projection).
--
-- Schema-only migration: NO data backfill here (migrations must not read
-- app-populated tables — the 000057/000054 dirty-DB scars). Backfill of the
-- M51 has_* groups into shared_resources lives in app code (a one-time
-- reconcile), not this file.

CREATE TABLE shared_resources (
  id              CHAR(26)     NOT NULL PRIMARY KEY,
  domain_id       CHAR(26)     NOT NULL,
  kind            ENUM('mailbox','calendar','addressbook','files') NOT NULL,
  -- local_part / email_cached are set only for kind='mailbox' (a shared
  -- mailbox is an SMTP recipient); NULL for the DAV-only kinds. NULL (not '')
  -- so the UNIQUE index permits many non-mailbox rows (MariaDB allows
  -- multiple NULLs in a UNIQUE key; multiple '' would collide).
  local_part      VARCHAR(64)  NULL,
  email_cached    VARCHAR(320) NULL,
  display_name    VARCHAR(255) NOT NULL DEFAULT '',
  -- Cached Stalwart projection ids (filled by the reconciler; empty until
  -- the host principal + collection are created).
  host_account_id VARCHAR(64)  NOT NULL DEFAULT '',
  collection_id   VARCHAR(64)  NOT NULL DEFAULT '',
  created_at      DATETIME(6)  NOT NULL,
  updated_at      DATETIME(6)  NOT NULL,
  UNIQUE KEY ux_shared_resources_email (email_cached),
  KEY ix_shared_resources_domain (domain_id),
  KEY ix_shared_resources_kind (kind),
  CONSTRAINT fk_shared_resources_domain
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Grants: which mailbox or group may access a shared resource, at what level.
-- grantee is polymorphic (mailbox or group) so it has no single FK; the
-- reconciler skips grants whose grantee no longer exists, and app code prunes
-- them on mailbox/group delete. rights is a coarse level the agent maps to the
-- per-resource Stalwart ACL keys (Mailbox/Calendar/AddressBook each differ).
CREATE TABLE shared_resource_grants (
  resource_id  CHAR(26)     NOT NULL,
  grantee_kind ENUM('mailbox','group') NOT NULL,
  grantee_id   CHAR(26)     NOT NULL,
  rights       ENUM('read','readwrite','admin') NOT NULL DEFAULT 'read',
  created_at   DATETIME(6)  NOT NULL,
  PRIMARY KEY (resource_id, grantee_kind, grantee_id),
  KEY ix_srg_grantee (grantee_kind, grantee_id),
  CONSTRAINT fk_srg_resource
    FOREIGN KEY (resource_id) REFERENCES shared_resources(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- M51 groups gain an explicit kind: 'distribution' (mail fan-out only, the
-- existing queryRecipient behaviour) vs 'resource' (a member bundle for
-- granting shared resources). Existing rows default to 'resource' to preserve
-- behaviour. The has_mailbox/has_calendar/has_addressbook/has_files columns are
-- DEPRECATED (stop being read once M52 ships) but kept one release for a clean
-- backfill — not dropped here.
ALTER TABLE mail_groups
  ADD COLUMN group_kind ENUM('distribution','resource')
    NOT NULL DEFAULT 'resource' AFTER description;
