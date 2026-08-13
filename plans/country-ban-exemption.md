# Country ban exemption (CrowdSec)

> Blueprint + runbook for the "never block selected countries" feature.
> ADR: [docs/adr/0166-country-ban-exemption.md](../docs/adr/0166-country-ban-exemption.md).
> Branch: `feat/country-ban-exemption`.

## Goal

Let an operator pick ISO-3166 countries whose IPs must **never be blocked by
CrowdSec**, from any decision source: local scenario bans, AppSec inband 403s,
CAPI/community blocklists, console blocklists, captchas.

This is the complement of the existing AppSec geoblock ("Block Country" tab),
which is an HTTP-layer allow/deny gate and says nothing about bans.

## Mechanism (two layers)

1. **LAPI AllowList `jabali-country-allowlist`** seeded with each selected
   country's full CIDR set. LAPI AllowLists integrate with every CrowdSec
   component (AppSec, scenario overflows, console blocklists — CrowdSec docs,
   verified v1.7.8). This is the "never blocked" guarantee.
   CIDR source: ipdeny aggregated zones, IPv4 **and** IPv6 files (separate).
   Per-entry comment `country:<CC>` so removal/refresh can diff per country.
2. **GeoIP parser whitelist** at
   `/etc/crowdsec/parsers/s02-enrich/zz-jabali-country-allowlist.yaml`
   (`evt.Enriched.IsoCode in [...]`), rendered by the agent, atomic
   tmp+rename + `systemctl reload crowdsec`. Stops locally-parsed events
   (incl. AppSec-datasource events) from ever becoming alerts; saves CPU and
   covers CIDR-set churn between refreshes.

## Scale (measured on testserver, 2026-08-13)

- IPv4 CIDR counts: IL 775 · CN 5.5k · RU 8.6k · DE 8.7k · US 29.4k
- IPv6 CIDR counts: IL 162 · CN 2k · US 10.6k
- Import speed: 775 entries ≈ 3.5s; 5k entries ≈ 26s in a single cscli call
  (batch ≥4000/call; 500-batches are ~3× slower). Full US ≈ 2.5 min.
- Refreshes are diff-only; steady state is near-zero work.

## Precedence decisions

- Country exemption **wins over the AppSec geoblock deny-list** for those
  countries (LAPI AllowLists evaluate before AppSec `pre_eval` — ADR-0061).
  The UI warns when a country is selected in both.
- Exemption does **not** touch UFW, egress policies, or the AppSec geoblock
  itself — those remain operator-controlled layers.

## Steps

1. **Docs** — this blueprint + ADR-0166.
2. **Migration 000262** — `server_settings.country_exempt_countries
   VARCHAR(1000) DEFAULT ''` (CSV of ISO codes; empty = feature off).
3. **panel-agent** (`security_crowdsec.go`):
   - `security.crowdsec.country_exempt.set {countries:[CC,...]}` — render the
     s02-enrich whitelist atomically + reload; empty list = remove file.
     Golden test pins the rendered file.
   - `security.crowdsec.country_allowlist.sync {adds:[{value,comment}],
     removes:[value]}` — parameterize the existing allowlist helpers
     (`jabaliAllowlistName` was a baked-in const) so a second named allowlist
     works; chunk-safe.
4. **panel-api**:
   - Routes `GET|PUT /admin/security/crowdsec/country-exemption` in
     `security_crowdsec.go` (agent-first-then-DB, geoblock pattern).
   - `internal/countryexempt/`: ipdeny fetch (v4+v6), snapshot cache under
     `/var/lib/jabali-panel/country-zones/`, diff vs `cscli allowlists
     inspect` (LAPI truth), push deltas to the agent in ≤4000-entry chunks.
     Fetch failure → keep last good snapshot, log + retry next tick.
   - Refresh goroutine (in-process, ctx-tied — repo convention): daily tick,
     re-sync when the snapshot is older than 7 days or the selection changed.
5. **CLI** — `jabali crowdsec country-exempt get|set|sync` in
   `crowdsec_cmd.go`, mirroring geoblock; audits `crowdsec.country_exempt_set`.
6. **UI** — "Country ban exemption" card on the Allowlist tab
   (`AdminSecurityCrowdsec.tsx`), reusing `ISO3166_COUNTRIES` + the
   memoised Select pattern; hooks in `useSecurityCrowdsec.ts`; warns when a
   selected country is also in the geoblock deny-list; i18n keys in
   `en/common.json`.
7. **Tests** — agent golden + handler tests; api handler tests (validation,
   agent-first ordering, empty-list = off); diff logic unit tests.
8. **Docs finish** — runbook section here, ADR flip to Accepted, BLUEPRINT
   entry.

## Runbook

### Verify the parser whitelist

```
printf '93.184.216.34 - - [13/Aug/2026:10:00:00 +0000] "GET /wp-login.php HTTP/1.1" 404 522 "-" "curl/8"' \
  | cscli explain -f- --type nginx
# expect: jabali/country-allowlist (~2 [whitelisted])
```

### Verify the LAPI allowlist

```
cscli allowlists inspect jabali-country-allowlist | head
cscli allowlists inspect jabali-country-allowlist | grep -c country:
```

### Force a re-sync

```
jabali crowdsec country-exempt sync
```

### Existing bans are NOT retroactively removed

Whitelists prevent new decisions only. Clean up per docs:
`cscli decisions delete --scope ip --value <ip>` or by scenario.

### ipdeny is unreachable

The last good snapshot under `/var/lib/jabali-panel/country-zones/` keeps
the feature converged; the refresher logs a warning and retries next tick.
