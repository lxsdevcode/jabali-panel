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
   countries' full CIDR sets (ipdeny aggregated zones, IPv4 **and** IPv6 —
   the v6 file is separate and easy to forget). Per-entry comment
   `country:<CC>` makes per-country diff/removal possible. panel-api fetches
   the zones (agent never opens outbound — ADR-0050), snapshots them under
   `/var/lib/jabali-panel/country-zones/`, diffs against `cscli allowlists
   inspect` (LAPI is truth — ADR-0061) and pushes only deltas to the agent.
   Refreshed on a weekly staleness check by an in-process goroutine.
2. **GeoIP parser whitelist** rendered by the agent at
   `/etc/crowdsec/parsers/s02-enrich/zz-jabali-country-allowlist.yaml`,
   atomic tmp+rename + SIGHUP reload (same write discipline as the geoblock
   AppSec config). Empty country list = file removed, feature off.

State lives in `server_settings.country_exempt_countries` (CSV). The PUT
handler calls the agent first, then persists (geoblock ordering, ADR-0060),
so a failed host write never drifts the DB.

**Precedence: exemption wins over the geoblock deny-list.** AllowLists
evaluate before AppSec `pre_eval` (ADR-0061), so an exempted country ignores
a deny-list entry for the same country. This is the intended semantic —
"never blocked" is the stronger statement. The UI warns when a country
appears in both lists.

## Consequences

- A third-party CIDR source (ipdeny) enters the supply chain. Failure mode is
  soft: keep the last good snapshot, warn, retry. DB-IP Lite is a drop-in
  alternative if ipdeny ever goes away.
- First jabali-managed file under `/etc/crowdsec/parsers/s02-enrich/`;
  same drift story as the geoblock — rewritten on every PUT/sync, preserved
  across `jabali update` because the renderer is deterministic from DB state.
- Existing bans are not retroactively removed (CrowdSec semantics); the
  runbook documents `cscli decisions delete` for cleanup.
- AppSec inband exemption via LAPI AllowLists is claimed by CrowdSec docs;
  flagged for live end-to-end verification on a test box before the feature
  is called prod-proven (trigger a benign AppSec match from an exempted IP).
- The US-scale initial import takes minutes; it runs as a background sync
  with operator-visible progress, never inline in the PUT request.
