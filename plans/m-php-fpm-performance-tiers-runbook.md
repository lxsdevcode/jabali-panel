# Runbook — PHP-FPM Performance Tiers

Operational guide for the package-gated 3-tier PHP-FPM tuning (GH #339 phase 2,
ADR-0155). Feature shipped in `feat/pkg-fpm-controls` (PR #36) + the admin tab
(PR #37).

## The model in one line

Admin sets a **hosting package** policy → tenants on that package get **nothing**,
a **Performance Mode dropdown** (L1), or **clamped advanced knobs** (L2). Admins
always have raw control (L3). Every non-admin value is clamped to the package cap
at write time.

## Admin: opt a package in

1. **Admin → Packages → edit a package → "PHP-FPM Performance Policy":**
   - `Max children per user (FPM cap)` — the hard ceiling for tenants on this
     package (≤ 2000). Memory budget ≈ cap × "Est. RAM per worker".
   - `Users can pick a PHP Performance Mode` (L1 gate).
   - `Users can use Advanced mode` (L2 gate — implies the above).
2. Save. Existing pools keep their values; new tenant changes clamp to the cap.

## Admin: tune the mode presets

**Server Settings → "PHP Performance Modes"** — expand any of the 4 presets
(Balanced / Low-memory / High-traffic / WordPress-optimized) and edit its `pm.*`.
These are the templates users pick from; each is re-clamped to the package cap
when a user applies it, so a low-cap package silently caps a high preset. The 4
rows are seeded on first boot and never deleted.

## Admin: raw per-pool tuning (L3)

**Admin → PHP Manager → "FPM Pools" tab** — filter by PHP version, see every
user's pool, click Edit for the raw `pm.*` form (unclamped). Or the direct route
`/jabali-admin/php-pools/edit/:id`. The `/php-pools` API stays `RequireAdmin`.

## Tenant experience

**PHP Settings → "Performance" tab:**
- Package allows editing → a **Performance Mode** dropdown (+ an **Advanced**
  expander if the package enables L2). Applying fires `php.pool.apply`; the pool
  reloads within a reconciler tick.
- Package disables editing → an info hint, no controls.

## Troubleshooting

- **User sees the "not enabled" hint but should have access** — the user's
  account has no `package_id`, or the package's `fpm_user_can_edit` is off. Check
  `GET /me/php-fpm-policy` (returns `can_edit`/`advanced`/`max_children_cap`).
- **A dynamic pool won't apply / status=error** — the agent rejects a bad FPM
  combo; the clamp + `resolvePMTuning` should prevent this. Check
  `journalctl -u jabali-panel | grep 'php.pool.apply failed'` for the reason
  (e.g. `min_spare <= start <= max_spare <= max_children`).
- **Preset change didn't reach a tenant** — presets are templates applied only
  when a user *selects* a mode; changing a preset does not retro-apply to pools
  already set to that mode. The user re-applies to pick up the new values.
- **Verify the rendered conf** — `grep '^pm' /etc/php/<v>/fpm/pool.d/jabali-<user>.conf`.

## Migrations

- `000214` hosting_packages FPM policy columns.
- `000215` php_performance_modes (4-row preset table; app-seeded).
- `000216` php_pools.performance_mode.

## Known follow-ups

- Playwright E2E across the three tiers (the harness mocks the API via
  `tests/e2e/fixtures.ts`; the new `/me/php-*` + `/php-performance-modes` routes
  need mock wiring). Tracked as the remaining Step 7 item.
- Per-domain `pm.*` overrides — deferred to #329 Phase 2.
