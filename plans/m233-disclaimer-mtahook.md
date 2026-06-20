# Blueprint: Disclaimer via MTA Hook (lossless HTML) — GH #233

**Status:** DRAFT (spike complete, contract confirmed on mx)
**Issue:** #233 — disclaimer corrupts HTML mail
**Target ADR:** 0143 (supersedes 0052; amends 0050/M25)

## Problem

The Sieve disclaimer (ADR-0052) cannot append to mail without corrupting it:

- `replace` (no `:mime`) rewrites a part as `text/plain` → the `text/html`
  alternative gets downgraded and leaks raw `<hr><p>` markup (johnnyq's report).
- `replace :mime` fixes the content-type, BUT Sieve's `extracttext`
  **de-tags HTML** (`<p>Hello <b>x</b></p>` → `Hello x`), so the original HTML
  formatting is destroyed regardless.

Sieve fundamentally cannot do a lossless inline HTML disclaimer. The only
mechanism that can is a DATA-stage body rewrite in real code: an **MTA hook**.

## Why this reopens M25

Stalwart's MtaHook posts over `reqwest` (`http(s)://` TCP only — confirmed in
`crates/smtp/src/inbound/hooks/client.rs`). `MtaMilter` is `hostname:port`
(TCP). Neither supports a unix socket. M25/ADR-0050 forbade TCP loopback
("Spike B"). A disclaimer body-rewrite needs one of these, so we reopen that
rule **narrowly**: a single loopback-only, token-authenticated HTTP endpoint.

## Confirmed contract (spiked live on mx, Stalwart 0.16.6)

**Register an `MtaHook` (JMAP `x:MtaHook/set`, `@type` form):**
```json
{"@type":"MtaHook","url":"http://127.0.0.1:<port>/","stages":{"data":true},
 "enable":{"@type":"Expression","else":"true"},
 "httpAuth":{...bearer...},"timeout":30000,
 "tempFailOnError":false,"maxResponseSize":<bytes>,"allowInvalidCerts":false}
```
- `stages` is a JMAP **set** → map `{"data":true}` (NOT an array).
- `enable`/Expression-typed fields → `{"@type":"Expression","else":"true"}`.
- `timeout` is a **number of ms** (not a duration string).
- Needs a `ReloadSettings` action after the set (same as the Sieve path).

**Request Stalwart POSTs (JSON):**
```
{ context:{stage,client,sasl,server,queue,protocol},
  envelope:{ from:{address}, to:[{address}] },
  message:{ headers:[[k,v],…], contents:"<raw body only>", size:N } }
```
- `envelope.from.address` → the sender domain we key the disclaimer on.
- `message.headers` = top-level headers (incl. `Content-Type` + boundary).
  Values carry a leading space and trailing `\r\n` as received.
- `message.contents` = **body only** (the MIME parts, with their *original*
  Content-Transfer-Encoding, e.g. base64 — NOT decoded, NOT incl. top headers).

**Response:**
```
{ "action":"accept",
  "modifications":[ {"type":"replaceContents","value":"<new body>"} ] }
```
- `replaceContents` → `ReplaceBody`: **replaces body only**. Top headers are
  preserved by Stalwart. Return body-only (same boundary), NOT a full message.
- **DKIM is signed AFTER the hook** — body edits don't break the signature
  (verified: delivered message carried a valid `bh=` over the new body).
- Fires on authenticated `:587` submission (the real webmail/client path).
- `action:accept` + empty `modifications` = clean passthrough (no disclaimer).

## Design

New tiny service **`jabali-mailhook`** (Go), systemd unit, bound
`127.0.0.1:<port>`:

1. One `POST /` handler. Bearer-auth (shared token; Stalwart `httpAuth`).
2. Parse request → `envelope.from.address` → domain.
3. Look up disclaimer: `SELECT disclaimer_text FROM domains WHERE name=?
   AND disclaimer_enabled=1 AND email_enabled=1` via the existing
   **`jabali-stalwart-ro`** MySQL grant (read-only; add `domains` read).
   None/disabled → `{action:accept}` (no modifications). Fail-open.
4. Rewrite: reconstruct `headers + "\r\n" + contents`, parse with
   **`github.com/emersion/go-message`** (battle-tested MIME r/w, handles CTE).
   Walk leaf parts:
   - `text/plain` → decode, append `\r\n\r\n-- \r\n<text>`, re-encode (keep CTE).
   - `text/html`  → decode, insert disclaimer before `</body>` (case-insensitive;
     append if absent), re-encode. HTML-escape operator text.
   **Preserve the original top-level boundary** so the returned body matches the
   preserved `Content-Type` header. Strip top headers; return body only.
5. Size guard: if rewritten body > `maxResponseSize`, passthrough + log.

### Why a dedicated service (not the agent)

The agent is unix-socket-only (M25). Folding a TCP listener into it muddies that
boundary. A separate minimal binary is easier to firewall, reason about, and
keeps the agent's no-TCP invariant intact. Many-small-services.

## Stalwart config (agent owns it)

`domain.disclaimer_apply` no longer renders Sieve. Instead the agent ensures the
single global `MtaHook` exists (idempotent create/verify + ReloadSettings), and
**removes the legacy `jabali-disclaimer` Sieve script + MtaStageData binding**.
The hook reads disclaimer state from the DB live, so per-domain apply is just
"ensure hook registered" — the DB write (panel-api) is the source of truth.

## install.sh

- Generate hook bearer token → `/etc/jabali-panel/mailhook.token` (0640 root:jabali-mail).
- `jabali-stalwart-ro` grant += `SELECT` on `jabali_panel.domains`.
- Install `jabali-mailhook` binary + systemd unit (After=mariadb; loopback bind;
  reads token + DB ro password; hardening: NoNewPrivileges, ProtectSystem, etc.).
- Register the `MtaHook` in Stalwart (via agent on first boot / reconcile).
- Defensive nft/ufw note (loopback not externally routable; document).
- Remove any leftover `jabali-disclaimer` Sieve script on upgrade.

## Removal of the Sieve path

Delete `renderDisclaimerSection`/`buildDisclaimerScript`/`parseDisclaimerSections`
Sieve generation + `ensureDataStageDisclaimerBinding`. Keep `domain.disclaimer_apply`
as the "ensure hook + (DB already written)" trigger. Drop the `replace :mime`
interim patch (branch `fix-disclaimer-mime-233`) — superseded.

## Tests

- Unit (mailhook): MIME rewrite over plain-only / html-only /
  multipart-alternative / multipart-mixed+attachment / base64 + quoted-printable
  + 8bit CTE / non-ASCII / missing `</body>` / oversized → passthrough.
  Assert: HTML markup preserved, disclaimer present in both parts, boundary
  unchanged, attachments untouched.
- Agent: hook register/verify idempotent; legacy Sieve removal.
- Live (mx): the 3 message types from a real `:587` submit; assert delivered
  HTML keeps `<b>`/structure AND shows the disclaimer; DKIM still valid;
  attachment passthrough; disabled domain = no change.

## Security (ADR-0143)

- Loopback-only bind; bearer token; one endpoint; read-only DB user.
- Fail-open (deliver without disclaimer) on hook error/oversize — a disclaimer
  outage must not bounce mail. `tempFailOnError:false`.
- Private data: the hook sees full message bodies in memory only; never logs
  bodies; logs domain + decision only.

## Steps

1. ADR-0143 (supersede 0052; amend 0050) + this blueprint.
2. `jabali-mailhook` service: HTTP handler + auth + DB lookup.
3. MIME rewrite (go-message) + unit tests.
4. Agent: register MtaHook (JMAP, confirmed shape) + remove Sieve path.
5. install.sh: token, grant, unit, hook registration, legacy cleanup.
6. panel-api/reconciler: disclaimer_apply → ensure-hook (no Sieve).
7. Live validation on mx (3 message types + DKIM + attachments + disabled).
8. Update memory + ADR status ACCEPTED after smoke.

## Open question for review

- Port choice for the loopback hook (e.g. 127.0.0.1:8462) — confirm no clash.
- Disclaimer placement in HTML: before `</body>` (chosen) vs end-of-document.
