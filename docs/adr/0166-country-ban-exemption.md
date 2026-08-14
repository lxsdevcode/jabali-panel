# ADR-0166 — Country ban exemption via LAPI AllowList + GeoIP parser whitelist

- Status: Accepted
- Date: 2026-08-13
- Tracks: country ban exemption feature
- Plan: [plans/country-ban-exemption.md](../../plans/country-ban-exemption.md)

## Context

Operators asked for a way to make selected countries **never get blocked** —
exempt from every CrowdSec decision, not just filtered at the HTTP layer.

The existing AppSec geoblock (ADR-0060, "Block Country" tab) is an HTTP-edge
allow/deny gate. It does not stop the log-processing pipeline from creating
ban decisions, and a ban decision drops the IP at the firewall on *all* ports
— including HTTP, before the AppSec layer ever sees it. So geoblock
Allow-list mode cannot deliver "this country is never blocked".

CrowdSec has no country primitive. The supported building blocks (docs,
verified live on CrowdSec v1.7.8):

- **LAPI AllowLists** (cscli, ≥1.6.8) exempt an IP/CIDR across **all**
  components: scenario overflows, the AppSec component (inband 403s),
  console/community blocklists, captchas. IP/CIDR only.
- **Parser whitelists** (s02-enrich expressions) discard events before
  scenarios; with geoip-enrich they can match `evt.Enriched.IsoCode`.
  Verified: a `zz-`-prefixed file runs after `geoip-enrich` and whitelists
  by country. AppSec is an engine datasource (`source: appsec`), so its
  events pass through the same pipeline.
- Scale measured on testserver: US v4 ≈ 29.4k CIDRs imports in ≈ 2.5 min
  with ≥4000-entry cscli batches; typical countries are 1–9k.

## Decision

Two layers, reconciled from `server_settings`:

1. **`jabali-country-allowlist` LAPI AllowList** seeded with the selected
   countries' full CIDR sets. **Source of truth is the local MaxMind mmdb**
   (`/var/lib/crowdsec/data/GeoLite2-City.mmdb`, Country DB as degraded
   substitute): the agent (root — the mmdb is `0600 root`, panel-api runs
   unprivileged) walks the tree with `maxminddb-golang`, applies the exact
   `GeoIpCity` IsoCode precedence (Country → RegisteredCountry →
   RepresentedCountry) and returns the raw networks
   (`security.crowdsec.country_zones.derive`); panel-api merges adjacent
   CIDRs, snapshots them under `/var/lib/jabali-panel/country-zones/`,
   diffs against `cscli allowlists inspect` (LAPI is truth — ADR-0061) and
   pushes only deltas to the agent. **ipdeny aggregated zones (IPv4 + IPv6)
   remain as fallback** when no mmdb exists or a country has no
   data-bearing networks. Per-entry comment `country:<CC>` makes
   per-country diff/removal possible. Converged by a 60s refresher tick
   (selection change, last-sync failure, or a 15-min verify); snapshots
   self-manage 7-day staleness, and mmdb-sourced snapshots additionally
   expire when the classifier's mmdb mtime passes the snapshot marker
   (`security.crowdsec.mmdb.stat`).
2. **GeoIP parser whitelist** rendered by the agent at
   `/etc/crowdsec/parsers/s02-enrich/zz-jabali-country-allowlist.yaml`,
   atomic tmp+rename + SIGHUP reload (same write discipline as the geoblock
   AppSec config). Empty country list = file removed, feature off.

State lives in `server_settings.country_exempt_countries` (CSV). The PUT
handler calls the agent first, then persists (geoblock ordering, ADR-0060),
so a failed host write never drifts the DB.

**Supplemental CIDRs** (`server_settings.country_exempt_extra_cidrs`, CSV;
migration 000263): operator-supplied IPs/CIDRs merged into the same
allowlist tagged `country:extra` — managed entries, removed when removed
from settings. For stragglers a country's zone data misses and known-good
hosts abroad. Validated at the API/CLI edge (`NormalizeCIDR`: prefixes
masked, bare IPs → host prefixes), capped at 1000 entries.

**Precedence: exemption wins over the geoblock deny-list.** AllowLists
evaluate before AppSec `pre_eval` (ADR-0061), so an exempted country ignores
a deny-list entry for the same country. This is the intended semantic —
"never blocked" is the stronger statement. The UI warns when a country
appears in both lists.

## Consequences

- The two layers used to read different geo data sources — parser whitelist
  MaxMind, CIDR sets ipdeny — so an IP could geo-locate to an exempt country
  in MaxMind while ipdeny's set did not cover it (observed: 182.54.236.64 =
  IL in MaxMind, absent from ipdeny's IL zone), leaving it exposed to
  CAPI/community decisions. **Resolved 2026-08-14**: the CIDR sets are now
  derived from the same mmdb the classifier reads, so allowlist ≡ classifier
  by construction (an IP that resolves to a country record always falls
  inside the data-bearing network that yielded it, and that network is in
  the allowlist). Freshness tracks the mmdb's mtime. Residual risks,
  accepted: (a) decisions whose *range* only partially overlaps an exempt
  country are not skipped by LAPI (containment check); (b) while the ipdeny
  fallback is in effect (no mmdb), the original mismatch class applies.
- A third-party CIDR source (ipdeny) remains in the supply chain as
  fallback only. Failure mode is soft: keep the last good snapshot, warn,
  retry. DB-IP Lite is a drop-in alternative if ipdeny ever goes away.
- First jabali-managed file under `/etc/crowdsec/parsers/s02-enrich/`;
  same drift story as the geoblock — rewritten on every PUT/sync, preserved
  across `jabali update` because the renderer is deterministic from DB state.
- Existing bans are not retroactively removed (CrowdSec semantics); the
  runbook documents `cscli decisions delete` for cleanup.
- AppSec inband exemption via LAPI AllowLists is claimed by CrowdSec docs;
  **verified live on testserver (2026-08-13)**: the AppSec acquisition layer
  logs `… is allowlisted by <ip> from jabali-country-allowlist (country:IL),
  skipping` and does not evaluate the request.
- The US-scale initial import takes minutes; it runs as a background sync
  with operator-visible progress, never inline in the PUT request.
