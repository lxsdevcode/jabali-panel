# ADR-0125: M50 Directory Privacy — per-subdirectory HTTP Basic Auth, DB-as-truth, fail-closed

**Date**: 2026-05-30
**Status**: Accepted
**Deciders**: shuki + Claude
**Related**: ADR-0002 (DB is source of truth), ADR-0004 (reconciler-driven convergence), ADR-0009 (nginx file-per-vhost), ADR-0021 (M7 reveal-once password lifecycle)

> Written retroactively (2026-06-14) to document a shipped feature that
> landed without an ADR. Reflects the implementation merged 2026-05-30
> (PR #107 `feat(m50)`, PR #114 `fix(dir-privacy): path == /`), not a
> forward-looking proposal.

## Context

cPanel's "Directory Privacy" lets a user password-protect a
subdirectory of their docroot with HTTP Basic Auth. jabali had no
equivalent: the only way to gate a path was to hand-edit nginx, which a
tenant can't do and which the reconciler would overwrite on the next
vhost regen (ADR-0009).

The feature is small but touches three boundaries — DB schema, the
panel↔agent wire, and the nginx vhost template — and makes one
security-sensitive choice (what an empty-credential rule does). Those
warrant a recorded decision.

Questions that shaped it:

1. **State of truth** — htpasswd files on disk, or rows in the panel DB?
2. **Password storage** — reversible (to re-render htpasswd) or one-way?
3. **Empty-credential rule** — allow-all (no-op), or deny-all?
4. **Scope + ownership** — per-domain, per-path; who may edit?

## Decision

**Rules + credentials live in the DB; htpasswd files are derivative.**
Two tables (migration `000148`):

- `domain_directory_privacy` — one row per `(domain_id, path)` rule:
  `id` ULID, `path`, `realm` (default `Restricted`), timestamps. Unique
  on `(domain_id, path)`; FK to `domains` `ON DELETE CASCADE`.
- `domain_directory_privacy_credentials` — N rows per rule: `username`,
  `password_hash`. Unique on `(rule_id, username)`; FK to the rule
  `ON DELETE CASCADE`.

The agent renders one htpasswd file per rule under
`/etc/jabali-panel/dir-privacy/<rule_ulid>.htpasswd` (`0640
root:www-data`, **outside** the docroot) and the vhost template emits one
`location ^~ <path>/ { auth_basic "<realm>"; auth_basic_user_file <file>; }`
block per rule. The reconciler injects the rules during vhost
convergence (`Reconciler.WithDomainDirectoryPrivacy`); the agent
`syncDirectoryPrivacyFiles` writes current files and prunes orphans
(rule deleted on the panel, file still on disk). This keeps the feature
inside the existing DB-as-truth + reconciler model (ADR-0002/0004/0009):
delete the domain → cascade drops rules+creds → next convergence removes
the location block and the orphaned htpasswd file.

**Passwords are bcrypt-hashed at the API boundary and never read back.**
`auth.HashPassword` runs in the handler; only the `$2[abxy]$` hash
crosses the wire to the agent and lands in the htpasswd file. Plaintext
never touches the wire, agent logs, or the DB — the same reveal-once
posture as M7 database users (ADR-0021). List/GET responses never echo
the hash.

**An empty-credential rule is fail-closed.** A rule with zero
credentials makes the agent write a placeholder htpasswd that no
password can match, so the location returns `401` — "take this directory
offline" is deny-by-default, never accidentally allow-all.

**Scope is per-domain, per-path; the resource is owned, not
admin-only.** Routes mount under
`/api/v1/domains/:id/directory-privacy[/:rule_id[/credentials[/:cred_id]]]`,
inheriting the same domain-ownership semantics as every other
per-domain resource — the domain's owner and admins manage its rules.
A rule whose `path` is `/` protects the whole docroot (PR #114).

Inputs are re-validated at the agent boundary: path
(`^/[A-Za-z0-9_./-]+$`), username (`^[A-Za-z0-9._-]{1,64}$`), bcrypt
prefix, and rule ULID are all re-checked before any file write.

## Alternatives Considered

### Alternative 1: tenant-managed `.htaccess`-style files
- **Pros**: zero panel surface; the apache-era mental model.
- **Cons**: nginx has no `.htaccess`; a tenant file in the docroot would
  be clobbered by reconciler vhost regen (ADR-0009) and gives the tenant
  a write path into server config.
- **Why not**: incompatible with nginx + DB-as-truth.

### Alternative 2: store credentials reversibly to re-render htpasswd
- **Pros**: could regenerate htpasswd from a recoverable secret.
- **Cons**: htpasswd *is* the rendered artifact — it only ever needs the
  one-way hash. Reversible storage is a needless plaintext-at-rest risk.
- **Why not**: bcrypt one-way + reveal-once is strictly safer and loses
  nothing.

### Alternative 3: empty-credential rule = allow-all / no-op
- **Pros**: marginally simpler (no placeholder file).
- **Cons**: a rule the operator created to *restrict* a directory would
  silently leave it open — a fail-open security hole.
- **Why not**: security defaults are deny-by-default. The placeholder
  htpasswd is a few lines.

## Consequences

### Positive
- **No drift.** Rules/creds in the DB, files rendered by the agent,
  cascade-deleted with the domain. The reconciler is the only writer of
  the location blocks and htpasswd files.
- **Small blast radius.** One migration, one model+repo, one handler,
  one agent command, one vhost-template addition, one UI section.
- **Secrets contained.** Plaintext never leaves the handler; hashes are
  `0640 root:www-data` outside the docroot; nothing is logged.
- **Fail-closed by construction.** The worst case (empty rule) is a
  `401`, not an open directory.

### Negative
- **htpasswd is per-rule, not per-credential.** Editing one credential
  rewrites the whole rule's file. Fine at realistic credential counts.
- **No per-rule realm uniqueness enforcement** beyond the `(domain,
  path)` key — two paths may share a realm string (cosmetic only).
- **Basic Auth only.** No digest, no IP-allowlist combinator in this
  iteration (per-domain IP ACLs are M36, a separate concern).

## Implementation

- **Migration**: `000148_domain_directory_privacy.{up,down}.sql`.
- **Model/repo**: `panel-api/internal/models/domain_directory_privacy.go`,
  `panel-api/internal/repository/domain_directory_privacy_repository.go`.
- **Handler**: `panel-api/internal/api/domain_directory_privacy.go`
  (bcrypt at boundary; mounted in `app.go` via
  `RegisterDomainDirectoryPrivacyRoutes`).
- **Reconciler**: `Reconciler.WithDomainDirectoryPrivacy` +
  location-block injection in vhost convergence.
- **Agent**: `panel-agent/internal/commands/dirprivacy.go`
  (`syncDirectoryPrivacyFiles`, deny-by-default placeholder;
  `dirprivacy_test.go` covers `TestWriteHtpasswd_EmptyCreds_DenyByDefault`).
- **UI**: `panel-ui/src/shells/admin/domains/DomainDirectoryPrivacySection.tsx`
  in the domain edit **Security** tab + `DomainDirectoryPrivacyModal`.
- **Commits**: `398576d2` / PR #107 `feat(m50)`; `5923a6a9` / PR #114
  `fix(dir-privacy): protect whole docroot when path == /`.
- **User docs**: `docs/site/admin/directory-privacy.md`,
  `docs/site/user/directory-privacy.md`.
