# Backups

Jabali uses Restic for encrypted, deduplicated backups with native remote backend support.

## Features

- **Encryption** — all backups encrypted with a password at `/etc/jabali/restic-password`
- **Deduplication** — only changed blocks are stored (saves disk/bandwidth)
- **Remote destinations** — SFTP, S3, Backblaze B2 (configured in panel)
- **Selective restore** — restore individual domains, databases, mailboxes, DNS zones, or SSL certs
- **File browser** — browse backup snapshots and download individual files
- **MySQL user backup** — includes MySQL users and grants (`users.sql`)

## First-Time Setup

On first visit to the Backups page, a setup wizard guides you through:

1. Setting the encryption password
2. Configuring a remote destination (with connection validation)
3. Choosing a backup schedule

## CLI Commands

```bash
jabali backup list                    # List all backups
jabali backup create <user>           # Create backup for a user
jabali backup info <id>               # Show backup details
jabali backup restore <path> --user=X # Restore from backup
jabali backup delete <id>             # Delete a backup
jabali backup password                # Show encryption password
jabali backup password --set=NEW      # Change encryption password
```

## Restore Wizard

The panel includes a restore wizard with:

- Selectable components (domains, databases, MySQL users, mailboxes, DNS, SSL)
- Conflict resolution (overwrite, merge, skip, rename)
- Optional safety backup before restore
- File browser for individual file download
