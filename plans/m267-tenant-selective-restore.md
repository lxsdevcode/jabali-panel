# M267 — Tenant Self-Service Selective Restore

**Issue:** GH #267 ("No Tenant Restore Functionality under Tenant Panel")
**Status:** Wave 1 SHIPPED (read-only manifest preview, `c31bf675`); Waves 2–5 pending
**Target ADR:** ADR-0148
**Depends on:** M30/M30.1 backup foundation, ADR-0075/0078/0080; #245 RBAC scopes (ADR-0144)

## Context

Admins can restore a whole account (`POST /backups/restore`, `Kind=account_restore`,
`backup.restore` agent command). Tenants have **no restore path at all** — the
`/jabali-panel/backups` page only creates + downloads. The reporter wants
self-service restore of **domains, databases, whole mail-domains, individual
mailboxes, and DNS** — i.e. *selective*, not just whole-account.

Two gaps:
1. **No tenant-scoped restore** — restore is admin-only and takes an arbitrary
   `target_user_id`.
2. **No selectivity** — `backup.restore` applies every stage (home + all DBs +
   all mailboxes); there is no "just this database" or "just this mailbox".

The backup format already supports selectivity for **home / databases /
mailboxes** (this is the load-bearing fact): a backup is a **manifest snapshot**
plus **per-stage restic snapshots** (`stage=home`, `stage=db`, `stage=mail`,
`stage=meta`), each with its own `SnapshotID` and an `Items[]` list (dbs[],
mailboxes[]). Restore reads the manifest, then resolves stage snapshots by tag.
So selective restore of those three = "apply a subset of stages / items", not a
new capture format.

**DNS is the exception — verified, not assumed.** The `meta` stage
(`MetadataDomain`) captures domain *config* (docroot, nginx/PHP, SSL flags,
email/DKIM, DNSSEC flags + keys, mailboxes, forwarders) but **not the
`dns_records` rows** (the A/MX/TXT/CNAME content). There is no DNS *data* stage
(`home, db×N, mail, meta, manifest` — confirmed on a live snapshot). Consequence:
jabali's *managed* records (BootstrapRecords + mail records) re-derive
automatically from restored domain config via the reconciler, but a tenant's
**hand-added custom DNS records are unrecoverable** from any existing backup.
See "DNS caveat" + Phase 0.

## Goals

- A tenant restores **into their own account only** — `target_user_id` is forced
  to `claims.UserID`, never client-supplied.
- A **preview**: given one of the caller's own succeeded backup jobs, return its
  manifest's stages + items so the UI can show a restore picker.
- **Selective restore**: caller chooses any subset of
  `{home, database:<name>, mail-domain:<domain>, mailbox:<addr>, dns:<domain>}`.
- Restore runs as a tracked `BackupJob` (`Kind=account_restore`) with status, so
  the existing jobs list + notifications cover it.
- **Overwrite is explicit** and per-resource; default is fail-if-exists.

## Non-goals (v1)

- Cross-user / cross-host restore (admin-only, already exists for whole-account).
- Point-in-time / partial-file restore inside a home dir (whole-home only).
- Restoring resources the tenant no longer owns (a DB/domain deleted since the
  backup) without re-provisioning — v1 restores data into an **existing** owned
  resource; "recreate the domain then restore" is a follow-up.
- Restoring DNS *records the tenant edited by hand* without overwrite — gated
  behind the same explicit overwrite flag.

## Architecture

### Existing (reuse)
- `internal/backup/manifest.go` — `AccountManifest{Stages[]ManifestStage}`,
  each stage `{Name, Tag, SnapshotID, Items[]}`.
- `panel-agent` `backup.restore` — whole-account apply.
- `panel-api` `backupHandler.restore` — admin whole-account.
- `backup.materialize` — already restores a snapshot to a dir (used by download);
  the selective restorer can reuse the same materialize→apply split.

### New

**1. Manifest preview (read-only).**
`GET /api/v1/me/backups/:id/manifest` →
- owner-check (job.UserID == claims.UserID), job.Status==succeeded.
- new agent command `backup.manifest_read {manifest_snapshot_id}` → restic
  `dump` of the manifest snapshot → return parsed `{stages:[{name,items,status}]}`.
- panel filters to the **caller's currently-owned** resources (so a tenant can't
  see/restore another tenant's data even if a stale manifest names it — defence
  in depth on top of the owner-check).

**2. Selective restore (mutating).**
`POST /api/v1/me/backups/:id/restore` body:
```json
{
  "selection": {
    "home": false,
    "databases": ["sampleco_wp"],
    "mail_domains": ["example.com"],
    "mailboxes": ["info@example.com"],
    "dns_domains": ["example.com"]
  },
  "overwrite": false
}
```
- owner-check + force `target_user_id = claims.UserID`.
- validate every selected item against the caller's **owned** resources AND the
  manifest's items (reject anything in neither — fail-closed).
- create a `BackupJob{Kind:account_restore, UserID:caller}` → dispatch new agent
  command `backup.restore_selective` with the resolved per-stage snapshot IDs +
  the item subset + overwrite.

**3. Agent `backup.restore_selective`.**
- params: `{job_id, target_username, manifest_snapshot_id, stages:{home?, db_snapshot+dbs[], mail_snapshot+mailboxes[], dns: zones[]}, overwrite}`.
- materialize only the needed stage snapshots; apply per kind:
  - **home** → rsync into `/home/<u>` (overwrite-gated); chown <u>.
  - **database:<name>** → load that one dump into the existing DB (must already
    exist + be owned; overwrite = drop+recreate tables, else fail if non-empty).
  - **mailbox:<addr>** → Stalwart import for that mailbox only (reuse the
    per-mailbox restore the whole-account path already shells).
  - **dns:<domain>** → see the DNS caveat below. v1 restores the **domain
    config** (which re-derives the *managed* records via the reconciler); custom
    user records are NOT in current backups.
- per-item status back into the job; partial failures are reported, not fatal.

## Security (CRITICAL)

- **target_user_id is server-derived** (`claims.UserID`), never from the body.
  A tenant can only restore into themselves. Admin keeps the existing arbitrary-
  target whole-account path.
- **Double ownership check**: the selected DB/domain/mailbox must be (a) listed
  in the manifest AND (b) currently owned by the caller. Reject otherwise — a
  stale manifest must never be a lever to write into another account.
- **RBAC scope** (#245/ADR-0144): the new routes map to `write:backups`
  (restore mutates) under the existing `EnforceUserTokenScopes` resolver — add
  `/api/v1/me/backups/:id/restore` + `/manifest` to `userScopePrefixes`
  (`backups` area). A scoped API token without `write:backups` is 403'd.
- **Overwrite is explicit + per-run**; never default-true. Destructive applies
  (DB drop, home overwrite, DNS replace) require `overwrite:true` AND the item to
  already exist.
- **Rate-limit** restore the same as backup-create (one running restore per user;
  reject concurrent).

## DNS caveat (decided before Phase 3d)

Current backups do not capture `dns_records`. Two options for "restore DNS":

- **(A) v1 — managed-only.** "Restore DNS for `<domain>`" restores the domain
  config; the reconciler regenerates the managed records (apex A, www, mail, NS
  glue, DKIM/SPF/DMARC, MTA-STS). Custom user records are explicitly **not**
  restored — surface this in the UI ("custom DNS records aren't included in
  backups yet"). Cheapest; honest.
- **(B) Phase 0 prerequisite — capture custom records.** Add a `dns` capture
  stage at *backup* time: snapshot each owned zone's `dns_records` rows into the
  meta JSON (or a dedicated `stage=dns` snapshot with `Items=zones[]`). Only then
  is "restore DNS" lossless. This is a **backup-format change** (new schema_version
  field), and only backups taken *after* it ships carry custom records.

Recommendation: ship **(A)** with v1 and treat **(B)** as Phase 0 of a later
iteration — do NOT promise lossless DNS restore for backups taken today.

## Phasing (waves)

0. **(optional, gates lossless DNS)** capture `dns_records` at backup time
   (option B above) — bumps manifest `schema_version`. Skip for a managed-only v1.
1. **Manifest preview** — ✅ SHIPPED (`c31bf675`): agent `backup.manifest_read` + `GET /me/backups/:id/manifest`, owner-scoped, live-verified (home/db×3/mail/meta with item names). Original text: — `backup.manifest_read` agent cmd + `GET /me/backups/:id/manifest`
   + owner/scope checks. Read-only; ship + verify first (no mutation risk).
2. **Selective restore API** — `POST /me/backups/:id/restore`, validation,
   job creation, dispatch. Agent stub returns "not implemented" per stage.
3. **Agent selective apply** — `backup.restore_selective`, one stage at a time:
   3a home, 3b databases, 3c mailboxes, 3d dns. Each independently testable.
4. **Tenant UI** — restore drawer on `/jabali-panel/backups`: pick a backup →
   preview stages/items → check resources → overwrite confirm → progress via the
   jobs list. Reuse `SearchableTable`/Drawer conventions.
5. **E2E + runbook** — backup → delete a DB row / mailbox → selective restore →
   assert recovered; ADR-0148.

## Risks / open questions

- **DB restore into an existing DB**: drop-and-reload vs merge. v1 = overwrite
  drops+reloads the whole DB (simplest, predictable); document that it's not a
  row-merge.
- **Mailbox import idempotency**: Stalwart import may duplicate messages if run
  twice without overwrite — gate on overwrite = purge-then-import, else
  fail-if-mailbox-nonempty. Confirm Stalwart import semantics against the
  whole-account path before 3c.
- **DNS restore is managed-only in v1** (see DNS caveat): custom user records
  aren't captured by current backups, so "restore DNS" re-derives managed records
  from restored domain config and cannot bring back hand-added rows. Lossless DNS
  needs Phase 0 (capture at backup time). The UI must say so, or a tenant will
  expect their custom records back and silently not get them.
- **Resource no longer exists**: v1 requires the target DB/domain/mailbox to
  exist. "Recreate from manifest then restore" is a bigger follow-up (provisioning
  + restore in one job).

## Acceptance

A tenant, with no admin rights, can: open Backups → pick one of their own
succeeded backups → see its databases / mail-domains / mailboxes (and domain
config for managed-DNS re-derivation) → select a subset → restore into their own
account, with explicit overwrite, tracked as a job — and cannot restore into or
read another tenant's data. DNS restore is documented as managed-only unless
Phase 0 shipped.
