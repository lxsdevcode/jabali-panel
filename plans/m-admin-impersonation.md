# Blueprint — Admin act-as impersonation (GH #183, ADR-0128)

**Branch:** `feat/admin-impersonation`. **Merge only after thorough tests + live verify.**

Effective-user override (authorization), not session swap. See ADR-0128 for the
security model. Build backend-first, each step tested, then frontend, then live.

## Backend

1. **Migration `000167_impersonation_grants`.** `id CHAR(26) PK`,
   `admin_user_id CHAR(26)`, `target_user_id CHAR(26)`, `created_at`,
   `expires_at`, `ended_at NULL`. FKs → `users(id)` ON DELETE CASCADE. Indexes
   on `expires_at`, `admin_user_id`. Schema-only (no seed).

2. **Model + repo.** `models.ImpersonationGrant`;
   `repository.ImpersonationGrantRepository`: `Create`, `FindActive(id)` (not
   expired, `ended_at IS NULL`), `End(id)`, `ListActiveByAdmin`, `ReapExpired`.
   sqlmock test for SQL shape.

3. **Claims field.** Add `ImpersonatedBy string` to `auth.AccessClaims` (the
   real admin while acting-as; empty otherwise).

4. **Override middleware `ResolveImpersonation`.** Mounted after
   `RequireKratosSession` on the v1 group. No `X-Jabali-Act-As` header → no-op.
   Else: require real claims `IsAdmin`; load grant; require
   `grant.admin_user_id == realClaims.UserID` + active + target exists +
   target **non-admin**. On pass → override claims (`UserID=target`,
   `IsAdmin=false`, `ImpersonatedBy=admin`). On header-present-but-invalid →
   **403 `impersonation_invalid`** (SPA drops back to admin; never silently act
   as admin). Unit-test every branch (forged header by non-admin, wrong admin,
   expired, ended, admin target, valid).

5. **Endpoints `internal/api/impersonation.go`** (RequireAdmin):
   `POST /admin/impersonation {user_id}` → validate target exists + non-admin +
   not self → create grant (TTL e.g. 60m) → return `{id, target, expires_at}`;
   `DELETE /admin/impersonation/:id` → end (only own grant);
   `GET /admin/impersonation` → list own active grants. Mount in app.go.

6. **Audit accountability.** M49 `AuditRecord`: actor = `ImpersonatedBy` when
   set (real admin), else `UserID`; add `impersonated`/`acting_as` to meta so
   the target is never logged as actor. Typed events on grant create/end
   (M49 reserved these). Test that an act-as request audits the admin, not the
   target.

7. **Reaper.** Expire stale grants (a ticker like the SSO reaper, or fold into
   an existing reconciler tick). Low priority — `FindActive` already enforces
   `expires_at` at read time; reaper is cleanup only.

## Frontend

8. **apiClient interceptor.** When `sessionStorage.actAsGrant` is set, send
   `X-Jabali-Act-As`. On 403 `impersonation_invalid` → clear + toast + route to
   admin.

9. **Initiate + banner.** Admin user list → "View as user" → POST grant → store
   id → route to `/jabali-panel`. Persistent "Viewing as <email> — Exit" banner
   (Exit = DELETE grant + clear + back to admin). Allow the admin to render the
   user shell while a grant is active (route guard exception).

## Verification (gate before merge)

- Unit: middleware branches, repo SQL, audit-actor-is-admin, endpoint validation.
- Integration (real MariaDB): create grant → user-scoped request with header
  returns target's data; admin endpoint 403s under act-as; expired grant 403s.
- E2E/live on 10.0.3.14: admin "view as user", see the user's domains/mailboxes,
  make a change, confirm audit logs the admin as actor with acting_as=target,
  exit returns to admin — all in ONE browser, admin session intact.
- Security re-check: forged header as non-admin (ignored), grant for a different
  admin (ignored), act-as an admin (refused), confined to the one target.
