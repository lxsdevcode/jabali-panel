# Runbook — Send-as delegation (GH #347)

Lets one mailbox send email **From** another mailbox's address without receiving
that mailbox's mail. Mechanism + rationale: `docs/adr/0156-send-as-delegation.md`.

## Configure (operator / tenant)

1. Mail → open the **delegate** mailbox (the login that will send, e.g. `support@`)
   → **Edit**.
2. **Can send as** section → pick a mailbox in the same domain (e.g. `sales@`) →
   **Add**. Repeat for each grantor.
3. Remove a permission with the trash icon.

That is all — the change is pushed to Stalwart immediately. The grantor keeps its
own inbox; only *sending* is delegated.

## What happens under the hood

- Row written to `mailbox_send_delegations` (delegate + grantor mailbox FKs).
- panel-api recomputes **all** pairs and calls the agent `mail.sendas.reconcile`.
- The agent rebuilds `MtaStageAuth.mustMatchSender` into an expression that is
  `false` only for verified `(delegate, grantor)` pairs, then
  `stalwart-cli create Action/ReloadSettings`.
- On every panel-api boot the expression is re-derived from the table (self-heals
  after a DB restore / Stalwart reset).

## Verify (server)

```bash
TOK=$(cat /etc/jabali-panel/stalwart-admin.token)
export STALWART_URL=http://127.0.0.1:8446 STALWART_USER=admin STALWART_PASSWORD=$TOK
# current expression (should list your delegate/grantor pairs; "true" if none):
stalwart-cli get MtaStageAuth --json | python3 -c 'import sys,json;print(json.load(sys.stdin)["mustMatchSender"]["else"])'
```

SMTP submission test (587, STARTTLS, auth as the delegate):

```
MAIL FROM:<grantor@dom>   -> 250  (delegated: accepted)
MAIL FROM:<someoneelse@>  -> 501 5.5.4 not allowed to send from this address (rejected)
```

Mail sent **to** the grantor still lands in the grantor's own inbox.

## Troubleshoot

- **Delegated send is rejected.** Check the expression carries the pair (command
  above). If the table has the row but the expression is stale, restart
  `jabali-panel` (boot reconcile re-applies) or re-toggle the permission in the UI.
- **Config edits look ignored.** MtaStage config is applied by
  `stalwart-cli create Action/ReloadSettings`, **not** `InvalidateCaches`.
- **Everyone's submission is rejected.** A malformed expression is rejected at
  `stalwart-cli update` before it can apply, so this should not happen; if it does,
  reset with `stalwart-cli update MtaStageAuth --json '{"mustMatchSender":{"else":"true","match":{}}}'`
  then `create Action/ReloadSettings`, and check panel-api logs for a build error.
- **Grantor stopped receiving mail.** Not caused by this feature — `queryRecipient`
  is untouched; investigate separately.

## Scope / limits

- Grantor must be an enabled mailbox in the **same domain** as the delegate.
- The expression grows one clause per delegation pair (fine at normal scale; the
  reconcile logs the pair count).
