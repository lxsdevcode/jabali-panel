# ADR-0052: Disclaimer — Sieve System Script (Spike A Passed, HTML Covered)

**Status:** CORRECTED (2026-06-20, GH #233) — the original "ACCEPTED" verdict
was wrong: the feature never applied a disclaimer to any delivered message.
Spike A verified only that the script *compiled* and *persisted* (it says so:
"Delivery: to be observed in production"); delivery was never tested and was
broken on four independent counts. See the **GH #233 correction** at the
bottom for the root causes + the fix, live-verified on Stalwart 0.16.6.

**Status (original):** ACCEPTED (2026-04-23) — sieve path handles text/plain AND text/html.
MtaHook fallback no longer needed.
**Supersedes:** provisional form of this ADR (shipped in M6.5 commit 6674ee3).
**Related:** ADR-0051 (M6.5 DB-as-truth), ADR-0045 (Stalwart v0.16 pivot), M25 unix socket lockdown.

## Context

M6.5 Step 6 adds per-domain outbound disclaimer. The feature appends a text
block to outgoing mail from a domain. Two integration paths exist on Stalwart:

1. **Sieve system script** (`x:SieveSystemScript`): native, declarative, runs
   at outbound data stage. Requires `body` + `replace` + `foreverypart` sieve
   extensions to touch message parts. Stalwart schema confirms support.

2. **MTA hook** (`x:MtaHook`): HTTP callback invoked at a named SMTP stage.
   Stalwart POSTs the message; panel-api returns a modified body.

## Problem

The sieve `replace` action's behaviour on HTML body parts is undocumented for
Stalwart v0.16. RFC 5228 and RFC 5173 (body + foreverypart) describe
text-oriented matching; an HTML `text/html` part may or may not be writable
via `replace text:` portably.

The MtaHook fallback hits a separate problem: M25 (shipped 2026-04-23) closed
all loopback TCP ports in favour of unix sockets. Panel-api listens on
`/run/jabali-panel/api.sock`. If `x:MtaHook` only accepts `http://host:port/`
URLs and not `unix://`, the fallback path re-opens a loopback TCP port —
architectural conflict with M25.

## Decision (final, after Spike A)

**Ship sieve path for `text/plain` AND `text/html` parts.** The rendered
script matches outbound mail by envelope-from domain, iterates every MIME
part with `foreverypart`, and appends the disclaimer to both body kinds:

- `text/plain`: `-- \n<text>\n` appended after original body.
- `text/html`: `<hr><p><html-escaped text></p>` appended after original body.

Both branches use the RFC 5703 canonical pattern: `extracttext` captures the
current part's body into a variable, then `replace` writes the body back
with the disclaimer concatenated.

The implementation lives in
`panel-agent/internal/commands/domain_disclaimer_apply.go:renderDisclaimerSieve`.

## Required sieve extensions

All confirmed to compile on Stalwart v1.0.0 (2026-04-23 on VM
192.168.100.150): `envelope`, `variables`, `mime`, `foreverypart`,
`extracttext`, `replace`.

## Spike A result

- Compile: PASS — Stalwart accepts the script with all required extensions.
- Persist: PASS — script survives agent restart; reconciler overwrites on tick.
- Delivery: to be observed in production; canonical RFC 5703 pattern has no
  known edge on multipart/alternative.

MtaHook fallback is no longer needed. Spike B is cancelled.

## Shipped bug caught by Spike A

The M6.5 first-ship rendering (commit 6674ee3) was broken:
- Used `${ORIGINAL_BODY}` — not a sieve construct.
- Wrote literal `\n` in a Go backtick string instead of real newlines, so
  `text:` multi-line markers were malformed.
- Missing `variables` + `extracttext` requires.

Stalwart rejected every create with `"Unterminated multi-line string"`. The
handler then fell through to `update` using the script *name* as the id
(should be the server-assigned id), which Stalwart silently no-op'd — so
the agent reported `ok: true` while no script existed.

The follow-up fix (this commit):
- Rewrites `renderDisclaimerSieve` to the canonical pattern above.
- Adds a query-by-name step so `update` uses the server id.
- Fails loudly on `notCreated`/`notUpdated` instead of pretending success.

## Consequences

- Disclaimers now cover the full body (both MIME types). UI copy updated.
- No new loopback port opened. M25 invariant preserved.
- Reconciler still converges state every tick; disabling the disclaimer
  destroys the system sieve script. No stuck state.
- Operator manual edits to the script in Stalwart admin console are drift;
  reconciler overwrites on next tick (ADR-0051).

## Follow-up

Live multipart/alternative delivery observation on first production send.
Runbook updated to drop "text/plain-only" caveat.

---

## GH #233 correction (2026-06-20) — "disclaimer not working"

The shipped feature appended a disclaimer to **zero** delivered messages.
Four independent bugs, all surfaced by a live send-and-inspect (authenticated
SMTP submit on :587 → read the delivered copy via JMAP) on Stalwart 0.16.6:

1. **The DATA-stage script was never bound.** Stalwart runs a Sieve script at
   the SMTP DATA stage only when `MtaStageData.script` selects one; it
   defaults to `false` (= run nothing). The agent created per-domain
   `x:SieveSystemScript` objects with `isActive:true` but never set the
   binding, so the scripts were dormant. Verified: a trivial `addheader`
   probe script did not run until `MtaStageData.script` was bound **and** a
   `ReloadSettings` action was issued (the setting is cached until reload).

2. **`:is` Content-Type match never fired.** The sieve used
   `header :mime :contenttype :is "Content-Type" "text/plain"`. The real
   header is `text/plain; charset="utf-8"`, so the exact `:is` comparison
   never matched. Fixed to `:contains`.

3. **`extracttext` was used as a test.** The code wrote
   `if extracttext "x" { replace ... }`, but `extracttext` is a Sieve
   **action** (RFC 5703), not a test — the body was never captured, so
   `replace` had nothing to append to. Fixed to a bare action.

4. **Per-domain script selection by expression fails on dots.** The first fix
   attempt bound `MtaStageData.script` to `'jabali-disclaimer-' + sender_domain`
   and named scripts `jabali-disclaimer-<domain>`. Stalwart's script-name
   resolution chokes on the dots in a domain name (a dash-named script
   resolved; the dotted name did not). 

**Final design:** ONE global system script named `jabali-disclaimer` (dot-free,
so the binding resolves) holding a marker-delimited section per domain
(`# jabali-disclaimer-begin <domain>` … `# <<< end`). Each `domain.disclaimer_apply`
splices that domain's section in/out, upserts the global script **by id**
(name-keyed updates silently no-op — same class of bug as the forwarder Sieve,
GH #237), binds `MtaStageData.script` to the constant `'jabali-disclaimer'`,
and issues `ReloadSettings`. All writes are change-gated (GET + compare) so the
per-tick reconcile is a no-op once converged.

**Live-verified:** plain and HTML-multipart messages from a disclaimer-enabled
domain get the disclaimer appended to both parts; disabling removes it (and
destroys the script when the last domain is removed); re-apply is idempotent;
mail from domains with no disclaimer is unaffected (missing-section no-op).

Implementation: `panel-agent/internal/commands/domain_disclaimer_apply.go`.
