# ADR-0155 — PHP-FPM performance tiers (package-gated)

**Status:** Accepted — GH #339 phase 2. Implemented in the `feat/pkg-fpm-controls`
branch (Steps 1–6 + Step 2 UI); plan at `plans/m-php-fpm-performance-tiers.md`.

## Context

GH #339 asked for per-domain PHP-FPM pool tuning in the UI. Phase 1 (ADR/PR on
`main`) exposed the raw `pm.*` knobs to users, then PR #34 locked them to admin
only after the operator decided tenants must not resize the shared worker pool
directly. But some tenants genuinely benefit from tuning (WordPress/WooCommerce
sizing, low-memory VPS). We need to give users access **safely**, with the admin
in control of how much rope each plan gets.

## Decision

A **three-tier, hosting-package-gated** model. The hosting package (admin-only)
decides which tier a tenant gets:

- **L1 — Performance Mode.** A safe dropdown of 4 presets (Balanced / Low-memory
  / High-traffic / WordPress-optimized), no raw numbers. Gated by package
  `fpm_user_can_edit`. Endpoint `PUT /me/php-performance-mode`.
- **L2 — Advanced.** The individual `pm.*` knobs, but every value is clamped to
  the package caps. Gated by package `fpm_advanced_mode` (implies can-edit).
  Endpoint `PUT /me/php-pool-tuning`.
- **L3 — Admin.** Full raw `pm.*` control, unclamped, via the existing admin
  `/php-pools` surface (`RequireAdmin`).

**Package controls** (`hosting_packages`, migration 000214): `fpm_max_children_cap`,
`fpm_worker_mem_mb` (advisory memory-budget estimate = cap × mem; FPM has no hard
per-pool memory limit), `fpm_user_can_edit`, `fpm_advanced_mode`,
`fpm_version_defaults`.

**Presets** (`php_performance_modes`, migration 000215) are a fixed set of 4,
admin-editable in Server Settings, seeded app-side (not from the migration).

**Clamp-at-write is the load-bearing safety.** `clampToPackageCap` bounds any
non-admin-produced value to the owner's package cap and re-runs the FPM dynamic
constraint before the pool is written. The UIs are convenience; the clamp is the
enforcement — a request that bypasses the UI still cannot exceed the cap.

## Consequences

- The user-facing surface is a NEW user-scoped endpoint family (`/me/php-*`), NOT
  a re-opening of the admin `/php-pools` routes — those stay `RequireAdmin`.
- Scope is **per-user-per-PHP-version** (today's `php_pools` model). True
  per-domain `pm.*` overrides remain deferred to **#329 Phase 2**.
- Presets are ceilings; a low-cap package silently clamps a high-traffic preset,
  which is the intended shared-hosting safety behavior.
- Not yet done (optional follow-ups): a dedicated admin "FPM Pools" tab in the
  PHP Manager (admins currently edit pools via `/jabali-admin/php-pools`), and
  Playwright E2E across the three tiers.
