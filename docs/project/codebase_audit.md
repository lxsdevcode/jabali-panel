## Codebase Audit Report - 2026-03-28

### Executive Summary

Jabali Panel is a 356-file PHP codebase with solid foundations (98% strict_types, clean Eloquent usage, no SQL injection, strong CSRF/security headers) but significant structural debt. The three lowest-scoring areas — Security (2.8), Code Quality (3.2), and Concurrency (3.5) — share a common root cause: the codebase grew fast and privileged features over hardening. The critical login rate-limiting race and bypassable cron command denylist need immediate attention before 1.0. The god class problem (AgentClient at 1360 lines/150 methods) and zero PHP enums across 577 status string comparisons are the top structural debts.

### Compliance Score

| Category | Score | Notes |
|----------|-------|-------|
| Security | 2.8/10 | Critical: cron command denylist bypassable; file uploads missing type validation |
| Build Health | 7.2/10 | All tests pass; 2 npm CVEs fixable with `npm audit fix` |
| Architecture & Design | 6.1/10 | Good service extraction; password generation duplicated with security divergence |
| Code Quality | 3.2/10 | 7 god classes; AgentClient 1360 lines; zero enums for 577 status strings |
| Dependencies & Reuse | 4.7/10 | 2 HIGH npm CVEs; extraneous packages; stale Tailwind v3 config |
| Dead Code | 5.6/10 | 935 lines of unused agent service facades; deprecated methods |
| Observability | 5.0/10 | Good audit log foundation; no request tracing, no job failure handlers |
| Concurrency | 3.5/10 | Critical: non-atomic login rate limiting; SSL/git deploy race conditions |
| Lifecycle | 6.3/10 | Correct bootstrap order; queue worker lacks stopwaitsecs for long jobs |
| **Overall** | **4.9/10** | |

### Severity Summary

| Severity | Count |
|----------|-------|
| Critical | 3 |
| High | 24 |
| Medium | 52 |
| Low | 39 |

### Strengths

- **98.3% strict_types adoption** across 356 PHP files (350/356)
- **Zero SQL injection risk** — all database access uses Eloquent ORM with parameterized queries
- **No hardcoded credentials** — passwords use `Crypt::encryptString()` or encrypted cast
- **Strong security headers** — CSP, HSTS, X-Frame-Options, CORP, COOP on all panel responses
- **CSRF protection** active on all web routes with no exclusions
- **Clean adapter pattern** in FileBrowser with interfaces having 2+ implementations each
- **Well-extracted services** — SSL management, backup orchestrator, domain/user deletion services
- **Consistent `escapeshellarg()` usage** in shell commands across artisan commands
- **Agent communication** via Unix domain socket (no network exposure)
- **All 167 tests pass** (407 assertions) with clean PHP syntax across all files
- **Correct bootstrap ordering** in systemd, supervisor, and container entrypoint
- **Impersonation security** — one-time tokens with IP binding, POST + CSRF for stop

### Findings by Category

#### 1. Security (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| CRITICAL | app/Filament/Jabali/Pages/CronJobs.php:376 | Cron job command validation uses bypassable denylist; user-controlled commands run via `sudo -u` | Command Injection | Replace denylist with allowlist or sandbox (nsjail/firejail) | L |
| HIGH | app/Filament/Jabali/Pages/Databases.php:396 | SQL file upload missing `acceptedFileTypes()` — any file type reaches disk | File Upload Validation | Add `acceptedFileTypes()` for SQL/gzip/zip MIME types | S |
| HIGH | app/Http/Controllers/AutomationApiController.php:88 | `createDomain` endpoint missing domain format validation; input used in path construction | Input Validation | Add domain format validation via regex or `filter_var(FILTER_VALIDATE_DOMAIN)` | S |
| MEDIUM | app/Http/Middleware/SecurityHeaders.php:23 | CSP script-src includes permissive inline/dynamic execution directives | XSS / CSP | Investigate CSP nonces with Livewire; use Alpine.js CSP-compatible build | M |
| MEDIUM | package.json (picomatch) | ReDoS + method injection (GHSA-c2c7-rcm5-vvqj, GHSA-3v7f-55p6-f55p) | Insecure Dependencies | `npm audit fix` | S |
| MEDIUM | package.json (rollup) | Arbitrary file write via path traversal (GHSA-mw96-cpmx-2vgc) | Insecure Dependencies | `npm audit fix` | S |
| MEDIUM | app/Filament/Admin/Pages/DirectAdminMigration.php:189 | Admin migration FileUpload missing `acceptedFileTypes()` and `maxSize()` | File Upload Validation | Add type and size restrictions | S |
| MEDIUM | routes/web.php:146-157 | Webmail SSO stores plaintext passwords in filesystem token files | Secrets Management | Use Laravel Cache (Redis) or encrypt with `Crypt::encryptString()` | M |
| MEDIUM | routes/api.php:79 | Internal API domain input used in path construction without format validation | Path Traversal | Validate domain format before path construction | S |
| LOW | stubs/phpmyadmin/jabali-signon.php:46-47 | phpMyAdmin signon disables SSL verification for localhost call | Insecure Configuration | Pin Jabali panel CA certificate | S |

#### 2. Build Health (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| HIGH | package-lock.json (picomatch) | npm vulnerability: method injection + ReDoS | Dependency Security | `npm audit fix` | S |
| HIGH | package-lock.json (rollup) | npm vulnerability: arbitrary file write via path traversal | Dependency Security | `npm audit fix` | S |
| LOW | scripts/check-pages.php | Pint formatting violations | Code Style | `vendor/bin/pint scripts/check-pages.php` | S |
| LOW | scripts/zap/generate-session.php | Pint formatting violations | Code Style | `vendor/bin/pint scripts/zap/generate-session.php` | S |
| LOW | tailwind.config.js | Dead v3 config file; Tailwind v4 uses CSS-first `@theme` | Dead Config | Remove file; update UpgradeCommand reference | S |
| LOW | postcss.config.js | Redundant; autoprefixer handled by `@tailwindcss/vite` | Dead Config | Remove file and autoprefixer dependency | S |
| LOW | composer.json:91 | `pestphp/pest-plugin` in allow-plugins but Pest not installed | Unused Config | Remove entry | S |

#### 3. Architecture & Design (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| HIGH | WordPress.php:318, Databases.php:513, PasswordGenerator.php:9 | Password generation copy-pasted 3x; copies use weaker `str_shuffle()` vs Fisher-Yates | DRY (security divergence) | Replace inline generation with `PasswordGenerator::generate()` | M |
| MEDIUM | app/Support/SafeError.php, app/FileBrowser/Support/SafeError.php | Identical 30-line class duplicated across two namespaces | DRY | Delete FileBrowser copy; update 3 imports | S |
| MEDIUM | app/Support/Formatter.php, app/FileBrowser/Support/Formatter.php | Near-identical `bytes()` method in two Formatter classes | DRY | Merge nullable handling into `App\Support\Formatter`; remove FileBrowser copy | S |
| MEDIUM | app/Console/Commands/Jabali/ImportProcessCommand.php:270-360 | Three near-identical extract-copy-chown blocks for cpanel/hestiacp/directadmin | DRY | Extract private `extractAndCopyDomainFiles()` method | M |
| MEDIUM | app/Services/MailboxSharingService.php:47-120 | Uses `DB::table('user_shares')` directly; `MailboxShare` Eloquent model exists | Laravel Best Practice | Use Eloquent model instead of raw DB facade | M |
| MEDIUM | app/Filament/Jabali/Pages/Concerns/ManagesEmailSharing.php:89,118,212 | `MailboxSharingService` instantiated with `new` 3x instead of DI | Dependency Injection | Use `app(MailboxSharingService::class)` or constructor injection | S |
| LOW | 35+ Filament pages | Generic `__('Error')` notification title in 35+ catch blocks | DRY | Consider shared `$this->notifyError()` trait method | M |
| LOW | Project root | No ARCHITECTURE.md or CONTRIBUTING.md for developers | Best Practices Guide | Create `docs/architecture.md` | S |

#### 4. Code Quality (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| CRITICAL | app/Services/Agent/AgentClient.php (1360 lines, 150 methods) | Extreme god class wrapping every RPC call to system agent | God Classes | Complete migration to domain-specific services (partially done) | L |
| HIGH | app/Filament/Admin/Pages/CpanelMigration.php (2380 lines) | God class: connection, discovery, restore, UI, status tracking | God Classes | Extract wizard steps into Concerns/Livewire components | L |
| HIGH | app/Filament/Admin/Pages/ServerSettings.php (1726 lines) | God class: manages all server settings tabs in one file | God Classes | Extract each tab into a dedicated Concern | L |
| HIGH | app/Filament/Jabali/Pages/CpanelMigration.php (1681 lines) | God class: user-panel duplicate of admin CpanelMigration | God Classes | Extract shared migration logic into trait or base class | L |
| HIGH | app/Services/Migration/WhmMigrationOrchestrator.php:35-49 | `run()` accepts 13 parameters including 5 booleans | Too Many Params | Introduce `WhmMigrationConfig` value object | M |
| HIGH | app/Console/Commands/Jabali/ImportProcessCommand.php:244-365 | `importFiles()` ~120 lines with 5-6 levels of nesting | Deep Nesting | Extract per-source-type importers via strategy pattern | M |
| HIGH | app/Console/Commands/Jabali/ImportProcessCommand.php:367-476 | `importDatabases()` ~110 lines with 6 levels of nesting | Deep Nesting | Extract database decompression helper; flatten with early returns | M |
| HIGH | app/Filament/Admin/Pages/SslManager.php:408-462 | `issueAllPending()` iterates domains calling ACME per-domain; blocks request | N+1 / Batch | Dispatch per-domain jobs instead of blocking | M |
| MEDIUM | Entire codebase (577 occurrences, 88 files) | Status strings as raw strings everywhere; zero PHP enums | Magic Strings | Create backed enums: `BackupStatus`, `SslStatus`, `ImportStatus`, etc. | M |
| MEDIUM | app/Filament/Admin/Pages/WhmMigration.php (1298 lines) | God class: connection, selection, execution, status polling | God Classes | Extract into Concern traits | L |
| MEDIUM | app/Services/Migration/CpanelApiService.php (1326 lines) | Borderline god class for cPanel API wrapper | God Classes | Group into traits (CpanelSshMixin, CpanelBackupMixin, etc.) | M |
| MEDIUM | app/Filament/Admin/Pages/SslManager.php:370-406 | `runAutoSslForUser()` calls artisan per-domain in foreach loop | N+1 / Batch | Refactor to accept multiple domains or dispatch queued jobs | M |
| MEDIUM | app/Filament/Jabali/Pages/Databases.php (1288 lines) | God class: MySQL, PostgreSQL, users, grants in one file | God Classes | Extract ManagesDatabaseUsers concern | L |
| MEDIUM | app/Services/Migration/WhmMigrationOrchestrator.php:106-253 | `migrateAccount()` ~150 lines; well-structured but long | Long Methods | Extract SSH key setup and backup transfer into private methods | M |
| MEDIUM | WhmMigration.php:1162-1236 + WhmMigrationOrchestrator.php:312-379 | `waitForBackupFile()` duplicated between page and orchestrator | DRY | Extract into shared trait or utility | M |
| MEDIUM | CpanelApiService.php + WhmApiService.php | Duplicated SSH key import/authorize/delete patterns | DRY | Extract shared `CpanelSshKeyService` | M |
| LOW | CpanelMigration.php:800-816 | Duplicated match expressions for status→color/prefix mapping | Magic Strings / DRY | Extract `MigrationStatusHelper` utility | S |
| LOW | SslCertificate.php + PanelCertificate.php | Raw status string comparisons | Magic Strings | Create `SslStatus` enum | S |
| LOW | WhmApiService.php:131,163 | Loose comparison `== 1` instead of strict `=== 1` | Type Safety | Use strict comparison | S |

#### 5. Dependencies & Reuse (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| HIGH | picomatch <=2.3.1 | Method injection + ReDoS (GHSA-3v7f, GHSA-c2c7) | CVE | `npm audit fix` | S |
| HIGH | rollup 4.0.0-4.58.0 | Arbitrary file write via path traversal (GHSA-mw96) | CVE | `npm audit fix` | S |
| MEDIUM | node_modules | 3 extraneous npm packages: echarts, chartjs-plugin-datalabels, zrender | Unused Dependencies | `npm prune` | S |
| LOW | package.json | autoprefixer redundant with Tailwind v4 | Redundant Dependency | Remove autoprefixer and postcss.config.js | S |
| LOW | tailwind.config.js | Stale v3 config not used by v4 pipeline | Dead Config | Remove file | S |
| LOW | composer.json | allow-plugins lists pestphp/pest-plugin, php-http/discovery (not installed) | Unused Config | Remove entries | S |
| LOW | composer.json | PHP constraint `^8.2` too broad; targets PHP 8.4 | Version Constraint | Tighten to `^8.4` | S |
| LOW | 10 PHP + 7 npm packages | Semver-safe updates available | Outdated | `composer update` + `npm update` | S |

#### 6. Dead Code (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| MEDIUM | app/Services/Agent/FileAgentService.php (95 lines) | Unused class — never instantiated or injected | Unused Code | Delete file | S |
| MEDIUM | app/Services/Agent/BackupAgentService.php (85 lines) | Unused class — never instantiated or injected | Unused Code | Delete file | S |
| MEDIUM | app/Services/Agent/DomainAgentService.php (45 lines) | Unused class — never instantiated or injected | Unused Code | Delete file | S |
| MEDIUM | app/Services/Agent/SystemAgentService.php (358 lines) | Unused class — never instantiated or injected | Unused Code | Delete file | S |
| MEDIUM | app/Services/Agent/DatabaseAgentService.php (139 lines) | Unused class — never instantiated or injected | Unused Code | Delete file | S |
| MEDIUM | app/Services/Agent/EmailAgentService.php (120 lines) | Unused class — never instantiated or injected | Unused Code | Delete file | S |
| MEDIUM | app/Services/FileBrowser/AgentTrashStore.php (93 lines) | Unused class — implements TrashStore but never bound | Unused Code | Delete file | S |
| MEDIUM | app/Filament/Admin/Pages/Backups.php:359 | Unused private method `verifyRepoAction()` (60 lines) | Unused Code | Register in getHeaderActions() or delete | S |
| LOW | app/Services/Migration/CpanelApiService.php:962 | Deprecated `createBackupToScp()` — never called | Legacy Code | Delete method | S |
| LOW | app/Console/Commands/Jabali/ConfigureDovecotAclCommand.php | Legacy Dovecot ACL command — unreferenced; Stalwart is now the only mail backend | Legacy Code | Delete (Stalwart migration is complete) | S |

#### 7. Observability (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| HIGH | app/Http/Middleware/ (absent) | No request/correlation ID middleware — cross-component tracing impossible | Request Tracing | Add UUID middleware with `Log::shareContext()` | S |
| HIGH | routes/web.php:15-18 | `/up` health check returns static 200 without verifying DB/Redis/agent | Health Checks | Add dependency checks; return 503 on failure | S |
| HIGH | app/Jobs/*.php (7 classes) | No `failed()` method on any job — failures are silent | Queue Monitoring | Add `failed()` with admin notification | M |
| HIGH | 80 of 172 Log calls | 47% of log calls use string interpolation without structured context | Structured Logging | Establish static message + context array convention | M |
| HIGH | app/Filament/Admin/Pages/*.php, Services/*.php | Incomplete audit coverage — backups, SSL, DNS, settings not logged | Audit Trail | Add AuditLog calls to admin operations | M |
| MEDIUM | config/logging.php | No custom log channels for domain-specific concerns | Structured Logging | Add backup, migration, agent, security channels | M |
| MEDIUM | config/logging.php | No JSON/structured log formatter configured | Structured Logging | Configure JsonFormatter for log aggregation | S |
| MEDIUM | routes/web.php, bootstrap/app.php | No readiness/liveness probe separation | Health Checks | Create `/ready` endpoint with dependency checks | S |
| MEDIUM | Codebase-wide | No time-series metrics storage; metrics are live-only | Metrics | Integrate `laravel/pulse` or persist metrics | M |
| MEDIUM | Codebase-wide | No application-level metrics (requests/s, queue throughput, etc.) | Metrics | Add counters for key operations | M |
| MEDIUM | Agent calls | Agent socket calls carry no trace ID | Request Tracing | Pass correlation ID to agent | S |
| MEDIUM | Codebase-wide | No `Log::shareContext()` for automatic request context | Request Tracing | Add to request ID middleware | S |
| MEDIUM | Codebase-wide | critical/alert/emergency log levels never used | Log Levels | Reserve for system-down events | S |
| MEDIUM | DomainDeletionService, UserDeletionService | Inconsistent log level assignment (warning vs error) | Log Levels | Document log level policy | S |
| MEDIUM | Codebase-wide | No queue monitoring or alerting for failed jobs | Queue Monitoring | Add `queue:monitor` or Pulse | M |
| MEDIUM | app/Jobs/*.php | 5 of 7 jobs have `$tries=1` with no retry/backoff | Queue Monitoring | Add retry configuration for transient failures | S |
| MEDIUM | app/Filament/Admin/Pages/Domains.php | Admin domain operations not audited (only user-panel) | Audit Trail | Add AuditLog to admin domain management | S |
| MEDIUM | app/Http/Controllers/ImpersonationController.php | Impersonation sessions not audited | Audit Trail | Log impersonation start/stop to AuditLog | S |
| MEDIUM | bootstrap/app.php:47-49 | Empty `withExceptions()` — no custom exception reporting | Exception Handling | Configure error tracking or custom handlers | M |
| MEDIUM | app/Console/Commands/Jabali/DiagnosticReportCommand.php:72-298 | Silent exception swallowing — 15 try/catch blocks with empty catch | Exception Handling | Log skipped sections at minimum | S |
| MEDIUM | routes/console.php | No schedule monitoring or dead-man's switch | Scheduled Tasks | Add `schedule:monitor` ping | S |
| LOW | config/logging.php:35 | Deprecations channel set to null — warnings silently discarded | Log Levels | Set to `log` channel | S |
| LOW | CpanelApiService.php:61-84 | Logs request/response bodies at info level — high volume | Log Levels | Use debug level for request bodies | S |
| LOW | composer.json | No Pulse or Horizon installed for queue dashboard | Metrics | Consider `laravel/pulse` | M |
| LOW | app/Jobs/RunGitDeployment.php | Missing `$tries` and `$timeout` configuration | Queue Monitoring | Add explicit tries and timeout | S |
| LOW | tests/ | No tests for audit log entries | Audit Trail | Add feature tests for critical audit actions | M |
| LOW | routes/console.php:69-72 | `jabali:sync-mailbox-logins` has no output capture | Scheduled Tasks | Add `appendOutputTo()` | S |
| LOW | routes/console.php (all appendOutputTo) | Schedule output files grow indefinitely — no rotation | Scheduled Tasks | Add logrotate config | S |

#### 8. Concurrency (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| CRITICAL | Admin/Auth/Login.php:59-64, Jabali/Auth/Login.php:76-81 | Login rate-limiting race: non-atomic `Cache::get` + `put` allows brute-force bypass | CWE-362 / Auth | Replace with atomic `Cache::increment()` | S |
| HIGH | app/Jobs/IssueSslCertificate.php:38-47 | Check-then-act race: concurrent jobs can issue duplicate SSL certs | CWE-362 | Add `ShouldBeUnique` with domain+service uniqueId | S |
| HIGH | app/Jobs/RunGitDeployment.php:18-67 | No uniqueness constraint; rapid webhooks cause concurrent git deploys | CWE-362 | Add `ShouldBeUnique` with deployment uniqueId | S |
| HIGH | app/Services/Migration/WhmMigrationStatusStore.php:88-98 | Non-atomic read-modify-write on cache; status entries can be lost | CWE-362 | Use `Cache::lock()` around get-modify-put cycle | M |
| MEDIUM | app/Services/MailboxSharingService.php:47-74 | Agent socket call (120s timeout) held inside DB transaction | CWE-833 / Deadlock | Move agent calls outside transaction; use compensating action | M |
| MEDIUM | app/Console/Commands/RunBackupSchedules.php:46-48 | Concurrent backup dispatch if previous takes >5 minutes | CWE-362 | Check for existing running/pending backup before creating new | S |
| MEDIUM | app/Console/Commands/RunUserCronJobs.php:80-116 | User cron jobs can execute concurrently if previous exceeds 1 minute | CWE-362 | Track `is_running` flag atomically | M |

#### 9. Lifecycle (Global)

| Severity | Location | Issue | Principle Violated | Recommendation | Effort |
|----------|----------|-------|-------------------|----------------|--------|
| HIGH | docker/supervisord.conf:84-92 | Queue worker lacks `stopwaitsecs`; default 10s may SIGKILL long-running jobs | Graceful Shutdown | Add `stopwaitsecs=130` (above --timeout=120) | S |
| HIGH | app/Jobs/*.php (7 classes) | No `failed()` methods — partial state from failed jobs never cleaned up | Resource Cleanup | Add `failed(Throwable $e)` to critical jobs | M |
| MEDIUM | bin/jabali-agent:3808-3822 | SIGTERM handler doesn't close active mysqli connections | Resource Cleanup | Track and close mysqli in signal handler | S |
| MEDIUM | install.sh:1191-1207 | Systemd services lack `TimeoutStopSec` directives | Signal Handling | Add `TimeoutStopSec=30` (panel/agent), `130` (queue) | S |
| MEDIUM | routes/web.php:15-18 | `/up` returns static 200 without dependency checks | Probes | Add DB/Redis/agent health checks | S |
| LOW | bin/jabali-agent:4290-4329 | Unreachable dead code block after `return` in `mysqlGetPrivileges()` | Dead Code | Remove lines 4290-4329 | S |

### Advisory Findings (Context-Validated)

| Location | Original Severity | Rule Applied | Evidence | Note |
|----------|-------------------|-------------|----------|------|
| app/Console/Commands/SmokeTest.php (1273 lines) | MEDIUM | Rule 3: Cohesion | 137 test methods sharing `$this` state; single-purpose test harness; CC=1 per method | [High cohesion module] — test runner, not business logic |
| app/Filament/Jabali/Pages/WordPress.php (1375 lines) | LOW | Rule 3: Cohesion | Filament page with table+actions; shared `$this->agent()` state; standard SDUI pattern | [High cohesion module] — Filament convention |
| app/Console/Commands/Jabali/UpgradeCommand.php (874 lines) | LOW | Rule 3: Cohesion | Sequential upgrade steps sharing `$this` state; single workflow entry point | [High cohesion module] — upgrade orchestrator |

### Recommended Actions (Priority-Sorted)

| Priority | Category | Location | Issue | Recommendation | Effort |
|----------|----------|----------|-------|----------------|--------|
| 1 | Concurrency | Login pages (both panels) | Non-atomic rate limiting allows brute-force | Replace with `Cache::increment()` | S |
| 2 | Security | CronJobs.php:376 | Bypassable command denylist | Replace with allowlist or sandbox | L |
| 3 | Concurrency | IssueSslCertificate, RunGitDeployment | Missing job uniqueness constraints | Add `ShouldBeUnique` interface | S |
| 4 | Security | Databases.php:396, DirectAdminMigration.php:189 | File uploads missing type validation | Add `acceptedFileTypes()` | S |
| 5 | Security | AutomationApiController.php:88 | Domain format not validated | Add domain format validation | S |
| 6 | Lifecycle | supervisord.conf | Queue worker lacks `stopwaitsecs` | Add `stopwaitsecs=130` | S |
| 7 | Observability | All 7 job classes | No `failed()` methods | Add failure handlers with admin notification | M |
| 8 | Observability | Middleware (absent) | No request/correlation ID | Add UUID middleware + `Log::shareContext()` | S |
| 9 | Observability | /up endpoint | Superficial health check | Add DB/Redis/agent dependency checks | S |
| 10 | Dependencies | npm packages | 2 HIGH CVEs (picomatch, rollup) | `npm audit fix` | S |
| 11 | Architecture | PasswordGenerator duplication | Security-divergent copies use `str_shuffle()` | Replace with `PasswordGenerator::generate()` | M |
| 12 | Code Quality | AgentClient.php (1360 lines) | Extreme god class with 150 methods | Complete migration to domain-specific services | L |
| 13 | Code Quality | Entire codebase (577 occurrences) | Zero PHP enums for status strings | Create backed enums per domain | M |
| 14 | Dead Code | 7 Agent service files (935 lines) | Unused facade classes never wired in | Delete or wire into callers | S |
| 15 | Code Quality | CpanelMigration (admin+user) | 2380+1681 line god class pair | Extract shared logic into traits/base class | L |

### Priority Actions

1. Fix all Critical issues before next release (login race condition, cron command denylist)
2. Address High issues within current sprint (job uniqueness, file uploads, stopwaitsecs, failed() handlers)
3. Plan Medium issues for technical debt sprint (enums, god class extraction, structured logging)
4. Track Low issues in backlog (dead config cleanup, stale dependencies, log rotation)

### Sources Consulted

- Laravel 12.x documentation: https://laravel.com/docs/12.x (Context7 /websites/laravel_12_x)
- Filament v5 documentation: https://filamentphp.com/docs/5.x (Context7 /websites/filamentphp_5_x)
- Livewire v4 documentation: https://livewire.laravel.com/docs/4.x (Context7 /websites/livewire_laravel_4_x)
