# Plan: One-Time Login Tokens

**Objective**: Shareable one-time login URLs (15 min TTL) for admin and user panels, generated via CLI and dashboard UI.

## Current State

- `GenerateLoginTokenCommand` (`jabali:login:token`) generates cache-based one-time tokens for admin access
- `/auto-login?token=X` route consumes the token, logs in the user, redirects to the correct panel
- `ImpersonationToken` model exists (DB-backed, IP-bound) — used for admin→user impersonation only
- No UI in either dashboard for generating shareable tokens
- CLI command works but only has `--user=admin` default, no user selection UX

## Design Decisions

- **Reuse the existing cache-based approach** (not ImpersonationToken). The cache tokens are simpler, don't require IP binding (tokens are shared with third parties at unknown IPs), and auto-expire. ImpersonationToken is for admin→user impersonation which is a different feature.
- **15 minute default TTL**, configurable up to 60 min in CLI
- **One-time use** — `Cache::pull()` deletes on first use (already implemented)
- **No IP binding** — the token is meant to be sent to someone else
- **Audit log** — log token generation (who generated, for whom, when)

## Steps

### Step 1: CLI — add `jabali login user` command

**Files**: `app/Console/Commands/Cli/LoginTokenCommand.php`

The existing `GenerateLoginTokenCommand` is an artisan command (`jabali:login:token`). Add a new CLI command that routes as `jabali login token` with interactive user selection.

- Signature: `jabali:login:token {--user=} {--ttl=15} {--panel=} {--json} {--yes}`
- Extend `JabaliCommand` (same as other CLI commands)
- If `--user` not provided, list all users and prompt to select
- If `--panel` not provided, auto-detect: admin users → admin, regular users → user
- Default TTL: 15 minutes (not 5)
- Output: the login URL
- Log to audit: `AuditLog::log('login_token_generated', ...)`

This replaces the existing `GenerateLoginTokenCommand`. Remove the old one to avoid two commands doing the same thing.

**Verification**: `jabali login token --user=shuki` outputs a URL, opening it logs in.

### Step 2: Admin Dashboard — "Generate Login Link" action

**Files**: `app/Filament/Admin/Pages/Dashboard.php`

Add a header action button "Generate Login Link" on the admin dashboard.

- Filament `Action::make('generateLoginLink')`
- Form: Select user (all users), TTL dropdown (15m, 30m, 1h), panel selector (admin/user, auto-selected based on user)
- On submit: generate token via `Cache::put()`, build URL, show in a modal with a copy button
- Only visible to admin users (`->visible(fn () => auth()->user()?->is_admin)`)
- Audit log entry

**Verification**: Admin sees button on dashboard, can select a user, gets a copyable URL.

### Step 3: User Dashboard — "Generate Support Access" action

**Files**: `app/Filament/Jabali/Pages/Dashboard.php`

Add a header action "Generate Support Access" on the user dashboard.

- Filament `Action::make('generateSupportAccess')`
- No user selector (generates for the current user)
- TTL fixed at 15 minutes
- Panel fixed to `user`
- On submit: generate token, show URL in modal with copy button
- Description: "Share this link with your developer. It expires in 15 minutes and can only be used once."
- Audit log entry

**Verification**: Regular user sees button, generates link for their own account.

### Step 4: Cleanup scheduler

**Files**: `routes/console.php`

The cache-based tokens auto-expire, but add a daily cleanup for any edge cases:

- `Schedule::call(fn () => ...)` to purge any stale `login_token:*` cache keys (already handled by cache TTL, but good hygiene)
- Actually, `Cache::pull()` + TTL already handles this. Skip this step unless cache driver doesn't support TTL expiry (file driver).

Check current cache driver. If Redis — no cleanup needed. If file — add cleanup.

**Verification**: Tokens expire after TTL regardless of use.

## File Summary

| File | Action |
|------|--------|
| `app/Console/Commands/Cli/LoginTokenCommand.php` | Create (new CLI command) |
| `app/Console/Commands/GenerateLoginTokenCommand.php` | Delete (replaced by CLI command) |
| `app/Filament/Admin/Pages/Dashboard.php` | Edit (add header action) |
| `app/Filament/Jabali/Pages/Dashboard.php` | Edit (add header action) |
| `routes/web.php` | No change (auto-login route already works) |

## Security Notes

- Tokens are stored as SHA-256 hashes in cache (already implemented)
- No IP binding (intentional — tokens are shared with third parties)
- One-time use via `Cache::pull()` (already implemented)
- 15 min TTL (reduced from potential 60 min in CLI)
- Audit logged for accountability
- Regular users can only generate tokens for themselves (not others)
