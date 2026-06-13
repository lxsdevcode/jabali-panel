# Mail-provider DNS templates (GH#181)

**Status:** BLUEPRINT — not yet dispatched. Advisor-reviewed.
**Target ADR:** docs/adr/0120-mail-provider-dns-templates.md
**Origin:** GH#181 (johnnyq) — "select a DNS template when adding a domain;
if you don't use Jabali for mail (Microsoft 365 / Google Workspace / no
mail at all), don't add Jabali's mail DNS records or mail SSL cert."

## Why

Today every domain that has email enabled gets Jabali's full mail record
set (MX→mail.<d>, autoconfig/autodiscover CNAMEs, IMAP/submission SRVs,
DKIM, TLS-RPT) **and** a mail SSL cert whose SANs include
mail/autoconfig/autodiscover/mta-sts.<d>. For a domain whose mail lives at
Microsoft 365 / Google / nowhere, those records are wrong and the mail
cert's aux SANs can't validate (the GH#132 autodiscover 404 class). The
operator wants to pick the mail destination at domain-add and have the
panel publish the right records — or none.

## What already exists (reuse, don't reinvent)

- **`domain.EmailEnabled`** (migration 000054) — gates the whole Jabali
  mail path: DNS email records (reconciler `ensureTenantEmailEnabled` →
  `dnscompile.BuildEmailRecords`), the mail cert
  (`mail_certificate_reconciler.go` skips when `!EmailEnabled`), mailbox
  creates, disclaimer/catch-all.
- **`domain.SkipAutoSAN`** (ADR-0070) — excludes the auto-added
  mail/autoconfig SANs from the domain cert. Exactly the "don't put mail
  SANs on the cert" primitive.
- UI: `DomainCreate.tsx`, `DomainEdit.tsx`, `DomainEmailSection.tsx`,
  `DomainSkipAutoSANToggle.tsx`.

So "No mail" is ~80% already possible (EmailEnabled=false + SkipAutoSAN);
it just isn't a first-class choice at create. The genuinely new work is
the **external-provider record templates** + the **provider selector**.

## Key design decisions

1. **One enum field drives everything: `domain.mail_provider`.**
   Values: `jabali` (default) | `none` | `m365` | `google`. EmailEnabled
   and SkipAutoSAN are **derived** from it, never set independently —
   this avoids the drift class (`domain.Update allowlist silent drop`,
   `EmailEnabled vs provider` getting out of sync):
   - `jabali` → `EmailEnabled=true`, `SkipAutoSAN=false` (today's behavior).
   - `none`   → `EmailEnabled=false`, `SkipAutoSAN=true`, publish **no**
     mail records.
   - `m365`/`google` → `EmailEnabled=false` (Jabali is not the MTA),
     `SkipAutoSAN=true`, publish the **provider's** records.
   The API computes EmailEnabled/SkipAutoSAN from mail_provider on
   create + edit; the columns stay for the reconciler/cert paths that
   already read them (no churn there).

2. **External records come from a pure compiler, mirroring
   `BuildEmailRecords`.** New `dnscompile.BuildExternalMailRecords(
   provider, zoneName string, tokens ProviderTokens) []DNSRecord`. Record
   content is provider **constants** (no user input in MX/SPF strings);
   only the optional token fields (below) carry user data, and they are
   validated.

3. **Provider record sets (v1):**
   - **m365:**
     - `@` MX `0 <domain-dashed>.mail.protection.outlook.com`
       (`example.com` → `example-com`).
     - `@` TXT `"v=spf1 include:spf.protection.outlook.com -all"`.
     - `autodiscover` CNAME `autodiscover.outlook.com`.
     - *(token)* `selector1._domainkey` CNAME
       `selector1-<dashed>._domainkey.<onmicrosoft>` and `selector2`
       likewise — emitted only when the `m365_onmicrosoft` token is set.
   - **google:**
     - `@` MX `1 smtp.google.com` (modern single-MX; legacy 5×ASPMX is
       out of scope — Google recommends the single record for new setups).
     - `@` TXT `"v=spf1 include:_spf.google.com ~all"`.
     - *(token)* `google._domainkey` TXT with the operator-supplied DKIM
       value — emitted only when `google_dkim` token is set.
   - **none:** empty.
   - **All providers** also get a baseline `_dmarc` TXT
     `"v=DMARC1; p=none; rua=mailto:postmaster@<zone>"` (same as the spirit
     of today's set; safe default, user can tighten).

4. **Non-apex records are tagged by a provider-specific `ManagedBy`
   sentinel** (`mail-provider-m365`, `mail-provider-google`) so a provider
   switch prunes the previous set cleanly (delete-by-ManagedBy scope, same
   pattern as `EmailRecordsManagedBy`, whose value is `m6`). The
   non-apex rows are: jabali's `m6` set (DKIM/autoconfig/autodiscover/SRVs/
   TLS-RPT from `BuildEmailRecords`) vs the provider's CNAME/DKIM rows.
   Switching to `jabali` re-publishes the `m6` set + prunes `mail-provider-*`;
   to `m365`/`google` publishes that provider's rows + prunes `m6`; to
   `none` prunes both. The apex MX/SPF/DMARC are a SEPARATE scope handled in
   decision #5 — do not conflate them with this set.

5. **CENTRAL DECISION — apex MX/SPF/DMARC ownership across all four
   modes.** This is the spine of the feature; getting it wrong yields
   invalid DNS. Today the apex `@ MX mail.<d>`, apex `@ TXT v=spf1 mx
   ip4:… ~all`, and `_dmarc TXT` come from **`BootstrapRecords`** with
   **`managed_by = NULL`** (deliberately, so the `m6` email-disable prune
   can't touch them) — they are written ONCE at domain-create and are then
   operator-editable. `BuildEmailRecords` does NOT emit them. So a provider
   switch cannot use the `m6`/`mail-provider-*` sentinel prune to fix the
   apex; it must **replace** the apex MX + SPF in place, and there must
   never be two `v=spf1` TXT at the apex (RFC 7208 → permerror, silently
   breaks SPF for the whole domain).

   Resolution: introduce a managed scope **`mail-apex`** that owns exactly
   one apex MX + one apex SPF + one `_dmarc`, written per-provider by the
   mail-provider reconciler (delete-by-`managed_by=mail-apex` then insert,
   so a switch can never duplicate SPF). Per provider:
   - `jabali` → MX `mail.<d>` pri 10; SPF `v=spf1 mx ip4:<v4>[ ip6:<v6>] ~all`
     (today's `BuildSPFString`); DMARC today's `p=quarantine` default.
   - `m365` → MX `<dashed>.mail.protection.outlook.com` pri 0; SPF
     `v=spf1 include:spf.protection.outlook.com -all`; DMARC `p=none` baseline.
   - `google` → MX `smtp.google.com` pri 1; SPF
     `v=spf1 include:_spf.google.com ~all`; DMARC `p=none` baseline.
   - `none` → no apex MX/SPF/DMARC at all.

   **Migration of the existing NULL-managed apex rows** (the load-bearing
   bit): `BootstrapRecords` stops emitting apex MX/SPF/DMARC; the
   mail-provider reconciler becomes the single authority, re-stamping the
   existing apex mail rows to `managed_by=mail-apex` (or deleting+reinserting
   under the new scope). Operator hand-edits to apex MX/SPF/DMARC are
   superseded once a provider is selected — the operator picks a *preset*
   OR hand-manages mail DNS via `none`, not both. The apex **A** record and
   the `mail` **A** record (web + MX target) are NOT touched — web stays put.

6. **CAA is a web-cert concern, not mail.** It currently lives in
   `BuildEmailRecords` but gates the LE *web* cert, which issues regardless
   of mail provider. Move CAA into the always-published web/bootstrap set
   so it survives a switch to external/none mail.

7. **Token validation (anti-injection):**
   - `m365_onmicrosoft`: `^[a-z0-9-]{1,63}\.onmicrosoft\.com$` (or bare
     label → suffix it). Reject anything else.
   - `google_dkim`: DNS-TXT-safe (base64 + `p=`/`v=DKIM1` shape), length
     ≤ 2048, no embedded quotes/newlines.
   Tokens are optional; empty → DKIM records simply omitted.

8. **Cert path is already correct via SkipAutoSAN.** No mail SANs on the
   cert when provider≠jabali; the web cert (domain + www) issues normally.
   A provider switch must trigger a cert **reissue** (drop/restore mail
   SANs) — wire the existing reissue trigger on the edit path.

## Steps / waves

| Step | Summary | Key files |
|------|---------|-----------|
| 1 — ADR + migration | ADR-0120. Migration: `domains.mail_provider VARCHAR(16) NOT NULL DEFAULT 'jabali'`, `domains.m365_onmicrosoft VARCHAR(255) NULL`, `domains.google_dkim TEXT NULL`. Backfill per Open Question 1 (email_enabled=0→none; email_enabled=1+skip_auto_san=0→jabali; the email_enabled=1+skip_auto_san=1 ADR-0070 combo maps to jabali but PRESERVES skip_auto_san — no clobber). | new migration, `docs/adr/0120-*` |
| 2 — model + API derive | `models.Domain.MailProvider/M365Onmicrosoft/GoogleDKIM`. Create + edit handlers: validate provider+tokens, **derive** `EmailEnabled`/`SkipAutoSAN` from provider. Dedicated `UpdateMailProvider` repo method (no allowlist). | `internal/models/domain.go`, `internal/api/domains.go`, `internal/api/domain_email.go`, repo |
| 3 — external record compiler | `dnscompile.BuildExternalMailRecords` + provider ManagedBy sentinels + token-validated DKIM rows. Move CAA to the web/bootstrap set. Unit tests per provider (record shape, dashed-MX, token on/off, injection-reject). | `internal/dnscompile/external_mail_records.go` (new), `email_records.go` |
| 4 — apex re-ownership (load-bearing) | Move apex MX/SPF/DMARC out of NULL-managed `BootstrapRecords` into a `mail-apex` managed scope; reconciler writes exactly one of each per provider (delete-by-scope + insert → never a duplicate SPF). Migration re-stamps existing apex mail rows. | `internal/dnscompile/bootstrap.go`, `internal/dnscompile/apex_mail_records.go` (new), migration |
| 5 — reconciler publish/prune | Reconciler chooses the per-provider record set (apex `mail-apex` + provider CNAME/DKIM `mail-provider-*` OR `m6`) and prunes the other scopes on switch. Idempotent (publish-on-diff). Purge PDNS zone cache after a switch. | new `mail_provider_reconcile.go` |
| 6 — cert reissue on switch | Confirm `SkipAutoSAN` excludes mail SANs (exists); ensure a provider change enqueues a domain-cert reissue. | `mail_certificate_reconciler.go`, domain cert reconciler |
| 7 — UI | `DomainCreate`: "Mail" select (Jabali mail / No mail / Microsoft 365 / Google Workspace) + conditional token inputs. `DomainEdit`: `DomainMailProviderSection.tsx` with the same + a "switching re-publishes DNS and reissues the cert" note. Hide the raw EmailEnabled/SkipAutoSAN toggles when a non-jabali provider is selected (they're derived). | `panel-ui/src/shells/admin/domains/*` |
| 8 — E2E + runbook + docs | Playwright: create domain as m365 → assert MX/SPF/autodiscover rows + no mail SANs on cert + EmailEnabled=false. Switch google→jabali → assert prune + jabali records + mail cert reissue. Update `docs/site/dns.md` + `docs/site/mail.md` + `docs/site/admin/domains.md`. | tests, `plans/mail-provider-dns-templates-runbook.md`, docs |

## Security invariants

- `mail_provider` validated against the enum server-side; unknown → 400.
- Record content for MX/SPF/autodiscover is **constant** per provider —
  no user string flows into it. Only DKIM tokens carry user data and are
  regex/charset-validated before becoming record content.
- EmailEnabled/SkipAutoSAN are derived, never client-settable alongside a
  provider (one source of truth).
- Switching providers is **destructive to the previous provider's managed
  DNS rows** (by design) but scoped strictly by ManagedBy sentinel — never
  touches operator-authored (unmanaged) records.
- PDNS cache: purge the zone after any provider switch (per the
  `pdns_control purge` rule) so resolvers don't serve stale MX.

## Out of scope (v1)

- Providers beyond M365 + Google (Zoho, Fastmail, Proton, custom). The
  enum is forward-compatible; add cases later.
- Auto-fetching DKIM keys from the provider's API (M365 Graph / Google
  admin) — operator pastes the token, or adds DKIM manually post-create.
- Legacy Google 5×ASPMX record set (single `smtp.google.com` only).
- M365 Intune/enterprise enrollment CNAMEs.
- Per-subdomain provider override beyond what a separate domain row gives.
- Changing the apex web A record or the web cert behavior (mail-only).

## Open questions

1. **Backfill mapping + the ADR-0070 carve-out.** Map
   `email_enabled=0 → none`; `email_enabled=1 AND skip_auto_san=0 → jabali`.
   The third combo, **`email_enabled=1 AND skip_auto_san=1`**, is the
   ADR-0070 state ("Jabali mail but suppress mail SANs on the cert") and
   has no clean provider — the model says `jabali ⇒ skip_auto_san=false`.
   To avoid a SILENT REVERT (backfilling it to `jabali` would force
   skip_auto_san=false → next reconcile re-adds mail SANs → cert reissue
   the operator didn't ask for), the migration **preserves existing
   `skip_auto_san`** and the `jabali ⇒ skip_auto_san=false` derivation
   fires only on an **explicit** provider re-selection in the UI. So a
   legacy ADR-0070 row reads as `provider=jabali` with `skip_auto_san=1`
   until the operator actively changes it — a documented carve-out, not an
   invariant violation in flight. (Box check 2026-06-13: 0 such rows on
   the test host, but other deployments may have them.)
2. DMARC default for external providers — publish `p=none` baseline, or
   leave DMARC entirely to the user? Proposal: publish the baseline (safe,
   visible, tightenable) — but make it omittable if the user already has a
   `_dmarc` record (don't clobber an operator-authored one).
3. Should `none` still allow the operator to later flip on Jabali mailboxes
   without re-adding the domain? Yes — provider is editable; flipping to
   `jabali` enables mail. No re-create needed.
