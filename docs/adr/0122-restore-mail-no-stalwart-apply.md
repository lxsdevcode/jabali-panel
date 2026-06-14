# ADR-0122: Account-restore does NOT run stalwart-cli apply — DB rows are authoritative

**Date**: 2026-06-14
**Status**: accepted
**Supersedes**: ADR-0121
**Deciders**: shukiv

## Context

ADR-0121 proposed re-enabling the restore-side `stalwart-cli apply` of
the mail `plan.json` behind a scoped/additive/dry-run-gated filter (the
plan begins with an unfiltered `destroy Account`, a multi-tenant wipe
risk — PR #376 disabled it).

Before building that filter, the load-bearing assumption was tested on
10.0.3.14: *does a Bug-B-restored mailbox (panel-DB row only, no Stalwart
apply) actually function?*

Findings:

- **Stalwart uses its SQL directory (ADR-0045).** `jabali_panel.mailboxes`
  is the authoritative principal source; Stalwart re-reads the row on
  auth and on SMTP recipient lookup. `mailbox.create` is an agent-side
  no-op for auth; it only best-effort pre-registers the JMAP principal
  (`accountEnsureInRegistry`) for pre-first-auth JMAP ops.
- **Bug B (PR #373/#374)** reconstructs mailbox + forwarder +
  autoresponder rows in the panel DB on restore.
- **SMTP RCPT probe confirms full function with NO apply:**
  - `RCPT sales@demotenant.com` (restored mailbox) → `250 OK`
  - `RCPT contact@demotenant.com` (restored alias forwarder) → `250 OK`
  - `RCPT nonexistent99@demotenant.com` (control) → `550 does not exist`
- The mail backup runs `stalwart-cli snapshot Account` — **Account type
  only**. The plan therefore carries bare principals (seen: two `User`
  entries, `name`=localpart, everything else empty) — no Sieve, no
  Vacation, no MailboxConfig. Principals self-register on first auth.

So `stalwart-cli apply Account` restores the one thing already covered
twice (DB row + self-registration) and nothing else, while carrying the
destroy-all blast radius.

## Decision

**Account-restore does NOT run `stalwart-cli apply`.** Mail accounts,
aliases, forwarders, autoresponders, and quotas are restored solely by
the panel-DB reconstruction (Bug B); Stalwart's SQL directory makes them
live for auth and SMTP routing immediately. The mail restore stage stays
disabled (prints "plan.json missing — skip"); the plan-filter library
(ADR-0121 Wave A) is **not built**.

Stalwart-only state is handled out of band:

- **Messages** (`bodies.tar`, whole RocksDB) — documented manual restore
  (stop Stalwart → `tar -xf` → start). Unchanged.
- **Sieve scripts / JMAP-side settings** — not currently backed up in a
  per-user, apply-safe form; restoring them needs a per-type, per-account
  scoped snapshot/apply, deferred until there is a concrete need.
- **JMAP principal pre-registration** — optional parity only. Bug B's
  direct-repo writes bypass `accountEnsureInRegistry`, so restored
  mailboxes self-register on first JMAP auth instead of eagerly. Delivery
  and auth do not depend on it (verified). If pre-registration is ever
  wanted, the restore path can call `accountEnsureInRegistry` per restored
  mailbox — a few JMAP calls, no plan, no destroy.

## Alternatives Considered

### ADR-0121: scoped/additive/dry-run-gated stalwart-cli apply
- **Why not**: substantial code (NDJSON plan parser, account-ref graph,
  golden fixtures) to make a redundant-and-dangerous apply less
  dangerous. The plan carries only principals, which the DB row +
  self-registration already provide. Net unique value ≈ zero.

### Keep apply but only `--no-destroys`
- **Why not**: still redundant with the DB path, and the host-wide
  (non-user-scoped) snapshot would additively touch other tenants'
  principals. Effort with no benefit over the DB path.

## Consequences

### Positive
- No new code; the dangerous apply stays off permanently by design, not
  by accident.
- Mail restore is already correct via Bug B — verified end to end (RCPT
  resolution + alias routing).

### Negative / Risks
- Sieve scripts and JMAP-only per-mailbox settings are not restored by
  account-restore. Acceptable: not in scope, and no current backup
  captures them in an apply-safe per-user form.
- Historical messages still require the manual `bodies.tar` path.
