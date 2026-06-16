# ADR-0131: Python Application Manager via native per-user systemd + nginx proxy (not Passenger)

**Date**: 2026-06-16
**Status**: Proposed
**Deciders**: shuki + Claude
**Related**: ADR-0023 (M9 PHP-FPM per-user pools), M18 (per-user cgroup
slices), M25 (unix-socket lockdown), M34 (per-user egress firewall), the
Docker-app subsystem. Implements GH #203.

## Context

jabali runs only PHP today (PHP-FPM per-user pools). Operators want to host
**Python** web apps (Django/Flask/FastAPI) the way cPanel's Application
Manager does: register an app, point it at a domain/path, pick a runtime, set
env vars, start/stop — no shell. cPanel implements this with **Phusion
Passenger** (a web-server module that spawns and supervises the app).

## Decision

Run each app as a **per-user systemd service on a unix socket**, reverse-
proxied by nginx `proxy_pass` — NOT via a Passenger nginx module.

- Python WSGI apps → **gunicorn**; ASGI apps → **uvicorn**, bound to
  `/run/jabali-app/<app_id>.sock`.
- The service runs `User=<owner>` inside the owner's **user slice** (cgroup
  cpu/mem/pids limits, M18) and **egress firewall** (M34).
- nginx serves it via a per-domain include (`location <base_uri> { proxy_pass
  http://unix:<sock>; }`) — the existing per-domain include dir + proxy
  directives, `nginx -t`-gated.
- DB-as-truth: a `python_apps` registry (mirroring `docker_apps`) + a
  reconciler hook converge the venv, the systemd unit, and the nginx include.
- **Opt-in**: `server_settings.python_apps_enabled` (default off); a Server
  Settings → Apps tab toggle installs the runtimes and reveals the feature.

## Rationale

- **Reuses jabali's machinery.** Per-user systemd services, the user slice,
  the egress firewall, per-domain nginx includes, and the reconciler already
  exist (FPM pools + Docker apps). The native model drops straight in; a
  Passenger module would be a parallel, foreign mechanism.
- **No nginx coupling.** jabali deliberately runs Debian-native nginx
  (`feedback_nginx_debian_native_not_sury`). Passenger needs its apt repo +
  a dynamic nginx module — a standing coupling and upgrade-fragility we avoid.
- **Same containment as PHP.** Apps run as the user, slice-limited and
  egress-filtered — identical blast radius to PHP-FPM, not a new root-adjacent
  surface.
- **Same UX.** Register → domain/path, runtime+version, entrypoint, env,
  restart, logs — delivered without Passenger.

## Alternatives considered

- **Phusion Passenger (cPanel-identical).** Closest to cPanel and less
  process-management code, but couples nginx to a Passenger module on
  Debian-native nginx and bypasses jabali's per-user-systemd/reconciler/slice
  model. Rejected for coupling + architectural mismatch.
- **Docker-only (run Python apps as containers).** The Docker-app subsystem
  already exists, but it's a different UX (operator supplies an image/compose)
  and not the native "App Manager" experience #203 asks for. Kept as a
  separate offering, not the answer here.

## Consequences

### Positive
- Native Python hosting with the cPanel App-Manager UX, fully inside jabali's
  existing isolation + convergence model; no new web-server module.
- Extends cleanly to Node (NodeSource + `node` service) and Ruby (puma) by
  adding `runtime` enum values + per-runtime installers — same shape.

### Negative
- jabali owns the process-management glue (venv build, gunicorn/uvicorn unit,
  health) that Passenger would otherwise provide — more agent/reconciler code.
- A new always-available runtime to keep patched (offered Python versions).
  Mitigated: opt-in (default off), per-venv server installs, resource caps on.

## Implementation

Blueprint `plans/m-python-app-manager.md`. Migration `000168`
(`python_apps` + `python_app_env`); `server_settings.python_apps_enabled`;
`app.python.*` agent commands; `jabali-app@<id>.service` template;
`reconcilePythonApps`; Application Manager UI + Server Settings → Apps tab.
Waves A–F; live-verify a Flask + a FastAPI app before Accepted.
