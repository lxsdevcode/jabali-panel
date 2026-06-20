# ADR-0144: Scope-restricted user API tokens (RBAC)

**Status:** ACCEPTED (2026-06-21) — phase 1+2 shipped + live-verified on mx.
**Issue:** GH #245
**Commit:** `38810f53`
**Blueprint:** `plans/m245-api-key-rbac.md`

## Context

`user_api_tokens.scopes_json` existed but was never enforced — every token had
full per-user access. A token put in a router for DynDNS could do anything its
owner could (GH #245). The admin automation tokens already had the pattern:
`<action>:<area>` scopes + `RequireScope` middleware.

## Decision

Enforce user-token scopes, **fail-closed**, with backward compatibility:

- **Empty scopes = full access** — every existing token is unchanged, no
  migration. Browser (Kratos cookie) sessions are never scope-checked.
- A token with a **non-empty** scope set is capability-restricted: allowed only
  on a route whose required scope it `Has()`, and 403'd everywhere else.
- Enforcement is one middleware (`EnforceUserTokenScopes`) on the `/api/v1`
  group, keyed on gin's `c.FullPath()` (the registered route *pattern*, so
  `:params` match exactly with no hand-rolled path parsing). A route absent from
  the scope map denies scoped tokens (**fail-closed**) — so unmapped areas are
  safe-by-default during the phased rollout.
- Scope vocabulary: `read:<area>` / `write:<area>` (+ `read:*` / `write:*`
  wildcards via the existing `Has`), plus a narrow `ddns` grant for the DynDNS
  shim only. Phase 1+2 defines `read:dns`, `write:dns`, `ddns`.
- The DDNS shim (`/nic/update`) accepts empty scopes, `ddns`, or `write:dns`;
  everything else is `badauth`. A `ddns`-only token can use the router shim but
  is 403'd on the DNS REST API.
- Token-create validates scopes against the known vocabulary (rejects typos that
  would silently fail-close).

## Why fail-closed + FullPath (not per-route RequireScope)

Per-route `RequireUserScope` is explicit but, during a phased rollout, leaves
not-yet-mapped routes open to scoped tokens — a hole. A central `FullPath`→scope
resolver that denies unmapped routes is fail-closed: a still-unmapped area is
*unusable* by a scoped token rather than *wide open*, and a route/typo mismatch
results in denial, never escalation. A coverage test guards drift.

## Consequences

- Safe DynDNS / router keys today (`ddns` scope), DNS read/write scoping, and a
  UI permission preset — johnnyq's request is met.
- Remaining areas (mail, files, databases, apps, domains, cron, ssl, backups)
  ship one phase at a time; until mapped, a scoped token simply can't reach
  them. Full tokens are unaffected throughout.
- Per-resource scoping ("tie a token to one DNS record") is a future phase.

## Verification

Live on mx (2026-06-21, via the api unix socket): full token reaches an unmapped
route (200); a `ddns`/`read:dns` scoped token is 403'd on unmapped routes; a
`read:dns` token reads DNS records (200, real route FullPath matches the map)
but the `ddns` token is 403'd on the DNS REST API; `/nic/update` returns a
non-`badauth` result for a `ddns` token and `badauth` for a `read:dns` token.
Unit tests cover the full/scoped × mapped/unmapped × read/write matrix.
