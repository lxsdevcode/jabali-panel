# GH #598 — auto-allowlist successful panel + SSH login IPs in CrowdSec

**Status:** designed, pre-advisor. Panel half already ships; this adds the SSH
half + a configurable toggle/TTL and unifies both under one server setting.

## What already exists (do NOT rebuild)

- `middleware.WhitelistLoginIP` (`panel-api/internal/middleware/login_whitelist.go`),
  mounted at `app.go:438` for ALL authenticated users after the Kratos session
  check. Redis-dedup (24h) → agent `security.crowdsec.allowlists.add` with
  `expiration:"168h"`, `reason:"auto-whitelist: login <email>"`. Skips
  loopback/private/link-local via `whitelistableIP`.
- Agent `security.crowdsec.allowlists.{add,list,remove}` — `add` validates
  IP/CIDR, ensures the `jabali-*` allowlist, honours `expiration`, reason 3..200.
- Agent starts always-on workers in `jabali-agent/main.go` via `commands.StartX(ctx)`
  (e.g. `StartBlocklistsRefresher`) — the pattern the SSH watcher plugs into.

## Gaps vs the issue

1. **SSH logins are not allowlisted** — panel middleware only fires on panel HTTP.
2. **Not configurable / not disableable** — TTL hardcoded 168h, no toggle.
3. TTL: title says "7 days, configurable"; AI-triage body says 14 days.
   **Decision:** default **168h (7d)** (matches shipped behaviour + the title),
   admin-configurable. Note the discrepancy in the issue reply.

## Design

One server setting drives both halves. Panel reads DB directly; the agent SSH
watcher reads a config file the panel pushes (agent has no DB access) — the same
"DB is truth → panel applies to host" model as `crowdsec_bouncer_mode`.

### Trusted SSH capture — agent journald watcher (NOT PAM, NOT the tenant shell)

The tenant's login shell (`jabali-ssh-shell`) is unprivileged/sandboxed and
tenant-controlled → cannot be the trigger (IP forgery, no agent-socket access).
A PAM `pam_exec` hook modifies the auth path (lockout risk). Instead: a root-side
background worker in the agent tails sshd's own success log — trusted source,
zero auth-path change, mirrors how CrowdSec itself acquires. Worst-case failure
= no allowlist (degrades to today), never a lockout.

- `commands.StartLoginAllowlistWatcher(ctx, log)` in `jabali-agent/main.go`.
- `journalctl -u ssh.service -u sshd.service -f -o cat --since now` (fall back to
  tailing `/var/log/auth.log` if journald unit absent). Line-parse regex:
  `Accepted (?:publickey|password|keyboard-interactive(?:/pam)?) for (\S+) from (\S+) port`.
- On match: read `/etc/jabali-panel/login-allowlist.conf` (JSON `{enabled,ttl}`),
  bail if `!enabled`; skip `!whitelistableIP(ip)` (reuse the same rules — extract
  `whitelistableIP` to a shared spot or duplicate the tiny predicate agent-side);
  in-memory dedup `map[ip]time` under a mutex, 24h window; then the SAME add path
  as `csAllowlistsAddHandler` (extract a `func addToJabaliAllowlist(ctx, value,
  reason, expiration)`), reason `"auto-whitelist: ssh <user>"`.
- Restart-resilient: `--since now` avoids re-allowlisting old log on every boot;
  the in-memory dedup handles the live stream.

### Toggle + TTL

- **Migration** (`000131`? — verify next free): `server_settings`
  + `crowdsec_login_allowlist_enabled BOOL NOT NULL DEFAULT 1`
  + `crowdsec_login_allowlist_ttl_hours INT NOT NULL DEFAULT 168`.
  Schema-only; seeded values come from the column defaults (never SELECT-seed a
  migration — the app owns data).
- **Model** `server_settings.go`: two fields + JSON tags, mirroring
  `CrowdsecBouncerMode`.
- **Panel middleware**: read the setting (cache via the existing settings
  accessor); gate on `enabled`; use `ttl_hours` for `expiration`. If disabled →
  no-op passthrough.
- **Agent verb** `security.crowdsec.login_allowlist.apply {enabled, ttl_hours}` →
  atomically writes `/etc/jabali-panel/login-allowlist.conf`. The watcher reads it
  live each hit (cheap) so no agent restart needed on toggle.
- **Reconciler**: write-on-diff — when the setting changes, call the apply verb
  (compare to last-applied, like other crowdsec settings). Also on panel boot so
  a fresh value converges.
- **install.sh**: drop a default `login-allowlist.conf` (`{enabled:true,ttl:"168h"}`)
  so the watcher works before the first reconcile. Add nothing to PATH; file only.

### Admin UI

Security settings page (where `CrowdsecBouncerMode`/sensitivity live): a toggle
+ TTL-hours number input, wired to the settings PATCH. Reuse the existing form.

### Audit

Panel middleware already encodes the reason; add an `audit` event
`security.login_allowlist` (subject = user, target = IP) on the panel side only
(the SSH watcher logs via slog — no panel audit row, documented). Keep it out of
the manual-operator-change surface.

## Steps / waves

1. Migration + model fields (schema only).
2. Panel middleware reads setting (enabled + ttl); default-safe when unset.
3. Agent: extract `addToJabaliAllowlist`; `StartLoginAllowlistWatcher` + journald
   parse + dedup + conf read; wire into `main.go`.
4. Agent verb `security.crowdsec.login_allowlist.apply` + conf writer.
5. Reconciler write-on-diff + boot converge; install.sh default conf.
6. Admin UI toggle + TTL.
7. Tests: middleware (on/off/ttl), watcher regex (golden Accepted lines +
   negative: Failed/Disconnected), verb conf write, whitelistableIP.
8. E2E/live-verify on 10.0.3.14: real SSH login from a routable IP → allowlist
   entry appears with the right TTL; toggle off → no new entry.
9. ADR + issue reply (note 7d-vs-14d decision), keep #598 open.

## Risks

- journald tail is the only long-running new surface; read-only, best-effort,
  bounded (dedup). A parse miss = no allowlist, never a lockout.
- Don't re-scan historical log on boot (`--since now`) — else a busy host
  re-adds churn every restart.
- `ssh.service` vs `sshd.service` unit name differs by distro — try both.
