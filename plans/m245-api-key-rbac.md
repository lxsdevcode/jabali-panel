# Blueprint: API key permissions (per-area RBAC) — GH #245

**Status:** DRAFT (for review) — **security-sensitive (auth surface)**
**Issue:** #245 — scoped API keys (johnnyq: "safe DynDNS key" → broadened to full RBAC)
**Target ADR:** 0144

## What already exists (don't rebuild)

- `models.UserAPIScopes []string` on `user_api_tokens.scopes_json` — present but
  **never enforced**; every token is full-power today.
- DDNS shim (`api/ddns.go`, `GET /nic/update`) — full DynDNS v2; auths a `jat_`
  token via Basic auth, updates any of the owner's DNS records.
- Automation tokens (admin) already have the pattern to copy:
  `<action>:<area>` scopes, wildcards (`read:*`), `AutomationScopes.Has`,
  `middleware.RequireScope("read:domains")` applied per route.
- User-token bearer auth: `middleware.RequireUserAuth` → `authenticateUserAPIToken`
  stashes the token row (with scopes) on the gin context.

**The gap:** nothing checks user-token scopes. A token minted for a router can do
everything its owner can. #245 = enforce scopes across the ~206 user routes.

## Scope model

`<action>:<area>` strings, mirroring automation tokens:
- actions: `read`, `write` (write implies the area's POST/PUT/PATCH/DELETE).
- areas: `dns`, `mail`, `files`, `databases`, `apps`, `domains`, `cron`, `ssl`,
  `backups`, `php`, `ssh`, `tokens` (managing your own API tokens).
- wildcards: `read:*`, `write:*`, `*` (full). **Empty scopes = full** (every
  existing token keeps working — no migration, no breakage).
- special narrow scope: `ddns` (or `write:dns` — decide) for the DynDNS shim.

`UserAPIScopes.Has(want)` already supports exact + `read:*`/`write:*` wildcards
(reuse the AutomationScopes logic; consolidate into one helper).

## Enforcement (the load-bearing decision)

Two candidates — **review must pick**:

- **A. Per-route `RequireUserScope("write:dns")`** (like automation). Explicit,
  auditable, but edits all ~206 route registrations and a forgotten route
  silently stays open to scoped tokens. Mitigate with a default-deny wrapper.
- **B. Central method+path→scope resolver** in one middleware on the user mount.
  One place to read, but a path table DRIFTS as routes change (the documented
  route-map scar) and is easy to get wrong for `:param` paths.

**Recommendation: A + fail-closed.** A small wrapper on the user-token path:
if the token is scoped (non-empty) and the handler chain never asserted a
satisfied scope, **deny** (403). Full tokens (empty scopes) bypass entirely.
Implement by having `RequireUserScope` mark the context "scope satisfied"; a
final guard 403s scoped tokens that reached a handler without a mark. This makes
an unmapped route fail CLOSED for scoped tokens (safe) while full tokens are
unaffected. Every area's route group gets exactly one `RequireUserScope`.

`RequireUserScope(want)`: no token on ctx (Kratos cookie) → allow (browser
session already authorized); token with empty scopes → allow (full); token with
scopes → `tok.Scopes.Has(want)` or 403 + mark satisfied.

DDNS (`/nic/update`): allow when scopes empty OR `Has("ddns")` (or `write:dns`).

## API surface mapping

Walk every `RegisterXxxRoutes`; tag each group:
- read endpoints → `read:<area>`, mutating → `write:<area>`.
- `/me/api-tokens` → `write:tokens` (a scoped token managing tokens is a
  privilege-escalation risk — consider DENY entirely for scoped tokens).
- admin-only routes are unreachable by tenant tokens anyway (IsAdmin gate).

Deliverable: a checked-in table (area → routes → scope) so the mapping is
reviewable and testable. A test enumerates the router and asserts every
user route carries a scope (fail-closed audit).

## UI

Token-create (`UserAPITokensPage`): replace the implicit "full access" with a
permission editor — a grid of areas × {read, write} checkboxes, plus a "Full
access" shortcut (empty scopes) and a "DDNS only" preset. Show the granted
scopes on the token list. (Reuse the antd patterns already in the page.)

## Security review checklist (must pass)

- [ ] Empty-scope tokens behave exactly as today (no regression).
- [ ] Scoped token is 403'd on every route lacking its scope (fail-closed test).
- [ ] `write:*` does NOT grant `read:*`-only routes by accident, and vice-versa.
- [ ] Scoped token cannot mint/modify tokens (no privilege escalation) unless
      explicitly granted `write:tokens` — and reconsider even then.
- [ ] DDNS token can ONLY update DNS records, nothing else (live test).
- [ ] Admin routes remain IsAdmin-gated regardless of scope.
- [ ] Audit log records the token + scope decision.

## Phasing (ship incrementally, each independently safe)

1. **Foundation:** consolidate `Has`, add `RequireUserScope` + the fail-closed
   guard, scope constants, ADR-0144. No routes mapped yet → scoped tokens can't
   do anything (safe; none exist). Full tokens unchanged.
2. **DDNS + DNS area:** map the DNS routes + the DDNS shim; ship the "DDNS only"
   preset in the UI. This alone closes johnnyq's request.
3. **Remaining areas:** mail, files, databases, apps, domains, cron, ssl,
   backups — one area per PR, each with the route-coverage test.
4. **UI permission editor** (full grid) + scope display on the token list.
5. (Future) per-resource scoping — "tie to one DNS record" (johnnyq) — a
   resource-id allowlist on the token; ddns.go + DNS handlers enforce.

## Open questions for review

- Enforcement A (per-route + fail-closed) vs B (central map)? (lean A.)
- `ddns` narrow scope vs reuse `write:dns`? (lean narrow `ddns` so a DDNS key
  can't rewrite arbitrary records — pairs with phase 5 per-record.)
- Should scoped tokens be hard-denied from `/me/api-tokens` regardless?
- Default for unmapped routes during the phased rollout: fail-closed (chosen)
  means a not-yet-mapped area is unusable by scoped tokens until phase 3 maps it
  — acceptable since scoped tokens are new.
