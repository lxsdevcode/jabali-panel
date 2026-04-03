# Mail Server Management

Jabali Panel uses Stalwart Mail Server as the only email backend, supporting SMTP, IMAP, JMAP, and ManageSieve protocols.

## Server Architecture

Stalwart stores all account data, domain configuration, and settings in MySQL. The panel manages mailboxes, quotas, and policies through Stalwart's database schema and CLI commands.

## Account Management

### Create Mailbox
```bash
jabali mail:create <email> [--password=] [--quota=]
```
Creates a new mailbox with optional quota (e.g., `1GB`, `500MB`). If `--password` is omitted, a secure prompt appears. Default quota is unlimited.

### Delete Mailbox
```bash
jabali mail:delete <email>
```
Permanently removes a mailbox and all stored messages.

### Change Password
```bash
jabali mail:password <email> [--password=]
```
Updates mailbox password. If `--password` is omitted, a secure prompt appears.

### Set Quota
```bash
jabali mail:quota <email> [--size=]
```
Adjusts storage quota for a mailbox (e.g., `2GB`). Omit `--size` to remove quota limit (unlimited).

### List Accounts
```bash
jabali mail:list [--domain=] [--json]
```
Shows all mailboxes on the system, optionally filtered by domain.

## Quota System

Quotas are stored in MySQL and enforced by Stalwart. The panel reads current usage from Stalwart's database tables:

- `accounts` — mailbox accounts with quota settings
- `principal_blob` — stores quota metadata

When quota is exceeded, Stalwart rejects incoming mail with `452 Insufficient storage` error.

## Mail Logs

### View Logs
```bash
jabali mail:log [--lines=50] [--domain=] [--type=]
```
Displays recent mail server activity. Filter by domain and log type:
- `smtp` — SMTP delivery (incoming mail)
- `delivery` — local delivery and forwarding
- `auth` — authentication attempts
- `jmap` — JMAP/webmail access

Logs are read from system files and Stalwart's database depending on log type.

### Log Retention

Stalwart logs are persisted in:
- `/var/log/stalwart/` — mail server application logs
- Database transaction logs — stored in MySQL for audit trail

## Mail Queue

### View Queue
```bash
jabali mail:queue [--json]
```
Lists messages pending delivery. Shows message ID, sender, recipient, timestamp, and attempt count.

### Retry Message
```bash
jabali mail:queue-retry [id] [--all]
```
Retries delivery for a specific message or all queued messages. Stalwart will attempt re-delivery with exponential backoff.

### Delete Message
```bash
jabali mail:queue-delete <id>
```
Permanently removes a queued message without delivery attempt.

## Domain Configuration

Each domain can have:
- **MX records** — pointing to mail server hostname (automatically created via DNS)
- **SPF record** — `v=spf1 mx -all` (recommended)
- **DKIM** — (optional, depends on Stalwart configuration)
- **DMARC** — (optional, email authentication policy)

The panel manages DNS records via PowerDNS API. Mail domain operations:

```bash
# Add domain MX record
jabali dns:add <domain> "" MX "<priority> <mail-server-hostname>"

# List domain records
jabali dns:records <domain>
```

## Database Tables

Key Stalwart database tables (read-only from panel perspective):

- `accounts` — mailbox accounts, passwords (hashed), quota
- `principal` — user principals (account identities)
- `principal_blob` — quota usage tracking
- `queue` — message queue for delivery
- `auth_attempt` — failed login attempts (brute-force protection)

The panel queries these tables for statistics and management. Never modify directly — always use Stalwart CLI or panel commands.

## Integration with Panel

The panel communicates with Stalwart via:

1. **Direct database queries** — read quota usage, list accounts, check queue
2. **Stalwart CLI commands** — via agent, for account creation/deletion
3. **REST API** — (if enabled, for advanced features)

Mail operations are delegated to the agent for system-level access:

```php
// Example: Create mailbox via agent
AgentClient::call('mailCreate', [
    'email' => 'user@example.com',
    'password' => 'secure-password',
    'quota' => '1GB',
]);
```

## Security

- **Passwords** — hashed with Argon2 by Stalwart
- **IMAP/SMTP Auth** — `PLAIN` over TLS (port 993, 587 respectively)
- **JMAP Auth** — token-based via webmail SSO
- **Brute-force** — rate-limited by IP via `auth_attempt` table
- **Quota enforcement** — Stalwart rejects mail when exceeded

## Troubleshooting

### Mail not being delivered
1. Check MX record exists: `jabali dns:records <domain>`
2. Check queue: `jabali mail:queue`
3. View logs: `jabali mail:log --domain=<domain> --type=smtp`
4. Check account exists: `jabali mail:list --domain=<domain>`

### Quota issues
1. Check current usage: `jabali mail:quota <email>`
2. Increase quota: `jabali mail:quota <email> --size=2GB`
3. Check database: `SELECT email, quota FROM accounts WHERE email = '...'`

### Authentication failures
1. Verify password: reset with `jabali mail:password <email>`
2. Check rate limiting: `SELECT * FROM auth_attempt WHERE principal_id LIKE '%user%'`
3. View auth logs: `jabali mail:log --type=auth`
