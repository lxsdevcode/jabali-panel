# M49 — User-Level Docker Apps: Operator Runbook (GH #170)

How to turn on tenant Docker apps, how the isolation works, and how to recover.
Companion to [`plans/m49-user-docker-apps.md`](./m49-user-docker-apps.md) + ADR-0117.

## What it is

Tenants can install a **curated subset** of catalog apps onto a domain they own.
Each tenant container is hardened (cap_drop ALL + a verified cap allowlist,
no-new-privileges, loopback-only) and nests in the tenant's M18 cgroup slice so
the hosting-package CPU/memory caps apply. A container-root escape lands as the
unprivileged `dockremap` subuid, never host root.

## Prerequisites

- M48 Docker app marketplace installed (docker-ce + the catalog).
- Hosts use the **systemd** cgroup driver with cgroup v2 (Docker 29 default;
  proven in the Phase 0 spike).

## Enabling tenant docker on a host (one-time, per host)

Tenant docker is **off by default** and gated on a host flag. Enabling it turns
on daemon-wide `userns-remap`, which is **daemon-wide and changes Docker's
storage root** — existing pulled images become invisible until re-pulled, and
existing admin-app data ownership is shifted into the remap range.

```bash
sudo jabali docker enable-tenant            # dry-run: prints what it will do
sudo jabali docker enable-tenant --yes      # actually do it
```

The `--yes` run, in order: `docker compose down` every installed app → write
`userns-remap` into `/etc/docker/daemon.json` → restart dockerd → `chown -R` each
app data tree into the `dockremap` subuid range → `docker compose up` + health →
**only on full success** write `/etc/jabali/docker-tenant-enabled`. If any app
fails health post-remap, the flag is NOT written and the host stays "tenant
docker off".

Verify:
```bash
test -f /etc/jabali/docker-tenant-enabled && echo "tenant docker ON"
grep dockremap /etc/subuid                 # dockremap:100000:65536
# a running container's root maps to an unprivileged host uid:
sudo cat /proc/$(docker inspect -f '{{.State.Pid}}' <ctr>)/uid_map   # base != 0
```

## Granting tenants access

A tenant can only install Docker apps if **both** hold:
1. the host flag exists (above), and
2. their hosting **package** has `max_docker_apps > 0` (Packages admin page →
   "Max Docker Apps"). `0` = not included.

Quota: a tenant install is refused with 403 `docker_apps_not_in_package`
(package=0) / 403 `docker_tenant_not_enabled` (no host flag) / 409
`docker_app_quota_exceeded` (at the limit).

## Curating the tenant catalog

An app is tenant-installable only when its `app.yaml` has
`tenant_installable: true` AND a verified `tenant_caps` allowlist. Establish caps
empirically before flipping:

```bash
# run under the hardened profile and watch what it needs:
docker run --rm --cap-drop=ALL --security-opt=no-new-privileges:true <image>
# add back ONLY the caps it provably needs (chown-then-drop apps usually need
# CHOWN, SETUID, SETGID, DAC_OVERRIDE), record them in app.yaml tenant_caps,
# then set tenant_installable: true.
```

Apps with a `default_bind: public` port can never be tenant-installable
(loopback-only rule, enforced by the catalog filter).

## How a tenant install is contained (defense in depth)

1. **panel-api render** injects per-service `security_opt: no-new-privileges`,
   `cap_drop: ALL` + `cap_add: <tenant_caps>`, `pids_limit`, and
   `cgroup_parent: jabali-user-<username>.slice`.
2. **agent gate** (`validateTenantCompose`) re-checks the *resolved*
   `docker compose config` and refuses to `up` if any service is `privileged`,
   adds a cap outside the allowlist, or bind-mounts outside the app data tree.
3. **userns-remap** maps container root to `dockremap` — escape ≠ host root.
4. **M18 slice** caps the tenant's aggregate CPU/memory.

## Recovery / troubleshooting

- **Tenant install 403 `docker_tenant_not_enabled`** → host flag missing; run
  `jabali docker enable-tenant --yes`.
- **`enable-tenant` failed mid-way, flag not written** → an app failed health
  after remap. Check `docker compose -f /var/lib/jabali/docker-apps/<x>/compose.yml ps`
  and logs; fix, then re-run `enable-tenant --yes` (idempotent — it skips if the
  flag already exists).
- **Admin app broken after enabling remap** → its data wasn't chowned into the
  range. `chown -R <dockremap_base>:<dockremap_base> /var/lib/jabali/docker-apps/<x>`
  then `docker compose ... up -d`.
- **Tenant deleted, container lingering** → the user-delete cascade dispatches
  `docker_app.delete` best-effort; if the agent was down, remove manually:
  `docker compose -f /var/lib/jabali/docker-apps/<instance_slug>/compose.yml down -v`.

## Residual risk (v1)

- Single shared `dockremap` range — no cross-container uid isolation between
  tenants (apps are still separate + slice-capped). Per-tenant subuid ranges +
  rootless-podman land in the v2 (out of scope).
- App-data disk overage is metered (`data_bytes`), not hard-blocked at the FS
  layer yet (Phase 5 follow-up).
