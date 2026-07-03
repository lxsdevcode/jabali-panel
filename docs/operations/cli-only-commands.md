# CLI-only operator commands

From the GUI/CLI gap audit (2026-06-20). These `jabali …` subcommands have **no
GUI affordance** and run only from a root shell on the host. Most are deliberate
escape hatches — destructive, one-time migrations, or key-material operations
that don't belong behind a click. This file records the intent so the gap is
documented rather than mistaken for an oversight, and flags the few worth a GUI.

## Intentionally CLI-only (escape hatches / recovery / one-time)

| Command | File | Why CLI-only |
|---|---|---|
| `admin backfill-usernames` | `backfill_usernames_cmd.go` | One-time data backfill; idempotent but bulk-mutating. |
| `admin purge-orphan-identities` | `purge_orphan_identities_cmd.go` | Destructive Kratos identity GC; run after a manual cleanup. |
| `admin rebuild-kratos` | `kratos_rebuild_cmd.go` | Disaster recovery — reprovisions the identity store. |
| `admin relabel-identifiers` | `relabel_identifiers_cmd.go` | One-time identifier migration. |
| `admin slice-cutover` | `admin_slice_cutover.go` | FPM master→per-user-slice cutover; masks distro units. Run `--dry-run` first. |
| `sso rotate-key` | `sso_rotate_key.go` | Rotates SSO signing key material + reloads; needs careful rollback. |
| `sso prune-tokens` | `sso_rotate_key.go` | Maintenance GC; also timer-backed (`jabali-sso-reap`). |
| `migrate pull-source` | `migrate_pull_cmd.go` | Migration-wizard internal sub-step. |
| `migrate reap-secrets` | `migrate_reap_cmd.go` | Migration secret GC. |
| `domain prune-orphans` | `domain_orphan_prune_cmd.go` | Destructive orphan-vhost cleanup. |
| `pdns backfill` | `pdns_cmd.go` | One-time PowerDNS backfill (ADR-0047). |
| `appsec render-config` | `appsec_cmd.go` | Install-time config render; not an operator action. |
| `ufw migrate-ip-bans` | `ufw_cmd.go` | One-time M43 ban migration (ADR-0089). |
| `audit verify` / `audit prune` | audit CLI | GH #571: hash-chain integrity verification + retention pruning. Integrity verification is a forensic/operator action (a GUI button that says "chain valid" is a weak assurance a compromised panel could forge); retention pruning is destructive. The GUI browses/filters audit events; verification + pruning stay CLI-only by design. |
| `nspawn build` / `nspawn prune` | nspawn CLI | GH #574: sealed nspawn image build (minutes-long, disk-heavy) + prune (destructive). The GUI lists available images and points here to build; the build/prune lifecycle is an operator CLI op, not a per-request GUI action. |

> **GUI note:** these are operator escape hatches. The admin UI should *not*
> grow buttons for them; surface this doc from the admin "Support"/"Updates"
> area instead so an operator knows where the recovery levers live.

## GUI candidates (backend already exists, only a button is missing)

| Command | Agent verb (already shipped) | Proposed GUI |
|---|---|---|
| `domain email-dkim-rotate` | `domain.email_dkim_rotate` | **Best candidate.** DKIM rotation is a normal per-domain mail op; add a "Rotate DKIM" action on the domain's Mail/DNS area with a confirm + "republish DNS" hint. Low risk — the verb + reconciler already converge the new key. |
| `apparmor flip-mature` | `security.apparmor.set_mode` | Maybe — a "graduate complain→enforce" control in Security → AppArmor, behind a confirm. Risk: premature enforce can break a profile. |
| `per-user-egress flip-mature` | `user.egress.apply` | Maybe — same shape as apparmor; graduate the egress firewall from log to enforce. |
| `malware-purge` | `security.malware.quarantine.delete` | The quarantine list already has per-item delete in the UI; a bulk "purge all" is the only gap. Low value. |

Recommendation: build **`domain email-dkim-rotate`** first (clean, useful,
backend-complete). Treat the two `flip-mature` controls as a follow-up only if
operators ask — the maturity-flip is a deliberate, low-frequency decision.
