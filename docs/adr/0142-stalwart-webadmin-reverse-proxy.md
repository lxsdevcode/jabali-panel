# ADR-0142: Stalwart WebAdmin reverse-proxy (opt-in, default-off)

**Date:** 2026-06-21
**Status:** Accepted (code complete; live unauth-401 smoke pending a deploy)
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
3. **nginx reverse-proxy vhost** (`mail.webadmin.apply` agent verb) on the
   subdomain **`admin.<panel-hostname>`** — served at the subdomain *root*, so
   the WebAdmin SPA's absolute asset paths resolve without sub-path rewriting.
4. **Defence in depth — every layer required:**
   - **TLS** (panel cert; an `admin.<host>` SAN add is a follow-up — until then
     a name-mismatch warning, but the channel is encrypted).
   - **nginx basic-auth** with a **dedicated generated gateway credential**
     (bcrypt htpasswd) — **never** the Stalwart admin token. Shown once at
     enable, not stored by the panel.
   - **Optional source-IP allowlist** (`stalwart_webadmin_allow_cidrs` →
     nginx `allow`/`deny`), validated as IP/CIDR before write.
   - **Stalwart's own admin login** underneath.
   - HTTP→HTTPS redirect.
5. **Apply on the PATCH** (synchronous, to surface the one-time credential) +
   idempotent enable/disable. Disable removes the vhost + symlink (htpasswd
   kept so a re-enable reuses the credential).

**Rejected:** raw public bind of `:8446` (no gate); a sub-path on the panel host
(SPA base-path breakage + cookie-scope coupling); panel-SSO `auth_request` as
the gateway (cross-host cookie/CORS against a unix-socket panel-api — flagged
dicey in the plan; basic-auth is self-contained and unambiguous).

## Security checklist status

- [x] Stalwart listener still loopback-only (verified on mx).
- [x] No path to `:8446` without nginx auth (vhost renders `auth_basic` —
      unit-tested; live unauth→401 pending deploy).
- [x] TLS enforced; HTTP → 301.
- [x] Gateway credential is a separate bcrypt htpasswd, not the admin token.
- [x] Allowlist emits `allow … ; deny all;` when set (unit-tested).
- [x] Disable removes the vhost (unit-tested).
- [x] Off by default on fresh install + upgrade (migration default 0).
- [ ] Live unauth→401 + CrowdSec-covers-vhost smoke — pending a deploy that has
      the new agent verb.

## Consequences

- New opt-in attack surface, but gated four ways and off by default. Net posture
  is stricter than the existing phpMyAdmin/Adminer exposure (mail admin > DB admin).
- Operator must create a DNS A record for `admin.<panel-hostname>` and, for a
  clean cert, add it to the panel cert SAN (follow-up to auto-add when enabled).
- No reconciler convergence in v1 — the vhost is a file on disk that survives
  restarts; drift-repair is a follow-up.
