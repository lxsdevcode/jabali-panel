# ADR-0142: Stalwart WebAdmin reverse-proxy (opt-in, default-off)

**Date:** 2026-06-21
**Status:** Accepted — live-verified on 10.0.3.14 (jabali-panel.local) 2026-06-22
**Owner:** shuki
**Issue:** GH #243
**Companion plan:** [`plans/stalwart-webadmin-toggle-243.md`](../../plans/stalwart-webadmin-toggle-243.md)

## Context

Stalwart ships a native WebAdmin UI (queues, tracing, fine-grained settings the
panel doesn't surface), served on its management HTTP listener at
`127.0.0.1:8446`. M25 / ADR-0050 deliberately pinned that listener to loopback
so the mail-server admin is unreachable from outside the box. GH #243 asks to
*optionally* expose it via a Server Settings → Email toggle.

This is the **highest-value target on the host** — whoever reaches the admin can
read every mailbox, send as any hosted domain, and rewrite DKIM. So exposure
must be opt-in, defence-in-depth, and reversible.

## Decision

1. **Stalwart stays loopback (`127.0.0.1:8446`).** The M25 invariant is
   untouched — confirmed by `ss -ltnp` showing 8446 on 127.0.0.1 after enable.
   nginx is the only door.
2. **Default OFF.** `server_settings.stalwart_webadmin_enabled` defaults `0`
   (migration 000182). Existing + fresh installs expose nothing.
3. **nginx reverse-proxy vhost** (`mail.webadmin.apply` agent verb) on a
   **dedicated port of the panel hostname** (`https://<panel-hostname>:8449/`),
   reusing the panel cert. Rationale: a `admin.<host>` *subdomain* was tried
   first but browsers hard-refuse a self-signed `.local` subdomain cert
   (non-overridable), and minting a matching cert is a yak-shave. A port on the
   already-trusted panel hostname sidesteps the name-mismatch entirely, and
   Stalwart owns the origin root so its absolute asset paths (`/admin`, `/api`,
   `/logo`) resolve without rewriting. The bare root redirects to `/admin/`
   (`location = / { return 302 /admin/; }`) — Stalwart serves its WebAdmin SPA
   at `/admin/` (base href=/admin/), not `/`, which 404s.
4. **Defence in depth — every layer required:**
   - **TLS** (panel cert, reused on the panel hostname:8449 — name already
     matches, no warning, no extra SAN needed).
   - **nginx basic-auth** with a **dedicated generated gateway credential**
     (bcrypt htpasswd) — **never** the Stalwart admin token. Shown once at
     enable, not stored by the panel.
   - **Optional source-IP allowlist** (`stalwart_webadmin_allow_cidrs` →
     nginx `allow`/`deny`), validated as IP/CIDR before write.
   - **Stalwart's own admin login** underneath.
   - **CrowdSec exemption (server-scope `access_by_lua_block { return }`).** The
     global CrowdSec nginx bouncer/AppSec WAF false-positives on the WebAdmin's
     own admin paths (`/admin`, `/login`, `/webadmin`, `/api/auth` read as
     admin-panel scans → 403), so the SPA never loaded. A no-op access_by_lua
     override exempts *only* this vhost; every other server still runs CrowdSec.
     Acceptable because the door is already IP-allowlisted to operator IPs
     (stricter than CrowdSec's banlist) and basic-auth-gated.
5. **Apply on the PATCH** (synchronous, to surface the one-time credential) +
   idempotent enable/disable. Enable opens UFW `8449/tcp`; disable removes the
   vhost + symlink + UFW rule (htpasswd kept so a re-enable reuses the
   credential). The agent writes `sites-available/` and symlinks into
   `sites-enabled/` only when absent.

**Rejected:** raw public bind of `:8446` (no gate); a sub-path on the panel host
(SPA base-path breakage + cookie-scope coupling); a `admin.<host>` subdomain
(browsers hard-refuse a self-signed `.local` subdomain cert, non-overridable —
abandoned for the port-on-panel-hostname approach above); panel-SSO
`auth_request` as the gateway (cross-host cookie/CORS against a unix-socket
panel-api — flagged dicey in the plan; basic-auth is self-contained).

## Security checklist status

- [x] Stalwart listener still loopback-only (verified on mx).
- [x] No path to `:8446` without nginx auth (vhost renders `auth_basic` —
      unit-tested; live unauth→401 pending deploy).
- [x] TLS enforced; HTTP → 301.
- [x] Gateway credential is a separate bcrypt htpasswd, not the admin token.
- [x] Allowlist emits `allow … ; deny all;` when set (unit-tested).
- [x] Disable removes the vhost (unit-tested).
- [x] Off by default on fresh install + upgrade (migration default 0).
- [x] Live unauth→401 verified on 10.0.3.14 (2026-06-22): `/admin/` without
      credentials → 401; with the gateway credential → 200 (SPA loads, assets
      200); allowlist `deny all` confirmed present after restore.

## Consequences

- New opt-in attack surface, but gated four ways and off by default. Net posture
  is stricter than the existing phpMyAdmin/Adminer exposure (mail admin > DB admin).
- No DNS record or extra cert needed: the vhost lives on the panel hostname's
  port 8449 and reuses the panel cert. Operator just opens the port (automatic
  via UFW on enable) and reaches `https://<panel-hostname>:8449/`.
- No reconciler convergence in v1 — the vhost is a file on disk that survives
  restarts; drift-repair is a follow-up.
