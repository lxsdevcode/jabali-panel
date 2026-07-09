# Module install-on-enable (complete M353 modular install)

**Objective:** enabling an optional module from Server Settings → Modules must
INSTALL that module's packages/services if they aren't present (and start them),
not just flip a DB flag. Today the flags only gate nav + routes, so on a
`--minimal` host "DNS = On" leaves `pdns` inactive (never installed).

**Priority:** high (operator-reported). **Depends on nothing external.**
**Base branch:** `main`.

---

## Background (cold-start context)

- Module flags live in `server_settings` (`dns_enabled`, `mail_enabled`,
  `security_enabled`, `quota_enabled`, `api_enabled`) — see
  `panel-api/internal/models/server_settings.go:104`. The PATCH handler
  (`panel-api/internal/api/server_settings.go` ~line 118) currently just writes
  the bool. No install.
- **Proven reference pattern already in the tree** — copy it, don't invent:
  - **Docker** toggle → background goroutine dispatches an agent install
    (`server_settings.go` ~line 660; `docker_app.install` etc.).
  - **PostgreSQL** toggle → `db.postgres.{install,uninstall,enable,disable,status}`
    agent commands (`panel-agent/internal/commands/db_postgres_lifecycle.go`),
    dispatched from the PATCH in a `go func(){}` with a 5-minute bg context
    (`server_settings.go` ~line 636). install.sh no longer runs postgres on fresh
    installs — the panel installs it on demand. **This is the exact shape to
    replicate for the 5 modules.**
- **install.sh already knows each module's install steps** — `main()` runs them
  as `run_if_module <key> <fn>` (verified line ~12760+):
  | Module | install.sh functions (in order) | Needs install? |
  |---|---|---|
  | `quota` | `configure_disk_quota` | yes |
  | `dns` | `install_powerdns`, `bootstrap_pdns_self_zone`, `install_pdns_recursor` | yes |
  | `mail` | `install_stalwart`, `install_stalwart_apply`, `install_jabali_mailhook`, `install_bulwark` | yes |
  | `security` | `install_crowdsec`, `configure_crowdsec_mariadb`, `install_crowdsec_appsec`, `install_crowdsec_nginx_bouncer`, `install_crowdsec_profiles`, `install_login_allowlist_default_conf` | yes |
  | `python_apps` | `install_python_apps_runtime` | yes (not surfaced in the 5-toggle UI yet) |
  | `api` | — | **no** — REST API is flag-only, no packages |
- Service→module map (for status + the Services-card filter already shipped):
  mail → `jabali-stalwart`,`jabali-webmail`; dns → `pdns`(+`pdns-recursor`);
  security → `crowdsec`.

### Architecture decision (the load-bearing choice)

**Reuse install.sh as the single source of truth — do NOT reimplement each
module's install in Go.** The postgres path reimplemented in Go because it's
small; mail/dns/security are large bash blocks that would drift immediately.

Add a **targeted install mode** to install.sh: `install.sh --install-module <key>`
runs ONLY that module's `run_if_module` function list, idempotently, and **exits
without touching core** (never re-runs nginx/mariadb/panel on a live host). The
agent shells to the release-bundled install.sh in this mode.

### Feasibility facts (verified this session — the approach is viable)

- **install.sh is sourceable + persisted.** It ends with
  `if [[ "${BASH_SOURCE[0]:-$0}" == "${0}" ]]; then main "$@"; fi`, so it can be
  `source`d (skips `main`) or invoked with a new arg parsed *before* that guard.
  It lives on every installed host at `/opt/jabali-panel/install.sh`
  (`REPO_DIR=${JABALI_REPO_DIR:-/opt/jabali-panel}`) — the agent has a real path
  to invoke. No "persist install.sh" prerequisite needed.
- **The screenshot bug is a seed failure, not a transition bug.** `seed_module_flags`
  (install.sh ~line 355) does `UPDATE server_settings SET dns_enabled=… WHERE id=1`
  from `JABALI_MODULES`, but on failure it only `_warn`s (line ~375) — and the model
  columns default `1`. So a minimal install where the seed didn't run/failed leaves
  **every flag = 1 (all On in the UI) with nothing installed** — exactly DNS-On /
  pdns-inactive. **This means transition-triggered install alone does NOT fix the
  reported state** (no false→true edge ever happens). See the convergence
  requirement below — it is the primary deliverable, not a nice-to-have.

### THE risk this plan must retire (Step 0 gate)

install.sh module functions were written for **fresh-install context** — they
assume env vars (`JABALI_SRV_HOSTNAME`, `JABALI_SRV_IPV4`…), a specific ordering,
and that earlier functions already ran. Run standalone post-install they may hit
missing preconditions (e.g. `bootstrap_pdns_self_zone` needs the server-settings
env; `install_stalwart` assumes the panel primary domain exists). **Each module's
function list MUST be audited for standalone-runnability and idempotency before
it's wired to a runtime toggle** — a half-run install on a live host is worse than
"module shows inactive". This is Step 0 and it gates everything.

### Invariants (verify after every step)
- `install.sh --install-module <key>` runs ONLY that module's functions; core
  installs (nginx/mariadb/panel/agent) are never invoked. Prove with a dry-run
  trace.
- Every module install is idempotent: running it on an already-installed host is
  a no-op success, not a re-install or an error.
- A failed background install SURFACES to the operator (status endpoint / toast),
  never fails silently — and never rolls back the DB flag (intent is recorded;
  retry converges).
- `bash -n install.sh` clean; `go build ./...`; `npx tsc -b` in panel-ui.

---

## Step 0 — Audit each module's install functions for standalone runtime (gate)

**No production code.** For each module (quota, dns, mail, security), read the
`run_if_module` functions and confirm:
1. Every env var / precondition they read is available at runtime (server_settings
   is populated; the panel primary domain exists) — OR list what must be re-derived
   / passed in.
2. They are idempotent (safe re-run) — note any that aren't (e.g. a function that
   appends to a config without a guard, or fails if a user/table already exists).
3. Ordering: the functions can run as a standalone block without core-only setup
   that main() did earlier.

Also confirm (feasibility, already largely verified above): the env/logger setup
these functions need is sourced at the TOP of install.sh (outside `main()`), so the
targeted mode can run it without the core package installs. **`security` is the
highest-risk module** — `install_crowdsec_appsec` has a documented render-config-
before-binary ordering scar (memory: "install appsec before binary GH#109"); audit
its standalone ordering hardest, and consider shipping it last.

**Exit:** a written per-module verdict — "runnable standalone as-is" vs "needs
these guards/env fixes first". Any "needs fixes" module is done in its own follow-up
step before wiring; do NOT wire a module whose functions aren't standalone-safe.

---

## Step 1 — install.sh: targeted `--install-module <key>` mode

**Depends on:** Step 0 (only wire audited-safe modules).

### Tasks
1. Refactor `main()`'s per-module blocks into named functions so both `main()` and
   the targeted entry call the SAME list (DRY, no drift):
   `install_module_quota`, `install_module_dns`, `install_module_mail`,
   `install_module_security`. Each body is exactly the current `run_if_module`
   sequence for that key (minus `run_if_module`, since the targeted mode implies
   the key). `main()` becomes `run_if_module dns install_module_dns` etc.
2. Add arg parsing: `install.sh --install-module <key>` sets a mode that, after
   sourcing the function/env setup install.sh already does at the top, calls
   `install_module_<key>` and exits — **bypassing the core install path**. Guard:
   reject unknown keys; require the panel already be installed (refuse to
   "module-install" onto a host with no panel).
3. Ensure the top-of-script env/bootstrap that these functions need (settings env,
   logger `_log/_ok/_warn/_err`) runs in this mode too, but NOT the core package
   installs. (install.sh already sources its env early; scope carefully.)

### Verify
- `bash -n install.sh`.
- Dry-run trace (`bash -x` with core functions stubbed) shows only the module's
  functions run for `--install-module dns`.

### Exit
`install.sh --install-module dns|mail|security|quota` runs just that module,
idempotently, without core.

---

## Step 2 — Agent: `system.module.install` + `system.module.status`

**Depends on:** Step 1.

### Tasks
1. `system.module.status` — for a given key, report `{installed: bool, active:
   bool}` by probing the module's marker (dns → `pdns` unit present + package;
   mail → `jabali-stalwart` unit + stalwart binary; security → `crowdsec` binary;
   quota → `quota` tooling + `/home` quota on). Read-only, fast.
2. `system.module.install` — shells to the release-bundled install.sh
   (`JABALI_INSTALL_SH` path, same one the TUI installer/bootstrap use) with
   `--install-module <key>`, streams stdout/stderr to the agent log (NOT to the
   API — reuse the JAB-68 discipline: log full, return a generic result +
   structured status), long timeout (apt can take minutes), idempotent. Returns
   `{ok, installed, active}` from a final `status` probe. Register both in the
   dispatch table.
3. Validate the key against an allowlist (dns/mail/security/quota) — never pass an
   arbitrary string into the shell.
4. **Serialize installs behind a single agent-side install mutex.** Two module
   installs (operator flips mail then security; or a toggle + the convergence pass)
   would otherwise run `apt` concurrently and collide on the dpkg global lock —
   the project's own M29 "apt lock-timeout" scar. `system.module.install` must
   acquire a process-wide install lock (reuse the existing `aptMu` in
   `php_ext_shell.go` if it fits, or a dedicated module-install mutex) so installs
   run one at a time; queued callers wait, not fail.

### Verify
- `go build ./panel-agent/...`; unit test the key-allowlist + status marker parse
  + that two concurrent `system.module.install` calls serialize (don't both hold
  the lock).

### Exit
Agent can install + report status for each module on demand.

---

## Step 3 — panel-api: dispatch install-on-enable (mirror postgres)

**Depends on:** Step 2.

### Tasks
1. In the settings PATCH handler, for each of dns/mail/security/quota: when the
   flag flips false→true and `h.cfg.Agent != nil`, `go func(){}` with a
   5–10 min bg context calling `system.module.install` (exactly the postgres
   goroutine shape). Persist the flag first (intent recorded); the install is
   detached so the PATCH returns immediately.
2. Skip the install when `system.module.status` already reports installed — then
   just ensure services enabled+started (a cheap `system.module.install` is still
   idempotent, so calling it unconditionally is acceptable; prefer a status
   short-circuit to avoid needless apt runs).
3. On flip true→false: stop+disable the module's services (data kept) — either
   reuse existing disable behavior or add a `system.module.disable`. (Out of the
   core ask; include only if trivial.)
4. Record the install outcome so Step 4 can surface it: a per-module install
   status (in Redis or a small `module_install_state` the status endpoint reads).

### Verify
- `go build ./panel-api/...`; test that flip-true dispatches install, flip-false
  doesn't, and status=installed short-circuits.

### Exit
Toggling a module On installs it in the background; Off stops it.

---

## Step 3b — CONVERGENCE: install modules that are enabled-but-missing (the actual bug fix)

**Depends on:** Step 2. **This is the primary deliverable** — without it the
user's DNS-On/pdns-inactive screenshot is unchanged, because that row has no
false→true transition to trigger Step 3.

### Tasks
1. Add a convergence pass that, for each module flag = enabled, probes
   `system.module.status` and, if `installed == false`, dispatches
   `system.module.install` (serialized per Step 2). Natural homes, in order of
   preference:
   - The **reconciler** (already converges domain/service state on a tick) — add a
     periodic "modules" convergence phase, gated so it runs the install at most
     once per module until status flips (don't re-dispatch every tick while apt
     runs).
   - AND/OR a **`jabali repair` / cache-doctor-style CLI** the operator can run
     on demand for the immediate fix on existing hosts.
2. Idempotence + backoff: never dispatch a second install for a module while one
   is in flight or recently failed; surface a persistent "install failed, retry"
   state rather than hot-looping apt.
3. Fix the seed too (defense in depth): make `seed_module_flags` failure louder /
   retried, OR flip the model column defaults to `0` so a missing seed means
   "off + not shown" instead of "on + broken". (Design choice — a `0` default is
   the safer failure mode, but changes fresh-install behavior; decide in Step 0.)

### Verify
- On a minimal host with flags defaulted On but nothing installed, the convergence
  pass installs each enabled module exactly once and stops re-dispatching once up.

### Exit
An already-enabled-but-uninstalled module converges to installed+active without
any toggle — the reported bug is fixed for existing hosts, not just future flips.

---

## Step 4 — UI: surface install progress + result

**Depends on:** Step 3.

### Tasks
1. ModulesCard: when a toggle flips On, show an "installing…" state (spinner /
   disabled toggle) and poll `system.module.status` (via a panel-api status
   endpoint) until `installed && active`, then success; **on failure show the
   error AND a retry affordance** — because the plan deliberately does NOT roll the
   flag back on install failure (intent is recorded), the failure mode is exactly
   the reported "flag on, service down" state, so it MUST be visibly retryable, not
   a dead end the operator can't escape without toggling off/on.
2. The Server Status Services card (already module-filtered — shipped) will show
   the service once it's up; no change needed there beyond the filter.

### Verify
- `npx tsc -b`; a vitest/E2E that a toggle shows installing→installed.

### Exit
Operator flips DNS On → sees it install → pdns goes active in Server Status.

---

## Rollback
Each step is independently revertable. The install.sh `--install-module` mode is
additive (main() unchanged in behavior if the refactor preserves the exact
sequences). The panel dispatch is guarded on `Agent != nil` + status; reverting
the panel change returns to flag-only gating (pre-feature behavior) with no data
loss.

## Sequencing
- Step 0 is a hard gate — a module whose functions aren't standalone-safe is NOT
  wired until fixed. Security is the highest-risk audit.
- Steps 1→2→(3 + 3b) are serial. **3b (convergence) is what fixes the reported
  bug** — 3 (transition-install) alone leaves existing all-flags-default-On hosts
  broken. Ship both. Step 4 follows.
- Ship per-module: quota (smallest, 1 function) first as the walking skeleton,
  then dns, then mail, then security (largest + the appsec ordering scar — last).
  Don't big-bang all four.
- Serialize every install behind the apt/dpkg mutex (Step 2) — convergence + a
  manual toggle can otherwise collide.
- Live-VM smoke on `--minimal` install: flip each module On, confirm packages
  install + service active; flip Off, confirm stopped + data kept; flip On again,
  confirm idempotent (no re-install, fast).
