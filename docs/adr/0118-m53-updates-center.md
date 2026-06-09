# ADR-0118 — M53 Updates Center

**Status:** Accepted (2026-06-09)
**Supersedes:** —
**Amends:** ADR-0064 (M29 admin updates) — the M29 thin-proxy updates page becomes a stateful surface; the eight M29 agent-proxy endpoints are unchanged.

## Context

M29 (ADR-0064) shipped `/jabali-admin/updates` as a thin proxy: every value came from a live agent call, nothing was persisted, and the page was two stacked "check / run / poll" cards. The redesign (operator-requested, full build) wants a real admin surface: at-a-glance stat cards, **security severity** on OS packages, a **scheduled auto-update** path, **run history** that survives a page reload, and a **changelog** feed.

Three design questions had real forks; the operator chose:

1. **Auto-update mechanism** — *unattended apt + opt-in jabali*.
2. **Changelog source** — *GitHub releases API* (public mirror).
3. **Severity classification** — *apt security-suite tag* (coarse, zero deps).

## Decision

1. **Severity from the apt suite token, zero extra calls.** `apt list --upgradable` already prints the candidate's target suite as `name/SUITE`; `system.apt_check` flags a package `security` when `SUITE` contains `security` (e.g. `stable-security`) and returns a `security_total`. No debsecan, no second apt invocation.

2. **State persisted in three tables (migration 000163).**
   - `update_state` (singleton id=1) — last check snapshot (jabali behind/sha, apt total/security, checked-at). Lets the page render instantly without an agent round-trip.
   - `update_history` — one row per run; powers Recent History.
   - `update_autoupdate_config` (singleton id=1) — desired auto-update state.
   Schema only; the two singleton rows are seeded by the app on first serve (`EnsureDefault`), never by the migration (per the M24 000057 "Dirty database" scar).

3. **Run completion recorded by a reconciler, not by UI polling.** Runs are `systemd-run --no-block` transient units, so panel-api never synchronously sees terminal state, and UI-poll-only recording loses rows when the page is closed. panel-api logs a `running` history row at dispatch; the **run reconciler** polls the unit each tick and flips it to `success`/`failed` — independent of any open UI. To make the success/failure signal readable, both run handlers **drop `systemd-run --collect`**: a *failed* oneshot lingers in systemd `failed` state (readable), while a *successful* one is garbage-collected to inactive. Mapping: `active|activating|reloading` → still running; `failed` (or non-zero `ExecMainStatus`) → failed; anything else (inactive/gone) → success.

4. **Auto-update is DB-as-truth, converged by a reconciler.** `PUT /admin/updates/autoupdate` validates `HH:MM` and saves the singleton; the **autoupdate reconciler** calls the agent's `system.autoupdate_apply` each tick. apt security patches run via the stock `unattended-upgrades` path (toggled by `20auto-upgrades` periodic flags, scheduled by an `apt-daily-upgrade.timer` `OnCalendar` drop-in). jabali self-update is **opt-in, default OFF** — a `jabali-autoupdate.timer` runs `jabali update -f`; a bad self-update can brick the panel, so it never auto-applies unless the admin explicitly enables it.

5. **`autoupdate_apply` is self-gating.** It renders desired file contents, writes only files whose bytes changed, `daemon-reload`s only on change, and flips a timer's enable state only on `is-enabled` drift — so the per-tick reconcile is a cheap no-op once converged (per `feedback_per_tick_idempotent_loops`). Times are re-validated `HH:MM` at the agent boundary and rendered through fixed templates; no value is shell-interpolated into a unit file.

6. **Changelog from the public GitHub mirror, cached.** `services/changelog.go` fetches `api.github.com/repos/shukiv/jabali-panel/releases` (a fixed constant URL — no user input, no SSRF surface), caches 6h, and serves the last-good cache on any fetch error. Until the first release is cut the API returns `[]` and the UI shows an empty state.

7. **UI rebuild, keeping the M29 cards.** The page gains 3 stat cards, a security column + "security only" filter on the packages table, an Automatic Updates card (two switches + TimePickers), a Recent History table, and a Changelog timeline — while the M29 Jabali/Apt check+run+poll cards are preserved verbatim.

8. **New install dependency.** `unattended-upgrades` is added to the base package list in install.sh (per `feedback_deps_in_installer`). The `jabali-autoupdate.{service,timer}` units are written by the agent on converge, so install.sh ships no unit templates.

## Consequences

**Wins.**
- Operators see security exposure at a glance; the page loads without waiting on the agent.
- Run history is durable — a run logged from one browser is visible in another, and survives a panel restart.
- Hands-off OS security patching with a panel-controlled schedule; risky panel self-update is available but firmly opt-in.
- Changelog needs no new infra and degrades gracefully (empty / stale) instead of erroring.

**Costs / risks.**
- Dropping `--collect` leaves a failed run unit in `failed` state until the next run's `reset-failed`; acceptable and intentional (it's the readable-failure signal).
- The changelog is empty until the project starts cutting GitHub releases.
- `unattended-upgrades` running on its own timer means OS patches can land outside `jabali update`; the schedule is operator-controlled and security-only by default.

## Verification

`go build ./...`, `go vet`, package tests (agent commands, api, repository, reconciler, services) green; `tsc -b` + `vite build` clean. Live smoke on 192.168.100.150 pending: security badge against a real `*-security` package, schedule writes `20auto-upgrades` + the timer drop-in, a run produces a history row that the reconciler finishes, changelog empty-state.
