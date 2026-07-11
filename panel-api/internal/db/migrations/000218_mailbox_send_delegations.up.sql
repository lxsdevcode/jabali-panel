-- GH #347 (send-as delegation): a delegate mailbox may send FROM a grantor
-- mailbox's address WITHOUT receiving the grantor's mail. These rows are the
-- authoritative delegation set (DB-as-truth, ADR-0042). The reconciler/converger
-- materialises them into Stalwart's MtaStageAuth.mustMatchSender expression
-- (Mechanism C, project_jab347_spike_verdict): mustMatchSender = false ONLY for a
-- verified (delegate, grantor) pair, true otherwise — so non-delegated mailboxes
-- keep the built-in anti-spoofing intact.
--
-- Both endpoints are mailbox FKs (not raw emails) so the delegation tracks
-- renames, guarantees each side is a real mailbox, and cascades on mailbox
-- delete. queryRecipient is UNCHANGED — the grantor keeps receiving its own mail.
-- Schema only (migration-data-seed-ordering scar); no seed rows.
CREATE TABLE mailbox_send_delegations (
  id                  CHAR(26)    NOT NULL PRIMARY KEY,
  delegate_mailbox_id CHAR(26)    NOT NULL,
  grantor_mailbox_id  CHAR(26)    NOT NULL,
  created_at          DATETIME(6) NOT NULL,
  updated_at          DATETIME(6) NOT NULL,
  UNIQUE KEY uq_mailbox_send_deleg (delegate_mailbox_id, grantor_mailbox_id),
  KEY ix_msd_delegate (delegate_mailbox_id),
  KEY ix_msd_grantor (grantor_mailbox_id),
  CONSTRAINT fk_msd_delegate
    FOREIGN KEY (delegate_mailbox_id) REFERENCES mailboxes(id) ON DELETE CASCADE,
  CONSTRAINT fk_msd_grantor
    FOREIGN KEY (grantor_mailbox_id) REFERENCES mailboxes(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
