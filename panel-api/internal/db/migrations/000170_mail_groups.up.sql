-- M51: mailbox user groups with shared resources (issue #201, ADR-0132).
--
-- A mail group is a domain-scoped principal that owns shared resources
-- (mailbox / calendar / address book / file folder). Members are
-- mailboxes; membership grants every member access to the group's
-- resources in their own JMAP session (Stalwart-native sharing —
-- verified live, ADR-0132).
--
-- DB-as-truth: these rows are authoritative. The reconciler projects each
-- group into the Stalwart registry (x:Account/set @type:Group) and each
-- membership edge into the member account's memberGroupIds. Stalwart's SQL
-- directory only resolves the group as a *recipient* (queryRecipient
-- extension); queryMemberOf stays null (membership lives in the registry,
-- durable across re-auth — ADR-0132).
--
-- email_cached mirrors the mailboxes table: a denormalised
-- local_part + '@' + domains.name kept in sync by BEFORE INSERT/UPDATE
-- triggers, so SMTP recipient resolution can match it directly.

CREATE TABLE mail_groups (
  id              CHAR(26)        NOT NULL PRIMARY KEY,
  domain_id       CHAR(26)        NOT NULL,
  local_part      VARCHAR(64)     NOT NULL,
  email_cached    VARCHAR(320)    NOT NULL,
  display_name    VARCHAR(255)    NOT NULL DEFAULT '',
  description     VARCHAR(255)    NOT NULL DEFAULT '',
  has_mailbox     TINYINT(1)      NOT NULL DEFAULT 1,
  has_calendar    TINYINT(1)      NOT NULL DEFAULT 1,
  has_addressbook TINYINT(1)      NOT NULL DEFAULT 1,
  has_files       TINYINT(1)      NOT NULL DEFAULT 1,
  created_at      DATETIME(6)     NOT NULL,
  updated_at      DATETIME(6)     NOT NULL,
  UNIQUE KEY ux_mail_groups_email_cached (email_cached),
  KEY ix_mail_groups_domain (domain_id),
  CONSTRAINT fk_mail_groups_domain
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- email_cached sync — same pattern as mailboxes (no subquery generated
-- columns on MariaDB; triggers are the portable path).

CREATE TRIGGER trg_mail_groups_before_insert
  BEFORE INSERT ON mail_groups
  FOR EACH ROW
    SET NEW.email_cached = CONCAT(
      NEW.local_part, '@',
      (SELECT name FROM domains WHERE id = NEW.domain_id)
    );

CREATE TRIGGER trg_mail_groups_before_update
  BEFORE UPDATE ON mail_groups
  FOR EACH ROW
    SET NEW.email_cached = IF(
      NEW.local_part <> OLD.local_part OR NEW.domain_id <> OLD.domain_id,
      CONCAT(NEW.local_part, '@', (SELECT name FROM domains WHERE id = NEW.domain_id)),
      NEW.email_cached
    );

CREATE TRIGGER trg_domains_after_update_resync_mail_groups
  AFTER UPDATE ON domains
  FOR EACH ROW
    UPDATE mail_groups
      SET email_cached = CONCAT(local_part, '@', NEW.name)
      WHERE domain_id = NEW.id
        AND NEW.name <> OLD.name;

-- Membership: which mailboxes belong to which group. Both FKs cascade so
-- deleting a group or a mailbox cleans up edges; the reconciler then
-- converges the registry memberGroupIds.

CREATE TABLE mail_group_members (
  group_id   CHAR(26)     NOT NULL,
  mailbox_id CHAR(26)     NOT NULL,
  created_at DATETIME(6)  NOT NULL,
  PRIMARY KEY (group_id, mailbox_id),
  KEY ix_mail_group_members_mailbox (mailbox_id),
  CONSTRAINT fk_mail_group_members_group
    FOREIGN KEY (group_id) REFERENCES mail_groups(id) ON DELETE CASCADE,
  CONSTRAINT fk_mail_group_members_mailbox
    FOREIGN KEY (mailbox_id) REFERENCES mailboxes(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
