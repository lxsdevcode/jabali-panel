# ADR-0141: Per-domain SSL certificate modes

**Status:** ACCEPTED (2026-06-20) — shipped + live-verified on mx (all 4 modes).
**Issue:** GH #246
**Commits:** `eacd229c` (backend + create selector), `c53147ac` (SSL-section UI)
**Migration:** 000179
**Blueprint:** `plans/ssl-cert-modes-246.md`
**Related:** ADR-0070 (auto-SAN), ADR-0080 (email-on-by-default), M47 (MTA-STS).

## Context

A domain's TLS was driven by a single boolean `domains.ssl_enabled`: the
reconciler attempted ACME (Let's Encrypt) and silently fell back to a
self-signed cert on failure. Operators couldn't choose no certificate, pin a
domain to self-signed, install their own cert, or switch later (GH #246).

All the agent primitives already existed (`ssl.issue`, `ssl.self_sign`,
`ssl.install_custom`, `ssl.renew`, `ssl.revoke`). The work was an explicit
**mode** plus reconciler routing, not new crypto.

## Decision

Add `domains.ssl_mode ENUM('le','self','custom','none')` (default `le`,
migration 000179, backfill `none` where `ssl_enabled=0`):

| mode | behaviour |
|------|-----------|
| `le` | ACME + self-signed fallback (previous behaviour). Auto-renew. |
| `self` | Self-signed only, never ACME. Re-signs within 30 days of expiry. |
| `custom` | Operator-supplied cert+key via `ssl.install_custom`. No auto-renew. |
| `none` | No certificate; nginx serves HTTP only. |

`ssl_mode` is **authoritative**. `ssl_enabled` is kept for one release as a
**derived shadow** (`mode != 'none'`), written together with `ssl_mode` by a
dedicated `DomainRepository.UpdateSSLMode` — never independently — so the two
can't drift (avoids the GORM zero-value / Select-allowlist scars). A later
migration drops `ssl_enabled`.

## Reconciler

`reconcileSSLForDomain` switches on `ssl_mode`:
- `le` (and legacy empty): unchanged ACME-or-fallback state machine.
- `self`: `sslEnsureSelfSigned` — `ssl.self_sign` only when there is no usable
  self-signed cert (missing / wrong status / within 30d of expiry); change-gated.
- `none`: issued cert → `ssl.revoke`; any other status → `MarkRevoked` (clears
  `cert_path`, so the vhost renderer drops the `:443` block).
- `custom`: no-op — the upload endpoint installs synchronously; the reconciler
  never touches a custom cert (no auto-renew, no overwrite).

## API & invariants

- Create accepts `ssl_mode` (le/self/none; `custom` is upload-only). PATCH
  switches mode. Invariants enforced (422): the panel-primary domain can't be
  `none`, and a mail-enabled domain can't be `none` (MTA-STS / autoconfig need
  HTTPS). Legacy `POST`/`DELETE /ssl` map to le/none.
- `PUT /domains/:id/ssl/custom {cert_pem, key_pem}`: panel-api parses the leaf
  for `NotAfter` + `VerifyHostname(domain)`, then the agent validates the
  cert/key pair and writes them `0600`. **The private key is never logged,
  persisted to the panel DB, or echoed back.**

## UI

A "TLS certificate" selector on both create forms (le/self/none), and in the
domain SSL section a mode dropdown (le/self/custom/none) plus a Custom-cert
upload modal (cert/key PEM textareas). Current mode is read from
`GET /domains/:id` because cert status alone can't distinguish `self` from an
`le` self-signed fallback.

## Consequences

- **Mail-SAN caveat:** only `le` auto-adds the `mail.`/`autoconfig.`/`mta-sts.`
  SANs (ADR-0070). A `self`/`custom`/`none` cert on a mail-enabled domain won't
  cover the mail subdomains. The `none`+mail-enabled case is blocked at the API;
  `self`/`custom` on a mail-enabled domain are allowed but the operator owns SAN
  coverage. Follow-up: surface a warning and extend the `self` SAN set.
- Custom certs don't auto-renew — a future notification near `not_after` is a
  follow-up.

## Verification

Live on mx (2026-06-20): backfill → all domains `le`; `self` → `self_signed`
with no ACME call; `none` → revoked + zero `:443` lines in the vhost; `custom`
→ pasted cert installed (CN match, key `0600`), vhost serves it, reconciler
leaves it untouched; `le` re-converges. Reconciler mode-routing unit test
covers all four. Full panel-api suite green.
