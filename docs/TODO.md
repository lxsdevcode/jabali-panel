# TODO: Route Remaining System Writes Through Agent

Operations that write to system paths directly instead of going through the agent. Not security risks (fail gracefully as permission denied), but broken when triggered as www-data.

## Migration Commands

### CpanelMigration.php
- Lines 1634, 1684: `mkdir /var/backups/jabali/cpanel-migrations`
- Use `system.write_config` with `mkdir: true`

### WhmMigration.php
- Line 943: `mkdir /var/backups/jabali/whm-migrations`
- Use `system.write_config` with `mkdir: true`

### DirectAdminMigration.php
- Lines 279, 282: `mkdir + chmod /var/tmp/*`
- Low priority, www-data can write to /var/tmp/ anyway

### ImportProcessCommand.php
- Lines 291-292, 371-372: `exec chown -R` on `/home/{user}/domains/`
- Use agent's existing `file.chown` route

### MailMigrateCommand.php
- Line 190: `exec tar` to install stalwart binary at `/usr/local/bin/`
- Line 402: `mkdir /var/lib/stalwart-mail/data/dkim/`
- Line 405: `chmod` on DKIM key file
- Use `system.write_config` for dirs, new agent route for binary install

## Upgrade Command

### UpgradeCommand.php
- Lines 890-897: `chmod/chown/chgrp` on Redis config and ACL files
- Line 930: `exec systemctl restart redis-server`
- Use `system.write_config` for file ops, `system.systemctl` for restart

## Agent Route Additions Needed

- Add `/var/backups/jabali/` to `system.write_config` allowed prefixes
- Add `/var/lib/redis/` to `system.write_config` allowed prefixes
- Consider a `system.install_binary` route for downloading and installing binaries to `/usr/local/bin/`
