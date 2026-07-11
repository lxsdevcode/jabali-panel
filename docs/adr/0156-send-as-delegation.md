# ADR-0156 — Send-as delegation (mustMatchSender expression)

**Status:** Accepted — GH #347. Implemented on branch `jab347/send-as-delegation`
(Steps 2–6); plan at `plans/jab347-send-as-delegation.md`; spike record in the
`project_jab347_spike_verdict` memory. Live-verified end-to-end on Stalwart 0.16.12.

## Context

GH #347 (reporter johnnyq): a single login mailbox (`support@`) needs to send email
**From** other mailboxes' addresses (`sales@`, `billing@`) **without** receiving
those mailboxes' mail. The grantor mailboxes stay their own physical inboxes.

The obvious tools don't fit:

- **Aliases** couple send + receive — `support` would *receive* `sales@`'s mail.
- **Mailbox sharing** grants read access to the inbox.
- **Group membership** only grants send-as the *group* address, not an individual
  member's.

Stalwart enforces the sender at submission via `MtaStageAuth.mustMatchSender`
(default `true`): an authenticated user may only `MAIL FROM` its own login +
associated addresses.

## Decision

**Mechanism C — a conditional `mustMatchSender` expression.** `mustMatchSender` is
not a boolean but an Expression. We set it so it evaluates to `false` (skip the
built-in sender check) **only** for a verified `(delegate = authenticated_as,
grantor = sender)` pair, and `true` (full enforcement) for everyone and everything
else:

```
!(
  (authenticated_as == 'support@dom' && sender == 'sales@dom') ||
  (authenticated_as == 'support@dom' && sender == 'billing@dom') ||
  ...
)
```

- **Non-delegated mailboxes are byte-for-byte unchanged** — the expression is
  `true` for them, so the built-in anti-spoofing stays fully in force. The blast
  radius of a bug is a single delegation pair, never the whole server. This is why
  Mechanism C is preferred over a global `mustMatchSender=false` + a re-implemented
  Sieve guard.
- **The grantor keeps receiving its own mail** — `queryRecipient` is untouched.
- **The header `From:` the recipient sees is the grantor's address** (verified
  live), which is what the use case needs, not just the envelope `MAIL FROM`.

**Authoritative source: `mailbox_send_delegations`** (migration 000218), delegate +
grantor mailbox FKs (`ON DELETE CASCADE`, unique per pair). Jabali is truth
(ADR-0042).

**The expression is materialised, not queried at evaluation time.** `sql_query`
would let the expression look up a delegation table live, but Stalwart 0.16.12
exposes no addable named SQL store the expression can reach (the primary DataStore
is RocksDB; the directory's embedded MySql store and a `StoreLookup(MySql)` are both
rejected by `sql_query` as *"not a SQL store"*). So the panel enumerates the current
pairs and writes them into the expression text.

**Two converge-on-write paths, one builder** (`buildMustMatchSenderExpr`, full
replace + idempotent):

1. **panel-api → agent (`mail.sendas.reconcile`)** after every add/remove.
2. **panel-api boot reconcile** re-derives the expression from the table on every
   startup, so a fresh install / DB restore / Stalwart reset self-heals (this is
   the install/update-safe path — it runs on every boot, per install_sh_is_truth).

## Consequences / safety

- **Applied via `Action/ReloadSettings`, not `InvalidateCaches`** — MtaStage config
  is only re-read on ReloadSettings (spike finding; cost several hours to learn).
- **Injection guard.** The expression is code; each address is validated against a
  strict charset and **rejected** (never escaped) if it could break out of the
  single-quoted literal. Only `email_cached` values from the `mailboxes` table are
  ever emitted.
- **Fail-safe.** `stalwart-cli update` validates the expression server-side and
  rejects a malformed one before `ReloadSettings`, so the prior value stays in
  force. A malformed expression that somehow reached the server would fail *closed*
  (reject sends), never open (spoof).
- **Empty set → literal `true`** (the stock value), never `!()` (a parse error).
- **Policy:** grantor must be a real, enabled mailbox in the **same domain** as the
  delegate; no self-delegation; no duplicates. Owner-or-admin scoped, same as
  forwarders.
- **Scale caveat:** the expression grows one clause per pair. Fine for the reporter's
  use case; if delegation counts grow large, revisit a lookup-store (`key_exists`)
  data source. The reconcile logs the pair count.

## Alternatives rejected

- **Mechanism A — UNION the grantor into the delegate's `queryEmailAliases`.**
  Disproven live: adding a *real mailbox's* address to another account's aliases
  does not grant send-as — Stalwart resolves it to its own account (ownership
  conflict) and rejects. Only genuine non-account aliases work, and grantors are
  real mailboxes.
- **Mechanism B — global `mustMatchSender=false` + a session Sieve.** Works but
  removes the built-in check for every mailbox, making the hand-rolled guard the
  sole defense system-wide. Mechanism C achieves the same with a per-pair blast
  radius.
