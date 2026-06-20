# Blueprint: Optionally expose the Stalwart WebAdmin UI (GH #243)

**Status:** DRAFT (for review) — **security-sensitive**
**Issue:** #243 — "Optionally Expose the Stalwart WebAdmin UI" (toggle under Server Settings → Email)
**Target ADR:** 0142 (Stalwart WebAdmin reverse-proxy, default-off)

## Problem

Stalwart ships a built-in web admin UI, served on the same management
endpoint the agent uses — `http://127.0.0.1:8446`. M25 / ADR-0050 deliberately
pinned that listener to **loopback** so nothing outside the box can reach the
mail admin.

**Verified on mx (Stalwart 0.16.6, 2026-06-20):** the WebAdmin SPA is served
on `:8446`. Bare `/` returns a JSON 404 (it's an SPA — there is no root
route), but `/login` returns `200 text/html` (the admin login page) and
`/admin` + `/dashboard` `302` into it; `/jmap` + `/api` are the same listener.
So the proxy target is `http://127.0.0.1:8446` and the entry URL is `/login`,
NOT `/`. (There is a second Stalwart loopback HTTP listener on `127.0.0.1:18181`
that serves the identical surface; we proxy `:8446` because that's the
listener the apply-plan configures and the agent already talks to.) Operators who want to
use Stalwart's native admin (queues, tracing, fine-grained settings the panel
doesn't surface) currently can't, short of an SSH tunnel.

Goal: a **default-off** Server Settings toggle that exposes the WebAdmin
through nginx, with real access control — without undoing the M25 invariant
(Stalwart itself stays bound to loopback; nginx is the only door).

## Decision

- Keep Stalwart's listener on `127.0.0.1:8446` (M25 invariant untouched).
- A server setting `stalwart_webadmin_enabled` (bool, default **false**)
  gates an nginx **reverse-proxy** vhost that forwards to `127.0.0.1:8446`.
- Exposure URL: a dedicated host `admin.mail.<panel-hostname>` (mirrors the
  existing `mail.<domain>` webmail vhost pattern in `webmail_vhost.go`), or a
  path `/_stalwart-admin/` on the panel host. **Recommendation: subdomain** —
  cleaner cookie/scope isolation than a sub-path under the panel SPA.
- TLS only (reuse the panel/mail cert SAN machinery; add the SAN when enabled).
- **Two auth layers, both required:**
  1. nginx `auth_basic` (or panel-SSO `auth_request` to panel-api) so an
     unauthenticated internet client never even reaches Stalwart's login.
  2. Stalwart's own admin login (the existing `STALWART_RECOVERY_ADMIN`
     principal) behind that.
- Optional **IP allowlist** (`stalwart_webadmin_allow_cidrs`) — nginx `allow/deny`.
- Off by default; turning it on surfaces a prominent risk warning + the URL.

### Why default-off + double auth

Exposing the mail-server admin is the highest-value target on the box (it can
read every mailbox, rewrite delivery, mint credentials). The recovery-admin
token is long-lived. So: opt-in only, never reachable without passing an
nginx auth gate first, TLS-only, and IP-allowlistable. This is the same
posture as exposing phpMyAdmin/Adminer but stricter (mail admin > DB admin).

## Schema (migration 000180)

```sql
ALTER TABLE server_settings
  ADD COLUMN stalwart_webadmin_enabled  TINYINT(1)   NOT NULL DEFAULT 0,
  ADD COLUMN stalwart_webadmin_allow_cidrs VARCHAR(512) NOT NULL DEFAULT '';
```

(`allow_cidrs` empty = allow all source IPs that pass auth; non-empty =
nginx allow-list, deny everything else.)

## Agent

New (or reuse webmail vhost) command `mail.webadmin.apply`:

- params: `{ enabled, server_name, allow_cidrs[] }`.
- enabled=true → render `/etc/nginx/sites-available/jabali-stalwart-admin.conf`:
  `server_name admin.mail.<hostname>`, TLS (panel/mail cert), optional
  `allow/deny`, `auth_basic` (or `auth_request /._stalwart_authcheck`),
  `proxy_pass http://127.0.0.1:8446;` with websocket upgrade headers (the
  webadmin uses live updates), `client_max_body_size` modest, symlink into
  sites-enabled, `nginx -t` then reload.
- enabled=false → remove vhost + symlink + reload (idempotent).
- Mirrors `webmail_vhost.go` almost exactly; factor shared vhost helpers.

If using nginx basic-auth: a credential is needed. **Do not** reuse the
Stalwart admin token in an htpasswd. Generate a dedicated gateway credential
(write `/etc/nginx/.jabali-stalwart-admin.htpasswd` via the agent), shown
once in the UI like a mailbox password.

**Gateway-auth choice is a real product decision — leave it open, don't
pre-decide.** Two options, each with a genuine cost:
- **Dedicated basic-auth credential** — simplest, self-contained, but a second
  password to manage/rotate.
- **`auth_request` against a panel-api endpoint that checks an admin session**
  — no second password, but **dicey here**: panel-api listens on a unix
  socket (M25), and the panel session cookie is scoped to the panel host,
  NOT `admin.mail.<panel-hostname>`. Making `auth_request` work cross-host
  needs a cookie-domain / CORS / shared-parent-domain story that may not hold
  (and a wrong move re-opens an auth hole). Flagged as a known complication;
  resolve during design, don't assume it works.

## DNS / cert

- `admin.mail.<panel-hostname>` needs an A record + a cert SAN. Reuse the
  panel-hostname mail SAN auto-provisioning (ADR-0070) — add the SAN only
  while the toggle is on; drop it when off (or leave it, harmless).

## Reconciler

A `reconcileStalwartWebadmin(ctx)` step (or fold into the existing webmail
vhost reconcile): read the setting, dispatch `mail.webadmin.apply` only on a
state delta (enabled flag or CIDR list changed vs last applied), reload once.
Same change-gating as the disclaimer/forwarder reconcile.

## REST + UI

- Server Settings → **Email** tab: a "Stalwart WebAdmin" toggle (off), an
  optional CIDR allowlist field, and — on enable — the resulting URL +
  the gateway credential (once) + a red warning describing the exposure.
- `PATCH /admin/settings` already carries server settings; add the two fields.

## Security review checklist (must pass before merge)

- [ ] Stalwart listener still loopback-only (`ss -ltnp` shows 8446 on 127.0.0.1).
- [ ] No path to `:8446` without passing nginx auth (test unauth → 401).
- [ ] TLS enforced; HTTP → redirect or refuse.
- [ ] Gateway credential is NOT the Stalwart admin token.
- [ ] Allowlist denies non-listed source IPs when set.
- [ ] Toggling off fully removes the vhost (no orphan listener).
- [ ] Off by default on fresh install + after upgrade.
- [ ] CrowdSec/Bulwark covers the new vhost (brute-force on the gateway).

## Steps

1. Migration 000180 + 2 server_settings fields + model.
2. Agent `mail.webadmin.apply` (vhost render/remove; htpasswd gen).
3. Reconciler change-gated apply + cert SAN add when enabled.
4. REST fields + UI toggle/CIDR/credential/warning.
5. Tests: vhost render golden, idempotent off, unauth-401 live smoke on mx,
   loopback-still-pinned assertion.
6. ADR-0142 + runbook (how to enable safely, how to rotate the gateway cred).

## Open questions for review

- Subdomain (`admin.mail.<host>`) vs path (`/_stalwart-admin/`)? (leaning subdomain.)
- Gateway auth: dedicated basic-auth credential vs panel-SSO `auth_request`?
- Should enabling require re-typing the admin password / a confirm modal?
