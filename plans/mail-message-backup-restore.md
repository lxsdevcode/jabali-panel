# Blueprint: Per-user mail message backup + restore (JMAP Maildir)

**Status**: proposed (ADR-0123, amends ADR-0122)
**Owner**: shukiv
**Goal**: historical mail messages restore on a per-account restore,
multi-tenant-safe, reusing the existing import path.

## Mechanism (recap)

- Restore-import is DONE: `migration.import_mailboxes` (JMAP
  Blob/upload + Email/import, Maildir++ folders, Message-ID dedup =
  idempotent). Consumes a cpanel-style Maildir tree
  `mail/<domain>/<localpart>/{cur,new,.Subfolder/...}`.
- Missing: backup-side export Stalwart → that Maildir tree.
- Net: replace whole-store `bodies.tar` with per-user Maildir.

## Waves

### Wave A — JMAP Maildir exporter (agent, pure-ish, tested) — DISPATCHABLE
New `panel-agent/internal/commands/mailbox_export.go`:
- `exportMailboxToMaildir(ctx, accountEmail, destRoot string) (msgs int64, bytes int64, err error)`:
  1. Resolve accountId for the email (reuse the JMAP account lookup the
     importer/registry use).
  2. `Mailbox/get` (accountId) → folders: id, name, role, parentId.
  3. Map each folder to a Maildir path:
     - role=inbox → `mail/<domain>/<local>/` (cur,new)
     - role in {sent,drafts,junk,trash,archive} → `.Sent`/`.Drafts`/…
       (inverse of the importer's `maildirSubfolderRole`)
     - nested/custom → Maildir++ `.Parent.Child`
  4. Per folder: `Email/query` (filter inMailbox, paginate by position)
     → ids; `Email/get` (props: blobId, keywords, receivedAt) per page.
  5. Per message: `GET /jmap/download/<accountId>/<blobId>` → stream to
     `cur/<receivedAt>.<uid>.<host>:2,<flags>` (`\Seen`→`S` in cur/;
     unseen → `new/`). Map keywords→Maildir flags (S,R,F,T,D).
  6. Cap per-message at 64 MiB; skip + record oversize, don't fail the
     mailbox.
- Reuse `jmapCall`, `stalwartAdminCreds`, `/jmap` base URL helpers.
- Unit: fake JMAP server (httptest) → assert folder/flag mapping +
  Maildir filenames. (Mirror `mailbox_jmap_test.go` style.)

Acceptance: export a 2-folder account (INBOX + Sent, 1 seen + 1 unseen)
→ correct Maildir tree, flags in filenames, byte counts match.

### Wave B — wire export into backup.mailboxes — needs Wave A
`panel-agent/internal/commands/backup_mailboxes.go`:
- Replace `writeMailBodiesTarball` with a per-user Maildir export loop:
  for each `req.Mailboxes`, `exportMailboxToMaildir(email, stagingDir/mail)`.
- Keep `plan.json` (harmless; account config) OR drop it (ADR-0122 says
  apply is abandoned — drop plan.json too to slim the snapshot). Decision:
  drop plan.json + bodies.tar; stage only the Maildir tree.
- Restic-snapshot the Maildir tree, stage=mail, per-mailbox tags as today.
- Stalwart-down / cli-missing skip semantics unchanged.

Acceptance (box): back up demotenant (with a planted message) → stage=mail
snapshot contains `mail/demotenant.com/<local>/{cur,new,...}` with the
message, no bodies.tar.

### Wave C — wire restore to import_mailboxes — needs Wave B
`panel-agent/internal/commands/backup_restore.go` StageMail:
- Re-add the nested-path glob (reverted in #376) to find the materialized
  `mail/` tree under `stagingRoot/mail/run/jabali-backup/*/mail/`.
- Detect artifact: if a `mail/<domain>/...` Maildir tree exists → call the
  `migration.import_mailboxes` logic (factor its core into a shared func
  `importMaildirTree(ctx, srcMailDir, ownerEmail)`), passing
  `src_mail_dir` = the materialized tree. If only legacy `bodies.tar`
  exists → warn "legacy artifact, manual restore" (back-compat).
- Drop the old `plan.json missing — skip` + bodies.tar manual warning.

Acceptance (box, the real test):
- Plant a message in sales@demotenant.com (SMTP/JMAP). Back up.
- Delete the mailbox (row + JMAP), recreate via Bug B restore (rows).
- Restore stage=mail → message reappears in sales@ INBOX (IMAP/JMAP
  count = 1). Folder fidelity: a Sent message lands in Sent.
- Re-run restore → still 1 (Message-ID dedup, no dupes).
- Multi-tenant: a 2nd tenant's messages untouched throughout.

### Wave D — cleanup / docs
- Remove `writeMailBodiesTarball` + the bodies.tar restore branch once
  Wave C lands and legacy back-compat window is closed (or keep the
  legacy-detect warning indefinitely — cheap).
- Runbook: per-user message restore is now automatic; note the legacy
  bodies.tar manual path for pre-ADR-0123 snapshots.

## Validation matrix

| Risk | Guard |
|------|-------|
| Multi-tenant wipe | export + import are per-account JMAP; no store swap; multi-tenant box test asserts other tenant intact |
| Dupes on re-restore | Stalwart Message-ID dedup (verified in importer); box test re-runs restore |
| Flag/folder loss | inverse-map test + round-trip box test (Sent stays Sent, Seen stays Seen) |
| Large mailbox / OOM | paginate Email/query; stream blobs to disk; 64 MiB cap; 4h timeout |
| Legacy bodies.tar snapshots | restore detects artifact, warns, doesn't crash |

## Files
- `panel-agent/internal/commands/mailbox_export.go` (NEW, Wave A)
- `panel-agent/internal/commands/mailbox_export_test.go` (NEW, Wave A)
- `panel-agent/internal/commands/backup_mailboxes.go` (Wave B)
- `panel-agent/internal/commands/migration_import_mailboxes.go` (extract
  shared `importMaildirTree`, Wave C)
- `panel-agent/internal/commands/backup_restore.go` (Wave C)
- `docs/adr/0123-per-user-mail-message-backup-restore.md` (done)

## Out of scope
- Sieve scripts / JMAP per-mailbox settings (still not backed up).
- `uid_at_source` (separate).
