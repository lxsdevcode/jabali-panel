# ADR-0116: M48 Docker App Marketplace — Architectural Decisions

**Date:** 2026-06-05
**Status:** Proposed (queued behind blueprint review)
**Owner:** shuki
**Companion plan:** [`plans/m48-docker-app-marketplace.md`](../../plans/m48-docker-app-marketplace.md)

---

## Context

GH#127 asks for a curated "App Marketplace" of Docker apps that admins can install in one click, with the panel handling domain attachment + nginx reverse proxy + TLS + backup + update + rollback. The first apps in scope are Vaultwarden, Uptime Kuma, Gitea, n8n, Ghost, Plausible, Matomo, PrivateBin, FreshRSS, Immich, Mealie, Linkwarden, Nextcloud.

Docker is unforgiving on security: exposing `/var/run/docker.sock` to any container or unprivileged process is effectively root-equivalent on the host. The architecture has to make that exposure impossible. The 12 decisions below define the boundaries.

---

## Decision 1 — Scope v1: admin-only

Admins install apps; tenants don't see the catalog. Per-tenant docker support is queued (would need rootless podman + per-tenant socket scoping; out of scope for v1).

**Why:** simplest secure model. Lets us ship before designing per-tenant isolation. Matches the user's spec ("admins can install docker apps").

**Implications:**

- No tenant-side routes.
- All `/api/v1/admin/docker-apps/*` endpoints gated on `claims.IsAdmin`.

---

## Decision 2 — Engine: rootful Docker, agent-only socket access

Use rootful Docker CE on Debian. The agent (running as root, listening on `/run/jabali/agent.sock` for the panel) is the **only** process with access to `/var/run/docker.sock`. Panel-api never has it. Tenants never have it.

**Why:** Docker is the most documented, the catalog composes we want to ship were written against it, and the security model is acceptable IF the socket stays off everything except the agent.

**Rejected:** rootless Docker (filesystem permission gymnastics with bind-mounts on tenant data); Podman (mature but inconsistent compose-v2 support; would need parallel test matrix).

**Implications:**

- `/var/run/docker.sock` owner stays `root:docker`, mode `0660`.
- Agent runs as root (already does — same boundary as the rest of the panel).
- Operator console access to `docker` CLI works because the `jabali` user is in the `docker` group; we accept that an operator with root on the box can issue docker commands directly.

---

## Decision 3 — App data root: `/var/lib/jabali/docker-apps/<slug>/`

```
/var/lib/jabali/docker-apps/
└── <slug>/                       owner root:jabali, mode 0750
    ├── compose.yml               rendered from catalog template
    ├── .env                      install-time secrets (mode 0600)
    ├── config/  data/  db/  uploads/  secrets/
```

Per-app subdirs declared in the catalog `app.yaml` `volumes:` list. Container bind-mounts are explicit — no wildcard mounts.

**Why:** keeps every app's bytes in one tree (snapshotting + delete are trivial); explicit per-volume directories make the "config / data / database / uploads / secrets" separation in the spec real instead of conceptual.

---

## Decision 4 — Hostname binding: operator picks, reuse `domains` table

When the admin installs an app, they pick a fully-qualified hostname (e.g. `vault.example.com`). The panel creates a `domains` row with `managed_by='docker_app'` and `docker_app_id=<app id>`. Reconciler renders a tenant-style vhost that `proxy_pass`-es to the allocated upstream port. LE issuance reuses the same path as tenant domains (PR #194/#196 multi-resolver gate applies).

**Why:** reuses the entire `domains` + `nginx vhost` + `LE` + per-domain SAN pipeline instead of a parallel system. The only new piece is the `managed_by` discriminator + the `docker_app_id` link.

**Rejected:** auto-derived hostname like `<slug>.<panel-hostname>` — too inflexible; many operators want a separate brand domain per app.

---

## Decision 5 — Port allocation: dynamic pool 10000–19999

A `docker_app_ports` table seeded with the range 10000..19999. Install grabs the lowest free row, stamps `app_id`. Delete frees it (`app_id = NULL`).

**Why:** keeps app upstream binds on `127.0.0.1` (never exposed externally) without needing port-config gymnastics. 10k ports is two orders of magnitude more than realistic app counts per host. Range starts at 10000 to dodge common service ports.

---

## Decision 6 — DB mode v1: bundled in compose

Apps that need a database ship the DB as a sibling service in their own compose (`vaultwarden + sqlite-file`, `gitea + postgres`, etc.). No hookup to the jabali MariaDB instance.

**Why:** simplest portable model. Per the user's spec: "start with Docker isolated mode for Docker apps." Hybrid (use jabali-MariaDB) lands in M48.x after the catalog is exercised.

---

## Decision 7 — Resource limits source: catalog default + per-install override

`app.yaml` declares default `cpu`, `memory`, `pids`. Install drawer lets the admin override. Limits land as compose `deploy.resources.limits.cpus` / `.memory` plus `--pids-limit` via the agent.

**Why:** catalog authors know app-appropriate defaults; operators know their host. Both inputs land in the same compose render path.

---

## Decision 8 — First 3 apps to ship in Phase 1: Vaultwarden, Uptime Kuma, Gitea

Smallest blast radius, well-documented composes, cover the "single container", "container + DB", and "container with persistent state but no DB" cases respectively.

The rest of the spec list (n8n, Ghost, Plausible, Matomo, PrivateBin, FreshRSS, Immich, Mealie, Linkwarden, Nextcloud) queues as M48.x catalog additions — each one is a new directory under `install/docker-apps/` and tests of its compose render.

---

## Decision 9 — Update policy v1: Manual + Auto+rollback

Two modes per install:

- `manual` — UI shows "Update available" when poller detects a newer image SHA; admin clicks Update.
- `auto` — reconciler runs the snapshot → pull → up → health → rollback flow on its own.

"Security-only update notification" (third mode in the spec) deferred — needs CVE feed integration we don't yet have.

**Why:** ships the minimum useful split. `auto` covers the "I just want it to stay current" operator; `manual` covers the "I want control" operator. The CVE-feed mode wraps both and can land later.

---

## Decision 10 — Admin-only "Exec shell" / "Edit compose"

Both are admin features. Never exposed to tenants. "Exec shell" pipes through the agent's `docker_app.exec` verb which validates the slug against the catalog before running `docker exec`. "Edit compose" lets the admin write a custom compose for an installed app; the agent validates with `docker compose config` before writing.

**Why:** giving tenants shell inside a container effectively gives them code execution as that container's user. With a misconfigured app that could break out (CVE in docker, escape via volume, etc.) — risk vs. benefit doesn't pencil out for tenants. Admins already have root.

---

## Decision 11 — Catalog source: in-repo at `install/docker-apps/<slug>/`

Catalog ships with the panel. `jabali update` syncs new app entries to `/usr/local/share/jabali/docker-apps/`. Loader at startup parses every entry against `_schema/app.schema.json`; bad entry → loud error in the log + the catalog API skips it.

**Why:** keeps versioning aligned with the panel itself. No external registry to mirror, no signing infrastructure to design, no supply-chain question to answer in v1. Community-submitted apps are a separate problem.

---

## Decision 12 — Backup integration: separate `docker_app_backups` table + restic per app

`docker_app_backups` table tracks restic snapshot IDs per app. Snapshots go to the operator's existing backup destination (M30). Manual trigger via `POST /:id/backup`; auto-triggered before any update via the rollback flow (Decision 9).

**Why:** keeps app backups separate from `system_backup` and `account_full` so retention can be tuned per-app. Reuses the operator's existing restic repo so it lands in their existing backup destination + retention policy.

---

## Consequences

**Positive**

- Tight worker boundary: docker.sock never reaches an unprivileged context.
- Reuses three load-bearing pipelines (domains + nginx + LE + restic). Net new surface is small.
- Catalog format is plain YAML + Go template — easy to add apps over time without code changes.
- Update + rollback is real (not "auto-update and hope"). Snapshot is mandatory.

**Negative**

- Docker daemon is a new always-running thing on the host; adds ~150MB RSS baseline. Acceptable for hosts that opt into M48.
- Per-tenant support deferred. Operators sharing a box with multiple paying customers can't sell "your own apps" to tenants yet.
- Catalog is admin-curated; "I want to install $myCustomApp" requires the admin to edit panel-managed paths.
- Auto-update can still kill an app if upstream ships a corrupt image (rollback catches the next step, but the brief window is real and visible to users).

**Operational**

- Adds `docker-ce` + `docker-compose-plugin` to the install footprint. ~400MB disk for engine + a few common base images.
- `restic` is already installed for M30; per-app paths cost storage proportional to app size.
- Worker (agent) needs additional logging discipline — every docker call gets logged with `slug` + `app_id` so an operator debugging a failed install has a trail.

---

## Reviewed

- 2026-06-05 — drafted with the 12 decisions confirmed via PR conversation.
