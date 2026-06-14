# ADR-0121: Safe per-user restore-side mail apply (scoped, additive, dry-run-gated)

**Date**: 2026-06-14
**Status**: superseded by ADR-0122
**Deciders**: shukiv

## Context

Account-restore reinstates panel-DB mail rows (mailboxes/forwarders) via
`backupmetadata.Apply` (ADR-0042 SQL directory makes those rows the
auth source — PR #373/#374). The *Stalwart-side* account config
(identities, sieve, vacation, mailbox layout) is restored separately by
`stalwart-cli apply` against the `plan.json` the mail backup stage
captures.

That apply path was found unsafe and is currently disabled (PR #376
reverts #375; the mail stage prints "plan.json missing — skip"):

- `backup_mailboxes.go` runs `stalwart-cli snapshot Account …`, which is
  **host-wide** — there is no per-account selector. A per-user mail
  backup therefore captures the whole host's mail config.
- The emitted plan begins with an unfiltered
  `{"@type":"destroy","object":"Account"}`. `stalwart-cli apply`
  reconciles server state to the plan, so it would **destroy every
  account on the host** and recreate only the plan's set. On a
  multi-tenant box, restoring one account would wipe all others.
- The plan was also invoked with a positional path (`apply <path>`);
  stalwart-cli 1.0.8 requires `apply --file <path>`.

The backup side already produces `plan.json` + `bodies.tar` in every
stage=mail snapshot, so no captured data is lost — backups are
future-restorable once a safe apply lands.

## Decision

Re-enable mail apply only as a **scoped, additive, dry-run-gated**
operation:

1. **Never destroy.** Strip the destroy block from any plan before
   apply (and pass `--no-destroys` on new snapshots). Restore is
   additive: it creates/updates the restored account, never removes
   other accounts.
2. **Scope to the restored user.** Filter the plan to only the Account
   (and dependent Identity/Sieve/Vacation) entries that map to the
   restored account's addresses (the manifest mail stage `Items` =
   `user@domain` list). Drop every other account's ops.
3. **Dry-run gate.** Run `stalwart-cli apply --file <filtered> --dry-run`
   first; only execute for real if validation passes. Abort the mail
   stage with a warning otherwise — never a partial apply.
4. **Correct invocation.** `apply --file <path>` (1.0.8), with
   `--continue-on-error` so one bad op doesn't abort the rest, and the
   admin creds from `stalwartAdminCreds()`.
5. **Messages stay manual (this ADR).** `bodies.tar` is the whole
   RocksDB store; per-user live extraction is impossible. The documented
   stop-Stalwart → untar → start path remains the only message-replay
   mechanism for now. Per-account JMAP message import is a later phase.

## Alternatives Considered

### A: Backup-side scoping only (`--no-destroys` + post-filter at backup)
- **Pros**: clean data-at-rest (backup holds only the user's mail).
- **Cons**: existing host-wide backups still need restore-side filtering;
  a single guard at backup time can't protect restores of old snapshots.
- **Why not (alone)**: insufficient — restore-side filtering is the
  load-bearing safety and must exist regardless. We do `--no-destroys`
  at backup as hygiene on top, not instead.

### B: Re-enable apply as-is, document "single-tenant only"
- **Pros**: zero code.
- **Cons**: a documentation note does not stop a destroy-all on a
  multi-tenant box. Unacceptable blast radius.
- **Why not**: safety cannot rely on operator memory.

### C: Abandon stalwart-cli apply; rebuild Stalwart state purely from panel DB
- **Pros**: single source of truth (panel DB), no plan format coupling.
- **Cons**: panel DB does not hold sieve scripts / vacation / per-mailbox
  Stalwart settings; only the snapshot plan does. Lossy.
- **Why not**: loses config the plan captures.

## Consequences

### Positive
- Per-user mail restore becomes safe on multi-tenant hosts.
- Account config (identities, sieve, vacation) restores automatically,
  not just the auth rows.
- Dry-run gate turns a malformed plan into a warning, not a partial wipe.

### Negative / Risks
- Plan-filtering couples us to the stalwart-cli plan JSON shape; a CLI
  format change breaks the filter. Mitigation: golden-plan test fixture +
  a `--dry-run` gate that fails closed.
- Address→account mapping in the plan is non-obvious (`name` = local
  part; full address lives in Identity/Principal sub-objects) — the
  filter must resolve it correctly or it drops the wrong ops. Mitigation:
  unit tests over a real captured plan.
- "user delete doesn't purge Stalwart" (known issue): a recreate may hit
  an existing principal. Mitigation: `--continue-on-error` + treat
  already-exists as non-fatal.
