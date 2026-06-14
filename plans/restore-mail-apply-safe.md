> **SUPERSEDED by ADR-0122 (2026-06-14). DO NOT BUILD.** Verification on
> 10.0.3.14 (SMTP RCPT) proved Bug-B-restored mailbox rows route mail +
> aliases via Stalwart's SQL directory with no stalwart-cli apply. The
> Account apply is redundant + carries the destroy-all risk; the plan
> carries only bare principals. Account-restore runs NO mail apply.

# Blueprint: Safe per-user restore-side mail apply

**Status**: proposed (ADR-0121)
**Owner**: shukiv
**Target**: re-enable Stalwart mail account-config restore, scoped +
additive + dry-run-gated, after PR #376 disabled the unsafe version.

## Problem recap

`stalwart-cli apply plan.json` on restore is disabled because:
1. plan starts with unfiltered `destroy Account` → all-tenant wipe;
2. `stalwart-cli snapshot Account` is host-wide, not user-scoped;
3. invocation was positional (`apply <path>`), 1.0.8 wants `--file`.

Backup already produces `plan.json` + `bodies.tar` per stage=mail
snapshot. Mail *auth rows* already restore via the panel-DB path
(`backupmetadata.Apply`, ADR-0042). This blueprint restores the
Stalwart-side *account config* safely.

## Design (per ADR-0121)

Restore filters the captured (host-wide) plan down to the restored
account, drops the destroy block, dry-run-validates, then applies.
Backup additionally emits a clean scoped plan going forward.

## Waves

### Wave A — plan filter library (pure, tested) — DISPATCHABLE
New `internal/stalwartplan/` (repo-root internal, shared by agent):
- `Parse(r io.Reader) (Plan, error)` — NDJSON plan reader.
- `Plan.FilterToAddresses(addrs []string) Plan` — keep only `create`/
  `update` ops for Accounts whose addresses ∈ addrs (+ their dependent
  Identity/Sieve/Vacation/MailboxConfig ops by account-ref); DROP every
  `destroy` op and every unrelated account.
- `Plan.WriteNDJSON(w io.Writer) error`.
- Address resolution: map each Account entry to its address(es). The
  create value carries `name` (= local part); the full address is in the
  account's Identity/Principal sub-object — resolve via the plan's
  account-ref graph. **Unit-test against a real captured plan fixture**
  (commit a sanitized `testdata/plan-2acct.ndjson`).
- Hard rule: if filtering cannot positively resolve an op's account,
  DROP it (fail closed), and record it so the caller can warn.

Acceptance: table tests — 2-account host plan filtered to one address →
1 account, 0 destroys; unresolved op → dropped + reported.

### Wave B — restore-side wiring (agent) — needs Wave A
`panel-agent/internal/commands/backup_restore.go` StageMail:
1. Re-resolve the nested staging path (the #375 glob — re-add it; it was
   only reverted to keep apply OFF):
   `stagingRoot/mail/run/jabali-backup/*/mail/{plan.json,bodies.tar}`.
2. `addrs := st.Items` (manifest `user@domain` list for this restore).
3. `filtered := stalwartplan.Parse(plan).FilterToAddresses(addrs)` →
   write `plan-restore.json` beside the original.
4. Gate: `stalwart-cli apply --file plan-restore.json --dry-run --quiet
   --no-color` with `stalwartAdminCreds()`. On non-zero → warn + skip
   (no real apply).
5. Real apply only on dry-run success: same cmd minus `--dry-run`, plus
   `--continue-on-error`. Treat already-exists as non-fatal.
6. Keep the bodies.tar message-replay warning as-is (manual path).

Acceptance (live on 10.0.3.14, multi-account):
- Create a 2nd tenant mailbox on another domain. Back up tenant-1.
- Delete tenant-1's Stalwart account only.
- Restore tenant-1 → tenant-1 config reapplied, **tenant-2 untouched**
  (assert tenant-2 account still present + auth works).
- Restore with a deliberately corrupted plan → dry-run fails → skip, no
  partial state.

### Wave C — backup-side hygiene — independent of A/B
`panel-agent/internal/commands/backup_mailboxes.go`:
- Add `--no-destroys` to the `snapshot` args (plan no longer carries the
  destroy block at all).
- Optionally post-filter the snapshot to `req.Mailboxes` via
  `stalwartplan.FilterToAddresses` before staging, so new backups hold
  only the user's mail config (data-at-rest hygiene). Old backups still
  rely on Wave B's restore-side filter.

Acceptance: new stage=mail snapshot plan has 0 destroy ops; contains only
the backed-up user's accounts.

### Wave D — message replay (DEFERRED, own sub-blueprint)
bodies.tar = whole RocksDB. Options to scope per-account message import
(JMAP `Email/import`, or `stalwart-cli` per-account export if it gains
one). Out of scope here; keep the documented stop→untar→start manual
path. Track separately.

## Validation matrix

| Risk | Guard |
|------|-------|
| All-tenant wipe | filter drops every `destroy`; dry-run gate; multi-tenant live test asserts other tenant intact |
| Wrong-account ops | fail-closed address resolution + unit tests on real plan |
| CLI format drift | golden-plan fixture test; dry-run fails closed |
| Recreate vs stale Stalwart principal | `--continue-on-error`, already-exists non-fatal |
| Old host-wide backups | restore-side filter (Wave B) handles them regardless of Wave C |

## Out of scope
- Per-account Maildir/message replay (Wave D).
- `uid_at_source` honoring (separate; system-restore concern).

## Files
- `internal/stalwartplan/{plan.go,plan_test.go,testdata/}` (NEW, Wave A)
- `panel-agent/internal/commands/backup_restore.go` (Wave B)
- `panel-agent/internal/commands/backup_mailboxes.go` (Wave C)
- `docs/adr/0121-safe-restore-mail-apply.md` (done)
