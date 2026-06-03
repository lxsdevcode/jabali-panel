# ADR-0114 — Mail-bf via Stalwart webhook (SUPERSEDED)

**Status:** Superseded by [ADR-0115](./0115-stalwart-bruteforce-crowdsec-parser.md)
**Date:** 2026-06-03
**Relates-to:** ADR-0061 (CrowdSec extensions), PR #173 (panel-bf), PR #174 (webmail/api-token-bf)

## Why superseded

This ADR proposed a webhook → panel-api → cscli pipeline for catching
Stalwart auth failures. Before any code landed, two pieces of context
made the design obsolete:

1. **Stalwart has a built-in auto-ban mechanism** (`security.authBanRate`,
   `security.scanBanRate`, etc.) — covered in the Stalwart `auto-ban`
   docs at <https://stalw.art/docs/server/auto-ban/>. It tracks both
   source-IP *and* login-name (catches distributed bruteforce against
   one account, which the webhook approach would not). Enforcement is
   in-process only — no firewall propagation — but the detection logic
   is already there and is upstream-supported.

2. **A community CrowdSec integration exists** at
   <https://github.com/bu5hm4nn/crowdsec-stalwart> — parser plus five
   scenarios (auth-bf, user-enum, SMTP scan, rate limit, HTTP vuln
   scan). Tested against Stalwart 0.10+ (we run 0.16, so the regex
   will need re-validation on real log lines), but the YAML shape is
   directly drop-in for CrowdSec.

Together these collapse the design to "configure Stalwart's tracer
to emit plaintext log lines, point CrowdSec at them, vendor the
community parser+scenarios, disable Stalwart's built-in to avoid
double-banning." That removes the need for: a new panel-api socket
route, an in-memory leaky-bucket implementation, an `internal/crowdseccli`
subprocess wrapper, and ownership of the auth-failure detection
contract.

The replacement decision lives in ADR-0115. The original webhook
sketch below is preserved as historical context.

---

## Original context (preserved)

After PRs #173 + #174 landed, the jabali panel has CrowdSec coverage
for every HTTP-side bruteforce surface (`/self-service/login`,
`/self-service/recovery`, `/sessions/whoami`, `/webmail/auth`,
`/api/v1/* 401`). The remaining blind spot is the mail protocol
family: SMTP submission (587/465), IMAPS (993), POP3S (995), and
SIEVE (4190). All four expose `AUTH` over the internet on every
jabali host, all four are standard credential-stuffing targets, and
none of the stock CrowdSec scenarios (`crowdsecurity/postfix-bf`,
`crowdsecurity/dovecot-bf`) parse Stalwart's log format.

Live investigation on puzzle (2026-06-03) found:

- `/etc/stalwart/config.json` is a 2-key bootstrap pointer to a
  RocksDb internal store. ALL service config — tracer, ports,
  certs, auth — lives inside `/var/lib/stalwart/`.
- Stalwart 0.16+ defaults to `tracer.store` (writing structured
  events into the same RocksDb). It does NOT log to stdout, file,
  or journal by default.
- Editing tracer keys in `config.json` is silently ignored on
  restart unless they reach the store.

## Original decision (preserved as the design that was rejected)

1. Stalwart fires an `auth.failure` webhook to panel-api on every
   failed mail-protocol auth.
2. panel-api exposes `POST /internal/mail/auth-failure` on the
   panel-api unix socket only.
3. panel-api emits CrowdSec decisions via cscli, not via direct
   LAPI HTTP.
4. Scenario lives in panel-api code, not in CrowdSec — in-memory
   leaky-bucket per source IP keyed on the Sensitivity preset.
5. Source-IP handling honours only the connecting peer (no
   Forwarded/X-Forwarded-For trust).
6. Sensitivity tuning table extends symmetrically with the other
   bf scenarios (relaxed/balanced/strict).
7. Scope: mail-bf only.

## Why this was the wrong call

- **Duplicated detection logic** that Stalwart already implements
  (distributed-IP per-account tracking is a feature of
  `security.authBanRate` we'd have re-implemented in the panel-api
  bucket).
- **Coverage gap**: webhook fires for auth failures only. The
  bu5hm4nn integration covers four additional categories (user-enum,
  SMTP scan, rate-limit, HTTP vuln scan) we'd otherwise need separate
  ADRs and PRs for.
- **Larger code surface in panel-api** for behaviour that fits
  CrowdSec's native shape exactly.
- **Stalwart admin-API work is still required either way** (webhook
  subscription OR tracer config), so we don't save the admin-API
  client by going the webhook route.

ADR-0115 picks up from here.
