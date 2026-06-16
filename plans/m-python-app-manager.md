# Blueprint: Python Application Manager (cPanel-style) — native systemd + nginx

**Issue**: GH #203. **ADR**: 0131 (proposed). **Status**: blueprint for review.
**Decision (operator)**: native per-app systemd + nginx proxy (NOT Passenger);
**Python first** (Node/Ruby as later waves); opt-in under Server Settings → Apps.

## Goal

Let a user register and run a **Python web app** (Django/Flask/FastAPI/any
WSGI or ASGI app) on their domain or a sub-path, the way cPanel's Application
Manager does — without touching the shell. The panel manages: app root, the
runtime (Python version + virtualenv), the entrypoint (WSGI/ASGI module),
the mount point (domain + base URI), environment variables, and
start/stop/restart + logs.

PHP stays on PHP-FPM (unchanged). This adds a *parallel* runtime for Python.

## Architecture (ADR-0131)

Each app runs as a **per-user systemd service** bound to a **unix socket**;
nginx `proxy_pass`es to that socket under the app's domain + base URI. This
is the exact shape jabali already uses for PHP-FPM pools (`jabali-fpm@<user>`)
and Docker apps (service + nginx proxy) — so it slots into the existing
per-user slice (cgroup limits, M18), egress firewall (M34), and reconciler.

```
browser → nginx (domain vhost, location <base_uri>)
        → proxy_pass http://unix:/run/jabali-app/<app_id>.sock
        → systemd: jabali-app@<app_id>.service  (User=<owner>, in the user slice)
            → gunicorn/uvicorn  (Python) running the app's WSGI/ASGI entrypoint
               inside /home/<user>/.../<app_root>/venv
```

Why not Phusion Passenger (cPanel's engine): it couples nginx to a Passenger
module (apt repo + dynamic module on Debian-native nginx — against
`feedback_nginx_debian_native_not_sury`) and doesn't reuse jabali's
per-user-systemd / reconciler / slice machinery. The native approach gives the
same UX with full control and no nginx-module coupling.

- **Python WSGI** (Django/Flask) → **gunicorn** `--bind unix:<sock>`.
- **Python ASGI** (FastAPI/Starlette) → **uvicorn** (or gunicorn w/ uvicorn
  workers) on the same socket.
- The app's start command is derived (`gunicorn <module>:<app>`) but
  operator-overridable for advanced cases.

## Data model (migration 000168 — `python_apps`)

Mirrors `docker_apps` (proven registry shape):

| column | notes |
|---|---|
| id (ULID), user_id, domain_id | owner + which domain |
| name | display |
| runtime | `python` (enum, future: node/ruby) |
| python_version | "3.11" / "3.12" — must be installed (see Apps tab) |
| app_root | path under the user's home, scope-validated (filesafe) |
| app_type | `wsgi` \| `asgi` |
| entrypoint | module:callable, e.g. `myapp.wsgi:application` / `main:app` |
| base_uri | `/` or `/app` (sub-path mount) |
| start_command | derived; overridable |
| status | pending/running/stopped/failed (reconciler-owned) |
| cpu_limit / memory_limit / pids_limit | per-app slice drop-in (M18 pattern) |
| last_error | surfaced in UI |
| created_at / updated_at | |

Plus `python_app_env` (app_id, key, value) — like `docker_app_env`. Secrets
stored per the reveal-once / encrypted pattern already used for app env.

## Opt-in (Server Settings → Apps tab)

- New `server_settings.python_apps_enabled` bool (default 0) — mirrors
  `docker_marketplace_enabled` / `postgres_enabled`.
- A new **Apps** tab in Server Settings (doesn't exist yet) with the toggle.
  Enabling it dispatches an agent install step: ensure `python3`, the
  `python3.X-venv` + `python3.X-dev` packages for the offered versions, the
  build toolchain (`gcc`/`build-essential`, `libffi-dev`, `libssl-dev`) so
  C-extension wheels (psycopg2, cryptography, lxml, …) compile, and a pinned
  `gunicorn`/`uvicorn` installed **per-venv** (not global, so versions don't
  collide). Idempotent; added to `install.sh` so a
  fresh host that has the flag on provisions on boot
  (`feedback_install_sh_is_truth`).
- When the flag is off: the Application Manager UI is hidden and the API
  endpoints 403 — same gating as Docker marketplace.

## Reconciler convergence (DB-as-truth)

A `reconcilePythonApps` hook (sibling to the FPM-pool / docker-app reconcilers):
for each app row, the agent converges:
1. **venv**: create `<app_root>/venv` for the chosen python_version (as the
   user), `pip install` the app's `requirements.txt` + the server
   (gunicorn/uvicorn). Idempotent (skip when present + unchanged).
2. **systemd unit**: render `jabali-app@<app_id>.service` (User=<owner>,
   WorkingDirectory=<app_root>, ExecStart=<start_command> bound to the
   per-app unix socket, in the user's slice for cgroup limits, EnvironmentFile
   from the app_env). Drop-in for cpu/mem/pids limits (M18 pattern).
3. **nginx**: write a per-domain include
   (`/etc/nginx/jabali/<domain>/app-<app_id>.conf`) with `location <base_uri>
   { proxy_pass http://unix:<sock>; … }` — reuses the existing per-domain
   include dir + the `nginxrules` proxy directives. `nginx -t` gate.
4. **state**: enable/restart the unit; **liveness** = `systemctl is-active`
   AND an HTTP GET to `base_uri` over the socket returning non-5xx within a
   startup deadline (a socket that merely accepts ≠ a working app — gunicorn
   can be up while the app 500s on import). Write status/last_error from that.
   Side-effects gated behind change-compare (`feedback_per_tick_idempotent_loops`).

Delete tears down unit + socket + nginx include + (optionally) the venv.

## Agent commands

`app.python.ensure_venv`, `app.python.write_unit`, `app.python.control`
(start/stop/restart), `app.python.logs`, `app.python.remove`. All run the
build/run **as the owning user** (never root) — `sudo -u <user>` for pip/venv,
`User=` in the unit. Scope-validated app_root via `filesafe`.

## UI

- **Application Manager** page (user shell): list apps; "Create app" (domain,
  base URI, python version, app_type, entrypoint, app_root picker via File
  Manager); per-app: Restart/Stop/Start, env-var editor (reuse the
  docker-app env UI), logs viewer (reuse logs exec), status + last_error.
- **Server Settings → Apps** tab (admin): the opt-in toggle + which python
  versions to offer.

## Security

- Apps run as the **owning Linux user**, inside their **user slice**
  (cgroup cpu/mem/pids, M18) and **egress firewall** (M34) — same blast-radius
  containment as PHP-FPM.
- No code runs as root; pip/venv/gunicorn all `sudo -u <user>`.
- app_root + requirements path scope-validated (`filesafe`); the unix socket
  lives under `/run/jabali-app/` owned `<user>:www-data` 0660 so only nginx +
  the user reach it (M25 socket pattern).
- Resource caps default-on (a runaway Python worker can't starve the host).

## Waves (dispatchable)

- **A — foundation**: migration `python_apps` + `python_app_env`, models,
  repo, `python_apps_enabled` flag + Apps-tab toggle + install step. (No app
  execution yet — schema + opt-in.)
- **B — agent runtime**: `app.python.*` agent commands (venv, unit, control,
  logs, remove) + the per-app systemd unit template + nginx include template.
- **C — reconciler**: `reconcilePythonApps` convergence + status polling.
- **D — API**: CRUD endpoints (RequireAuth + owner scope + flag gate) + env.
- **E — UI**: Application Manager page + Server Settings Apps tab.
- **F — E2E + runbook + ADR-0131 accepted**; live-verify a Flask + a FastAPI
  app on a real host.

Later milestones: Node (NodeSource + `node` service), Ruby (puma) — same
shape, new `runtime` enum values + per-runtime venv/installer.

## Open questions for review

1. venv location: under app_root (`<app_root>/venv`, user-visible/backups) vs
   `/var/lib/jabali-app/<app_id>/venv` (hidden, cleaner). Lean app_root for
   cPanel parity + backup inclusion.
2. python versions: ship system python only, or also deadsnakes/uv for
   multi-version? v1: system python3 + the distro's `pythonX.Y-venv` for the
   versions the Apps tab installs.
3. Sub-path mounting is a REAL concern, not free: a WSGI/ASGI app under
   `/app` emits wrong URLs (redirects, url_for, static) unless told its
   prefix. v1 DECISION: derive the prefix into the start command —
   gunicorn gets `SCRIPT_NAME=<base_uri>` (env), uvicorn gets
   `--root-path <base_uri>`. nginx alone won't set it. (If that proves
   fragile per-framework, fall back to v1 = `base_uri=/` only — a dedicated
   subdomain whose base_uri=/ — and defer sub-path.)
