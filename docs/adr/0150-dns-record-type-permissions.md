# ADR-0150: Admin-controlled DNS record-type permissions for tenants

**Status:** Draft (2026-06-27) — pending operator review before Wave A lands.
**Driven by:** Gitea #466.
**Blueprint:** `plans/m-dns-record-permissions.md`.

---

## Context

Tenants manage their own DNS zones. Any zone owner can currently create/edit/
delete any supported record type (`A AAAA CNAME MX TXT NS SRV CAA`); only
`Managed` SOA/NS records are protected. Hosting providers need to lock
provider-owned records — delegation (NS), mail routing (MX), mail-security TXT
(SPF/DKIM/DMARC), and certificate issuance policy (CAA) — while still letting
tenants manage ordinary website records. This is a standard cPanel/DirectAdmin
control.

## Decision

Add a **server-wide** policy controlling which record types non-admin users may
create / edit / delete, enforced authoritatively in `panel-api`.

1. **Storage**: a single JSON column `dns_user_record_policy` on the singleton
   `server_settings` row (migration `000197`), not a new per-type table. The
   policy is read with the already-loaded settings, so DNS mutations add no
   join; the matrix lives in one place that the admin settings PUT already
   manages.
2. **Model**: `{ "<TYPE>": {"create","edit","delete"} }` over the
   user-manageable types `A AAAA CNAME MX TXT SRV CAA`. `NS`/`SOA` stay
   admin-only and are never represented in the matrix (they remain `Managed`).
3. **Default** (set by migration and assumed for any missing key):
   **hosting-safe** — full CRUD on `A AAAA CNAME TXT MX SRV CAA`. A missing or
   unknown type key is treated as **denied** for non-admins (fail-closed) so a
   future new type is locked until policy is set.
4. **Presets** (UI): `permissive`, `hosting-safe`, `locked-down`
   (`A AAAA CNAME` only). Presets expand to and are stored as the resolved
   matrix.
5. **Enforcement**: `createRecord`/`updateRecord`/`deleteRecord` consult the
   matrix for non-admins; `updateRecord` that changes a record's type requires
   the *edit* right on the old type and the *create* right on the new type. The
   DDNS write path is gated on `A`/`AAAA` for non-admin callers. Admins bypass.
   UI hiding is cosmetic; the server is the gate.

## Alternatives considered

- **Per-type rows in a dedicated table** — rejected: heavier (join or extra
  query per mutation) for a singleton-settings concept; JSON column is simpler
  and matches the existing settings shape.
- **Per-user / per-package policy** — out of scope; the issue asks for a
  server-wide control. A future ADR can layer package-level overrides on top.
- **Default permissive** — rejected: the point of the feature is provider
  protection; a fail-open default would ship the vulnerability the issue is
  about.

## Consequences

- New admin control + a user-facing `GET /dns/policy` for the tenant UI to
  reflect the effective rights.
- Existing records that violate a newly-tightened policy are left intact; the
  policy gates mutations, not stored state.
- Tightly coupled to the `Managed` SOA/NS guard, which remains the mechanism for
  delegation/zone-metadata protection.
