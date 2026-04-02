# Changelog

All notable changes to Jabali Panel will be documented in this file.

## [Unreleased]

### Added

- **FrankenPHP panel independence** -- The panel now runs on its own FrankenPHP web server on port 2223, independent of nginx. If nginx goes down, users can still log in to diagnose problems. Uses self-signed SSL by default, upgraded to Let's Encrypt automatically if the hostname resolves. Certbot deploy hook ensures renewed certs are picked up. FrankenPHP is pinned to v1.12.1 with auto-upgrade on version bump.
- **Panel certificate tracking** -- New `PanelCertificate` model and `jabali:panel-cert-sync` command to track FrankenPHP's TLS certificate status. Panel Certificate widget on the SSL Manager page shows hostname, issuer, expiry, and renewal status.
- **Service architecture refactoring** -- Extracted business logic from god objects into focused, testable services:
  - `SslManagementService` -- consolidates SSL issue/renew/check logic from 4 places into one service
  - `UserDeletionService` / `DomainDeletionService` -- model cleanup logic extracted from boot hooks
  - `ServerSettingsService` -- settings persistence extracted from the admin page
  - `BackupOrchestrator::execute()` -- backup workflow extracted from the job (173→35 lines)
  - `WhmMigrationOrchestrator` -- WHM migration workflow extracted from the job (609→58 lines)
  - `CpanelMigrationOrchestrator` -- cPanel migration workflow extracted from the page
  - 7 agent service facades (File, Database, Email, Domain, DNS, Backup, System) wrapping AgentClient's 162 methods into focused interfaces
- **PowerDNS replaces BIND9** -- DNS zone management now uses PowerDNS with a REST API and MySQL backend. The panel calls the PowerDNS API directly (no agent involvement). Eliminates ~500 lines of zone file templating from the agent. DNSSEC managed natively via PowerDNS cryptokeys API.
- **Restic replaces custom backup** -- Backups now use Restic for deduplication, encryption, and native remote backend support (SFTP, S3). Eliminates ~1,400 lines of tar/rsync agent code. Every backup is incremental by design. Auto-generated encryption password at `/etc/jabali/restic-password`.
- **Auto-generate password for `jabali user password`** -- Running without `--password` now generates and displays a secure 16-character password instead of erroring.
- **`jabali update` command** -- One-command panel update: pulls latest code, runs composer/npm, migrates database, rebuilds caches, and upgrades infrastructure (PHP, nginx, systemd configs). Use `--force` to re-run all steps even when already up to date.
- **Granular backup restore** -- Selective restore from rsync incremental snapshots. Users can restore individual domains, databases, MySQL users, mailboxes, DNS zones, or SSL certificates instead of full account restores. Includes file browser, conflict resolution (overwrite/merge/skip/rename), and optional safety backup. New user-facing Restore page at Backups > Restore.
- **Install script re-install detection** -- Running `install.sh` on an existing installation now detects it and offers to re-install. Backs up `.env` and credentials to `/root/.jabali_reinstall_backup_<timestamp>/` before wiping.
- **Install script `upgrade` command** -- `install.sh upgrade` re-runs safe infrastructure config functions (PHP, nginx, systemd, cron, logrotate) without touching database or Redis. Called automatically by `jabali update`.
- **jabali-security integration** -- The installer now automatically installs jabali-security at the end of installation. `jabali update` also updates jabali-security if installed. jabali-security provides real-time file monitoring, multi-engine malware scanning, brute-force protection, WAF integration, automated quarantine, threat intelligence, and SSH/SFTP jail management.
- **Cloudflare real IP support** -- Nginx and Laravel now automatically restore real visitor IPs when behind Cloudflare CDN. Configured via `CF-Connecting-IP` header with all Cloudflare IP ranges trusted.
- **Force Infrastructure Upgrade button** -- System Updates page now has a button to re-apply nginx, PHP, systemd, and cron configs without needing SSH access.

- **First-time backup setup wizard** -- Modal wizard on the Backups page guides admins through encryption password, remote destination (with connection validation), and schedule creation. Dismissable via "Don't show again" or the X button.
- **Per-user page cache directories** -- Nginx fastcgi cache now stored at `/home/{user}/cache/nginx/` instead of global `/var/cache/nginx/fastcgi/`. Per-user `keys_zone` in nginx. Purging is instant (`rm -rf` instead of file-by-file scanning). Upgrade command migrates existing servers automatically.
- **MySQL user backup and restore** -- Backups now include MySQL users and grants (`users.sql`). Restore wizard shows selectable MySQL users alongside databases. Users are validated through `validateMysqlUsersFile()` to prevent privilege escalation.
- **Bandwidth usage widget** -- User dashboard shows disk usage and bandwidth in a single "Usage" card with progress bars, plus package limits (domains/databases/mailboxes) with used/limit counters.
- **SSH shell toggle in hosting packages** -- New `ssh_shell_enabled` boolean on hosting packages. Users created with an enabled package automatically get jailed shell access.
- **Users page tabs** -- Admin Users page split into "Users" and "Administrators" tabs. Default shows normal users. Removed redundant Admin column and filter.
- **Diagnostic report expansion** -- 10 new debug sections: agent socket connectivity, queue status, listening ports, firewall status, directory permissions, .env key validation, storage symlink check, recent audit log, nginx vhost count, mail service status.

### Fixed

- **Page cache purge not working** -- WordPress plugin was calling port 443 (nginx) instead of 2223 (FrankenPHP panel). Fixed all 3 internal API URLs to use the correct panel port. Also fixed `sync_page_cache_with_jabali()` and phpMyAdmin signon.
- **Page cache enable failing** -- Nginx regex `listen 443 ssl;` didn't match `listen 443 ssl http2;`. Changed to `listen 443 ssl[^;]*;` to match any SSL listen directive variant.
- **Jabali Cache settings not saving** -- `flush_transients_by_patterns()` was calling `$redis->close()` on the shared object cache connection, killing Redis for the rest of the request. WordPress couldn't save options to the DB.
- **Admin login redirecting to user panel** -- Simplified auth flow; `canAccessPanel()` now enforces admin-only access to admin panel and blocks admins from user panel. Non-admin users get a proper validation error instead of silent redirect.
- **Installer creating non-admin user** -- `is_admin` is not in `$fillable` (intentional security). Installer now sets it directly on the model after `updateOrCreate`.
- **Server Settings Logs tab not working** -- `normalizeTabName()` was missing `'logs'` from its match expression, bouncing back to General tab.
- **Service notification showing `:service`** -- `successTitle` was using literal `:service` string instead of the `$service` variable.
- **500 error on fresh install** -- jabali-security's `Security.php` page called `auth()->user()?->isAdmin()` but User model only had `is_admin` attribute. Added `isAdmin()` method.
- **Diagnostic report gzencode failure** -- Falls back to uncompressed encryption when `gzencode()` throws stream error under FrankenPHP.
- **Scheduler cron detection** -- Diagnostic report now checks both root and www-data crontabs instead of only root.
- **Selective restore restoring all files** -- `restore_files` was set to `true` whenever the backup contained domains, even when no files were selected. Fixed to only restore when `selectedPaths` is non-empty.
- **SSL issue CLI missing username** -- `jabali:ssl:issue` now resolves the username from the Domain model automatically.
- **Silent service failures during install** -- PHP-FPM start failure is now fatal instead of silently ignored. Cron, nginx, Postfix, and Dovecot reload failures now emit warnings.
- **Uninstall home directory safety** -- The home directory deletion prompt now always appears, even in `--force` mode. Defaults to "no" when piped via `curl | bash`.

### Security

- **cronRun command injection** (CRITICAL) -- `cronRun()` now validates commands against the allowlist before execution. Was bypassing `validateCronCommand()`, allowing arbitrary shell commands as any system user.
- **Impersonation token IP bypass** (CRITICAL) -- IP check now hard-fails when either IP is missing. Was silently skipping the check when null.
- **Domain ownership check in Logs page** (HIGH) -- `selectedDomain` is now validated against the authenticated user's domain list before forwarding to the agent.
- **Internal API path validation** (HIGH) -- Domain format regex and paths array sanitization added to `page-cache` and `page-cache-purge` endpoints.
- **WebhookEndpoint secret_token** (MEDIUM) -- Now encrypted at rest and hidden from JSON serialization.
- **XXE in AutoDiscover** (MEDIUM) -- Added `LIBXML_NOENT` flag to prevent entity expansion attacks.
- **Health monitor escapeshellarg** (MEDIUM) -- Service names now properly escaped in systemctl calls.
- **UUID generation** (LOW) -- Replaced insecure `mt_rand()` with `Str::uuid()` in AutoconfigController.

- **SSL Manager redesign** -- Custom Blade view with per-user accordions and per-domain grouping (root domain → subdomains). Certbot log viewer. Issue/renew/check actions per domain. Removed HasTable interface in favor of native Alpine.js collapsibles.
- **SSL CLI commands** -- `jabali ssl panel` shows panel cert status (issuer, expiry, SANs). `jabali ssl panel-issue` issues Let's Encrypt cert for the panel and restarts FrankenPHP. `jabali ssl list` lists all domain SSL certificates via agent.
- **Auto SSL on domain creation** -- New domains automatically get SSL certificates issued via a queued job (`IssueSslCertificate`). Skips domains that already have valid Let's Encrypt certs.
- **Webmail SSO patch versioning** -- Bulwark patches (basePath, SSO route, auth-store fallback, proxy.ts) are now extracted into a shared `patch_bulwark()` function called from both install and upgrade paths. A patch version marker ensures patches are re-applied when they change, even without upstream Bulwark updates.
- **FrankenPHP performance tuning** -- OPcache preloading (`bootstrap/preload.php`), `resolve_root_symlink false`, `max_threads`, realpath cache (4MB/600s), `GOMEMLIMIT=512MiB`.

### Fixed

- **Webmail SSO broken after Bulwark upgrade** -- `upgrade_bulwark()` ran `git reset --hard` which wiped all Jabali patches (SSO route, basePath, auth-store fallback) without re-applying them. Also fixed the SSO fallback to use PUT instead of GET for session retrieval, since upstream Bulwark now strips passwords from GET responses.
- **SSL certificate detection always showing Pending** -- `SslManagementService::check()` was reading flat `$result['has_ssl']` instead of nested `$result['ssl']['has_ssl']`. Snakeoil certs now correctly mapped to `type=none` instead of `lets_encrypt`.
- **Panel certificate wrong cert installed** -- `sslPanelIssue()` now verifies hostname in SANs before using an existing cert. Certbot deploy hook also checks SANs match panel hostname before copying.
- **Panel certificate key permissions** -- Fixed from `0600 root:root` to `0640 root:www-data` so FrankenPHP can read the private key.
- **SSL certificate cascade delete** -- Changed `domain_id` foreign key from `cascadeOnDelete` to `nullOnDelete` so cert records survive accidental domain deletion.
- **Panel Branding buttons not working** -- Filament v5 `FormAction` closures silently fail in schema context on this page. Replaced with native Livewire `wire:click` in Blade with `WithFileUploads` trait.
- **Logo preview not showing** -- Changed from `Storage::disk('public')->url()` (generates IP-based URL) to relative `/storage/...` paths. Removed FrankenPHP Caddyfile rule that blocked `/storage/*`.

### Removed

- **Old security remnants** -- Removed 10 files (7 firewall CLI commands, 2 WAF blade views, 1 test), fail2ban from Docker, WAF constant from agent, `fw` alias from CLI, dead `AuditLog::logFirewallAction`, health monitor fail2ban entry.
- **Dead code cleanup** -- Removed `RoundcubeIdentityService`, `MailAutoconfigSyncCommand`, 24 dead agent RPC routes, jabali-cache dead settings (`browser_cache`, `object_cache`, `minify_css`, `expired_cache`, `cache_debug`), 6 dead plugin methods, Developer card from plugin UI.
- **Terminal Access from user panel** -- Shell access is now admin-controlled only via hosting packages. Removed toggle, methods, and blade section.
- **Security page and tools** -- Removed the Security admin page (Fail2ban, ClamAV, UFW, ModSecurity/WAF, Lynis, WPScan, Nikto). All security is now handled by jabali-security, a standalone daemon installed automatically during panel installation. Removed ~10,000 lines of security-related code from the panel and agent.
- **Debian package** -- Removed `packaging/` directory and deb build scripts. Installation is now exclusively via `curl | bash` with `install.sh`.

## [0.9-rc124] - 2026-03-17

### Added

- **Tabbed Databases page** -- MySQL and PostgreSQL are now on a single "Databases" page with tabs instead of two separate nav items. The old `/postgresql` URL redirects automatically. PostgreSQL inputs now enforce username-prefixed alphanumeric validation matching MySQL. (#38)
- **Passphrase password generator** -- Optional 3-random-word passwords (e.g., `falcon-meadow-stella`) for panel login, mailbox, and database users. Controlled by a toggle in Server Settings > General > Security. Uses a curated 2048-word list of fruits, vegetables, animals, happy words, and names. (#39)
- **Email disclaimers** -- Per-domain disclaimer text appended to all outbound emails. New "Disclaimer" tab on the Email page. Uses altermime with a Postfix content_filter on submission/smtps ports. DKIM signing happens after disclaimer insertion so signatures stay valid. (#33)
- **Standard IMAP folders on mailbox creation** -- New mailboxes get Sent, Drafts, Trash, Junk, and Archive folders pre-created with a subscriptions file so they show up in mail clients immediately. (#41)
- **Diagnostic report email** -- "Send via Email" button in the diagnostic report modal opens the user's default email client with the encrypted report pre-addressed to Jabali support.
- **APT repository** -- Packages now published to `deb.jabali-panel.com` for easy install via `apt install jabali-panel`.

### Fixed

- **Mail SSL SNI overwrite** -- `sslMailConfigure()` was overwriting the global Postfix/Dovecot cert every time a mail cert was issued, breaking SMTP for previously configured domains. Now uses per-domain SNI entries (Postfix `sni_maps` and Dovecot `local_name` blocks). Global cert only set when replacing snakeoil. (#35)
- **Mail cert renewal** -- Scheduled SSL check now properly calls `sslMailIssue` for mail-service certificates instead of `sslRenew` which only handles web certs.
- **Passphrase passwords rejected by mailbox creation** -- When passphrase mode was enabled, mailbox password fields still enforced uppercase/number regex rules, rejecting word-style passwords. (#42)

### Changed

- **Support page** -- Diagnostic report modal rebuilt with native Filament components, removing all custom CSS and Alpine.js modal code.

## [0.9-rc123] - 2026-03-13

### Added

- **Encrypted diagnostic report** -- New `jabali:report` artisan command and "Diagnostic Report" button on the Server Status page. Collects system info, service statuses, logs, and DB connectivity into an encrypted report that only the Jabali team can decrypt. Users can paste it into GitHub issues for faster troubleshooting.

## [0.9-rc122] - 2026-03-13

### Fixed

- **502 Bad Gateway after creating a new domain** — `domainCreate()` wrote the PHP-FPM pool config but never reloaded PHP-FPM (`createFpmPool` was called with `reload=false`), so the FPM socket didn't exist when Nginx tried to proxy to it. Now reloads PHP-FPM after creating the pool so the socket is ready before Nginx starts using it. (Closes #31)

## [0.9-rc121] - 2026-03-13

### Added

- **Per-domain mail TLS via SNI** — When an SSL certificate covering `mail.domain` is installed, Postfix and Dovecot are automatically configured with SNI (`tls_server_sni_maps` and `local_name` blocks) so each domain presents its own certificate during IMAP/SMTP handshake. The installer pre-configures the SNI map infrastructure. (Closes #30)

## [0.9-rc120] - 2026-03-13

### Fixed

- **Autoconfig/autodiscover recommends wrong SMTP settings** — Changed primary outgoing server from port 587/STARTTLS to port 465/SSL (implicit TLS) across autoconfig (Thunderbird), autodiscover (Outlook), and iOS mobile profile. Port 587/STARTTLS kept as fallback in autoconfig. Updated connection info in the Email page to show 465/SSL/TLS. (Closes #29)

## [0.9-rc119] - 2026-03-13

### Fixed

- **Confusing disk quota error on systems without quota support** — `quotaSet` now checks if quota tools are installed and quotas are active on the filesystem before running `setquota`. Shows a clear message ("Enable them in Server Settings > Quotas first") instead of a raw `setquota: Cannot stat() mounted device` error.

## [0.9-rc118] - 2026-03-13

### Fixed

- **Webmail 502 on user domains** — User domain nginx vhosts had no `/webmail/` location block, so Roundcube requests hit the user's PHP-FPM pool which can't access `/var/lib/roundcube/`. Added webmail location using the panel FPM socket. Also added `mail.domain` to vhost `server_name` so mail subdomain requests route correctly. (Closes #28)
- **SSL cert missing mail subdomain** — `sslIssue()` now includes `mail.domain` as a SAN in Let's Encrypt certificates when the subdomain resolves, so `https://mail.domain.ext` gets a valid cert.

## [0.9-rc117] - 2026-03-13

### Fixed

- **Webmail SSO 500 error** — The SSO token directory `/var/lib/jabali/sso-tokens/` was never created during install, causing `file_put_contents` to fail when clicking the Webmail link. The installer now creates the directory with correct ownership. The route also handles the missing directory gracefully with an actionable error message instead of a raw 500. (Closes #27)

## [0.9-rc116] - 2026-03-13

### Fixed

- **Mail Server Hostname incorrect after install** — The installer wasn't setting `mail_hostname` in DnsSetting, so Server Settings > Email showed `mail.<system-hostname>` (e.g., `mail.web03.REDACTEDDOMAIN.com`) instead of `mail.<root-domain>`. Now explicitly sets `mail_hostname` to `mail.{root_domain}` during install. (Closes #26)

## [0.9-rc115] - 2026-03-13

### Fixed

- **DNS zone not active after install** — The `dns.sync_zone` call after DKIM generation was missing the `records` parameter, causing the zone file to be overwritten with only SOA/NS records (no A, MX, TXT, etc.). Now passes full records from the database, matching the "Rebuild Zone" behavior. (Closes #25)

## [0.9-rc114] - 2026-03-12

### Fixed

- **SMTP connection drops on ports 25, 587, 465** — Set `inet_interfaces=all` in Postfix (Debian 13 defaults to `loopback-only`, blocking external connections). Set `myhostname` to the configured FQDN so the SMTP banner shows the correct domain instead of the system hostname. Fixed debconf pre-seed to use `$SERVER_HOSTNAME` instead of `hostname -f`. (Closes #23)
- **SMTPS port 465 not configured** — Added `smtps` service to Postfix `master.cf` for legacy mail clients using implicit TLS.

## [0.9-rc113] - 2026-03-12

### Changed

- **Debian 13 only** — Dropped support for Debian 11 (Bullseye) and Debian 12 (Bookworm). Jabali now requires Debian 13 (Trixie) with Dovecot 2.4.

### Removed

- Dovecot 2.3 configuration paths in the installer and `jabali:configure-dovecot-acl` command
- Version detection logic for Dovecot 2.3 vs 2.4
- Legacy `dovecot-sql.conf.ext` and `dovecot-dict-sql.conf.ext` external config file support

## [0.9-rc103] - 2026-03-12

### Added

- **Shared Folders** - New "Shared Folders" tab on the Email page for managing IMAP folder sharing between mailboxes. Users can share individual folders with other mailboxes on the same domain using four permission levels: Read, Read & Write, Full Access, and Admin. Recipients automatically discover shared folders via IMAP. Backed by Dovecot ACL plugin with vfile backend and SQL-based shared mailbox discovery (`user_shares` table).
- **Dovecot ACL plugin configuration** - Dovecot is now configured with the ACL plugin and shared namespaces out of the box. New installations get this automatically; existing installations can run `php artisan jabali:configure-dovecot-acl` to enable the feature.
- **Shared folders translations** - All shared folder UI strings are translated in 7 languages (en, es, ar, fr, ru, pt, he).

### Deployment Notes

1. For existing installations, run `php artisan jabali:configure-dovecot-acl` to configure Dovecot ACL support and create the `user_shares` table
2. Run `php artisan migrate` to create the shared folder permissions table
3. Dovecot will be automatically restarted after ACL configuration

## [0.9-rc101] - 2026-03-12

### Added

- **IMAP Sync** - New Migration tab for migrating mail from external IMAP servers. Supports single mailbox sync and bulk migration (multiple mailboxes at once). Uses `imapsync` as the backend engine with PHP `imap_open()` for connection testing. Includes sync history table with status tracking, retry, and cancel actions. (`app/Filament/Jabali/Pages/ImapSync.php`, `app/Jobs/RunImapSync.php`, `app/Models/ImapSyncTask.php`)
- **Mail subdomain redirect** - Visiting `mail.domain.ext` in a browser now redirects to webmail (Roundcube) instead of showing the panel login. Autoconfig/autodiscover paths are excluded so mail client auto-discovery still works. (`app/Http/Middleware/MailSubdomainRedirect.php`)
- **Installer --debug flag** - Verbose output is now suppressed by default with an animated spinner. Pass `--debug` to see full command output for troubleshooting.
- **Server hostname DNS record** - When a domain matching the server's base domain is created (e.g., `example.com` on server `web02.example.com`), an A record for the hostname subdomain is automatically added. (`app/Observers/DomainObserver.php`)

### Fixed

- **Domain setup during install** - The installer now properly calls the agent's `domainCreate()` and sets the correct user-scoped `document_root` path instead of `/var/www/html`. This fixes the issue where the base domain had no vhost or web directory after install, and users couldn't re-add it through the panel. (Closes #16)
- **Debian 13 detection** - Debian 13 (trixie) is now correctly identified as a stable release instead of "testing/unstable". The OS detection logic now checks `VERSION_ID` first. (Closes #21)
- **Dovecot MySQL authentication** - Dovecot was configured for SQLite but the app uses MySQL. Fixed to use Dovecot 2.4 MySQL block format. Also fixed empty password issue by reading credentials from `/root/.jabali_db_credentials` instead of `.env` (which doesn't exist yet at configure_mail time).
- **IMAP test connection hanging** - Replaced `imapsync --dry` with PHP `imap_open()` for connection testing, since imapsync validates both source and destination hosts even in dry-run mode.
- **IMAP folder checkboxes not appearing** - Replaced conditional schema building with `->visible()` closures for dynamic field visibility in Livewire/Filament forms.
- **Installer .env path** - Fixed `$INSTALL_DIR` undefined variable references, replaced with `$JABALI_DIR`.
- **Installer uninstall hanging** - Wrapped `apt-get autoremove` in error-tolerant block to prevent `set -e` from killing the script.
- **Refresh button redirect** - Fixed Refresh button redirecting to Livewire update endpoint.

### Changed

- **Installer output** - All verbose command output (apt, npm, composer) is now suppressed with an animated spinner. Failures show the last 20 lines of output for debugging.
- **DNS Records table** - Default pagination changed to 25 rows per page.

## [0.9-rc86] - 2026-03-11

### Security

Pre-1.0 security audit remediation. This release addresses vulnerabilities across authentication, authorization, input validation, and configuration.

#### Critical

- **Fix command injection in disk quota check** - `CheckDiskQuotas` now escapes usernames with `escapeshellarg()` before passing to shell commands (`app/Console/Commands/CheckDiskQuotas.php`)
- **Fix command injection in file integrity check** - `CheckFileIntegrity` now escapes all `$basePath` interpolations with `escapeshellarg()` (`app/Console/Commands/CheckFileIntegrity.php`)
- **Fix CSRF on impersonation stop** - Changed `/impersonate/stop` from GET to POST with CSRF token. The impersonation banner now uses a form instead of a plain link (`routes/web.php`, `ImpersonationController.php`, `JabaliPanelProvider.php`)
- **Encrypt DKIM private keys at rest** - Added `'dkim_private_key' => 'encrypted'` cast to `EmailDomain` model. Existing plaintext values require a one-time migration (`app/Models/EmailDomain.php`)
- **Enable TLS verification for migration APIs** - WHM and cPanel API calls now verify SSL certificates by default. Set `JABALI_IMPORT_INSECURE_TLS=true` in `.env` to opt out for self-signed certificates (`app/Services/Migration/WhmApiService.php`, `app/Services/Migration/CpanelApiService.php`, `config/app.php`)

#### High

- **Move webmail SSO tokens out of /tmp** - SSO token files now stored in `/var/lib/jabali/sso-tokens/` with `0600` permissions and `LOCK_EX` atomic writes instead of world-readable `/tmp` (`routes/web.php`, `install.sh`)
- **Upgrade page-cache secret to SHA-256** - WordPress plugin API secret verification upgraded from `md5()` to `hash('sha256', ...)`. Both server and bundled WordPress plugin updated simultaneously (`routes/api.php`, `resources/wordpress/jabali-cache/jabali-cache.php`)
- **Restrict admin backup download paths** - `adminDownload()` now validates that backup paths are under `/home/`, `/var/backups/`, or `storage/app/backups/` before serving files (`app/Http/Controllers/BackupDownloadController.php`)
- **Sanitize terms and policy HTML** - Raw HTML output in terms/policy views now filtered through `sanitizeHtml()` (`resources/views/terms.blade.php`, `resources/views/policy.blade.php`)

#### Medium

- **Verify impersonation session state** - `ImpersonationController::stop()` now checks that `session('impersonated_by')` exists before clearing session data (`app/Http/Controllers/ImpersonationController.php`)
- **Prevent user enumeration via timing** - Admin login performs a constant-time dummy `Hash::check()` when the user is not found, preventing timing-based email enumeration (`app/Filament/Admin/Pages/Auth/Login.php`)
- **Hide internal error details** - Page-cache API endpoints now return generic error messages to callers and log detailed errors server-side (`routes/api.php`)

#### Other

- **Fix XSS in impersonation banner** - User name and username are now HTML-escaped with `e()` in the impersonation notice (`app/Providers/Filament/JabaliPanelProvider.php`)
- **Fix XSS in OpenGraph meta tags** - Site name, title, and description are now HTML-escaped in meta tag attributes (`app/Providers/Filament/JabaliPanelProvider.php`)
- **Use config() for TLS flag** - Migration services read `JABALI_IMPORT_INSECURE_TLS` via `config('app.import_insecure_tls')` instead of `env()` directly, ensuring compatibility with config caching

### Breaking Changes

- **Impersonation stop route** is now POST instead of GET. Any external links or bookmarks to `/impersonate/stop` will no longer work.
- **WordPress page-cache plugin** must be updated to match the new SHA-256 secret verification. The bundled plugin in `resources/wordpress/jabali-cache/` is already updated. Deployed sites should update the plugin simultaneously with the panel.
- **DKIM private keys** will need a one-time encryption migration for existing data. New keys are encrypted automatically.
- **SSO token directory** changed from `/tmp/` to `/var/lib/jabali/sso-tokens/`. The installer creates this directory automatically.

### Deployment Notes

1. Run `php artisan config:clear` after updating `.env` with any new variables
2. Create SSO token directory: `mkdir -p /var/lib/jabali/sso-tokens && chown www-data:www-data /var/lib/jabali/sso-tokens && chmod 700 /var/lib/jabali/sso-tokens`
3. If using DKIM, run a migration to encrypt existing private keys
4. Update the jabali-cache WordPress plugin on all managed sites
5. Run `php artisan config:cache` to cache the new configuration
