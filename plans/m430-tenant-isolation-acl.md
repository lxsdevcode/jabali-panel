# M430 — Tenant home isolation: decouple nginx from the shared www-data group

**Status:** PLANNED (deferred 2026-06-24). Tracks Gitea **#430**.
**Risk:** HIGH — touches live PHP serving for every tenant. Must not deploy to a
real host without multi-tenant end-to-end validation + operator sign-off.

## Problem (#430)

Every tenant is a member of `www-data`, and tenant home/docroot files are group
`www-data` so nginx (which runs as `www-data`) can read them. Therefore DAC, on
its own, lets any tenant read any other tenant's group-`www-data` files
(`wp-config.php`, `.env`, dumps). Cross-tenant confidentiality currently depends
entirely on the *execution* lockdown (`open_basedir`, `disable_functions`,
command-allowlisted cron), not on the permissions — one bypass away from total
disclosure. Defense-in-depth gap, not a standalone live exploit.

## Why the naive fix breaks serving (verified live on 10.0.3.14, 2026-06-24)

See [[project_wwwdata_loadbearing]]. The per-user FPM **master runs as the
tenant** (`jabali-fpm@<u>.service` drop-in `User=<u> Group=<u>`). The pool sets
`listen.owner={{.User}}  listen.group=www-data  listen.mode=0660`; live socket is
`srw-rw---- <u> www-data /run/php/jabali-<u>/fpm.sock`. A non-root process can
only `chgrp` to a group it belongs to, so the tenant MUST be in www-data for its
master to set the socket group to www-data — that's how nginx reaches the socket.
nginx also reads docroots via www-data. **Remove www-data from tenants and both
the FPM socket group and docroot read break → PHP down for every tenant.**

Because nginx and tenants are both www-data, ANY grant to www-data (group or
`u:www-data` ACL) is also a grant to every tenant. The entanglement is structural
— the fix must decouple nginx's identity from tenants.

## Redesign — per-tenant groups + root-applied POSIX ACL for nginx

1. **Drop www-data from tenants.** `user_create.go` (no `--groups www-data`),
   `user_slice_ensure.go` (remove the `usermod -aG www-data`). Reconcile existing
   tenants: `gpasswd -d <u> www-data`.
2. **FPM socket per-tenant group.** Pool template `listen.group={{.User}}` (or a
   dedicated `<u>-web` group). nginx no longer reaches it via www-data.
3. **Grant nginx the socket via root ACL.** Sockets are recreated on every FPM
   (re)start, so a root step must reapply `setfacl -m u:www-data:rw <sock>` after
   the socket appears — `ExecStartPost=+…` in `jabali-fpm@.service` (root) or a
   systemd `.path` unit watching `/run/php/jabali-<u>/fpm.sock`.
4. **Docroot read for nginx via per-path ACL, not group.** docroot dirs/files
   group → `<u>` (not www-data); apply `setfacl -R -m u:www-data:rX` +
   `setfacl -dR -m u:www-data:rX` (default ACL so new files inherit) on each
   docroot. Tenants (no longer in www-data) can't use it; nginx (www-data) can.
   Touch `domain_create.go` docroot prep + a reconciler backfill for existing
   docroots + `files_write.go`/`files_mkdir.go` ownership (already moving to
   fd-anchored — keep group `<u>`, rely on default ACL for nginx).
5. **Home perms.** Live homes are `root:<u> 0711` (listing already blocked,
   traverse allowed) — keep; ensure no intermediate dir is group-www-data
   readable. Re-map the CURRENT live posture before editing (source
   `user_create.go` says `<u>:www-data 0750`, which the live box does NOT match —
   the model has drifted; trust the box).
6. **phpMyAdmin / shared pools** (`jabali-fpm@pma`) use www-data today — audit
   that the dedicated `pma` user path still works (it's not a tenant).

## Validation (gating — do NOT skip)

- Spin up a **2-tenant** test box. Confirm BEFORE: tenant A can `cat`
  B's docroot file. AFTER: A gets EACCES on B's home/docroot/socket, while both
  sites still serve **static AND PHP** (curl a .php through nginx→FPM).
- Confirm FPM restart re-applies the socket ACL (restart `jabali-fpm@<u>`, curl
  the site again).
- Confirm a fresh `install.sh` run + a freshly-created tenant get the new model
  (no www-data membership, ACLs present) — `jabali repair`-style reconcile for
  existing tenants.
- Only after all green on the test box, stage to one real host with operator
  sign-off.

## Open questions

- ACL persistence across reboots for `/run` sockets (tmpfs) — the ExecStartPost/
  .path reapply covers it, but verify after a full reboot.
- Backup/restore + `account_full` must preserve/reapply ACLs (manifest stages).
- Interim option if the redesign slips: tighten known-sensitive files
  (`wp-config.php`, `.env`) to owner-only `0600` — PHP-FPM reads them as the
  owner, nginx never does — shrinking the highest-value disclosure without the
  group change. Partial; does not close #430.

---

## RESOLVED — minimal fix shipped (`181970b3`, 2026-06-25)

The 2-tenant experiment collapsed the redesign. The full per-path docroot-ACL
machinery (step 4) is **NOT needed**:

- Docroots are `2750` setgid `<user>:www-data`. Once a tenant leaves www-data,
  `other`=0 blocks them from even *traversing* another tenant's docroot →
  cross-read closes for free. The setgid bit forces tenant-created files to group
  www-data regardless of the creator's membership, so nginx reads docroot static
  via the group with no ACLs (avoids the chmod-mask treadmill).
- The only break is the FPM socket. Fix: pool `listen.group={{.User}}` + a root
  `fpm-post-start` ExecStartPost that `setfacl -m u:www-data:rw`s the socket
  (reapplied per restart; `acl` pkg added). Tenants dropped from www-data
  (`user_create` no `--groups`; `user_slice_ensure` `gpasswd -d` self-heals).

Rejected mechanisms (proven on the box): setgid socket-dir + drop `listen.group`
→ FPM re-chgrps the socket to the master's egid (`<user>`), nginx can't reach it.
The socket ACL is the working path.

**Validated** (2 tenants, 10.0.3.14): tenant not in www-data; PHP 200 + static
200 via the ACL'd socket; cross-read BLOCKED; ACL survives FPM restart.

**Remaining: gated production rollout** (operator sign-off). Reconcile self-heals
existing tenants; they pick up the new socket group + ACL on their next FPM
restart. The 0600 interim (`7cb5417c`) stays as defense-in-depth.
