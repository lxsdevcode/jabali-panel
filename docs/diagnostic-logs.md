# Diagnostic Log Sharing

Send encrypted server logs to Jabali support with a single command. Logs are uploaded to an encrypted paste service and the link is delivered via push notification — no public URLs, no manual pasting.

## CLI

```bash
jabali logs share          # Collect, encrypt, upload, notify support
jabali logs share --raw    # Print logs to terminal (no upload)
jabali logs share --yes    # Skip confirmation prompt
```

### Flow

1. Confirmation prompt with privacy notice
2. Optional issue description
3. Collects 25 diagnostic sections
4. Generates a random password
5. Uploads to `enclosed.jabali-panel.com` (password-protected, 24h TTL)
6. Sends link + password to `ntfy.jabali-panel.com/jabali-support`
7. Displays a ticket ID for the user to share in their GitHub issue

### Example

```
root@server:~# jabali logs share

  Collects: services, configs, error logs.
  No passwords or personal data. Sent encrypted to Jabali support.

 Send diagnostic logs? (yes/no) [no]: yes
 Describe the issue (optional, press Enter to skip): SSL not renewing

Collecting diagnostic logs...
Uploading encrypted logs...

Diagnostic logs sent to Jabali support.

  Ticket ID: A3F7B21C

Share this ID in your GitHub issue so we can find your logs. Expires in 24 hour(s).
```

## Admin Panel

The **Support** page has a **"Send Diagnostic Report"** button that:

1. Shows a modal with an optional issue description textarea
2. Collects and uploads logs
3. Shows the ticket ID in a copyable modal

## What's Collected

| Category | Sections |
|----------|----------|
| System | OS, uptime, memory, disk, versions (PHP/Node/nginx) |
| Services | Status of all Jabali services, failed units |
| Web | nginx errors, PHP-FPM log, FrankenPHP journal |
| Laravel | Application log, failed queue jobs |
| Agent | Agent log, health monitor |
| Mail | Stalwart journal, Stalwart config |
| Webmail | Bulwark journal |
| SSL | Certbot log, certificate list, panel cert details |
| DNS | PowerDNS journal |
| Database | MariaDB error log |
| Security | CrowdSec alerts, jabali-security journal, firewall |
| SSH | sshd config, auth log, groups, home dirs, containers |

## Privacy

- No passwords, API keys, or personal data collected
- Logs are encrypted client-side before upload
- Password-protected paste (only Jabali support has the password)
- Link expires in 24 hours
- User must explicitly consent before sending
