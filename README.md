<p align="center">
  <img src="public/images/jabali_logo.svg" alt="Jabali Panel" width="140">
</p>
<h1 align="center">Jabali Panel</h1>

<p align="center">
  <img src="https://img.shields.io/badge/PHP-8.4-777BB4?logo=php&logoColor=white" alt="PHP 8.4">
  <img src="https://img.shields.io/badge/Laravel-12-FF2D20?logo=laravel&logoColor=white" alt="Laravel 12">
  <img src="https://img.shields.io/badge/Filament-5-FDAE4B?logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI+PHBhdGggZD0iTTEyIDJMMiAyMmgyMEwxMiAyeiIgZmlsbD0id2hpdGUiLz48L3N2Zz4=&logoColor=white" alt="Filament 5">
  <img src="https://img.shields.io/badge/Livewire-4-FB70A9?logo=livewire&logoColor=white" alt="Livewire 4">
  <img src="https://img.shields.io/badge/Tailwind-4-06B6D4?logo=tailwindcss&logoColor=white" alt="Tailwind 4">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue" alt="GPL-3.0">
  <img src="https://img.shields.io/badge/Debian-13-A81D33?logo=debian&logoColor=white" alt="Debian 13">
</p>

A modern web hosting control panel for WordPress and general PHP hosting. Jabali focuses on clean multi-tenant isolation, safe automation, and a consistent admin/user experience. It ships with a privileged agent for root-level tasks, built-in mail and DNS management, migrations from common panels, and an integrated security daemon (jabali-security) for real-time threat detection and automated response. The UI is designed to be fast, predictable, and easy to operate on a single server.

Version: see `VERSION` (release candidate)

This is a release candidate. Expect rapid iteration and breaking changes until 1.0.

## Demo and Website

- Website: https://jabali-panel.com/
- Demo: https://jabali-panel.com/demo/

## Installation

GitHub install:

```
curl -fsSL https://raw.githubusercontent.com/shukiv/jabali-panel/main/install.sh | sudo bash
```

Optional flags:

- `JABALI_MINIMAL=1` for core-only install
- `JABALI_FULL=1` to force all optional components
- `--debug` to show full command output instead of spinner

If Jabali is already installed, the script will detect it and offer to re-install (uninstall + fresh install). Your `.env` and credentials are backed up to `/root/.jabali_reinstall_backup_<timestamp>/` before wiping.

Uninstall:

```
curl -fsSL https://raw.githubusercontent.com/shukiv/jabali-panel/main/install.sh | sudo bash -s -- uninstall
```

Force uninstall (no confirmation prompts, keeps home directories):

```
curl -fsSL https://raw.githubusercontent.com/shukiv/jabali-panel/main/install.sh | sudo bash -s -- uninstall --force
```

After install:

- Admin panel: `https://your-host:8443/jabali-admin`
- User panel: `https://your-host:8443/jabali-panel`
- Webmail: `https://your-host/webmail`

The panel runs on port 8443 via FrankenPHP, independent of nginx. If nginx goes down, the panel stays accessible so users can log in and diagnose problems.

## Container Deployment

Jabali can run as a single container with all services managed by supervisord (MariaDB, Redis, Nginx, PHP-FPM, jabali-agent, queue-worker, cron, PowerDNS, Stalwart Mail Server). The `Containerfile` uses a multi-stage build based on `debian:bookworm-slim`.

### Quick Start (Docker Hub)

```bash
docker pull shukivaknin/jabali-panel:latest

docker run -d --name jabali \
  --hostname panel.example.com \
  -p 80:80 -p 443:443 -p 8443:8443 \
  -p 25:25 -p 587:587 -p 993:993 -p 110:110 \
  -p 53:53/tcp -p 53:53/udp \
  -v jabali-mysql:/var/lib/mysql \
  -v jabali-storage:/var/www/jabali/storage \
  -v jabali-mail:/var/mail \
  -v jabali-home:/home \
  -v jabali-letsencrypt:/etc/letsencrypt \
  -e APP_URL=https://panel.example.com \
  -e SERVER_HOSTNAME=panel.example.com \
  --cap-add NET_BIND_SERVICE \
  --cap-add NET_RAW \
  shukivaknin/jabali-panel:latest
```

The entrypoint handles first-run initialization (database setup, key generation, migrations, self-signed SSL). Persistent data is stored in the named volumes listed above.

After the container starts, create an admin user:

```bash
docker exec -it jabali php /var/www/jabali/artisan tinker --execute="
\$u = new App\Models\User();
\$u->name = 'Admin';
\$u->username = 'admin';
\$u->email = 'admin@example.com';
\$u->password = bcrypt('changeme');
\$u->is_admin = true;
\$u->save();
"
```

Then open `https://panel.example.com:8443/jabali-admin` to log in.

### Build from Source

Requires `auth.json` for Filament packages:

```bash
podman build --secret id=composer_auth,src=auth.json -t jabali-panel:latest .
```

## Highlights

- Per-user Linux accounts and PHP-FPM isolation
- SSH shell access via nspawn containers with auto-start and 5-minute idle timeout
- Root agent for SSL, mail, backups, and migrations
- Health monitor with auto-restarts and alerts
- cPanel and WHM migrations with step-by-step logs
- IMAP sync for migrating mail from external servers
- Stalwart Mail Server with webmail SSO (Bulwark JMAP client)
- Shared mailbox folders via Stalwart Mail Server
- `mail.domain.ext` auto-redirects to webmail
- One-time login tokens (CLI + dashboard UI) with IP binding
- PowerDNS with REST API and native DNSSEC
- Restic backups with deduplication, encryption, and SFTP/S3 support
- First-time backup setup wizard with encryption password and remote destinations
- WordPress management (install, updates, and SSO)
- Integrated security suite (jabali-security) with real-time threat detection
- Encrypted diagnostic log sharing with ticket tracking
- Per-user page cache directories (moved from global nginx cache)
- Passphrase password generator (optional, 3 random words)
- GoAccess real-time statistics with WebSocket updates
- Domain bandwidth tracking synced daily from nginx logs
- 80+ CLI commands with full panel parity (noun:verb pattern)
- Audit logs and admin notifications

## Feature Map

### Admin Panel

- Dashboard with stats, health, and recent activity
- User management with suspension and quotas
- Service manager for systemd services
- PHP version and pool management
- DNS zones, templates, and DNSSEC
- SSL issuance and renewals
- IP address assignments
- Backups and restores (local + remote) with first-time setup wizard
- Migrations (cPanel restore, WHM downloads, IMAP sync)
- Security (jabali-security daemon with real-time monitoring)
- One-time login link generator for support access (IP-bound tokens)
- Diagnostic report (encrypted sharing to support via paste service)
- Database tuning and query analysis
- Email queue management with delivery logs
- Audit logs and notifications

### User Panel

- Domains, redirects, and Nginx config
- DNS records editor
- Mail domains, mailboxes, forwarders, shared folders, and per-domain disclaimers
- IMAP sync (single and bulk mail migration)
- Webmail SSO (Bulwark, Next.js JMAP client)
- WordPress manager (install, SSO)
- File manager plus SFTP/SSH keys
- SSH shell access via nspawn containers with 5-minute idle timeout
- Databases (MySQL and PostgreSQL in tabbed view)
- PHP settings per account
- SSL management
- Cron jobs
- Backups and restore
- Logs, statistics, and bandwidth usage
- Support access link generator (one-time IP-bound tokens)
- Protected directories

### Platform

- Root-level agent for privileged operations
- Queue-backed jobs for long-running tasks
- Health monitor with auto-restarts and alerts
- Redis ACL isolation for WordPress caching
- Multi-language UI

## Architecture

- Control plane: Laravel 12 app with Filament v5 and Livewire v4
- Panel web server: FrankenPHP on port 8443 (independent of nginx)
- Data plane: root agent handling privileged operations via Unix socket
- Job queue: async tasks and migration steps
- Webmail: Bulwark (Next.js JMAP client) at `/opt/bulwark`, served at `/webmail/` via nginx proxy to port 3000
- SSH shell: jabali-isolator (Python, separate repo) managing nspawn containers for SSH access isolation
- Security: jabali-security daemon (separate repo) with real-time threat detection and automated response
- Logging: panel and agent logs for troubleshooting
- Server metrics: live /proc filesystem reads

Service stack (single-node default):

- FrankenPHP (panel on port 8443, self-signed or Let's Encrypt SSL)
- Nginx (user domain sites, phpMyAdmin, webmail proxy, Bulwark proxy)
- PHP-FPM (user site pools)
- MariaDB (user databases)
- Stalwart Mail Server (SMTP, IMAP, JMAP, ManageSieve)
- PowerDNS (DNS with REST API, MySQL backend)
- Restic (encrypted, deduplicated backups)
- Redis
- GoAccess (real-time web analytics in daemon mode with WebSocket)
- jabali-isolator (nspawn container management for SSH shell isolation)
- jabali-security (real-time threat detection, brute-force protection, WAF)
- Bulwark (Next.js JMAP webmail client on port 3000)

## Requirements

- Fresh Debian 13 install (no pre-existing web or mail stack)
- A domain for panel and mail (with glue records if hosting DNS)
- PTR (reverse DNS) for mail hostname
- Open ports: 22, 80, 443, 8443, 25, 465, 587, 993, 995, 53

## Security Hardening

See [SECURITY.md](SECURITY.md) for the full security policy, architecture, and audit history.

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `TRUSTED_PROXIES` | Comma-separated proxy IPs/CIDRs (or `*` to trust all upstream proxies) | (unset) |
| `JABALI_INTERNAL_API_TOKEN` | Shared token for internal API calls from non-localhost | (unset) |
| `JABALI_IMPORT_INSECURE_TLS` | Disable TLS certificate verification for WHM/cPanel migration API calls | `false` |
| `SESSION_ENCRYPT` | Encrypt session data at rest | `false` |
| `SESSION_SECURE_COOKIE` | Send session cookies only over HTTPS | `false` |
| `PANEL_PORT` | HTTPS port for the FrankenPHP panel server | `8443` |
| `PANEL_HOSTNAME` | Hostname for the panel (used in APP_URL) | (auto-detected) |
| `PANEL_TLS_CERT` | Path to the panel TLS certificate | `/etc/ssl/jabali/panel.crt` |
| `PANEL_TLS_KEY` | Path to the panel TLS private key | `/etc/ssl/jabali/panel.key` |

### Key Security Features

- Shell arguments escaped with `escapeshellarg()` to prevent OS command injection
- Admin impersonation uses one-time IP-bound tokens; stop action requires POST + CSRF
- DKIM private keys encrypted at rest via Laravel's `encrypted` cast
- Migration API calls verify TLS certificates by default (opt-out with `JABALI_IMPORT_INSECURE_TLS`)
- Webmail SSO tokens stored in restricted directory with `0600` permissions and 5-minute expiry
- Admin backup downloads restricted to allowed directory prefixes
- WordPress page-cache API uses SHA-256 verification of `AUTH_KEY`
- CSP, HSTS, and other security headers on all panel responses
- Git deployment webhooks support signed payloads via `X-Jabali-Signature` / `X-Hub-Signature-256` (HMAC-SHA256)

## Updates

Update the panel (code, dependencies, database migrations, and infrastructure):

```
jabali update
```

This pulls the latest code from GitHub, runs composer/npm, applies database migrations, rebuilds caches, upgrades infrastructure (PHP config, nginx config, systemd services), and updates jabali-security if installed. Safe to run on a live server — the panel enters maintenance mode during the update.

Force a full update even if already on the latest version:

```
jabali update --force
```

## CLI

The `jabali` command uses a noun:verb pattern. Aliases: `wordpress` -> `wp`, `database` -> `db`, `email` -> `mail`.

### User Management
- `jabali user create <username> [--email=] [--password=]` — Create user
- `jabali user delete <username> [--force]` — Delete user
- `jabali user suspend <username>` — Suspend user
- `jabali user unsuspend <username>` — Unsuspend user
- `jabali user reset-password <username>` — Generate password reset link
- `jabali user quota set <username> <quota>` — Set disk quota
- `jabali user list [--status=] [--limit=]` — List users

### Domain Management
- `jabali domain create <user> <domain> [--type=]` — Add domain to user
- `jabali domain delete <domain> [--force]` — Delete domain
- `jabali domain verify <domain>` — Verify domain ownership
- `jabali domain point <domain> <ip>` — Update A record
- `jabali domain list [--user=] [--status=]` — List domains

### Database (db)
- `jabali db create <user> <name> [--type=mysql|postgres]` — Create database
- `jabali db delete <database> [--force]` — Delete database
- `jabali db user create <database> <username>` — Create DB user
- `jabali db user delete <database> <username>` — Delete DB user
- `jabali db backup <database>` — Backup database
- `jabali db list [--user=] [--type=]` — List databases

### Mail (email/mail)
- `jabali mail domain create <user> <domain>` — Add mail domain
- `jabali mail domain delete <domain>` — Delete mail domain
- `jabali mail user create <domain> <username>` — Create mailbox
- `jabali mail user delete <mailbox>` — Delete mailbox
- `jabali mail user password <mailbox>` — Set password
- `jabali mail forward create <domain> <address> <target>` — Create forwarder
- `jabali mail forward delete <address>` — Delete forwarder
- `jabali mail list-domains [--user=]` — List mail domains
- `jabali mail list-users [--domain=]` — List mailboxes

### SSL/TLS
- `jabali ssl create <domain> [--auto-renew]` — Issue Let's Encrypt certificate
- `jabali ssl renew <domain>` — Renew certificate
- `jabali ssl delete <domain>` — Delete certificate
- `jabali ssl list [--domain=]` — List certificates
- `jabali ssl check <domain>` — Check certificate status

### DNS
- `jabali dns zone create <domain> [--nameserver=]` — Create DNS zone
- `jabali dns zone delete <domain>` — Delete DNS zone
- `jabali dns record create <domain> <type> <value>` — Add DNS record
- `jabali dns record delete <domain> <id>` — Delete DNS record
- `jabali dns record list <domain>` — List DNS records
- `jabali dns dnssec enable <domain>` — Enable DNSSEC
- `jabali dns dnssec disable <domain>` — Disable DNSSEC

### Backups
- `jabali backup create <user>` — Backup user account
- `jabali backup list <user> [--remote]` — List backups
- `jabali backup restore <path> --user=<user>` — Restore from backup
- `jabali backup delete <backup-id>` — Delete backup
- `jabali backup destination add [--type=sftp|s3|b2]` — Configure remote destination

### Cron Jobs
- `jabali cron list <user>` — List cron jobs
- `jabali cron create <user> <expression> <command>` — Add cron job
- `jabali cron delete <user> <id>` — Delete cron job
- `jabali cron run <user> <id>` — Run cron job manually

### PHP
- `jabali php version list` — List available PHP versions
- `jabali php pool create <user> [--version=] [--memory=]` — Create PHP-FPM pool
- `jabali php pool delete <user>` — Delete pool
- `jabali php pool config <user>` — Show pool configuration
- `jabali php setting set <user> <setting> <value>` — Set PHP setting

### Services
- `jabali service list [--status=]` — List services
- `jabali service start <service>` — Start service
- `jabali service stop <service>` — Stop service
- `jabali service restart <service>` — Restart service
- `jabali service status <service>` — Check service status
- `jabali service enable <service>` — Enable service at boot
- `jabali service disable <service>` — Disable service at boot

### System Management
- `jabali system info` — Display system information
- `jabali system update` — Update panel to latest version (with --force option)
- `jabali system health [--verbose]` — Check system health
- `jabali system reboot [--force]` — Reboot server
- `jabali system logs [--service=] [--lines=]` — View system logs
- `jabali system status` — Overall server status

### WordPress (wp)
- `jabali wp install <user> <domain>` — Install WordPress
- `jabali wp delete <domain> [--force]` — Delete WordPress site
- `jabali wp update <domain>` — Update WordPress
- `jabali wp plugin list <domain>` — List plugins
- `jabali wp plugin install <domain> <plugin>` — Install plugin
- `jabali wp plugin activate <domain> <plugin>` — Activate plugin
- `jabali wp plugin deactivate <domain> <plugin>` — Deactivate plugin
- `jabali wp user create <domain> <username> <email>` — Create WordPress user

### Migration
- `jabali cpanel analyze <file>` — Analyze cPanel backup
- `jabali cpanel restore <file> <user>` — Restore cPanel backup
- `jabali whm download <host> <user> <password>` — Download WHM backup
- `jabali imap sync <user> [--source-host=] [--source-user=]` — Sync IMAP mail

### Agent Management
- `jabali agent status` — Check agent health
- `jabali agent restart [--force]` — Restart agent
- `jabali agent logs [--lines=]` — View agent logs

### Support Access
- `jabali login create <user> [--hours=24]` — Create one-time support login link
- `jabali login list [--user=]` — List active links
- `jabali login revoke <link-id>` — Revoke login link

### Diagnostics
- `jabali report` — Generate encrypted diagnostic report
- `jabali report encrypt <data>` — Encrypt data for support
- `jabali logs export [--service=] [--since=]` — Export logs

## Development

```
composer dev
php artisan test --compact
./vendor/bin/pint
```

### Versioning

The version string in the `VERSION` file must be kept in sync between the panel codebase and the installer (`install.sh`). When the installer clones the repository during a fresh install, it reads `VERSION` to display the installed version. If you bump the version in one place but not the other, the panel footer and installer output will show different versions.

Always update `VERSION` in a single commit that includes both the panel changes and any corresponding `install.sh` changes.

## License

GPL-3.0 — see [LICENSE](LICENSE) for details.

## Mail Subdomain

Visiting `mail.domain.ext` in a browser automatically redirects to webmail (Bulwark). Autoconfig and autodiscover paths are excluded so mail client auto-discovery continues to work.

## Documentation

See the [docs/](docs/) directory for comprehensive guides including:
- Architecture Decision Records (ADRs)
- Security policies and audit logs
- Installation and upgrade procedures
- API documentation
- Feature-specific guides
