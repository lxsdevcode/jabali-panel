# M-Security-GUI Phase 2 — full closure of #714/#715/#716/#717

Goal: satisfy every QA closure criterion across the four Security-tab GUIs. Phase 1
(drawers, classification, decision detail, incident triage) shipped. This plan
covers the remaining backend-heavy long tail, decomposed into PR-sized steps.
Each step: branch → PR → all 4 CI checks green → merge (the PR gate is now
enforced on `main`).

## Already landed (PR #719, bounded UI)
- #715 flip-to-enforce confirm gate (would-deny acknowledgement).
- #714 AIDE category + change-type filters.
- #717 per-rule incident history (loaded window).

## Steps

### Step 1 — #716 CrowdSec health cards (agent + panel + UI)  [HIGHEST VALUE: #716 has no Phase-2 yet]
- Agent: extend `security.crowdsec.status` (or new `security.crowdsec.health`) to return:
  engine running (`systemctl is-active crowdsec`), LAPI reachable (`cscli lapi status`),
  firewall bouncer + nginx bouncer active (`cscli bouncers list`), AppSec on,
  captcha configured, last `crowdsec -t` config-validation result + last reload.
- Panel: `GET /admin/security/crowdsec/health`.
- UI: a row of health-status cards atop the CrowdSec tab (green/red per layer) +
  a "Validate config" button surfacing `crowdsec -t`.
- Verify: mock the health payload in an E2E; assert cards render red/green.

### Step 2 — #714 per-file metadata + full-diff export
- Agent: enrich `aideSampleRow` with old/new sha256, size, owner/group, mode, mtime,
  package owner (`dpkg -S <path>`) when available (parse the AIDE report's detail lines).
- Panel: pass through; add `GET /admin/security/aide/diff` streaming the full report.
- UI: expandable row (old vs new metadata) + a "Download full diff" button.
- Verify: unit-test the report-detail parser against a fixture AIDE report.

### Step 3 — #715 app-profile coverage view + richer denial fields
- Agent: `security.apparmor.status` returns a coverage summary (expected app profiles
  vs loaded) + parse exe/uid/pid/requested_mask/denied_mask into `apparmorDenial`.
- UI: a coverage card (which app types confined vs unconfined) + show the richer
  denial fields in the profile drawer + a profile-mode/component filter.

### Step 4 — #717 enforcement-readiness scoring + disable-reason audit
- Agent: none (derive score client-side from incident volume/age) OR a small
  `readiness` field per rule.
- Panel: on rule-disable, require a `reason`; write an audit row (reuse the audit table).
- UI: a readiness badge per rule (green/amber/red from incident history) + a required
  reason prompt on disable.

### Step 5 — #716 alert↔decision linkage + recent-changes/audit panel
- Agent: link alert → scenario → decision in the decisions/alerts payload; expose
  a recent-changes feed (hub installs/removes, sensitivity/profile/captcha/geoblock
  edits) from the settings-change audit rows.
- UI: cross-links in the decision drawer + a "Recent changes" panel.

### Step 6 — #714 baseline preview + rebuild audit trail
- Agent: `aide rebuild --preview` (dry-run: what would be accepted) + record who/when
  rebuilt the baseline.
- UI: preview-before-rebaseline modal + confirmation + audit display.

## Ordering / parallelism
Steps 1–4 are independent (different tabs) → parallelizable. Steps 5–6 depend on the
audit-row infra from Step 4. Recommended: 1 (highest value), then 2/3/4 in any order,
then 5/6.

## Closure
An issue closes when its QA criteria are all met:
- #716: Steps 1 + 5.
- #714: Steps 2 + 6 (+ the shipped filters).
- #715: Step 3 (+ the shipped confirm gate).
- #717: Step 4 (+ the shipped history).
