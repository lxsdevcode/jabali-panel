# ADR-0119 — M54 Username-only login (hard cutover)

**Status:** Accepted (2026-06-10)
**Supersedes assumption in:** ADR-0034 (M20 Kratos — "email is the login identifier")

## Context

Email was the Kratos password identifier, so it had to be globally unique — two
accounts could not share an inbox (resellers, families, one shared company
email). Kratos enforces uniqueness on every credential identifier, so the only
model that allows duplicate emails is **username-only login**.

Wave 0 spike (Kratos v26.2.0) proved: ANY `verification.via`/`recovery.via` on
the email trait forces uniqueness. Allowing duplicate emails therefore requires
email to be a **plain** trait — which removes Kratos self-service recovery for
everyone. Operator re-confirmed the trade 2026-06-10: accept no self-service
recovery; admin-set password (reveal-once) is the only reset.

## Decision

1. **Username is the unique login identifier.** v2 identity schema flags
   `traits.username` as `credentials.password.identifier` + required;
   `traits.email` is a plain, non-unique, non-verified/non-recoverable contact.
   `install/kratos-identity-schema.json` IS the v2 schema (fresh installs are
   username-login from first boot; the bootstrap admin gets a derived username).
2. **Panel DB (migration 000164):** backfill any NULL username (ULID fallback)
   → DROP `ux_users_email` → `username NOT NULL`. Self-contained (users table
   only), so the backfill runs safely inside the migration; operator runs
   `backfill-usernames --apply` first for friendly names.
3. **Re-key, proven not assumed:** a `PATCH /admin/identities/{id}` on
   `/traits/username` re-derives `credentials.password.identifiers`
   email→username on existing identities, password hash preserved (spike +
   live box validation 2026-06-10). `jabali admin relabel-identifiers` does this
   for every identity; skips any with no resolvable username (never relabels to
   empty = never locks out).
4. **No self-service recovery** (Kratos constraint of plain email). The only
   reset is `POST /admin/users/:id/password/reset` — admin-set temp password,
   reveal-once. The DR rebuild CLI emits a temp password instead of a recovery
   link.
5. **Existing installs cut over via runbook** (`plans/m54-username-login-runbook.md`):
   migration auto-applies on update; the Kratos schema swap + relabel are a
   deliberate operator step (no auto-swap that would lock users out pre-relabel).

## Consequences

- Duplicate emails across accounts are allowed; users log in by username.
- No self-service password recovery; admins reset (reveal-once). Accepted.
- Fresh installs are username-login natively; existing installs need the one-time
  runbook cutover (validated end-to-end on 10.0.3.14: re-key, migration, dup-email
  create 201, admin healthy).

Numbering: blueprint authored as M44/ADR-0118/migration-after-000161 — all stale
(M44 = Automation API, ADR-0118 = M53). Corrected to M54 / ADR-0119 / 000164.
