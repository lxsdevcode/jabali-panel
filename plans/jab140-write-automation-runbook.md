# Runbook — Write-automation API (JAB-140)

Scoped remediation actions over the HMAC-signed Automation API, for the Sounder control
plane. Design + rationale: `docs/adr/0157-automation-write-endpoints.md`.

**A write token is a headless, server-wide admin key.** Treat it like a root credential:
short expiry, IP allowlist, rotate on suspicion.

## Mint a write token

```bash
jabali automation-token mint sounder-remediation \
  --scope write:services --scope write:cache --scope write:domains --json
# reveals the secret ONCE — store it in Sounder's secret store.
```

Scopes: `write:services`, `write:users`, `write:domains`, `write:cache`, `write:backups`
(or `write:*`). `read:*` never grants any write. `delete:*` is reserved (no destructive
endpoint yet).

Optional hardening (set via DB until the token form lands): `writes_enabled` (0 pauses all
writes), `expires_at`, `ip_allowlist_json` (JSON array of CIDRs).

## Sign a request

```
sig = hex(HMAC_SHA256(secret, METHOD "\n" URI "\n" ts "\n" sha256(body)))
Authorization: Jabali-HMAC kid=<token-id>, ts=<unix>, sig=<sig>
```

`ts` must be within `-5m .. +30s` of server time; a signed request may be sent ONCE (replay
gate). For `POST /backups`, send an `Idempotency-Key` header so a retry doesn't double-fire.

## Endpoints

| Method | Path | Scope | Notes |
|--------|------|-------|-------|
| POST | `/automation/services/:name/restart` | write:services | agent enforces the restartable allowlist |
| POST | `/automation/users/:id/{disable,enable}` | write:users | refuses to disable an admin (409) |
| POST | `/automation/domains/:id/{suspend,unsuspend}` | write:domains | flips `is_enabled` → reconciler converges nginx |
| POST | `/automation/cache/purge` | write:cache | body `{scope:"all"\|"domain", domain?}` |
| POST | `/automation/backups` | write:backups | async → `202 {operation_id}` |
| GET | `/automation/operations/:id` | any | poll `{status: pending\|running\|done\|error}` |
| GET | `/automation/capabilities` | any | mounted actions + scopes |

Success `{ok:true, status, message}` (200) or `{ok:true, operation_id, status:"pending"}` (202).
Error `{ok:false, error, message}`, `error ∈ scope_denied | not_found | conflict | unsupported
| rate_limited | internal`.

## Discover capabilities

`GET /automation/capabilities` returns only the actions this server actually mounts, so Sounder
shows available actions and stays forward-compatible.

## Troubleshoot

- **403 `scope_denied`** — token lacks the scope, OR `writes_enabled=0` (message says which).
- **401 `replay detected`** — the exact signed request was already used; re-sign with a fresh `ts`.
- **401 (generic)** — timestamp outside `-5m..+30s`, token expired, or client IP not in the
  token's allowlist.
- **429 `rate_limited`** — per-token write cap (30/min, burst 10); back off.
- **`unsupported`** — non-allowlisted service, or bad cache `scope`.
- **backup op → `error` "no single enabled backup destination"** — configure exactly one enabled
  backup destination in the panel first.
- **503** — automation API disabled, or the Redis replay store is down (fail-closed by design).

## Revoke / rotate

`jabali automation-token list`, then revoke the id (or `DELETE FROM automation_tokens`). Mint a
replacement and update Sounder. Every write is in `audit_events` (actor_kind=automation).
