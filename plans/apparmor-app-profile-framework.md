# Blueprint — Managed AppArmor Profile Framework for App Workloads (GH #690)

Extend AppArmor from the fixed jabali **daemon** set (jabali-panel, jabali-agent,
jabali-bulwark, stalwart-mail) to a managed framework that can attach, soak, and
enforce profiles for **application workloads** (per-user PHP-FPM pools running
tenant WordPress/Drupal/etc.). Confines a compromised tenant app to a
least-privilege file/capability/network set without breaking the app.

## Foundation already shipped (build ON this)

- `#679` missing/unloaded profiles reported as `mode:"missing"` (not omitted).
- `#682` panel daemon holds no `mac_admin`; mode flips delegate to `panel-agent`.
- `#687` update profile-sync honors the kernel-feature gate.
- `#688` complain-mode `ALLOWED` would-deny events + per-profile soak-readiness.
- `allowedProfiles` registry + `security.apparmor.{status,set_mode}` verbs.

## Load-bearing invariants

1. **Never break a running site.** New app profiles ALWAYS ship in **complain**
   mode; nothing flips to enforce until soak-readiness (0 would-deny over the
   window) is met (reuse #688). An app profile that fails to load must NOT block
   the FPM pool from starting (fail-open on the profile, log `missing`).
2. **Attach by the FPM master path, not per-request.** The per-user
   `jabali-fpm@<user>` master + its workers are the confinement unit. A profile
   attaches to the fpm-worker exec; the agent (holds `mac_admin`) loads it.
3. **Template-driven, tenant-uneditable.** Tenants never supply profile text.
   Profiles render from jabali-shipped templates keyed by app-type + the tenant's
   known paths (docroot, socket, tmp). Same trust boundary as the WP cache plugin.
4. **Kernel-gate respected** (#687): on kernels lacking `features/unix`, app
   profiles are not loaded (same as daemon profiles).

## Dependency graph

```
A profile template registry + renderer (gate)
        │
        ▼
B agent build/load/unload/prune verbs ── C attach lifecycle (install/delete hooks)
                                                │
                                                ▼
                                         D soak → enforce + admin UI
```

---

## Wave A — Template registry + renderer  [GATE]

**Context brief.** Define app-profile templates (base + per-app-type overlays,
mirroring the snuffleupagus 00-base + 10-wordpress layering) that render to a
concrete profile for a given tenant install.

**Tasks.**
1. `install/apparmor/app-templates/` — a `base.rules.tmpl` (allow the docroot RW,
   PHP/tmp, the Redis + MariaDB sockets; deny everything else) + per-app overlays
   (`wordpress`, `drupal`, `joomla`, …).
2. A renderer (Go) that fills a template from `{User, Docroot, Socket, PHPVer}`
   into `/etc/apparmor.d/jabali-app-<user>-<install>` — tenant-uneditable, agent-owned.
3. Register the app-profile names in a managed registry (extends `allowedProfiles`
   with a prefix match `jabali-app-*` so status/set_mode already cover them).

**Verify.** Rendering a WordPress template for a test install produces a valid
profile (`apparmor_parser -Q` parses it); the name matches `jabali-app-*`.

**Exit.** Concrete per-install profiles render from templates + are recognized by
the existing status/set_mode verbs.

---

## Wave B — Agent build/load/unload/prune verbs  [after A]

**Tasks.**
1. `security.apparmor.app.load` — render + `apparmor_parser -r` (complain) for an
   install (agent, `mac_admin`).
2. `security.apparmor.app.unload` — remove on install delete.
3. `security.apparmor.app.prune` — GC orphaned `jabali-app-*` profiles (no
   matching install), mirroring `nspawn prune`.
4. All fail-open on the profile (never block the pool) + honor the kernel gate.

**Verify.** load→status shows `jabali-app-<x>` complain; unload removes it; prune
drops orphans; a bad template errors without affecting the pool.

**Exit.** App profiles are agent-managed with a full lifecycle.

---

## Wave C — Attach lifecycle (install/delete hooks)  [after A]

**Tasks.**
1. Hook `app.install` / `wordpress.install` completion → `apparmor.app.load`
   (complain) for the new install.
2. Hook `app.delete` → `apparmor.app.unload`.
3. The FPM pool template gains an AppArmor attachment (`hat`/exec) for the
   install's workers — or the profile attaches by the fpm-worker path. Verify a
   confined worker still serves the site (complain mode logs, never blocks).

**Verify.** Installing a WP app loads its complain profile + the site works;
deleting it unloads the profile.

**Exit.** Every app install gets a soaking profile automatically.

---

## Wave D — Soak → enforce + admin UI  [after B, C]

**Tasks.**
1. Reuse #688 soak-readiness per app profile; a scheduled check (or the existing
   `flip-mature` CLI, extended to `jabali-app-*`) flips soaked-clean profiles to
   enforce — never one still logging would-deny.
2. Admin Security → AppArmor: an "App profiles" section listing `jabali-app-*`
   with mode + soak-readiness + would-deny counts (the columns already exist),
   grouped by tenant.
3. Runbook + ADR (new, superseding/extending ADR-0086) documenting the app-profile
   lifecycle, the fail-open guarantee, and the template trust boundary.

**Verify.** A soaked-clean app profile auto-flips to enforce; one with violations
stays complain + surfaces in the UI; enforce-mode confinement blocks a planted
path-escape without breaking normal requests (live-VM E2E).

**Exit.** Tenant app workloads run confined (complain→enforce by soak), managed +
observable in the panel, with a documented fail-open safety guarantee.

## Cross-cutting

- **Highest risk:** confining tenant PHP without breaking legitimate apps.
  Complain-first + soak-before-enforce (invariant 1) is the mitigation; the
  live-VM E2E in Wave D is mandatory, not optional.
- Start with **WordPress only** (the dominant workload); add per-app overlays
  incrementally once the base soak process is proven.
