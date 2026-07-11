# ADR-0157 — Write-automation endpoints + write scopes (JAB-140)

**Status:** Accepted — JAB-140. Additive write layer on the M44 Automation API for the
Jabali Sounder control plane's M2 remediation milestone. Blueprint at
`plans/jab140-write-automation.md`; live-verified on `.86`.

## Context

The M44 Automation API (`/api/v1/automation/*`, HMAC-signed, per-token `read:*` scopes) is
read-only. Sounder's M2 needs to take **scoped remediation actions** against managed panels
(restart a service, disable a tenant, suspend a domain, purge cache, trigger a backup). All
of it is additive; the read surface, its tokens, and envelopes are unchanged.

## Decision

**A write token is a headless, server-wide admin key** (automation tokens have no per-tenant
scoping), so the design is built to that threat model.

**Scopes.** Five least-privilege write scopes independent of read — `write:{services,users,
domains,cache,backups}` + `write:*`. The scope matcher's wildcard is **prefix-scoped**
(`read:*` matches only `read:X`), so `read:*` can never satisfy a write and `write:*` can
never satisfy a **reserved, distinct** `delete:*` (no destructive verb ships in M2; a future
delete must demand `delete:<resource>` + `confirm:true` + dry-run).

**Endpoints** (reversible only; each: HMAC → per-token write rate limit → `requireWriteScope`
→ validate the SIGNED body → act through the SAME repository/agent the GUI uses → audit):
`POST /services/:name/restart` (agent `service.restart`, agent-enforced allowlist),
`POST /users/:id/{disable,enable}` (`SetSuspended`; refuses to disable an admin),
`POST /domains/:id/{suspend,unsuspend}` (flips `is_enabled`, the nginx-vhost switch, so the
reconciler converges), `POST /cache/purge` (`nginx.cache.purge`, `scope:all` iterates
domains), `POST /backups` (async → the M30 `system.backup` job path). Idempotent by target
state; async backup returns `202 {operation_id}` polled at `GET /operations/:id`.
`GET /capabilities` advertises only the MOUNTED actions.

**Envelope.** Success `{ok:true, operation_id?, status, message}`; error `{ok:false, error,
message}` with codes `scope_denied | not_found | conflict | unsupported | rate_limited |
internal`.

## Consequences / security (the load-bearing part)

- **Replay defense tightened for writes** (a write replay = a duplicate action): the nonce
  TTL is now `2×maxSkew` (the old `maxSkew+1min` was shorter than the acceptance span — a
  hole where a leading-edge signature could be replayed after its nonce expired); future
  timestamps are rejected beyond +30s; non-idempotent creates (backups) are guarded by an
  idempotency key (`Idempotency-Key` or the request signature) so a retry never double-fires.
  Redis-down still fail-closes `503` — now load-bearing.
- **The handler parses the SIGNED body bytes** (`bindSigned` over the buffer the HMAC
  covered), never a re-read stream — validated == signed. Bodies are size-capped (`413`).
- **Token guards** enforced on every request (no-op for legacy read tokens): a `writes_enabled`
  master switch (pause all writes while leaving reads working), optional `expires_at`, optional
  per-token IP allowlist (CIDR). Write tokens are treated as admin-equivalent (rotation
  runbook, allowlist, expiry).
- **One write path.** Every mutation runs through the same repo/service/reconciler + agent
  command the GUI uses — no parallel path, no hand-rolled SQL — so DB-is-truth and reconciler
  convergence hold.
- **Audit every write** (M49 `audit_events`): actor = automation (token id in meta), action,
  target, client IP, request id, result — on success and failure.
- **Per-token write rate limit** (30/min, burst 10) + `429 rate_limited`.
- **No destructive verb in M2.** `delete:*` is reserved but no endpoint uses it.

## Live-verified (`.86`)

Write token → cache purge `200`; `read:*` token → same endpoint `403 scope_denied`; capabilities
lists mounted actions; backup `202` + poll transitions; the same signed write replayed → `401`;
`writes_enabled=0` → `403`; audit rows written with the automation actor.

## Follow-ups (not blockers)

- M14 notification on high-impact writes (suspend/disable) — audit covers the security record;
  notify is operator-convenience, deferred.
- panel-ui token form: surface `writes_enabled` / expiry / IP-allowlist controls (a write
  token is already mintable via `jabali automation-token mint --scope write:…`).
- Audit `source_ip` is empty behind the unix-socket nginx proxy (trust the proxy's `X-Real-IP`
  for automation) — a proxy-header config refinement.
- Refine `service.restart` agent-error → `unsupported` mapping for non-allowlisted units.
