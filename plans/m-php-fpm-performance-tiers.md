# M — PHP-FPM Performance Tiers (package-gated) — GH #339 Phase 2 follow-up

**Status:** DRAFT blueprint — self-reviewed (adversarial pass inline, per the
project's no-subagent rule). Ready for Wave A.

## 0. Objective

Re-introduce user access to PHP-FPM pool tuning — which PR #34 just locked to
admin-only — but **safely**, through a three-tier, hosting-package-gated model.
The admin decides, per package, whether tenants get nothing, a safe preset
dropdown, or clamped advanced knobs. Every value a non-admin can produce is
re-clamped to the package caps server-side at apply time.

### The three tiers

| Tier | Who | UI surface | Gate |
|------|-----|-----------|------|
| **L1** | normal user | "PHP Performance Mode" dropdown: Balanced / Low-memory / High-traffic / WordPress-optimized (presets only, no raw numbers) | package `fpm_user_can_edit` |
| **L2** | advanced user | the individual `pm.*` knobs, **clamped to package caps** | package `fpm_advanced_mode` (implies `fpm_user_can_edit`) |
| **L3** | admin | full raw `pm.*` in the admin **PHP Manager → new "FPM Pools" tab**; pick the PHP version first, then tune that version's pool | admin only |

### Admin controls, per hosting package

- `fpm_max_children_cap` — hard ceiling on `pm_max_children` for any tenant on this package.
- `fpm_worker_mem_mb` — per-worker RAM estimate; drives the soft **memory-impact budget** shown to the admin (`max_children × fpm_worker_mem_mb`). FPM has no hard per-pool memory cap, so this is advisory only.
- `fpm_user_can_edit` (bool) — L1 gate.
- `fpm_advanced_mode` (bool) — L2 gate (advanced implies can-edit).
- Per-PHP-version defaults — the `pm.*` a freshly-provisioned pool on this package gets (a small JSON map keyed by version, or the mode default).

### Performance Mode presets

The 4 modes map to concrete `pm.*` value sets, **admin-editable in the Server
Settings page**. A mode selection resolves to a `pm.*` set, which is then
**re-clamped to the package caps** before write — so a preset can never exceed
what the package allows.

### Scope + reuse (do NOT re-invent)

- **Scope = per-user-per-PHP-version** — today's `php_pools` model (one pool per user per version). True per-domain `pm.*` overrides stay the deferred **#329 Phase 2**; do not attempt them here.
- **Reuse:** `php_pools` schema + the GH #339 `pm.*` columns (migration 000213, already on main); `php.pool.apply` agent command + its template; `resolvePMTuning` (panel-api/internal/api/php_pools.go) for defaults/caps/FPM-constraint validation; both reconcile payloads (`php_pool_reconcile.go` + `reconciler.go`); `PHPPoolEdit.tsx` admin form; `PHPExtensionsTab.tsx` for the version-select tab pattern.
- **Current baseline:** PR #34 made `/php-pools` admin-only (`RequireAdmin`) and removed `UserPHPPoolCard`. This feature adds a NEW user-facing surface (a performance-mode endpoint + a clamped advanced form) — it does NOT simply re-open `/php-pools` to users.

### Migration numbering

Next free migration on `main` is **000214** (212 = mail_group internal_only, 213 = php_pool fuller pm). Use 214, 215 in order; renumber at merge if others land first (audit per the merge-migration scar).

### Conventions / scars to respect

- New route family under `/domains` or `/me`-style user scope, list envelope `{data,total,page,page_size}` — verify the real envelope against the handler, don't trust the blueprint schema (verify-wire-contract scar).
- Reconciler side-effects gated behind a no-change compare (per-tick-idempotent-loops scar).
- Every new system package/none here; `npx tsc -b` (not `--noEmit`) before UI push.
- Live-validate FPM rendering on 192.168.100.86 before declaring any wave done.
- No dedicated break-glass CLI. Migrations = schema only; data seeds live in the app.

---

## 1. Waves & steps

Dependency graph: Step 1 is the foundation. Steps 2 and 3 are parallel after 1.
Step 4 depends on 1. Steps 5 and 6 depend on 1+4 (and 5 needs 2). Step 7 is the
gate.

```
        ┌── Step 2 (presets/Server Settings) ──┐
Step 1 ─┤                                        ├─ Step 5 (L1 user) ─┐
(pkg)   ├── Step 3 (admin FPM Pools tab) ───────┘                     ├─ Step 7 (E2E+ADR)
        └── Step 4 (clamp at apply) ── Step 6 (L2 advanced) ──────────┘
```

---

### Step 1 — Hosting-package FPM controls (foundation)

**Context brief.** `hosting_package` (panel-api/internal/models/hosting_package.go, struct at line 10, has `PHPExecEnabled` at 53) is the per-package policy row. Packages are admin-managed via `panel-api/internal/api/packages.go` + the admin package editor UI. This step adds the FPM policy columns + surfaces them in the admin package editor. No behavior change yet — just the knobs exist.

**Tasks.**
1. Migration 000214: add to `hosting_packages` — `fpm_max_children_cap INT UNSIGNED NOT NULL DEFAULT 20`, `fpm_worker_mem_mb INT UNSIGNED NOT NULL DEFAULT 64`, `fpm_user_can_edit TINYINT(1) NOT NULL DEFAULT 0`, `fpm_advanced_mode TINYINT(1) NOT NULL DEFAULT 0`, `fpm_version_defaults VARCHAR(2000) NOT NULL DEFAULT '{}'` (JSON map `{"8.3":{pm_mode,...}}`; empty = use the global mode default).
2. Model: add the 5 fields (gorm column tags; watch initialisms per the GORM-column-tag scar).
3. API: `packages.go` create/update request + response + the admin handler wiring. Validate `fpm_max_children_cap` ≤ the global admin cap (2000), `fpm_advanced_mode` implies `fpm_user_can_edit` (reject the invalid combo or auto-set).
4. Admin package-editor UI: a "PHP-FPM policy" section — the two toggles, the max-children cap, the per-worker MB, and a read-only **memory-budget** display (`cap × mem_mb` MB). Per-version defaults editor can be a Step-1b stretch or a JSON textarea for MVP.

**Verify.** `go build ./... && go test ./panel-api/internal/api/ ./panel-api/internal/repository/`; migration up+down on 192.168.100.86; admin edits a package, values persist. `npx tsc -b`.

**Exit criteria.** Package columns exist + settable by admin; no user-facing or apply-path change yet. Rollback = migration down.

---

### Step 2 — Performance-mode presets in Server Settings (parallel with 1)

**Context brief.** `server_settings` (panel-api/internal/models/server_settings.go, singleton row) holds global admin config, surfaced in the Server Settings page. The 4 modes need concrete `pm.*` values, admin-editable.

**Tasks.**
1. Migration 000215: `php_performance_modes` table (`mode VARCHAR(24) PK`, `pm_mode`, `pm_max_children`, `pm_start_servers`, `pm_min_spare_servers`, `pm_max_spare_servers`, `pm_max_requests`, `request_terminate_timeout_seconds`, `process_idle_timeout_seconds`). Do NOT seed from the migration (migrations = schema only, per the scar) — seed the 4 default rows from the app on first boot (a repository `EnsureDefaults` called from serve.go, like ManagedIPRepository.EnsureDefault).
2. Default preset values (seeded by the app): Balanced `dynamic/10/2/1/3`, Low-memory `ondemand/5/-/-/-, idle 30`, High-traffic `dynamic/25/6/3/8`, WordPress-optimized `dynamic/15/4/2/6, max_requests 500`.
3. API: admin-only CRUD/PUT for the 4 mode rows (never delete/add — fixed set of 4).
4. Server Settings UI: a "PHP Performance Modes" card — edit the `pm.*` per mode. Reuse the numeric-field patterns; note these are the *ceiling* templates, clamped per-package at apply.

**Verify.** Seed produces exactly 4 rows on a fresh box; admin edits persist; migration up/down. Contract test the envelope against the handler.

**Exit criteria.** 4 admin-editable presets exist; not yet consumed by any tier. Rollback = migration down + drop the EnsureDefaults call.

---

### Step 3 — Admin PHP Manager "FPM Pools" tab (L3) (parallel with 2)

**Context brief.** `panel-ui/src/shells/admin/php/PHPVersionsPage.tsx` renders a tab bar (`versions` / `extensions`), switching between `VersionsTab` and `PHPExtensionsTab`. `PHPExtensionsTab.tsx` already does the "select a PHP version → load that version's data" pattern (`selectedVersion` + `loadExtensions`). `PHPPoolEdit.tsx` is the existing per-pool admin form (has the GH #339 fuller `pm.*` fields). `/php-pools` is admin-only (PR #34).

**Tasks.**
1. Add a third tab `pools` ("FPM Pools") to `PHPVersionsPage.tsx`.
2. New `FPMPoolsTab.tsx`: version-select first (mirror `PHPExtensionsTab`), then — for the selected version — list the pools (per-user) or the current admin's/selected user's pool, and render the `PHPPoolEdit` form inline (or a slimmed copy) for raw `pm.*` editing. Admin = uncapped (L3), full control.
3. Keep the standalone `/jabali-admin/php-pools/edit/:id` route working (or fold it into the tab; if folding, update nav.ts + remove the dead route).

**Verify.** Admin opens PHP Manager → FPM Pools tab → picks 8.4 → edits pm.* → saves → pool applies (live-check the rendered conf on .86). `npx tsc -b`.

**Exit criteria.** Admin tunes any version's pool from the PHP Manager tab. No user-facing change. Rollback = remove the tab.

---

### Step 4 — Clamp-to-package at apply (server-side enforcement)

**Context brief.** `resolvePMTuning` (php_pools.go) currently applies global caps (100/2000 children, etc.) + the FPM dynamic constraint. This step makes it ALSO clamp to the owner's **package** caps, so any non-admin-produced value (L1 preset or L2 knob) is bounded regardless of the request. This is the load-bearing safety layer — L1/L2 UIs are convenience; this is enforcement.

**Tasks.**
1. Extend `resolvePMTuning` (or add `clampToPackage`) to take the owner's `HostingPackage` and clamp `pm_max_children ≤ fpm_max_children_cap`, and the spare/start servers ≤ max_children (already enforced). Admin path skips package clamping (L3 uncapped).
2. Thread the owner's package into the pool create/update + the reconcile apply path (load the package by the pool's user). Cache/lookup carefully — avoid an N+1 in the reconciler loop (batch or per-tick cache).
3. Gate reconciler side-effects behind a no-change compare (idempotent-loops scar).

**Verify.** Unit tests: a request with `pm_max_children=100` on a package capped at 10 → clamped to 10 (not rejected). Admin request unclamped. Live: set a package cap, apply, confirm the rendered conf respects it.

**Exit criteria.** No pool can exceed its package cap via any path. Rollback = revert the clamp (values fall back to global caps).

---

### Step 5 — L1 user "PHP Performance Mode" dropdown

**Context brief.** User PHP Settings page (`UserPHPSettingsPage.tsx`). Per PR #34 the raw `/php-pools` is admin-only; this step adds a NARROW user endpoint that sets a *mode* (not raw numbers). The mode resolves to a preset (Step 2), clamped to the package (Step 4).

**Tasks.**
1. Migration or reuse: store the user's selected mode on the pool (`php_pools.performance_mode VARCHAR(24) NOT NULL DEFAULT 'balanced'`) — a 000216 column, or fold into 214 if not yet merged. Seed existing pools to 'balanced'.
2. User API `PUT /me/php-performance-mode` (or `/domains/... ` — verify the right scope): body `{php_version, mode}`. Auth = user-scoped (own pools only). Handler: reject if `!package.fpm_user_can_edit`; resolve mode→preset; `clampToPackage`; write the pool; fire apply. NOT under the `RequireAdmin` `/php-pools` group.
3. User UI: on PHP Settings, if `package.fpm_user_can_edit`, show a "PHP Performance Mode" Select (Balanced/Low-memory/High-traffic/WP-optimized) with plain-language descriptions — NO raw numbers. Show which mode is active. Place it as its own tab or card per the "should be a tab" feedback.

**Verify.** User with a can-edit package sets "High traffic" → pool gets the clamped High-traffic preset. User with a package that has the toggle off → no dropdown, 403 on the API. E2E.

**Exit criteria.** Opted-in users pick a safe mode; values always within package caps. Rollback = hide the card + 403 the endpoint.

---

### Step 6 — L2 advanced-user knobs (clamped)

**Context brief.** For packages with `fpm_advanced_mode`, expose the individual `pm.*` knobs to the user — but with maxes driven by the package cap (UI) and re-clamped server-side (Step 4).

**Tasks.**
1. Extend the user endpoint (or a sibling) to accept the raw `pm.*` set from a user, user-scoped, gated on `package.fpm_advanced_mode`, run through `clampToPackage`. Still NOT the admin `/php-pools` route.
2. User UI: if `package.fpm_advanced_mode`, an "Advanced" expander/tab with the `pm.*` fields (mirror the removed `UserPHPPoolCard` fields) but `max` = `package.fpm_max_children_cap`, with the dynamic-mode conditional (Form.useWatch) and the memory-budget hint.
3. Keep L1 and L2 coherent: selecting a mode sets the knobs; editing a knob switches to "custom".

**Verify.** Advanced-package user edits `pm_max_children` up to the cap; over-cap is clamped. Non-advanced package: no advanced UI, 403. E2E.

**Exit criteria.** Advanced users get clamped raw control; the cap holds. Rollback = hide + 403.

---

### Step 7 — E2E, runbook, ADR (gate)

**Tasks.**
1. Playwright E2E across the three tiers (admin sets package policy → user sees the right surface → values clamp). Run before declaring green (framework-bump/E2E scar).
2. Runbook `plans/m-php-fpm-performance-tiers-runbook.md`: the tier model, how an admin opts a package in, the preset editor, troubleshooting a pool that won't apply.
3. ADR: record the three-tier package-gated model + the per-worker-MB soft-budget decision + the scope=per-user-per-version boundary (per-domain deferred to #329).
4. Full live validation on 192.168.100.86: each tier end-to-end, FPM brings pools up.

**Exit criteria.** All tiers validated live; docs + ADR landed.

---

## 2. Risks

- **Clamp is the only real safety** (Step 4). The L1/L2 UIs are convenience; if Step 4 is wrong a user could exceed caps. Land Step 4 before 5/6 and test the clamp adversarially (request over-cap directly, bypassing the UI).
- **Package lookup in the reconciler** (Step 4) risks an N+1 across all pools each tick — batch-load or per-tick cache.
- **Preset seeding** must be app-side, not migration-side (dirty-DB scar). Exactly 4 rows, idempotent.
- **Advanced ↔ mode coherence** (Step 6) — a knob edit must not silently ignore the mode; show "custom".
- **PR #34 interaction** — this feature must NOT re-open `/php-pools` to users; it adds a separate user-scoped endpoint. Keep `RequireAdmin` on `/php-pools`.

## 3. Interim option (if full feature deferred)

Ship only Steps 1+2+3 (admin gains the package policy knobs, the presets, and
the PHP-Manager FPM tab) with tiers L1/L2 still admin-only — i.e., admins get
the better tooling now, user opt-in follows. Small, ships value, doesn't block
the user-facing waves.
