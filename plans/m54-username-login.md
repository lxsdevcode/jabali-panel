# M54 — Username-only login (hard cutover)

**Status:** Blueprint — Wave 0 spike DONE, design locked. Hard cutover (duplicate emails ⇒ plain email ⇒ NO self-service password recovery; admin-set password is the only reset) **re-confirmed by operator 2026-06-10**. Renumbered M44→**M54** (M44 = Automation API tokens), migration → **000164**, ADR → **0119**. Wave A (staged schema + backfill/relabel CLIs, dry-run) in progress; cutover Waves B/C/D deferred for live Kratos validation on the test host.
**Target ADR:** 0119  (blueprint said 0118 — taken by M53)
**Supersedes auth assumption in:** ADR-0034 (M20 Kratos — "email is the login identifier")

## Why

Email is currently the Kratos password identifier, so it must be globally
unique — two accounts can't share an email address. Operators want to
let multiple accounts (e.g. a reseller's sub-accounts, a family, a
company with one shared inbox) use the same contact email. Kratos
requires **every** credential identifier to be unique, so the only model
that allows duplicate emails is **username-only login**: username becomes
the unique login identifier; email is demoted to a non-identifier contact
attribute that can repeat.

Decision (operator, 2026-06-07): **hard cutover** — every identity logs in
by username, email stops being a login identifier entirely. No dual
email+username login period.

## Constraints / facts established during scoping

- Kratos: a trait flagged `ory.sh/kratos.credentials.password.identifier:
  true` is the login identifier AND is enforced unique. A trait can carry
  `recovery.via: email` / `verification.via: email` WITHOUT being an
  identifier — so email keeps delivering recovery codes while no longer
  being unique.
- Panel DB today: `users.email` is `UNIQUE` (ux_users_email, mig 000001);
  `users.username` already has `ux_users_username` (mig 000007) but is
  **nullable** (admins may have no OS account / no username).
- Login UI (`panel-ui/src/pages/Login.tsx`) renders Kratos flow fields
  dynamically (`f.kind`), so it shows whatever identifier field the schema
  declares — the UI change is mostly label/help text, not new form logic.
- Webmail SSO (`webmail_sso.go`) keys off the mailbox + the panel session,
  not the Kratos email identifier — low ripple.

## Design

### Identity schema (`install/kratos-identity-schema.json`)

- `username`:
  - add `ory.sh/kratos.credentials.password.identifier: true`
  - move into `required`
  - keep `pattern ^[a-z_][a-z0-9_-]*$`, `maxLength 32`
  - this is now THE login identifier (unique, enforced by Kratos)
  - **decouple from OS account**: a username as a login id does NOT imply a
    Linux user. Admins get a login username with no `/home/<u>` / no FPM
    pool. The reconciler's OS-account provisioning stays gated on
    `is_admin == false` (unchanged) — it just no longer also gates whether
    the identity HAS a username.
- `email` (FINAL — decided by Wave 0 spike + operator, 2026-06-07):
  - REMOVE `credentials.password.identifier: true`
  - REMOVE `verification.via` AND `recovery.via` — the spike proved ANY
    of these forces email uniqueness, which blocks the dup-email goal.
    Email becomes a **plain contact string**: required, free-form,
    NON-unique, no Kratos verification/recovery binding.
  - panel DB drops its UNIQUE on email too.
  - Consequence (accepted): no self-service password recovery; see
    "Password reset" below.
- `is_admin`: unchanged.

### Panel DB (migration 000164 — next free on main 2026-06-10)

```sql
-- allow duplicate contact emails
ALTER TABLE users DROP INDEX ux_users_email;
-- username is now the login identifier: required + unique.
-- (ux_users_username already exists; just enforce NOT NULL after backfill)
-- Backfill happens in the app/CLI BEFORE this runs (migrations = schema
-- only, per feedback_migration_data_seed_ordering). So:
--   1. app-level backfill assigns a username to every NULL-username user
--   2. THEN this migration sets NOT NULL
ALTER TABLE users MODIFY username VARCHAR(32) NOT NULL;
```

Ordering trap (ADR ref `feedback_migration_data_seed_ordering`): the
NOT NULL flip must run AFTER the backfill. Two options:
- (a) backfill in a `jabali users backfill-usernames` CLI run as an
  explicit cutover step BEFORE `jabali update` applies the NOT NULL
  migration; OR
- (b) split into two migrations across two releases.
**Choose (a)** — single release, explicit operator step in the runbook,
matches how M24/M25 cutovers were sequenced.

### Username backfill strategy

For every user with NULL/empty username (only admins, in practice):
- derive candidate from email local-part, lowercased, non-matching chars
  → `_`, trimmed to 32, ensured to start `[a-z_]`.
- on collision, append `-2`, `-3`, … until unique.
- record old→new in the cutover report so the operator can tell each
  admin their new login name.

### Kratos identity re-key (the load-bearing migration)

Every existing identity currently has `credentials.password.identifiers =
[<email>]`. After the schema change, identifiers must become
`[<username>]`. Kratos recomputes identifiers from traits when an identity
is updated under the active schema. So:

1. Deploy the new schema (Kratos reload).
2. Ensure every identity's `traits.username` is set (backfill via Admin
   API PUT for any missing — derive same as DB backfill, keep in sync with
   the panel `users.username`).
3. PUT each identity (`PATCH`/`PUT /admin/identities/{id}` with full traits)
   so Kratos re-derives `credentials.password.identifiers` from the new
   schema → username becomes the identifier, email drops out.
4. Password hashes are preserved across a traits-only update (we do NOT
   touch `credentials.password.config`), so users keep their passwords —
   only the identifier they type changes.

New CLI: `jabali kratos relabel-identifiers` (mirrors the existing
`jabali kratos rebuild` shape in `kratos_rebuild_cmd.go`):
- idempotent: skip identities already keyed on username.
- dry-run default; `--apply` to mutate.
- emits old-email → username map.

**Risk:** if an identity has no username trait AND can't be derived (no
email either — shouldn't happen, email is required), skip + report. Never
leave an identity with zero identifiers (would lock it out).

### Password reset (replaces self-service recovery — REQUIRED deliverable)

The plain-email schema has no recovery address, so Kratos self-service
recovery AND the admin recovery-link are both unavailable (spike-proven
404). The ONLY working reset is the admin directly setting a new
password via the admin API:

- `POST /admin/users/:id/password/reset` (admin-only) → panel-api calls
  Kratos `PUT /admin/identities/{id}` updating `credentials.password.
  config` with a freshly-generated strong temp password (bcrypt/argon
  per Kratos), returns the plaintext ONCE (reveal-once, same shape as
  the DB-user + mailbox reveal-once flows).
- UI: `UserResetPasswordAction` in the admin Users row dropdown, sibling
  of the existing `UserReset2FAAction`. Confirm modal → reveals the new
  password once → operator hands it to the user.
- The login page drops the "Forgot password?" link entirely (no self-
  service recovery exists). Replace with copy: "Lost your password?
  Contact your administrator."
- `kratos_rebuild_cmd.go` / `migrate_run_cmd.go`: their
  `CreateRecoveryCode`/recovery-link calls break under the plain schema
  — replace with the same admin password-set (return temp password to
  the operator running the migrate/rebuild CLI). Audit all
  CreateRecovery* callers.
- Break-glass for the last admin: the migrate/rebuild CLI (root on the
  box) can always admin-set any identity's password, so an operator with
  shell access is never locked out.

### REST / userops

- `users.go` create: `username` becomes **required** (drop `omitempty`);
  validate pattern + uniqueness. `email` validated as email format but NOT
  uniqueness-checked at the panel layer (dups allowed).
- update: allow editing email to a non-unique value; username edits must
  stay unique + must propagate to the Kratos identity trait (re-key).
- `userops` create path: pass username into Kratos identity traits as the
  identifier; ensure OS-account provisioning stays `!is_admin`-gated.

### UI

- `Login.tsx`: the Kratos flow now declares the identifier field as
  username — render label "Username" + helper "your account username (not
  your email)". Mostly copy; the field is already flow-driven.
- Recovery flow page: initiate by **username** (Kratos recovery accepts the
  identifier). Code still arrives by email. Copy update.
- User create/edit drawer (`panel-ui/src/shells/admin/users/`): username
  required + always shown (not just for non-admins); email field loses its
  "must be unique" inline validation; add helper "email may be shared
  across accounts".
- Account/profile page: show username as the login name.

### Ripple checks (verify, fix only if broken)

- `webmail_sso.go` — keys off mailbox + session; expected no change.
- phpMyAdmin SSO / Adminer SSO — session-based; verify.
- Notifications / audit log that display "user email" as an identity label
  → switch to username where it's used as an identity key, keep email where
  it's genuinely the contact address.
- Any `FindByEmail` used as a UNIQUE lookup (login-adjacent) → must become
  `FindByUsername`. Grep `FindByEmail` callers; email is no longer a key.

## Wave 0 — CRITICAL spike (DONE 2026-06-07 — see SPIKE RESULT below)

Kratos may enforce uniqueness on **verifiable** addresses
(`verification.via: email`) and possibly on **recovery** addresses
independently of credential identifiers. If so, demoting email from
identifier is not enough — two identities still can't share an email
that carries `verification.via`/`recovery.via`, and the feature is
blocked at CREATION, not just recovery. This reshapes the email-trait
design, so verify empirically FIRST (isolated throwaway Kratos, sqlite,
alt ports — never the live identity store):

1. Schema variant A: email = `verification.via` + `recovery.via` (not
   identifier), username = identifier. Create two identities, same
   email, different usernames. Does the 2nd succeed?
2. Variant B: email = `recovery.via` only.
3. Variant C: email = plain (no verification/recovery).

The lowest variant that ALLOWS the duplicate is the email-trait design
we ship.

**SPIKE RESULT (Kratos v26.2.0, isolated MySQL, 2026-06-07 — PROVEN):**
| email trait                          | 2nd identity, same email |
|--------------------------------------|--------------------------|
| verification.via + recovery.via      | REJECTED                 |
| recovery.via only                    | REJECTED                 |
| verification.via only                | REJECTED                 |
| PLAIN (no verification, no recovery) | **CREATED**              |

So duplicate emails require email to be a **plain trait** — ANY
verification/recovery binding forces uniqueness. Additional proven
facts under the plain-email schema:
- login by username → OK (session minted).
- login by email → REJECTED (email is not an identifier). Correct.
- admin recovery-LINK (`POST /admin/recovery/link`) → 404 / fails: a
  plain email has no recovery address, so no link can be generated.

**Therefore (hard, non-negotiable Kratos constraint):**
allowing duplicate emails ⇒ email is plain ⇒ **no self-service password
recovery for ANY user**, and the admin recovery-link path is unavailable
too. The only password-reset mechanism that works is **admin directly
sets a new password** via `PUT /admin/identities/{id}` (credentials.
password.config) and hands the temp password to the user (reveal-once,
mirrors the existing reveal-once flows). This replaces the recovery-code
minting in `kratos_rebuild_cmd.go` / `migrate_run_cmd.go`, which also
relies on a recovery address and would break under the plain schema.

Consequence to pre-decide based on the spike:
- If only the PLAIN variant allows dup email → email becomes a plain,
  non-verified, non-recoverable contact string. That removes self-
  service email recovery for everyone. The replacement is
  **admin-initiated password reset**: a sibling of the existing
  `UserReset2FAAction` (admin Users row → "Reset password" → mints a
  Kratos recovery/temp-credential for that identity). This becomes a
  REQUIRED deliverable, not optional.
- If the recovery-only or verif+recovery variant allows dup email →
  keep self-service recovery; recovery is initiated by the username
  identifier (confirm the recovery UI accepts the identifier, not a
  bare email-address lookup).

Also confirm in the spike:
- Username identifier **case normalization**: email identifiers are
  auto-lowercased by Kratos; a custom `username` identifier may NOT be.
  If not normalized, the backfill + uniqueness logic must lowercase (or
  the schema/validator must force lowercase) so `Alice` and `alice`
  don't become two logins. The schema pattern already forces lowercase
  (`^[a-z_]...`) — verify Kratos honours it as the identifier key.

## Waves

- **Wave A (additive prep, non-breaking):** schema file gains username
  identifier flag BUT not deployed to live yet; add `relabel-identifiers`
  + `backfill-usernames` CLIs (dry-run only); panel create-user starts
  requiring username for NEW users. Mergeable without breaking live login.
- **Wave B (DB + cutover migration):** drop email-unique, backfill CLI,
  NOT NULL migration, ordering per the runbook.
- **Wave C (Kratos cutover):** deploy new schema, run relabel-identifiers
  --apply, verify login-by-username on a test identity.
- **Wave D (UI):** login/recovery/user-form copy + behaviour.
- **Wave E (ripple + runbook + ADR-0118 + live validation):** FindByEmail
  audit, SSO smoke, the operator runbook (backfill → schema deploy →
  relabel → announce new usernames), live cutover on 192.168.100.150 then
  production boxes.

## Verification

- Unit: schema validates an identity with username+dup-email; rejects
  missing username. userops create requires username. relabel-identifiers
  is idempotent + dry-run safe.
- Live smoke (192.168.100.150):
  1. Two users, SAME email, different usernames → both create OK.
  2. Each logs in by username + password → success; login by email →
     rejected (no longer an identifier).
  3. Recovery initiated by username → code emailed → reset works.
  4. Existing pre-cutover user: relabel ran → logs in by their (backfilled)
     username with their OLD password.
  5. Admin with no OS account: has a login username, logs in, has no
     `/home/<u>`.

## Open questions for review

- (RESOLVED BY WAVE 0 SPIKE — do not re-litigate in review) Whether
  Kratos allows duplicate emails with verification/recovery present, and
  therefore whether self-service email recovery survives or is replaced
  by admin-initiated reset. The spike output is folded into the email-
  trait design + Wave 0 above before this doc goes to review.
- Username change post-cutover: editing a username re-keys the Kratos
  identifier. Confirm Kratos preserves the password on a traits-only
  identifier change (it does for config-untouched updates; verify on the
  test box).
- Do we want a grace alias (accept email OR username during a transition)?
  Operator chose HARD cutover → no. Documented here so it isn't re-litigated.
