# ADR-0123: Per-user mail message backup + restore via JMAP Maildir export/import

**Date**: 2026-06-14
**Status**: proposed
**Amends**: ADR-0122 (messages were "manual bodies.tar only")
**Deciders**: shukiv

## Context

ADR-0122 settled that mail *accounts* restore via the panel-DB path (Bug
B) and left *messages* on the manual `bodies.tar` path. `bodies.tar` is a
full snapshot of `/var/lib/stalwart` (one RocksDB store shared by ALL
accounts). Restoring it means `stop stalwart → tar -xf -C / → start`,
which **overwrites every account's mail** and the directory state — a
multi-tenant wipe, the same blast-radius problem as the account apply. So
messages effectively do not restore per-user.

Operators want historical messages restored on a per-account restore.

Two existing mechanisms make this tractable without inventing a format:

- **`migration.import_mailboxes`** (M35 cPanel importer) already pushes a
  Maildir tree into a Stalwart account via JMAP `Blob/upload` +
  `Email/import`: resolves INBOX by role, recreates Maildir++ subfolders
  (`.Sent`/`.Drafts`/`.Junk`/`.Trash` + custom) via idempotent
  `ensureMailbox`, per-message import. **Stalwart dedupes on Message-ID**,
  so re-import is a silent no-op — idempotent over both a fresh DR account
  and an existing one.
- The agent already has the JMAP plumbing (`jmapCall`, `/jmap/upload`,
  admin creds) to talk to Stalwart on the loopback admin port.

The import (restore) half is therefore already built and folder-faithful.
The only missing piece is a per-user **export** (Stalwart → Maildir) at
backup time.

## Decision

Replace the whole-store `bodies.tar` mail artifact with a **per-user
Maildir export** captured via JMAP at backup time, and restore it by
feeding that Maildir to the existing `migration.import_mailboxes`.

- **Backup** (`backup.mailboxes`): for each of the user's mailboxes,
  enumerate folders (`Mailbox/get`), enumerate messages per folder
  (`Email/query`, paginated), fetch each message's `blobId` (`Email/get`)
  and download the RFC822 bytes (`GET /jmap/download/<accountId>/<blobId>`),
  and write a cpanel-style Maildir tree
  `mail/<domain>/<localpart>/{cur,new,.Subfolder/...}` — the exact layout
  `migration.import_mailboxes` consumes. `\Seen` → `cur/`, unseen →
  `new/`; `receivedAt` preserved in the Maildir filename. Snapshot that
  tree (restic, stage=mail) instead of `bodies.tar`.
- **Restore**: materialize the stage=mail Maildir, then call
  `migration.import_mailboxes` with `src_mail_dir` = the materialized
  tree (+ `owner_email`). Message-ID dedup makes it safe + idempotent.
- **Drop `bodies.tar`** and the manual stop-untar warning. The mail
  account config (principals) still comes from the DB path (ADR-0122);
  this ADR adds only message bodies, per-user and multi-tenant-safe.

## Alternatives Considered

### Keep `bodies.tar`, restore it whole
- **Why not**: multi-tenant wipe; not per-user; already rejected.

### imapsync Stalwart-loopback → external, on restore
- **Pros**: reuses `migration.imapsync`.
- **Cons**: imapsync is IMAP↔IMAP (needs a live source server at restore;
  the backup is at-rest files, not a server). Operator-installed dep.
- **Why not**: doesn't fit at-rest backup artifacts; JMAP export+import
  needs no extra binary.

### Per-message rows in a DB table
- **Why not**: messages are large + numerous; restic-stored Maildir is
  the right at-rest format, and import already speaks Maildir.

## Consequences

### Positive
- Messages restore per-user, multi-tenant-safe, folder-faithful,
  idempotent (Message-ID dedup) — reusing the proven import path.
- `bodies.tar` (whole-store, unsafe, large) is gone.

### Negative / Risks
- Backup cost: per-message JMAP download is slower + larger than a single
  RocksDB tar. Mitigation: 4h stage timeout (as the importer), paginate
  `Email/query`, stream blobs to disk, cap per-message size (64 MiB, as
  the importer).
- JMAP export must map keywords→Maildir flags + folder roles correctly
  (inverse of the importer). Mitigation: golden round-trip test
  (export → import → compare counts/flags) on the live VM.
- Existing backups hold `bodies.tar`, not Maildir. Restore must detect
  which artifact a snapshot carries and handle the legacy one (warn:
  "legacy bodies.tar — messages need manual restore"). New backups use
  Maildir.
