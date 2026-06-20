# ADR-0143: Disclaimer via MTA Hook (lossless HTML)

**Status:** PROPOSED (2026-06-20)
**Issue:** GH #233
**Supersedes:** ADR-0052 (Sieve disclaimer)
**Amends:** ADR-0050 (M25 unix-socket lockdown / no TCP loopback)
**Blueprint:** `plans/m233-disclaimer-mtahook.md`

## Context

The disclaimer feature appended text via a Stalwart Sieve script
(`MtaStageData` → `extracttext`/`replace`). It corrupts mail:

- `replace` without `:mime` rewrites a part as `text/plain`, downgrading the
  `text/html` alternative and leaking raw `<hr><p>` markup (the GH #233 report).
- Even with `replace :mime` (content-type preserved), Sieve's `extracttext`
  **de-tags HTML** — the original formatting is destroyed.

Sieve cannot do a lossless inline HTML disclaimer. The only mechanism that can
is a DATA-stage body rewrite in real code. Stalwart offers `MtaHook` (HTTP) and
`MtaMilter` (TCP) — both confirmed TCP-only (no unix-socket support in
`reqwest`/the milter client). M25 (ADR-0050) had banned TCP loopback.

## Decision

Replace the Sieve disclaimer with a Stalwart **MtaHook** at the DATA stage,
pointing at a new loopback-only, token-authenticated HTTP service
**`jabali-mailhook`** that rewrites the MIME body in Go (`go-message`),
appending the per-domain disclaimer to `text/plain` (plain) and `text/html`
(before `</body>`) parts while preserving all markup, encodings, boundaries,
and attachments.

The hook reads the per-domain disclaimer from the panel DB live
(`domains.disclaimer_text`/`disclaimer_enabled`/`email_enabled`) via the
existing read-only `jabali-stalwart-ro` grant. `panel-api` remains the single
writer; the agent only ensures the global MtaHook is registered and removes the
legacy Sieve script.

### Confirmed contract (spiked live on mx)

- `message.contents` = body only (raw, original CTE); top headers separate.
- `replaceContents` = body-only replace; Stalwart preserves the top headers and
  **re-signs DKIM after the hook**, so body edits don't break the signature.
- `stages` is a JMAP set (`{"data":true}`); Expression fields use
  `{"@type":"Expression","else":"true"}`; `timeout` is milliseconds.
- `action:accept` with no modifications = passthrough.

## Amendment to ADR-0050 (M25)

M25's "no TCP loopback" is narrowed, not revoked. One exception is permitted:
the `jabali-mailhook` service, bound `127.0.0.1` only, requiring a bearer token
shared with Stalwart's `httpAuth`, exposing a single `POST /` endpoint, with a
read-only DB user. Justification: Stalwart's hook/milter have no unix-socket
transport, and a correct disclaimer requires DATA-stage body rewrite. All other
M25 invariants (Kratos, panel-api, MariaDB skip-networking, etc.) stand.

## Consequences

- Lossless HTML + plain disclaimers; attachments untouched; DKIM intact.
- One new always-on loopback service + one reopened (narrow) security exception.
- Fail-open: a hook error or oversize body delivers mail *without* the
  disclaimer rather than bouncing it (`tempFailOnError:false`).
- ADR-0052 and the `replace :mime` interim (branch `fix-disclaimer-mime-233`)
  are superseded; the Sieve script + MtaStageData binding are removed on upgrade.

## Verification

Live on mx: plain-only, html-only, and multipart/alternative messages submitted
over authenticated `:587` must deliver with the disclaimer present in every
shown part, HTML markup intact, DKIM valid, and attachments unchanged; a
disclaimer-disabled domain must pass through unmodified. (Pending implementation.)
