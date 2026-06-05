# Plan: M48 — Docker App Marketplace (admin-installable containerised apps)

**Status:** Draft (2026-06-05).
**Owner:** shuki
**Scope:** Admin-only catalog of curated Docker apps with one-click install, auto domain + nginx reverse proxy wiring, lifecycle controls, backup, and update flow with rollback. Closes the second half of [GH#127](https://github.com/shukiv/jabali-panel/issues/127). Admin-cron page (first half) shipped in PR #200.
**Depends on:**
- M2 (domains + nginx vhost rendering)
- M14 (notifications for update/rollback events)
- M18 (cgroup slices for resource accounting parity)
- M24 (managed IPs — apps will bind to the panel's public IPv4 by default)
- M30 (backup pipeline — restic per-app snapshots reuse the repo)
- M32 (panel cert + per-domain LE — app vhosts get TLS via the same path)
**Next migration:** see Schema section.
**Next ADR:** `0116-m48-docker-app-marketplace.md` — 12 decisions per the PR conversation.
**Working directory:** `/home/shuki/projects/jabali2` — branch `plan/m48-docker-app-marketplace`.

---

## 0. Operating assumptions

### Conventions inherited from this repo

- One PR per phase (small, reviewable). Conventional commits. Push to both Gitea + GitHub.
- Migrations: golang-migrate, both `.up.sql` and `.down.sql`. Down must not drop data — drop new tables only.
- Agent wire: NDJSON over UDS, `Default.Register("<command>", handler)`. Cross-boundary JSON tags pinned with golden fixtures per `feedback_cross_boundary_contracts`.
- Reconciler is the convergence engine. API writes DB intent; reconciler reads DB and drives the filesystem + agent. Handlers return `202` with the row; state converges in the background.
- Notifications: every long-running async outcome (install/update/rollback) fires an `events.publish` row so the bell + configured channels light up.
- Install footprint: every system package (`docker.io`, `docker-compose-plugin`, restic shim) lands in `install.sh`. Asset paths land in `update.go`'s sync step.

### What we are NOT doing in M48

- **Tenant self-install.** Admin-only for v1. Per-user docker networks + per-tenant socket scoping are queued for a follow-up.
- **Native Jabali-MariaDB mode.** Per the spec, v1 is compose-bundled DB only. The hybrid mode (point WordPress at the jabali MariaDB instance) lands in M48.x.
- **Multi-host / orchestrator.** Single-host docker engine. No Swarm, no k8s.
- **CVE-feed-driven "security-only" update mode.** Manual + Auto-with-rollback only in v1.
- **App-to-app linking / shared volumes between apps.** Each app is a closed compose project.
- **Custom user-supplied compose files.** Admin "Edit compose" lets you tweak an installed app, but the catalog is the source of available apps.

### Memory pointers

- `feedback_cross_boundary_contracts` — panel↔agent JSON tag drift.
- `feedback_deps_in_installer` — every new system package goes in install.sh AND update.go sync.
- `project_per_user_slices` — cgroup slice naming pattern; we reuse the same shape for docker app slices (`jabali-docker-app-<slug>.slice`).
- `project_m30_smoke_done` — restic backup pipeline; we add a per-app snapshot path.

---

## 1. Dependency graph

```
Phase 1 (engine + worker + ADR + schema)   ─┐
Phase 2 (catalog format + 3 first apps)     ─┤ parallel-safe with Phase 1
                                             │
Phase 3 (agent verbs + reconciler)          ─┤ needs Phase 1 + 2
Phase 4 (REST handlers + tests)             ─┤ needs Phase 3
                                             │
Phase 5 (admin UI: list + install + control)─┤ needs Phase 4
Phase 6 (nginx vhost auto-wire + LE)        ─┤ parallel with Phase 5; needs Phase 3
Phase 7 (update + rollback flow)            ─┤ needs Phase 5 + 6
Phase 8 (backup integration)                ─┤ parallel with Phase 7; needs Phase 3 + M30
Phase 9 (E2E + runbook + docs)              ─┘ needs all
```

True parallel groups:

- **Wave A:** Phase 1 ‖ Phase 2 (no shared files)
- **Wave B:** Phase 3 ‖ Phase 6 partial (vhost-template work can start during Phase 3)
- **Wave C:** Phase 4
- **Wave D:** Phase 5 ‖ Phase 6 (different packages)
- **Wave E:** Phase 7 ‖ Phase 8
- **Wave F:** Phase 9

Dispatch one wave at a time. Each wave gates the next on green CI + a manual smoke pass.

---

## 2. Model tier per phase

| Phase | Tier | Why |
|---|---|---|
| 1. engine + worker + ADR + schema | **opus** | design choices baked into the worker boundary + SQL are hardest to change later |
| 2. catalog format | default | YAML + 3 known-good composes |
| 3. agent verbs + reconciler | **opus** | state-machine correctness under crash/restart |
| 4. REST | default | mirrors existing application handlers |
| 5. admin UI | default | mirrors AdminApplicationList |
| 6. nginx vhost auto-wire | **opus** | wrong here = traffic blackhole on a panel-managed domain |
| 7. update + rollback | **opus** | rollback semantics are subtle |
| 8. backup integration | default | bolts onto restic |
| 9. E2E + runbook | default | end-to-end smoke through the catalog |

---

## 3. Architecture

```
┌──────────────────┐    REST /admin/docker-apps/*
│  panel-ui (React)│───────────────────────────────▶┐
└──────────────────┘                                 │
                                                     ▼
┌──────────────────────────────────────────────────────┐
│ panel-api (Go / Gin)                                  │
│   • handlers in internal/api/docker_apps.go           │
│   • repo: docker_app_repository.go                    │
│   • reconciler hook: reconcileDockerApps              │
└──────────────────────────────────────────────────────┘
                              │ NDJSON UDS /run/jabali/agent.sock
                              ▼
┌──────────────────────────────────────────────────────┐
│ panel-agent (Go)                                      │
│   • docker_app.install / .update / .start / .stop /   │
│     .restart / .rebuild / .delete / .logs /           │
│     .exec / .backup / .restore                        │
│   • runs as root; ONLY component with /var/run/docker.sock
│   • renders compose.yml.tmpl from catalog + per-install vars
└──────────────────────────────────────────────────────┘
                              │ docker.sock (filesystem)
                              ▼
                       /usr/bin/docker
```

Security boundary: `/var/run/docker.sock` is owned by `root:docker`, mode `0660`. Only the agent (running as root) and members of the `docker` group can touch it. The panel-api process never has access. Tenants never have access. The agent's docker-app verbs validate the slug against the catalog allowlist before any docker invocation.

### Per-app filesystem layout

```
/var/lib/jabali/docker-apps/
└── <slug>/                       owned by root:jabali 0750
    ├── compose.yml               rendered from catalog template
    ├── .env                      install-time generated secrets (DB pw, etc.)
    ├── config/                   bind-mounted at /config inside container
    ├── data/                     bind-mounted at /data
    ├── db/                       bind-mounted at /var/lib/postgres (or mysql)
    ├── uploads/                  bind-mounted at /uploads
    └── secrets/                  bind-mounted at /run/secrets (mode 0700)
```

Volume layout is per-catalog-entry — Vaultwarden uses `data/` only; Ghost uses `data/` + `db/`. Catalog metadata declares which subdirs to create.

### Port + reverse-proxy wiring (Phase 6)

Catalog declares all exposable container ports. Admin install drawer renders one row per declared port; each row has an Enabled toggle + Bind (loopback/public-IP) + Port (auto-pool or pinned) + Protocol (tcp/udp) + Reverse-proxy toggle.

- **Loopback + reverse_proxy=true** (the common case): port binds to `127.0.0.1:<host_port>`; a `domains` row with `managed_by='docker_app'` + `docker_app_id=<app id>` + `proxy_port=<host_port>` is created, and the reconciler renders an nginx vhost that `proxy_pass`-es to the upstream. LE issuance + per-domain SAN follow the same path as tenant domains — TLS for free.
- **Public-IP bind** (UDP, raw-TCP like Gitea SSH): port binds to `<managed_ip>:<host_port>`; no vhost, no nginx. UI surfaces it as `host:port` with a copy-button.
- **Loopback + reverse_proxy=false** (rare; metrics scrape targets): bound locally, no nginx, no domain — accessible only from operator console.

Allocation: free port found by walking 10000..19999 and asking the DB `INSERT ... uniq_dapp_pubport_global` to fail-on-conflict, retrying with the next number. Operator-pinned ports get the same uniqueness check; collision returns 400.

### Updates + rollback (Phase 7)

Three modes per install (catalog default + per-install override):

| Mode | Behavior |
|---|---|
| `manual` | UI shows "Update available" when poller detects a newer image SHA; admin clicks Update |
| `notify` | Same as manual + a notification fires to configured channels |
| `auto` | Reconciler runs the update flow automatically when a new SHA is detected; snapshot+rollback chain protects against bad updates |

Update flow:

```
1. restic snapshot of /var/lib/jabali/docker-apps/<slug>
2. docker compose pull && docker compose up -d
3. wait for HEALTHCHECK to report 'healthy' (timeout: 120s)
4. on failure → restic restore previous snapshot → docker compose up -d
5. fire notification: docker_app.updated  | .update_failed | .rolled_back
```

---

## 4. Schema

Migration numbers will be allocated at PR time; placeholders here.

### `XXX_docker_apps.up.sql`

```sql
CREATE TABLE docker_apps (
  id              CHAR(26) PRIMARY KEY,
  slug            VARCHAR(64) NOT NULL,                -- catalog slug (e.g. 'vaultwarden')
  name            VARCHAR(255) NOT NULL,               -- operator-chosen display name
  catalog_version VARCHAR(64) NOT NULL,                -- catalog entry version pin
  image_sha       VARCHAR(128) NULL,                   -- currently-running image digest
  status          VARCHAR(32) NOT NULL DEFAULT 'pending',
                                                       -- pending|installing|running|stopped|failed|updating|rolling_back|deleted
  domain_id       CHAR(26) NULL,                       -- FK domains.id when vhost is wired
  update_mode     VARCHAR(16) NOT NULL DEFAULT 'manual',
                                                       -- manual|notify|auto
  cpu_limit       VARCHAR(16) NULL,                    -- "0.5", "2.0" cores; NULL = catalog default
  memory_limit    VARCHAR(16) NULL,                    -- "512m", "2g"; NULL = catalog default
  pids_limit      INT NULL,                            -- NULL = catalog default
  last_error      TEXT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_docker_apps_slug_name (slug, name),
  CONSTRAINT fk_docker_apps_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE SET NULL
);

CREATE TABLE docker_app_published_ports (
  id              CHAR(26) PRIMARY KEY,
  app_id          CHAR(26) NOT NULL,
  port_name       VARCHAR(32) NOT NULL,                -- catalog-declared name ("http", "webhook", "ssh", ...)
  container_port  INT NOT NULL,
  bind_interface  VARCHAR(64) NOT NULL DEFAULT 'loopback',
                                                       -- 'loopback' OR 'public:<managed_ip_id>'
  host_port       INT NOT NULL,                        -- auto-allocated from 10000..19999 OR operator-pinned
  protocol        VARCHAR(8) NOT NULL DEFAULT 'tcp',   -- 'tcp' | 'udp'
  reverse_proxy   TINYINT(1) NOT NULL DEFAULT 1,
  enabled         TINYINT(1) NOT NULL DEFAULT 1,
  CONSTRAINT fk_dapp_pubport_app FOREIGN KEY (app_id) REFERENCES docker_apps(id) ON DELETE CASCADE,
  UNIQUE KEY uniq_dapp_pubport_per_app (app_id, port_name),
  UNIQUE KEY uniq_dapp_pubport_global (bind_interface, host_port, protocol)
);

CREATE TABLE docker_app_backups (
  id              CHAR(26) PRIMARY KEY,
  app_id          CHAR(26) NOT NULL,
  restic_id       VARCHAR(128) NOT NULL,
  size_bytes      BIGINT NOT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_docker_app_backups_app FOREIGN KEY (app_id) REFERENCES docker_apps(id) ON DELETE CASCADE
);
```

### `XXX_domains_managed_by.up.sql`

```sql
ALTER TABLE domains
  ADD COLUMN managed_by    VARCHAR(32) NOT NULL DEFAULT 'tenant',
  ADD COLUMN docker_app_id CHAR(26) NULL;
ALTER TABLE domains
  ADD CONSTRAINT fk_domains_docker_app FOREIGN KEY (docker_app_id) REFERENCES docker_apps(id) ON DELETE SET NULL;
```

`managed_by` values:
- `tenant` (default) — created by a user/admin via /domains
- `docker_app` — auto-managed; CRUD goes through the docker-app handler, not /domains

Reconciler must skip the tenant-side delete flow for `managed_by='docker_app'` rows; the docker-app delete path is authoritative.

---

## 5. API surface

All under `/api/v1/admin/docker-apps` (admin-only middleware).

```
GET    /catalog                       List available catalog entries.
GET    /                              List installed apps.
POST   /                              Install: {slug, name, domain, update_mode, overrides}.
GET    /:id                           Get one (with status, last_error, port, domain).
PATCH  /:id                           Update install-time settings (limits, update_mode).
DELETE /:id                           Uninstall (purges volumes by default; ?keep_volumes=1 to keep).
POST   /:id/start                     Start the compose project.
POST   /:id/stop                      Stop the compose project.
POST   /:id/restart                   Restart.
POST   /:id/rebuild                   docker compose up -d --force-recreate.
POST   /:id/update                    Manual update (snapshot → pull → up → health → rollback on fail).
GET    /:id/logs?lines=200            Tail logs (container stdout/stderr).
POST   /:id/exec                      {command} — admin shell. Returns one-shot exec result.
GET    /:id/compose                   Download rendered compose.yml.
PATCH  /:id/compose                   {compose_yml} — admin override (validated + nginx -t-equivalent compose lint).
POST   /:id/backup                    Trigger restic snapshot.
GET    /:id/backups                   List backups for this app.
POST   /:id/backups/:backup_id/restore  Restore from a snapshot (in-place).
```

---

## 6. Catalog format

```
install/docker-apps/
├── README.md
├── _schema/
│   └── app.schema.json              JSON-schema for app.yaml
├── vaultwarden/
│   ├── app.yaml                     metadata: name, description, icon ref, default limits, volumes list
│   ├── icon.svg
│   ├── compose.yml.tmpl             Go text/template
│   └── post-install.sh              optional; runs inside the agent after `up -d`
├── uptime-kuma/
│   └── ...
└── gitea/
    └── ...
```

### `app.yaml` shape

```yaml
slug: vaultwarden
name: Vaultwarden
version: "1.31.0"
description: Self-hosted password manager (Bitwarden-compatible).
icon: icon.svg
upstream: https://github.com/dani-garcia/vaultwarden
documentation: https://github.com/dani-garcia/vaultwarden/wiki

# Resource defaults (operator can override at install time).
resources:
  cpu: "0.5"
  memory: "256m"
  pids: 100

# Subdirectories to create under /var/lib/jabali/docker-apps/<slug>/.
volumes:
  - data

# Ports the container can expose. The admin install drawer renders one
# editable row per entry. Defaults pre-fill the form but everything is
# overridable. Apps with multiple ports list them all (Gitea ssh + http,
# n8n ui + webhook, etc).
ports:
  - name: http
    container_port: 80
    protocol: tcp
    default_enabled: true
    default_bind: loopback            # 'loopback' | 'public'
    default_reverse_proxy: true
    health_path: /alive               # used by Phase 7 health-gate

# Update channel: image tag we pull. operator sees a CTA when a newer SHA arrives.
image_channel: vaultwarden/server:latest

# Default update mode (operator can override).
update_mode: manual
```

### `compose.yml.tmpl`

```yaml
services:
  vaultwarden:
    image: {{ .ImageChannel }}
    container_name: jabali-app-{{ .Slug }}
    restart: unless-stopped
    environment:
      DOMAIN: "https://{{ .Domain }}"
      WEBSOCKET_ENABLED: "true"
    volumes:
      - {{ .DataRoot }}/data:/data
    ports:
      - "127.0.0.1:{{ .UpstreamPort }}:80"
    deploy:
      resources:
        limits:
          cpus: "{{ .CPULimit }}"
          memory: {{ .MemoryLimit }}
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1/alive"]
      interval: 30s
      retries: 3
```

Template variables filled by the agent at render time: `Slug`, `Name`, `Domain`, `UpstreamPort`, `DataRoot`, `CPULimit`, `MemoryLimit`, `ImageChannel`. Apps that need a DB include both services in the compose; the catalog generates DB passwords into `.env` at install time.

---

## 7. Phase steps

### Phase 1: engine + worker + ADR + schema

- `install.sh` installs `docker-ce`, `docker-compose-plugin`, `restic` if missing. Idempotent.
- Creates `/var/lib/jabali/docker-apps/` (`root:jabali 0750`).
- Creates `/etc/docker/daemon.json` drop-in: `{"live-restore": true, "log-driver": "journald", "default-ulimit": "nofile=8192:8192"}`. Restart docker if changed.
- Adds `jabali` system user to `docker` group (panel-agent runs as root anyway; this is belt-and-braces for direct `docker` CLI from operator console).
- Writes ADR-0116 with the 12 decisions.
- Lands the 2 migrations + `models/docker_app.go` + `docker_app_repository.go`.

### Phase 2: catalog format + 3 first apps

- Builds the catalog tree at `install/docker-apps/` with `_schema/app.schema.json`.
- Ships Vaultwarden, Uptime Kuma, Gitea complete (`app.yaml` + `compose.yml.tmpl` + `icon.svg`).
- Catalog loader in `internal/dockerapp/catalog.go` validates against schema at startup; bad entry → fail loud, not silent-skip.

### Phase 3: agent verbs + reconciler

- Verbs:
  - `docker_app.install` — render compose, write to disk, allocate port, `docker compose up -d`, wait healthy, return status.
  - `docker_app.update` — snapshot, pull, up, health-check, rollback path.
  - `docker_app.start` / `.stop` / `.restart` / `.rebuild` — wrappers around `docker compose <cmd>`.
  - `docker_app.delete` — `docker compose down -v`, optional purge of `/var/lib/jabali/docker-apps/<slug>`, free the port row.
  - `docker_app.logs`, `.exec`, `.backup`, `.restore`.
- Reconciler loop:
  - For each `docker_apps` row, target = derived from `status`.
  - Handles drift: row says `running` but container missing → `docker compose up -d`.
  - Re-renders compose.yml from catalog if catalog version differs from row's `catalog_version` (after admin clicks Update).

### Phase 4: REST + tests

- All endpoints from section 5.
- Admin-only middleware (mirror PR #195 gating pattern).
- Validation: slug must exist in catalog; name must satisfy `^[a-z0-9-]{1,32}$`; domain must be a valid hostname; per-install limits must satisfy regex (`^[0-9]+(\.[0-9]+)?$` for cpu, `^[0-9]+[kmg]$` for memory).
- Unit tests with stub agent: install happy path, install with bad slug → 400, delete cascades the domain row.

### Phase 5: admin UI

- `panel-ui/src/shells/admin/docker-apps/AdminDockerAppList.tsx` — Catalog tab + Installed tab.
- Install drawer: Catalog card → click → form (name, domain, update_mode, cpu/memory overrides, **ports table** with one editable row per catalog-declared port — Enabled / Bind interface / Host port / Protocol / Reverse-proxy) → POST → poll status until `running` or `failed`.
- Row actions: Start / Stop / Restart / Rebuild / Update / Logs (drawer) / Backup / Delete (popconfirm).
- Admin-only extras: Exec shell (xterm.js drawer), Edit compose (Monaco editor drawer), Force recreate.
- Nav: `/jabali-admin/docker-apps` after `applications` entry.

### Phase 6: nginx vhost auto-wire + LE

- Reconciler's domain path: when `domains.managed_by='docker_app'`, skip the tenant-style docroot probe; render a proxy_pass vhost pointing at `127.0.0.1:<port>` with WebSocket support enabled.
- LE issuance path unchanged — same routability gate + multi-resolver from PR #194/#196.
- Custom upstream block lives in `panel-agent/internal/commands/domain_create.go` template — gated on `IsDockerAppProxy=true` vhostData field.

### Phase 7: update + rollback flow

- Image SHA poller (every 6h via systemd timer) writes `image_sha` field on each app row when upstream has a newer digest.
- Notification kinds: `docker_app.update_available`, `docker_app.updated`, `docker_app.update_failed`, `docker_app.rolled_back`.
- Reconciler picks up rows with `update_mode='auto'` + newer SHA + `last_error IS NULL` and runs the flow.
- Rollback: restic snapshot taken at step 1; on health-check fail, restore + bring up.

### Phase 8: backup integration

- Per-app restic repo reuses the operator's existing backup destination (M30). Path-scoped backups under `docker-apps/<slug>/`.
- Manual `POST /:id/backup` triggers immediate snapshot.
- Restore goes through `docker compose down → restic restore → docker compose up -d` to avoid running-container file races.

### Phase 9: E2E + runbook + docs

- Playwright: install Vaultwarden → confirm domain shows up + cert issues + login page reachable.
- Runbook at `plans/m48-docker-app-marketplace-runbook.md`: deploy, health check, backup/restore, debug failed install.
- API docs page: `/jabali-admin/api-docs#docker-apps`.

---

## 8. Verification matrix

| Phase | Verify |
|---|---|
| 1 | `docker info` works; `/var/lib/jabali/docker-apps` exists with correct perms; migrations apply + rollback cleanly |
| 2 | `internal/dockerapp/catalog.go` `Load()` returns 3 entries; schema validation rejects malformed `app.yaml` |
| 3 | `docker_app.install` for Vaultwarden brings up the container + healthcheck reports healthy within 60s |
| 4 | unit tests above; manual `curl -X POST /api/v1/admin/docker-apps` round-trip returns 202 + the row |
| 5 | UI lists catalog, install drawer submits, install row appears with `pending` → `installing` → `running` transitions; logs drawer streams |
| 6 | nginx -t green for every `reverse_proxy=true` port; `curl -kI https://<chosen-domain>/` returns the app's index; LE cert provisions on next reconciler tick; public-IP-bound port (e.g. Gitea SSH on 222) reachable from the public internet on the chosen `<managed_ip>:<host_port>` |
| 7 | update flow: artificially set `image_sha` to old digest, set mode=auto; reconciler updates + healthchecks pass; corrupt image → rollback fires |
| 8 | `POST /:id/backup` creates restic snapshot; `POST /:id/backups/:bid/restore` round-trips |
| 9 | Playwright passes; runbook walks an operator through install + delete |

---

## 9. Risks + open questions

1. **Docker storage driver** on Debian 13 trixie defaults to `overlay2` — should be fine, but worth a sanity probe in Phase 1 (`docker info | grep "Storage Driver"`).
2. **Port collision** with tenant services — pool starts at 10000 to stay clear of common defaults (8080, 8443, etc.).
3. **Disk pressure**: docker images can grow fast. Phase 1 wires `docker system prune --volumes -f` as a weekly systemd timer. Per memory `feedback_per_tick_idempotent_loops` we ONLY run prune when image count > threshold to avoid wasted I/O.
4. **Update flow during heavy traffic** — rollback restores volumes but the brief "container down" window is real. Document the window in the runbook.
5. **Per-tenant docker support** is the obvious follow-up. Designs OK with podman rootless + per-tenant socket, but not in v1.

---

## 10. Out of scope (queued)

- Tenant self-install.
- Native Jabali-MariaDB mode (use the jabali MariaDB instance instead of bundled DB).
- App marketplace from an external registry (community submissions).
- Multi-host orchestration.
- "Security-only" update mode (needs CVE feed integration).
