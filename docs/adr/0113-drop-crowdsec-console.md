# ADR-0113: Drop CrowdSec Console integration

**Status:** Accepted — 2026-06-03
**Supersedes:** ADR-0062 (Console enrollment — enroll-only)
**Related:** ADR-0053 (CrowdSec over fail2ban), ADR-0061 (allowlists), ADR-0063 (per-scenario profile override)

## Context

jabali integrated with [CrowdSec Console](https://app.crowdsec.net)
in M27 Step 4 (ADR-0062) so operators could see alerts and decisions
across all their engines from one cloud dashboard. The integration
was minimal: enroll via `cscli console enroll <key>`, toggle alert-
forwarding flags (custom/tainted/manual/context), disenroll from
the web UI.

Two production hosts (mx + puzzle) have been running enrolled for
several weeks. Both hit the **community-tier alert cap** on 2026-06-02:

- Quota: 500 alerts/month per account, ALL engines combined
- Actual: mx 195/24h + puzzle 84/24h = ~280/day → ~8,400/month
- Result: Console shows "Recover 334 missing alerts" banner; the
  newest 90%+ of alerts are silently dropped at the upload

The cap is on the dashboard upload only — local enforcement is
unaffected. But the dashboard is the entire reason the integration
exists: with most alerts missing, the cross-engine view is unreliable
and the per-engine drill-down is strictly worse than the per-host
data jabali already surfaces locally.

Meanwhile the per-host `/jabali-admin/security` shell has grown to
cover every Console panel that matters:

| Console panel | jabali replacement |
|---|---|
| Engine identity (hostname, version, IP, last activity) | Overview tab + status card |
| Alerts list (24h) | Alerts sub-tab |
| Decisions table | Active decisions sub-tab |
| Scenarios installed | Hub sub-tab |
| Bouncers + last pull | Bouncers sub-tab |
| Blocklists subscribed | Blocklists sub-tab |
| Allowlists | Allowlist sub-tab |
| Per-scenario remediation | Per-scenario sub-tab |
| Captcha config | Captcha sub-tab |
| AppSec geoblock | Block Country sub-tab |
| Sensitivity preset | Sensitivity sub-tab |
| **Alerts over time chart (24h/7d/30d)** | Overview → AlertsOverTimeCard (PR #156) |

The only gap is **multi-engine fleet view**. That's an "if/when we
need it" problem, addressable via a jabali-controlled aggregator
(no third-party quota) and not blocked by removing Console.

## Decision

Remove CrowdSec Console integration from jabali entirely.

Specifically:

1. **install.sh** — on every `jabali update`, run
   `cscli console disable -a` if any forwarding flag is still active.
   cscli v1.7.8 has no `disenroll` verb; `disable -a` is the full off-
   switch (no alert ever leaves the host once all five flags are off,
   even though `online_api_credentials.yaml` stays put so CAPI
   blocklist pull keeps working). Remove the legacy
   `/etc/jabali/.cs-console-enrolled` marker.
2. **Agent** — delete the six `security.crowdsec.console.*` verbs
   (`enroll`, `status`, `enrollment`, `disenroll`, `enable`,
   `disable`). Drop registration from `init()`.
3. **panel-api** — delete the six `/admin/security/crowdsec/console/*`
   routes.
4. **panel-ui** — drop the `ConsoleCard` component, the `"console"`
   sub-tab from `Security → CrowdSec`, the `"console"` entry from
   the sub-tab whitelist guard, and the five Console-related hooks
   from `useSecurityCrowdsec.ts`.

### What stays

- **CAPI** (`api.crowdsec.net/v3/decisions/stream`) — community
  blocklist download. ~21k IPs/day. Free, unlimited, no enrollment
  beyond the install-time machine credential. `crowdsec-firewall-
  bouncer` continues pulling this list into nftables sets at the
  60s cadence set by PR #155.
- **Hub** — `cscli hub install/upgrade` keeps pulling parsers,
  scenarios, AppSec rules, and collections from the public Hub. No
  Console enrollment required.
- **All on-host enforcement** — LAPI bans, AppSec WAF, ssh-bf, vpatch
  CVE rules, per-scenario profiles, sensitivity preset, captcha
  remediation, geoblock — every detection and enforcement path is
  100% local to the host and unaffected.

### Operator migration

`jabali update` is idempotent: the heal runs once on the next
upgrade and the engine ends with all forwarding flags off. No
operator action required. The Console account at
[app.crowdsec.net](https://app.crowdsec.net) still exists and the
operator can manually re-enable forwarding flags via cscli if they want, but
the UI no longer surfaces this and install.sh will re-disable them
on the next update.

Hosts that subscribed to **console-managed blocklists** (Firehol
LEVEL1/2, etc.) stop receiving those blocklists once forwarding +
management flags are off. Only the
free `crowdsecurity/community-blocklist` (CAPI-served) continues to
populate the nftables drop sets. Operators wanting richer blocklists
can either re-enroll manually (off the jabali-supported path) or
add custom acquisition rules — install.sh's
`install_crowdsec_blocklists` function still teardown-only, but the
ground is clear for a local Firehol fetcher if demand emerges.

## Consequences

**Positive:**

- No more "Recover 334 missing alerts" banner
- No silent alert drop at the cloud upload — what the local UI shows
  is what the engine recorded, period
- One fewer outbound TLS connection per LAPI heartbeat cycle
- ~250 lines of agent verbs + ~70 lines of API routes + ~250 lines
  of UI deleted (cleanup)
- Removes a feature that violated the M27 "no third-party quota
  in the panel critical path" implicit constraint

**Negative:**

- Lose the cross-engine fleet view (was never quota-safe anyway)
- Lose console-managed blocklist subscriptions on hosts that had
  them (community blocklist still covers the high-value cases)
- Lose the Console-side "view alerts across the org" affordance
  when operators support customers who are also CrowdSec users

**Neutral:**

- ADR-0062 is superseded; M27 Step 4 retroactively reverted

## Implementation

Single PR. Files touched:

- `install.sh` — heal block + comment refresh
- `panel-agent/internal/commands/security_crowdsec.go` — delete
  ~250 lines (M27 Step 4 console section + 6 init registrations)
- `panel-api/internal/api/security_crowdsec.go` — delete 6 routes
- `panel-ui/src/shells/admin/security/AdminSecurityCrowdsec.tsx` —
  delete `ConsoleCard` + helper maps + tab item + subTabs entry +
  hook imports + two unused icon imports surfaced by tsc
- `panel-ui/src/hooks/useSecurityCrowdsec.ts` — delete 5 hooks +
  `CrowdsecConsoleOption` type + `CrowdsecConsoleEnrollment` type
- `docs/adr/0062-console-enrollment-machine-scope.md` — mark
  SUPERSEDED with a forward-pointing note

## Future

If jabali ever needs a fleet-wide view (multi-tenant operators,
support orgs running multiple jabali deploys), build a jabali-native
aggregator: one host opts in as central, others register via
long-lived API token, each pushes alerts every minute to
constellation API. Multi-host overview without anyone's quota on
the critical path. Not blocked by this ADR; just deferred until
demand exists.
