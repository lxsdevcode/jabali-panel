# M53 — Updates Center (Updates Page Redesign)

Status: BLUEPRINT (2026-06-09). Approved scope: "Full build (all panels)".
Branch: `m50/updates-center` off origin/main `3dcbf47a` (incl #313).
ADR target: **0118**. Next free migration: **000163** (000162 = docker_apps_data_bytes).
Milestone **M53** (M44/M45 are gaps; M46-M49 taken — sequential = M53).

## Goal

Rebuild `/jabali-admin/updates` from the M29 thin-proxy into a real admin
surface, per the mockup: stat cards (panel / OS / security count) → Panel
update card → System Packages table (with **security severity** badge) →
**Automatic Updates** schedule → **Recent History** → **Changelog**.

## Locked design decisions (user, 2026-06-09)

1. **Auto-update** = *unattended apt + opt-in jabali*. OS security patches via
   `unattended-upgrades` on a panel-controlled schedule; jabali self-update
   auto-applies ONLY if admin opts in (default OFF — bad self-update bricks
   the panel).
2. **Changelog source** = *GitHub releases API* — `api.github.com/repos/
   shukiv/jabali-panel/releases` (public mirror, no auth, 200). Caveat: 0
   releases today → empty-state until we cut releases. Cache in DB, TTL 6h.
3. **Severity** = *apt security-suite tag* (coarse, zero deps). `apt list
   --upgradable` already shows the target suite in the `name/SUITE` token
   (`Source` field = m[2]); `strings.Contains(lower(suite), "security")` →
   security. No debsecan.

## Existing surface (M29 — keep working through the rebuild)

- `panel-api/internal/api/admin_updates.go` — thin proxy, 8 routes
  (`/admin/updates/{jabali,apt}/{check,run,status}` + DELETE stop).
- agent: `system.update_check/run/status`, `system.apt_check/run/status`,
  `system.unit_stop`. Run = `systemd-run --unit=… --no-block --collect`;
  status reads journalctl since `started_at`.
- apt_check returns `{packages:[{name,current,new,source}],total}`.
- UI: `panel-ui/src/shells/admin/updates/SystemUpdatesPage.tsx`.

## Run-completion mechanism (resolves async/poll wrinkle)

`systemd-run --no-block --collect` means panel-api never synchronously sees
the run's terminal state, and UI-poll-only recording loses rows when the page
is closed. Resolution:
- panel-api inserts an `update_history` row `status='running'` at run-issue
  time (single API write path).
- a reconciler tick (`update_run_reconciler`) queries the agent for the unit's
  terminal Result/ExecMainStatus and marks the row finished — independent of
  whether any UI is polling. Idempotent: only acts on rows still `running`.
- DROP `--collect` for the run units (or add `system.update_result` reading
  `systemctl show <unit> -p Result,ExecMainStatus,ActiveState` before clear)
  so the terminal state survives long enough for the reconciler to read it.

## Schema (migration 000163)

`000163_update_center.up.sql` — three tables (down drops all three):

```sql
CREATE TABLE update_state (
  id              TINYINT PRIMARY KEY DEFAULT 1,
  jabali_behind   INT NOT NULL DEFAULT 0,
  jabali_current_sha CHAR(40) NULL,
  jabali_checked_at  TIMESTAMP NULL,
  apt_total       INT NOT NULL DEFAULT 0,
  apt_security    INT NOT NULL DEFAULT 0,
  apt_checked_at  TIMESTAMP NULL,
  CONSTRAINT chk_update_state_single CHECK (id = 1)
);

CREATE TABLE update_history (
  id           CHAR(26) PRIMARY KEY,
  kind         VARCHAR(16) NOT NULL,   -- 'jabali' | 'apt'
  action       VARCHAR(16) NOT NULL,   -- 'check' | 'run'
  status       VARCHAR(16) NOT NULL,   -- 'success' | 'failed' | 'running'
  security_count INT NOT NULL DEFAULT 0,
  summary      VARCHAR(255) NOT NULL DEFAULT '',
  log_excerpt  TEXT NULL,
  unit         VARCHAR(128) NULL,      -- transient unit name, for reconciler match
  started_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at  TIMESTAMP NULL,
  KEY idx_uh_started (started_at)
);

CREATE TABLE update_autoupdate_config (
  id              TINYINT PRIMARY KEY DEFAULT 1,
  apt_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
  apt_time        VARCHAR(5) NOT NULL DEFAULT '03:30',
  jabali_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
  jabali_time     VARCHAR(5) NOT NULL DEFAULT '04:30',
  updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT chk_uac_single CHECK (id = 1)
);
```

Seed singleton rows from the **app** (EnsureDefault on first serve), NOT the
migration — migrations are schema only ([[feedback_migration_data_seed_ordering]]).

## Waves (inline, no agents per project rule)

### Wave A — severity + persisted state (additive, ship-alone safe)
1. agent `system_apt_check.go`: add `Security bool json:"security"` to
   `aptUpgradablePackage` (`strings.Contains(strings.ToLower(suite),"security")`,
   suite = the `/SUITE` token) + `SecurityTotal int` on the response. Update
   the existing parse test with a `bookworm-security` line.
2. Migration 000163 (3 tables above).
3. models + repos: `UpdateState` (Get/Upsert), `UpdateHistory`
   (List(limit)/Insert/MarkFinished), `UpdateAutoupdateConfig` (Get/Upsert) +
   EnsureDefault. Mirror panel_certificate singleton repo.
4. Wire EnsureDefault in serve.go. `go test ./...`.

### Wave B — history + state endpoints (depends A)
5. `admin_updates.go`: on run-issue insert `running` history row; check/run
   completion upserts `update_state`. New routes:
   - `GET /admin/updates/state`   → UpdateState (cheap page-load summary)
   - `GET /admin/updates/history?limit=20` → []UpdateHistory
6. `update_run_reconciler.go` (NEW): mark `running` rows finished from agent
   terminal state. Idempotent ([[feedback_per_tick_idempotent_loops]]).

### Wave C — auto-update schedule (depends A; parallel to B)
7. agent `system_autoupdate.go` (NEW): `system.autoupdate_apply`
   {apt_enabled,apt_time,jabali_enabled,jabali_time}:
   - apt: write `/etc/apt/apt.conf.d/20auto-upgrades` + `apt-daily-upgrade.timer`
     override OnCalendar=apt_time; ensure `unattended-upgrades` pkg (install.sh).
   - jabali: write/remove `jabali-autoupdate.{service,timer}` (ExecStart
     `jabali update -f`) at jabali_time; enable/disable.
   - atomic tmp+rename; `systemctl daemon-reload` + enable/disable.
   - validate time `^([01]\d|2[0-3]):[0-5]\d$` at agent boundary.
   `system.autoupdate_status` reads back timer state.
8. endpoints: `GET /admin/updates/autoupdate`, `PUT /admin/updates/autoupdate`
   (validate → save DB → agent apply).
9. `update_autoupdate_reconciler.go` (NEW, mirror panel cert): converge DB→host;
   compare-gate side-effects ([[feedback_per_tick_idempotent_loops]]).
10. install.sh: `apt-get install -y unattended-upgrades`; ship
    `jabali-autoupdate.{service,timer}` DISABLED by default; `jabali update`
    prelude step to sync units on existing installs
    ([[feedback_jabali_update_prelude_vs_buildsteps]]).

### Wave D — changelog (depends nothing; parallel)
11. `panel-api/internal/services/changelog.go` (NEW): fetch fixed-const URL
    (SSRF-safe, no user input), 10s ctx timeout, cache+fetched_at TTL 6h,
    serve last-good on error. Map to `{tag,name,published_at,body}`.
12. `GET /admin/updates/changelog` → {entries:[],cached_at}.

### Wave E — UI rebuild (depends B/C/D shapes)
13. Rebuild `SystemUpdatesPage.tsx`: 3 stat cards (panel behind / OS upgradable
    / **security** red) · Panel card (Check/Update) · System Packages AntD Table
    w/ red **security** Tag + "security only" filter · Automatic Updates (2
    Switches + TimePicker → PUT) · Recent History Table · Changelog
    Timeline/List w/ "No releases yet" empty-state. Hooks in
    `panel-ui/src/hooks/useSystemUpdates.ts` (TanStack Query).
14. `npm run build` ([[feedback_panel_ui_use_npm_run_build]]); fresh worktree
    has no node_modules → symlink from main repo's `panel-ui/node_modules`,
    build, then unlink (don't rm).

### Wave F — ADR + verify
15. ADR 0118. Live smoke on 192.168.100.150: security badge real, schedule
    writes `20auto-upgrades` + timers, history rows on run, changelog empty-state.

## Open caveats
- Changelog empty until first GitHub release cut (offer to tag one after M53).
- jabali unattended self-update opt-in + default OFF (brick risk).
- All apt/time/kind inputs validated at REST **and** agent boundary; values
  rendered into systemd/apt files via fixed templates after charset/HH:MM
  validation — never string-interpolated into shell.
