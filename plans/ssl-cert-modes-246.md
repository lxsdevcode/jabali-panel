# Blueprint: Per-domain SSL certificate modes (GH #246)

**Status:** DRAFT (for review)
**Issue:** #246 — "Add Domain with no SSL Cert, Custom SSL Cert, Self Generated or LE Cert"
**Target ADR:** 0141 (per-domain `ssl_mode`)

## Problem

Today a domain's TLS is driven by a single boolean `domains.ssl_enabled`:
the reconciler attempts ACME (Let's Encrypt) and silently falls back to a
self-signed cert on failure. The operator cannot:

- choose **no** certificate (plain HTTP only),
- pin a domain to **self-signed** (skip ACME attempts entirely),
- install a **custom** cert + key they already own,
- or switch a domain between these later.

All the agent primitives already exist — `ssl.issue` (ACME), `ssl.self_sign`,
`ssl.install_custom`, `ssl.renew`, `ssl.revoke`. This feature is mostly about
introducing an explicit **mode** and routing the reconciler by it, instead of
the one-way `ssl_enabled` + auto-fallback.

## Decision

Add an explicit per-domain enum `ssl_mode`:

| mode     | behaviour |
|----------|-----------|
| `le`     | ACME via Let's Encrypt; **on ACME failure fall back to self-signed** (today's behaviour). Auto-renew. |
| `self`   | Self-signed only. No ACME attempts. Re-sign before expiry. |
| `custom` | Operator-supplied cert + key, installed via `ssl.install_custom`. **No auto-renew** (operator owns the lifecycle); warn near expiry. |
| `none`   | No certificate. nginx serves HTTP only; any existing managed cert is revoked/removed. |

`ssl_enabled` is **fully retired**, not kept as a half-derived mirror. Making
it a real column that new code derives from `ssl_mode` but old code still
reads/writes is the GORM-zero-value footgun that already bit us (EmailEnabled
`default:1` flipping a deliberate `false` → GH #216; the `DeriveMailFlags`
drift). Two sources of truth for the same fact is the bug. Choose ONE:

- **Preferred:** make `ssl_enabled` a computed, non-persisted field
  (`gorm:"-"`, value = `ssl_mode != 'none'`) for the one-release compatibility
  window, so there is no writable second column to drift.
- Or drop the column outright in migration 000179 and update every reader.

Do NOT leave a writable `ssl_enabled` column alongside `ssl_mode`.

### Why an enum, not separate booleans

A single enum makes the states mutually exclusive (you can't be both `none`
and `custom`) and maps 1:1 to a reconciler switch — mirrors how
`mail_provider` (ADR-0120) replaced the derived mail booleans.

## Schema (migration 000179)

```sql
ALTER TABLE domains
  ADD COLUMN ssl_mode ENUM('le','self','custom','none') NOT NULL DEFAULT 'le';
-- Backfill from the legacy boolean: enabled => le, disabled => none.
UPDATE domains SET ssl_mode = 'none' WHERE ssl_enabled = 0;
-- ssl_enabled stays for now (computed mirror); a later migration drops it.
```

No new columns for the custom cert/key: the **private key never lives in the
panel DB**. `ssl.install_custom` already writes the cert + key to the domain's
cert path on the agent host; `ssl_certificates` tracks status + fingerprint +
not-after. The REST upload streams the PEM straight to the agent (see below).

## Agent

No new commands — reuse the five existing handlers. One small addition:
`ssl.install_custom` must validate that (a) the key matches the cert, (b) the
cert covers the domain (CN/SAN), and return `not_after` + fingerprint so the
reconciler can populate `ssl_certificates`. Confirm the current handler already
does (a)/(b); add if missing.

## Reconciler

Replace the `ssl_enabled`-based switch in `reconcileSSLForDomain` with an
`ssl_mode` switch:

```
switch domain.SSLMode {
case "le":     // current tryACMEOrFallback path (unchanged)
case "self":   // ensure a self-signed cert exists / not near expiry → ssl.self_sign
case "custom": // ensure the installed cert matches the stored fingerprint;
               //   if the row says custom but no cert on disk → re-emit
               //   ssl.install_custom is operator-driven, so reconciler only
               //   *verifies*; it never fetches a key it doesn't have.
case "none":   // if a managed cert exists → ssl.revoke + drop :443 server block
}
```

Key rule (per the idempotent-loop scar): every branch gates its agent call on
a real state delta (cert missing / expiring / fingerprint mismatch), never an
unconditional re-issue per tick.

nginx vhost: the renderer already keys the `:443` server block on cert
presence. For `none`, ensure no cert path is referenced so only `:80` is
emitted (and decide redirect behaviour — `none` should NOT force-redirect to
https).

### Mail SAN interaction (CRITICAL — was invisible in the first draft)

An **email-enabled** domain's cert isn't just the apex: ADR-0070 auto-adds
`mail.<domain>` / `autoconfig.<domain>` / `autodiscover.<domain>` SANs, and
M47 MTA-STS adds `mta-sts.<domain>`. Today's `le` path provisions all of
these automatically. The new modes break that silently:

- `self` — the self-signed cert must cover the same SAN set, or the mail
  vhost + autoconfig + MTA-STS endpoints serve a name-mismatched cert.
- `custom` — an operator cert that only covers the apex breaks every mail
  subdomain. Validation must check SAN coverage and **warn** (not silently
  install) when an email-enabled domain's custom cert omits the mail SANs.
- `none` — no cert at all means the mail vhost / MTA-STS HTTPS endpoints have
  no cert. MTA-STS over plain HTTP is invalid → MTA-STS for that domain is
  effectively disabled.

Design rule: when `ssl_mode != le` **and** the domain is email-enabled,
surface a clear warning in the UI and in the domain's email-status payload,
and (for `self`) extend the self-signed SAN set to the mail names. `none` +
email-enabled is contradictory — see Invariants.

## REST

- `create domain`: accept `ssl_mode` (default `le`). For `custom` at create,
  require a follow-up cert upload (can't bind a key inline cleanly) — so the
  create flow allows `le|self|none` and `custom` is set via the edit path
  after upload. (Decision point for review: allow custom-at-create with the
  PEM in the request body?)
- `PATCH /domains/:id` : allow switching `ssl_mode`. le↔self↔none switch
  immediately (reconciler converges); → custom requires the cert to be present.
- `PUT /domains/:id/ssl/custom` : multipart or JSON `{cert_pem, key_pem}` →
  validate pair server-side → stream to agent `ssl.install_custom` → set
  `ssl_mode='custom'`. **Never log the key; never echo it back.** Enforce the
  existing `filesUploadSizeLimit`-style body cap.

## UI

- **Add Domain** form: a "TLS certificate" selector — Let's Encrypt
  (recommended) / Self-signed / None. (Custom offered after create.)
- **Domain → SSL** section (admin + tenant): show current mode + cert state
  (reuse `ssl_state`), let the operator switch mode, and for `custom` a
  cert + key upload (two textareas / file inputs). The key field is
  write-only (never populated from the server). A clear warning on `none`
  ("served over HTTP only — not recommended").

## Security

- Private key: never in DB, never logged, never returned. PEM travels
  panel→agent over the existing UDS; agent writes it `0600` to the cert path.
- Validate the cert/key pair + domain coverage before install (reject 400).
- `none` mode: surface a prominent warning; do not auto-redirect to https.
- Custom cert expiry: a notification (M14) when a `custom` cert is within N
  days of `not_after`, since there is no auto-renew.

## Risks / edge cases

- Mid-switch flapping (custom→le while a custom cert is live): le path issues
  a new ACME cert and overwrites — fine, but document.
- A `custom` row whose key was rotated out-of-band: reconciler only verifies,
  so it won't silently replace; surface "cert/row fingerprint mismatch".
- Backfill: domains currently sitting on self-signed *fallback* (mode would
  be `le`) keep retrying ACME — that's today's behaviour, unchanged.

## Steps

1. Migration 000179 + `Domain.SSLMode` model field (+ keep `SSLEnabled` mirror).
2. Reconciler `reconcileSSLForDomain` → switch on `SSLMode`; `none` vhost path.
3. Agent: confirm/extend `ssl.install_custom` validation + returns fingerprint/not-after.
4. REST: `ssl_mode` on create/patch + `PUT /domains/:id/ssl/custom`.
5. UI: create-form selector + domain SSL section + custom upload.
6. Tests: reconciler mode matrix, cert/key pair validation, backfill; live smoke
   on mx (le→self→none→custom round-trip, verify served cert each step).
7. ADR-0141 + runbook.

## Invariants (firm constraints, not preferences — guard at API + reconciler)

- The **panel-primary** domain (`is_panel_primary`) can never be `none` (and
  arguably must stay `le`): the panel + its mail SAN need real TLS. Reject the
  mode change with a 422, same as the existing panel-primary delete guard.
- **Email-enabled + `none`** is self-contradictory (MTA-STS / autoconfig need
  HTTPS). Either reject `none` while email is enabled, or require disabling
  email first. Reject at the API layer.

## Open questions for review (genuine product calls — hand back to the user)

- Allow custom-cert-at-create (PEM in create body) or edit-only? (leaning edit-only.)
