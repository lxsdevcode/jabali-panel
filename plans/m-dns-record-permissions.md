# Blueprint: Admin-controlled DNS record-type permissions for tenants (Gitea #466)

**Status:** Draft for review (2026-06-27)
**ADR:** ADR-0150 (draft, in this change)
**Next free migration:** `000197`

## Problem

Admins need a Server Settings control defining which DNS record *types* regular
(non-admin) users may create / edit / delete, so provider-owned records
(delegation, mail routing, mail-security TXT, CAA issuance policy) stay locked
even though tenants manage their own zones. Today any zone owner can CRUD any
supported type (`A AAAA CNAME MX TXT NS SRV CAA`); only `Managed` SOA/NS records
are protected (`dns.go:455,575`).

## Current shape (grounding)

- `panel-api/internal/api/dns.go` — `dnsHandler.{createRecord,updateRecord,deleteRecord}`
  already branch on `claims.IsAdmin`; `isValidDNSType` (`dns.go:599`) whitelists
  types; `validateDNSRecord` validates content per type.
- `Managed`/`ManagedBy` records (SOA, NS, bootstrap A/MX) are panel-owned and
  already refused to non-admins for SOA/NS.
- Settings are a **singleton row** (`models.ServerSettings`, `id=1`), read via
  the server-settings GET and written via the admin PUT (`server_settings.go`).
- UI: admin `panel-ui/src/shells/admin/settings/ServerSettingsPage.tsx`; user DNS
  UI under `panel-ui/src/shells/user/...` (zone/records).

## Decision summary (see ADR-0150)

1. **Storage = one JSON column on `server_settings`** (`dns_user_record_policy`),
   not a new per-type table. Settings is already a singleton; a JSON policy map
   keeps the per-type × per-op matrix in one place with one read, matches the
   existing settings pattern, and avoids a join on every DNS mutation (the
   policy is read from the already-cached settings row).
2. **Policy shape**: `{ "<TYPE>": {"create":bool,"edit":bool,"delete":bool}, ... }`
   for the user-manageable types `A AAAA CNAME MX TXT SRV CAA`. `NS`/`SOA` are
   **always admin-only** (not in the matrix) — they remain `Managed` and the
   existing guard stays; the matrix never grants them.
3. **Default = hosting-safe** (applied by migration + on any missing key):
   `A AAAA CNAME TXT MX SRV` → full CRUD; `CAA` → full CRUD. (Admins tighten via
   presets.) Missing/unknown type key ⇒ treated as **denied** for non-admins
   (fail-closed), so adding a new type later defaults to locked until policy is
   set.
4. **Presets** (UI convenience, expand to the matrix client-side, stored as the
   resolved matrix): `permissive` (all CRUD), `hosting-safe` (default),
   `locked-down` (`A AAAA CNAME` CRUD only; everything else denied).
5. **Enforcement is server-side and authoritative**; UI hiding is cosmetic.

## Enforcement points (all in `dns.go`, non-admin only)

- `createRecord`: deny if `!policy[type].create`.
- `updateRecord`: this can change the record's **type**. Require
  `policy[oldType].edit` AND, if the type changes, `policy[newType].create`.
  Keep the existing `Managed && (SOA|NS)` guard.
- `deleteRecord`: deny if `!policy[type].delete`. Keep the Managed guard.
- Audit any **bulk/zone-import** path (`updateZone`, DDNS) for the same matrix —
  DDNS (`ddns.go`) writes A/AAAA; gate it by the `A`/`AAAA` create+edit bits for
  non-admin callers, or document it as an admin/token path exempt.
- Admins bypass the matrix entirely (`claims.IsAdmin`).

Helper: `func (p DNSUserRecordPolicy) Allows(recordType, op string) bool` with
fail-closed default; unit-tested in isolation.

## Wire / API

- Server-settings GET/PUT carries `dns_user_record_policy` (admin only; the
  field is part of the existing settings envelope).
- **User-facing effective policy**: add `GET /api/v1/dns/policy` (auth, any user)
  returning the resolved matrix so the tenant DNS UI can disable/hide the
  create/edit/delete affordances per type. (Admins get an all-true view.)

## Waves

- **Wave A (backend, security-critical):** migration `000197` (JSON column,
  default hosting-safe) + `models` field + `DNSUserRecordPolicy` type with
  `Allows()` + defaults/preset resolution + enforcement in the three handlers +
  DDNS audit + `GET /dns/policy` + settings GET/PUT plumbing + unit tests
  (matrix allow/deny, type-change-on-update, fail-closed default, admin bypass).
  **This wave alone closes the vulnerability**; B/C are UX.
- **Wave B (admin UI):** ServerSettingsPage "DNS Permissions" card — a table of
  types × {create,edit,delete} checkboxes + the three preset buttons; PUTs the
  resolved matrix.
- **Wave C (user UI):** tenant DNS page reads `GET /dns/policy` and
  disables/hides the disallowed actions (with a tooltip "restricted by
  administrator"); server still enforces.
- **Wave D:** Playwright E2E (admin sets locked-down → tenant cannot add MX) +
  runbook note + ADR flip to Accepted.

## Edge cases / risks

- **Type change on edit** is the easy miss — covered above (needs new-type
  create right).
- **Fail-closed** on unknown/missing type keys: deny for non-admins so a future
  new supported type isn't silently world-writable.
- **DDNS** must not become an accidental bypass — explicitly decided in Wave A.
- **Existing records** that violate a newly-tightened policy are left in place
  (policy gates *mutations*, not existing state); admins clean up manually.
- Migration is **schema + default only** (no seeding from app-populated tables),
  per the migration-data-seed rule.

## Out of scope

- Per-user / per-package policies (this is server-wide, matching the issue).
- NS/SOA delegation editing for tenants (stays admin-only).
