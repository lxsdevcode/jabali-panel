# Security

Layered. CrowdSec is the IP-trust source; UFW handles the port baseline; AppSec WAF replaces ModSecurity; Snuffleupagus + AppArmor harden the application layer; AIDE watches the host; per-user egress firewall stops compromised tenants from being usable for outbound abuse.

## CrowdSec — single source of IP-trust (M43)

ADR-0089. CrowdSec is the only thing that decides whether an IP gets blocked.

- **Bouncers**: nginx (rate-limit + AppSec inspect), Stalwart (SMTP/IMAP), Bulwark, sshd.
- **Scenarios**: HTTP probe/scan, SSH bruteforce, IMAP/SMTP auth flood, app-specific WP/Drupal scan, malware-upload attempt.
- **Decisions**: BAN, CAPTCHA, ALLOWLIST.
- **Console**: enrol via `/jabali-admin/security` → CrowdSec → Console; central per-org view at `app.crowdsec.net`.

UFW is **demoted**: only port-open/port-close baseline. Old `ufw deny from <ip>` rules are migrated into CrowdSec decisions by `jabali ufw migrate-ip-bans`.

CrowdSec extensions (M27, ADR 0061-0063):
- **Per-IP allowlists** — admin-managed, persists across CrowdSec restarts.
- **Per-scenario override** — change a scenario's severity / leakspeed / capacity at admin level.
- **Alert routing** — alerts feed M14 notifications (`crowdsec_spike` event source).

## AppSec WAF (M27 — replaces ModSecurity)

ADR-0060. ModSecurity is **removed** (M27 cleanup_modsecurity purges packages + configs every install; migration 000074 drops the schema). Replacement is **CrowdSec AppSec**:

- Inline `appsec-block` bouncer in nginx (`/etc/nginx/conf.d/jabali-appsec.conf`).
- Rule packs from `hub.crowdsec.net/author/crowdsecurity` (vpatch family for CVE virtual-patching).
- AppSec install path is **flat** (`/etc/crowdsec/appsec-rules/`) — no `crowdsecurity/` subdir (the install-path scar that purged + reinstalled 170 vpatch rules every update, fixed in PR #69).

## AppArmor

`/jabali-admin/security` → AppArmor — per-profile status (enforce / complain / **missing**).

Jabali ships and manages these daemon profiles: `jabali-panel` (panel API), `jabali-agent`, `jabali-bulwark` (webmail), `stalwart-mail`, and `jabali-fpm-app` (GH #690 — the per-user PHP-FPM/WordPress workload profile, attached to fpm-exec; ships complain-first for the soak, flip to enforce per-host after soak-readiness shows 0 would-deny). A profile that fails to load or is purged is reported as **missing** (red) rather than silently omitted — an absent profile means an unconfined daemon.

New profiles ship in **complain** mode for a 7-day burn-in soak; each profile shows a **soak-readiness** indicator (complain-mode profiles with zero would-deny events are ready to flip to enforce). Complain-mode `apparmor="ALLOWED"` would-deny events are surfaced alongside enforce-mode `DENIED` denials, so a complain-mode profile actively logging violations is not mistaken for a clean state.

On kernels with broken unix-socket mediation (missing `/sys/kernel/security/apparmor/features/unix`), Jabali deliberately does **not** load the daemon profiles — on fresh install and on `jabali update` alike.

The panel API daemon holds **no** AppArmor policy-management capability (`mac_admin`): mode flips are delegated to `panel-agent`, so a compromised panel cannot disable its own confinement.

## Snuffleupagus

PHP runtime hardening loaded as a Zend extension on every PHP version. Default rules:

- Block `eval` against tainted request data.
- Disallow `include` / `require` from `php://`, `data:`, or remote URLs.
- Taint tracking from `$_GET` / `$_POST` into shell-execution sinks.
- Block known-bad shellcode patterns.

Per-app exceptions live in `/etc/php/<ver>/snuffleupagus.rules.d/`. WP, Moodle, NextCloud, etc. ship with pre-baked exception files.

## AIDE host-integrity

Daily timer (`aide.timer`) compares the host against the AIDE database. Changes outside the panel's drop-in paths fire an `aide_diff` notification.

## Per-user egress firewall (M34)

ADR-0084. nftables + cgroup v2 vmap. Each user's processes run in their slice; the nftables ruleset matches by cgroup ID and decides:

- Allow `:443` to anywhere (HTTPS — legitimate API use).
- Allow `:587/465/993` to the panel's own mail host (so PHP scripts can submit mail).
- Drop everything else by default.

**External database ports are dropped by default** (GH #638). The default
allowlist is web + mail only (`25, 53, 80, 443, 465, 587`); outbound to a
**remote** database — MSSQL `1433`, MySQL/MariaDB `3306`, PostgreSQL `5432`,
Redis `6379`, MongoDB `27017`, etc. — is dropped by the enforced chain. This is
intentional (a compromised tenant can't exfiltrate to or pivot through an
arbitrary remote DB), but it also blocks a *legitimate* app that connects to an
external managed database. A dropped connection surfaces in the panel under
**Users → Edit → Egress** (recent drops feed) so the operator can see *why* a
tenant's outbound DB connection is failing rather than guessing.

To allow a legitimate external DB, add the port (and optionally the destination
CIDR) to that user's egress allowlist under **Users → Edit → Egress**. Prefer a
CIDR-scoped rule (the specific DB host) over opening the port to `0.0.0.0/0`.

Admin overrides per-user under Users → Edit → Egress.

## Malware (M33, M33.2)

- **ClamAV** — on-demand only (daemons masked); `jabali-freshclam.timer` daily for signatures (M33 on-demand mode).
- **Linux Malware Detect (LMD)** — opt-in monitor (default off, mig 000082); apply-then-persist toggle.
- **YARA** — only the `php.yar` rule (clamscan rejects PMF whitelists/* due to libclamav YARA subset restrictions).
- **Tetragon** — eBPF tripwires; suspicious exec events ingested via `sessionwatcher` → M14 (`file_hit` + quarantine events).
- **M33.2 mail-yara-async** (ADR-0079) — async post-delivery JMAP-poll YARA scan; NOT MtaHook/MtaMilter.

## CrowdSec test-IP card

`/jabali-admin/security` → CrowdSec Test IP — paste any IPv4/IPv6, see whether CrowdSec would block / captcha / allow it right now, with the matching decision row.

## Audit log

`/jabali-admin/audit`. Append-only (ADR-0106). Every privileged mutation:
- Who (subject user, actor user, source).
- What (action, target).
- When.
- Result (ok / fail).
- Diff (where applicable).

CLI: `jabali audit list --since 24h --action db.root.rotate`.
