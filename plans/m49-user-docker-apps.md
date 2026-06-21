# Plan: M49 — User-Level Docker Apps (tenant-installable containerised apps)

GH #170. Extends M48 (admin-only Docker App Marketplace, ADR-0116) so a
**tenant** can install a curated subset of catalog apps onto a domain they
own, under hard resource + count limits enforced by their hosting package.

> M48 ADR-0116 **Decision 1** explicitly scoped v1 to admin-only and queued
> per-tenant as needing "rootless podman + per-tenant socket scoping." This
> plan is that follow-on. The headline problem is **isolation**: tenants are
> semi-trusted, the daemon is rootful, and a careless install must not let a
> tenant "make a mess" (the deferral note on the issue).

---

## 0. Operating assumptions

### Conventions inherited from this repo
- DB is truth; one write path (API); reconciler converges host state. Agent is
  the ONLY holder of `/var/run/docker.sock` (ADR-0116 Decision 2) — unchanged.
- Route families: admin under `/admin/*` (RequireAdmin); tenant under the
  auth-only base group, subject-scoped to `claims.UserID`.
- Drawer for create+edit, SearchableTable for lists, list envelope
  `{data,total,page,page_size}` (see docs/CONVENTIONS.md).
- API-token RBAC: tenant docker routes resolve to the `apps` area
  (user_token_scopes.go areaRules) — extend the rule table.

### What we are NOT doing in M49
- No rootless Podman rewrite (documented as the v2 isolation ceiling; §9).
- No tenant **exec shell** or **edit-compose** (ADR-0116 Decision 10 stays —
  shell-in-container ≈ code-exec; admin-only forever).
- No **public-bind** ports for tenant installs (loopback + nginx only; §3).
- No new DB engine hookup; bundled-in-compose DBs only (ADR-0116 Decision 6).
- No custom/tenant-submitted catalog entries; admin curates `tenant_installable`.

### Memory pointers
- [[project_m18_resource_limits]] — per-user cgroup-v2 slice (`jabali-user-<uid>.slice`)
  + POSIX quota + nginx limit_req. **Load-bearing for M49**: tenant container
  cgroups nest under this slice so package CPU/mem caps already apply.
- [[project_control_plane]] — DB-as-truth, reconciler owns convergence.
- ADR-0116 + `plans/m48-docker-app-marketplace.md` — the admin substrate M49 extends.

---

## 1. Dependency graph

```
Phase 0 (cgroup-nesting SPIKE on mx)               ── GATES THE ARCHITECTURE
   └─> Phase 1 (schema + catalog flags + ADR-0117) ── foundation, additive
          └─> Phase 2 (agent hardening overlay + cgroup_parent)   ── the security core
                 └─> Phase 3 (tenant REST + quota/ownership guards + scope)
                        ├─> Phase 4 (tenant "Apps" UI)
                        └─> Phase 5 (package max_docker_apps admin UI + disk accounting)
                               └─> Phase 6 (user-delete cascade teardown)
                                      └─> Phase 7 (E2E + security review + runbook + ADR accept)
```

**Phase 0 gates everything** (§7): the whole resource story (§5 "no per-app
accounting") rests on a rootful-docker container's cgroup actually nesting
under, and being capped by, the tenant's *systemd-managed* M18 slice. systemd
and docker fighting over a slice systemd thinks it owns is a known conflict —
if it doesn't hold, the architecture changes (per-app accounting needed), not
just an impl detail. Prove it on mx before any other phase starts.

Phase 2 is the build wave gate — nothing tenant-facing ships until the
hardening + cgroup binding is proven end-to-end on a real tenant slice.

---

## 2. Threat model (read before designing anything)

**Reframe first — what's actually new.** These tenants already run arbitrary
code as their unix user: M12 SFTP, M13 SSH sandbox, per-user PHP-FPM. So a
docker app is **not** a tenant's first foothold, and the marginal risk is **not**
"tenant runs code." The single new risk is **escape-to-root beyond the uid the
tenant already has** — a container-root breakout to host root. This cuts both
ways: it makes the feature *more* defensible (we're containing an existing trust
level, not granting a new one) **and** it makes `userns-remap` the load-bearing
control, because escape-to-host-root is precisely the one new risk left
otherwise unmitigated. Hence userns is a Phase-1 **gating** decision (§7), not an
open question.

Tenants are **semi-trusted**: they pay for the box-share, but a hostile or
compromised tenant must not reach the host or another tenant. The attack
surface a tenant docker app adds:

| Threat | M49 control |
|---|---|
| docker.sock reach | None added — agent mediates every verb; tenant REST never speaks to docker (ADR-0116 D2 unchanged). |
| Container escape → host **root** | Rootful daemon = residual risk. Mitigations: mandatory `no-new-privileges`, `cap_drop: ALL`, non-root `user:`, no `privileged`, no host bind-mounts; **recommend daemon `userns-remap`** on tenant-enabled hosts (§3, §9 open decision). |
| Resource exhaustion (CPU/mem/pids) | Container cgroup nests in `jabali-user-<uid>.slice` (M18) → package caps apply to the SUM of the tenant's processes + containers. Per-app `--pids-limit`. |
| Disk exhaustion | App data dir footprint (`data_bytes`, mig 162) counted against the package disk quota at install + on the reconciler size poll (§5). |
| Port / IP grab | Tenant installs are **loopback-only**, auto-allocated 10000–19999. No public bind, no host-port pinning. |
| Cross-tenant domain hijack | Install attaches only to a domain the caller owns (`dom.UserID == claims.UserID`); reuse the existing guard (docker_apps.go:465). |
| Catalog abuse (needs caps/privesc) | Only apps with `tenant_installable: true` (admin-verified to run under the hardened profile) appear in the tenant catalog. |
| Noisy neighbour (disk IO, daemon) | M18 io read/write caps on the slice; daemon-wide concurrent-pull is an operator concern (documented). |

**Stance:** rootful Docker + a mandatory hardening profile + a curated catalog
subset + package quotas + the M18 slice is a *defensible* v1. It is not as
strong as rootless-per-tenant; §9 documents the residual risk and the v2 path.

---

## 3. Architecture

### Ownership
`docker_apps.user_id` (new, NULL = admin/server-level — existing behaviour
preserved; non-NULL = tenant-owned). Every tenant verb filters
`WHERE user_id = :caller`. Admin verbs see all.

### Curated catalog
`app.yaml` gains `tenant_installable: bool` (default **false**). The tenant
catalog endpoint returns only entries with `tenant_installable: true` AND that
declare no `default_bind: public` port (loopback-only requirement). Admin
catalog is unchanged (sees everything).

### Hardening overlay (Phase 2 — the core)
Catalog `compose.yml.tmpl` stays shared between admin and tenant installs. For
**tenant** installs the agent writes a second compose file —
`compose.tenant-hardening.yml` — and runs `docker compose -f compose.yml -f
compose.tenant-hardening.yml up`. The overlay applies to every service:

```yaml
services:
  <each service>:
    security_opt: ["no-new-privileges:true"]
    cap_drop: ["ALL"]
    cap_add: [<app's verified minimal allowlist from app.yaml>]   # see below
    pids_limit: <from package/app>
    cgroup_parent: jabali-user-<uid>.slice     # M18 binding — package CPU/mem applies
```

- `cgroup_parent` is the load-bearing line: it nests the container's cgroup
  under the tenant's M18 slice so the package memory/CPU ceiling caps the
  tenant's containers + shell + FPM **in aggregate** — no new per-app resource
  accounting needed. **This is exactly what Phase 0 proves** (the systemd↔docker
  slice-ownership conflict can evict the docker-created child; if it does, §5
  must switch to per-app accounting). Requires the **systemd** cgroup driver (§8).
- **`cap_drop: ALL` alone breaks most real apps** — vaultwarden/linkwarden/ghost
  entrypoints `chown` their data dir then drop privileges, needing CHOWN +
  SETUID/SETGID + DAC_OVERRIDE. So "drop all and it must work" is false for the
  exact starter set. Each `tenant_installable` app declares a **verified minimal
  cap allowlist** in `app.yaml` (`tenant_caps: [...]`) — established by actually
  running the app under drop-ALL and adding back only what it provably needs, NOT
  guessed. Empty default; an app that won't run under a small explicit set is not
  `tenant_installable`.
- `privileged`, **any `cap_add` beyond the app's declared `tenant_caps`**, and
  host bind-mounts are **rejected** for tenant installs: the agent validates the
  merged `docker compose config` output and refuses otherwise.

### Ports
Tenant install forces `bind_interface=loopback`, `reverse_proxy=true`,
auto-allocated host port (no pin). The admin port drawer (ADR-0116 D5) is
hidden for tenants — a tenant app gets exactly its catalog loopback port(s).

### Domain + vhost + TLS
Unchanged from M48: install creates a `domains` row (`managed_by='docker_app'`,
`docker_app_id`, **`user_id = caller`**), reconciler renders the proxy_pass
vhost, LE issues through the tenant-domain path. The handler already refuses to
attach to a domain owned by someone else (docker_apps.go:465) — reuse verbatim.

### Filesystem
`/var/lib/jabali/docker-apps/<instance_slug>/` unchanged (root:jabali 0750).
Data still root-owned (the container's remapped/declared user writes inside);
the tenant never gets a shell on the host tree. Footprint is metered, not
quota'd at the FS layer (§5).

---

## 4. Schema — `000180_user_docker_apps.up.sql`

```sql
-- M49 (ADR-0117): tenant-owned docker apps.
-- user_id NULL  = admin/server-level install (M48 behaviour, preserved).
-- user_id set   = tenant-owned; every tenant verb filters on it; user delete
--                 cascades the row (host teardown enqueued by the delete path).
ALTER TABLE docker_apps
  ADD COLUMN user_id CHAR(26) NULL AFTER id,
  ADD CONSTRAINT fk_docker_apps_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  ADD KEY idx_docker_apps_user (user_id);

-- COLLISION FIX. The M48 unique key `uniq_docker_apps_slug_name (slug, name)`
-- plus a global `instance_slug` mean two different tenants could not both
-- install "memos" called "notes" — the row, the container name
-- (jabali-app-<instance_slug>) and the data dir would collide. Namespace by
-- owner: drop the global name key, add one scoped by user_id. (NULL user_id =
-- admin installs; MySQL treats NULLs as distinct in a unique key, so admin
-- installs keep their existing slug+name uniqueness via the app-layer check.)
ALTER TABLE docker_apps
  DROP KEY uniq_docker_apps_slug_name,
  ADD UNIQUE KEY uniq_docker_apps_owner_slug_name (user_id, slug, name);

-- Package gate: 0 = docker apps NOT included in this package (safe default,
-- opt-in per plan). >0 = max simultaneous tenant installs for users on the plan.
ALTER TABLE hosting_packages
  ADD COLUMN max_docker_apps INT UNSIGNED NOT NULL DEFAULT 0 AFTER max_tasks;
```

`down.sql` drops the FK + columns and restores `uniq_docker_apps_slug_name`.
Migrations are schema-only — `max_docker_apps` default 0 means **every existing
package keeps docker apps off** until an operator opts a plan in (no surprise
capability grant).

**`instance_slug` namespacing (Phase 1 app-layer):** tenant installs derive
`instance_slug = "<slug>-<short_uid>-<name>"` so two tenants installing the same
catalog app get distinct container names + data dirs. Admin installs keep the
M48 derivation (`<slug>-<name>`). The agent already keys all on-disk identity off
`instance_slug` (mig 160), so no agent change beyond the derivation.

`tenant_installable` is a **catalog YAML field**, not DB — lives in
`install/docker-apps/<slug>/app.yaml` + the `_schema/app.schema.json`.

---

## 5. Quota enforcement (where `max_docker_apps` bites)

On `POST /docker-apps` (tenant):
1. Resolve caller's package. `max_docker_apps == 0` → **403 `docker_apps_not_in_package`**.
2. `COUNT(*) FROM docker_apps WHERE user_id = :caller` ≥ `max_docker_apps` →
   **409 `docker_app_quota_exceeded`**.
3. Disk: `SUM(data_bytes) WHERE user_id=:caller` + the new app's catalog
   estimate vs `disk_quota_mb`. Soft-check at install (catalog has no reliable
   pre-size, so this is best-effort); the reconciler size poll (mig 162) is the
   real meter — on overage it flags the app `status='over_quota'` and the UI
   warns. (Hard FS quota on the root-owned tree is a §9 open item.)
4. Resource: no per-app math — the M18 slice caps the aggregate. The install
   still clamps per-app `cpu_limit`/`memory_limit` to ≤ the package
   `memory_limit_mb` / `cpu_quota_percent` so one app can't request the whole
   slice and starve the tenant's web/FPM.

Admin installs (`user_id` NULL) skip all four — unchanged.

---

## 6. API surface

Tenant routes mount on the auth-only base group, all subject-scoped. Mirror the
admin handler but with the guards above; **no exec, no edit-compose, no
port-drawer, no public bind.**

```
GET    /docker-apps/catalog            tenant_installable subset only
GET    /docker-apps/catalog/:slug/icon
GET    /docker-apps                     WHERE user_id = caller
GET    /docker-apps/:id                 own only (404 otherwise — no existence leak)
POST   /docker-apps                     install: quota + domain-ownership + loopback-force
DELETE /docker-apps/:id                 own only; enqueues host teardown
POST   /docker-apps/:id/(start|stop|restart)   own only
GET    /docker-apps/:id/logs            own only
PUT    /docker-apps/:id/env             own only (catalog-declared env only)
POST   /docker-apps/:id/backup          own only; destination MUST be tenant-owned (see below)
```

- **Backup destination scoping:** `POST /:id/backup` reuses M30 restic, but a
  tenant must not target the admin's or another tenant's `backup_destinations`
  row. Either restrict tenant app backups to a tenant-owned destination
  (validate `dest.user_id == caller`) or, if M30 destinations are admin-only
  today, ship tenant app backups to the **default repo only** in v1 and defer
  tenant-selectable destinations. Resolve in Phase 3; do not hand-wave "tenant
  dest." (Check M30 `backup_destinations` ownership model first.)
- `user_token_scopes.go`: add `{"/api/v1/docker-apps", "apps"}` to areaRules
  (tenant API tokens already use the `apps` area for `/applications`). Note this
  makes the `apps` scope coarse — one token area covers M19 php-apps AND M49
  docker-apps; acceptable for v1, documented so it's a deliberate choice.
- Reuse the existing `dockerAppHandler` internals where possible; split the
  ownership/quota guard into a middleware so admin + tenant share the verb body.

---

## 7. Phase steps

### Phase 0 — cgroup-nesting spike on mx (GATES THE ARCHITECTURE)
The discriminating fact none of the design can assume. On mx, by hand:
1. Pick a real tenant uid with an active `jabali-user-<uid>.slice`.
2. `docker run -d --cgroup-parent=jabali-user-<uid>.slice --name spike <small image>`.
3. Confirm nesting: the container's cgroup path is **under** the slice
   (`systemd-cgls` / `cat /sys/fs/cgroup/system.slice/.../jabali-user-<uid>.slice/...`).
4. Confirm capping: `systemctl set-property jabali-user-<uid>.slice MemoryMax=128M`,
   stress the container past it, confirm the **container** OOMs and the host is fine.
5. Confirm systemd doesn't evict the docker-created child on `daemon-reload` /
   slice restart.
- **Pass** → §5 stands, proceed to Phase 1. **Fail** (systemd fights docker for
  the slice, or no capping) → STOP: redesign §5 around per-app cgroup accounting
  (or a docker `--cpus/--memory` per-app mirror of the package budget) before
  any code. Record the result in ADR-0117.

### Phase 1 — schema + catalog flags + gating decisions + ADR-0117 (additive)
- Migration 000180 (above) incl. the owner-scoped unique key. Repo
  `DockerAppRepository`: add `UserID` + `ListByUserID`, `CountByUserID`,
  `FindByIDForUser`; tenant `instance_slug` derivation.
- `app.schema.json` + loader: `tenant_installable` (default false) **and**
  `tenant_caps: []` (the verified minimal cap allowlist, §3).
- Establish the **safe starter set** empirically: for each candidate
  (memos, dokuwiki, freshrss, privatebin, linkwarden, vaultwarden) actually run
  it under `cap_drop: ALL` + `no-new-privileges`, add back the minimum caps that
  make it boot+persist, record them in `tenant_caps`, and only then flip
  `tenant_installable: true`. An app that needs a writable rootfs or a cap you're
  unwilling to grant stays admin-only. Gitea (ssh), n8n, immich, onlyoffice stay
  admin-only.
- **GATING DECISION — userns-remap** (§2, §9): decide v1 posture and record it in
  ADR-0117: (a) mandatory daemon `userns-remap` on tenant-enabled hosts (accept
  the existing-bind-mount ownership retrofit), gated by a host flag the operator
  sets before `max_docker_apps>0` takes effect; OR (b) v1 ships rootful without
  remap for operators who explicitly accept the escape-to-host-root residual.
  Default recommendation: (a) with the host-flag gate.
- ADR-0117 (Proposed): rootful-Docker-+-hardening stance, curated-catalog +
  `tenant_caps` gate, package quota, the Phase-0 cgroup result, the userns
  decision, and the deferred rootless-podman v2.

### Phase 2 — agent hardening overlay + cgroup binding (WAVE GATE)
- `docker_lifecycle.go` (compose up): when params carry `OwnerUID`, write +
  merge `compose.tenant-hardening.yml` (security_opt/cap_drop/pids/cgroup_parent).
- Validate merged `docker compose config`: reject `privileged`, any `cap_add`,
  any bind-mount whose source is outside the app data tree.
- Agent verb params gain `owner_uid`. Reconciler passes it from `docker_apps.user_id`.
- **Live-verify on a tenant slice** (mx): install a tenant app, confirm the
  container's cgroup path is under `jabali-user-<uid>.slice`, confirm
  `systemctl set-property` mem cap throttles it, confirm no added caps
  (`docker inspect` CapAdd empty, no-new-privileges true).

### Phase 3 — tenant REST + guards + scope
- New `docker_apps_user.go` handler (or shared body + tenant guard middleware).
- Quota/ownership/loopback-force guards (§5, §6). Scope rule added.
- Table-driven handler tests: quota=0 → 403; over count → 409; foreign domain →
  409; public-bind request → 400; non-owner GET/:id → 404.

### Phase 4 — tenant "Apps" UI
- `panel-ui/src/shells/user/apps/` — SearchableTable list (own apps) + catalog
  grid (tenant_installable) + install Drawer: app pick → domain pick (own
  domains) → resource within package → install. Status polling reuses admin
  hook. No port/exec/compose controls.

### Phase 5 — package admin UI + disk accounting
- Add `max_docker_apps` to the hosting-package create/edit form + the model/repo
  + the package detail view.
- Reconciler size poll: on `SUM(data_bytes) > disk_quota_mb` for a user, set the
  newest over-budget app `status='over_quota'` + M14 notify the tenant.

### Phase 6 — user-delete cascade teardown
- Inline user-delete cascade: before the DB cascade nulls/deletes, enqueue
  `app.delete` (container + data tree) for each `docker_apps WHERE user_id=:u`.
  Without this the FK drops the row but leaves containers + bytes on the host.
- Test: delete a tenant with an installed app → container gone, data dir gone,
  domain row + vhost gone.

### Phase 7 — E2E + security review + runbook + ADR accept
- Playwright: tenant installs from catalog → reachable over TLS on own domain →
  stop/start → delete. Quota-exceeded path. Cross-tenant 404/409 paths.
- `security-reviewer` pass on the hardening overlay + the compose-config
  validator (the privesc gate is the highest-value test).
- Runbook `plans/m49-user-docker-apps-runbook.md`: enabling a package, the
  userns-remap host recommendation, teardown, incident steps.
- Flip ADR-0117 → Accepted after live verification.

---

## 8. Verification matrix

| Check | How |
|---|---|
| Container nests in tenant slice | `cat /sys/fs/cgroup/.../jabali-user-<uid>.slice/.../docker-<id>/cgroup.procs` |
| Package mem cap throttles container | install app → `systemctl set-property jabali-user-<uid>.slice MemoryMax` → stress → OOM in container, host fine |
| No added privilege | `docker inspect` → CapAdd empty, no-new-privileges true, Privileged false |
| privesc compose rejected | craft a `tenant_installable` app with `privileged: true` → install 400, nothing started |
| Quota gate | package max=1 → 2nd install 409; package max=0 → 403 |
| Domain ownership | install onto another tenant's domain → 409 |
| Loopback-forced | tenant install of a public-port app → port hidden/forced loopback |
| Teardown on delete | delete tenant → container + data + vhost gone |
| Migrations clean on fresh + existing | MariaDB 11.x fresh install + upgrade from pre-180 |

## 9. Risks + open decisions

1. **userns-remap — now a Phase-1 GATING decision, not open** (moved to §7
   Phase 1; see §2 reframe). Daemon-wide remap maps container root →
   unprivileged host uid, the strongest escape mitigation, but it is daemon-wide
   (shifts ownership of **existing** admin-install bind-mount data → retrofit)
   with no clean per-container range. Resolved in ADR-0117, not deferred —
   because it's the one control covering the single new risk.
2. **cgroup nesting — now Phase 0**, not an open item: it can invalidate §5, so
   it's spiked before anything else (see §7 Phase 0). Requires the systemd
   cgroup driver.
3. **Disk hard-quota.** App data lives on a root-owned tree, not the tenant's
   POSIX-quota'd home, so overage is *metered + flagged*, not hard-blocked at
   write time. Hard enforcement would need either a per-tenant XFS project
   quota on the app subtree or relocating data under the home. Phase-5 ships the
   meter; hard-quota is a follow-up.
4. **Rootful residual risk.** A kernel/docker CVE escape from a tenant container
   is host-root. Curated catalog + `tenant_caps` allowlist + hardening profile +
   userns-remap reduce it; rootless-podman-per-tenant (v2) is the real ceiling —
   documented, deferred.
5. **Noisy daemon.** Concurrent image pulls / one tenant churning installs hits a
   shared daemon. Out of scope to rate-limit in v1; note in runbook.

## 10. Out of scope (queued)
- Rootless Podman per tenant (v2 isolation ceiling).
- Tenant custom/uploaded catalog entries.
- Tenant exec shell / edit compose (admin-only, permanent).
- Public-bind / raw-TCP/UDP ports for tenant apps.
- Hard FS disk quota on the app data tree (Phase 5 meters; enforcement later).
- Hybrid DB (tenant app → jabali MariaDB) — inherits M48's deferral.
