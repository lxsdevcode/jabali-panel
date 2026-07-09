# JAB-62 — Per-install Redis ACL isolation for wp-cache

**Objective:** one compromised WordPress install must not be able to read, write,
scan, or unlink a **sibling** install's Redis object-cache keys, even under the
same Jabali (OS) user.

**Priority:** high (security). **Plane:** JAB-62.
**Target branch base:** `main` (all prior security work merged; tip was `ed84c16a`).

---

## Background (cold-start context)

The bundled `jabali-cache` object cache stores keys as
`jc:<osuser>:<installID>:<key>` — **already unique per install**. But the Redis
ACL that gates access is **per-OS-user**, not per-install:

- ACL user: `wp_<osuser>`
- Key fence: `~jc:<osuser>:*` — matches **every** install of that OS user
- Token: `cacheTenantToken(secret, osUser, salt)` — one token per OS user, stamped
  into every install's `wp-config.php`

So install A's stamped credential can `SCAN`/`GET`/`SET`/`UNLINK` install B's keys
(same OS user). That leaks cached user/options/transient data between a tenant's
client sites and lets one compromised site poison/evict siblings.

**Fix:** make the ACL user, fence, and token all **per-install**. The keys don't
move (they're already per-install), so this is purely tightening the credential +
fence to `…<installID>…`.

### Exact code map (verified this session)

| Thing | Location | Current | Target |
|---|---|---|---|
| Token derivation | `panel-api/internal/api/applications_cache.go:44` `cacheTenantToken(secret, osUser, salt)` | HMAC over `wp-cache-tenant:<osUser>:<salt>` | include installID |
| ACL provision | `applications_cache.go:420` `provisionTenantACL(ctx, osUser, token)` | user `wp_<osUser>`, fence `~jc:<osUser>:*` | user `wp_<osUser>_<installID>`, fence `~jc:<osUser>:<installID>:*` |
| ACL revoke | `applications_cache.go:66` `revokeTenantRedisACL(ctx, rdb, osUser)` | `DELUSER wp_<osUser>` | per-install `DELUSER wp_<osUser>_<installID>` |
| Enable path A | `applications_cache.go` `setCacheCore` (~line 137, prefix at ~190, provision at ~207) | provisions per-user | per-install |
| Enable path B | `applications_cache.go:372` `enableObjectCache` (prefix ~391, token ~401, provision ~402) | provisions per-user | per-install |
| Disable revoke lifecycle | `applications_cache.go:324-336` — revoke only when `CountCacheEnabledByUserID==0` | shared-user coordination | revoke this install's user unconditionally |
| User-delete revoke | `panel-api/internal/api/users.go:742` `revokeTenantRedisACL(rctx, Redis, username)` | one shared user | revoke every per-install user for that user |
| Agent username | `panel-agent/internal/commands/wordpress_cache.go:129` `setWPConfigCacheConstants(… "wp_"+p.OSUser …)` | `wp_<osuser>` | derive from `p.Prefix` → `wp_<osuser>_<installID>` |
| Cache doctor | `panel-api/cmd/server/app_cache_doctor_cmd.go` — `--repair` re-runs the exact enable path (re-stamp + re-provision) | already re-provisions | add orphan-reap of legacy shared users |

**Key leverage:** the doctor's `--repair` already sweeps every cache-enabled
install and re-runs the enable path. Once the enable path is per-install (Steps
1–2), `--repair` auto-migrates every install to its per-install ACL user *for
free*. The only new doctor work is reaping the now-orphaned legacy `wp_<osuser>`
users (Step 3).

### Invariants (verify after EVERY step)

- `go build ./panel-api/... ./panel-agent/...` clean.
- Agent-derived username **exactly equals** panel-provisioned username for the
  same `(osUser, installID)` — otherwise the plugin can't auth. Both derive
  `wp_<osuser>_<installID>`; the agent must build it from `p.Prefix`
  (`<osuser>:<installID>`), which never contains extra `:` (osUser = linux name,
  installID = ULID). Guard with a test.
- Fence pattern is `~jc:<osUser>:<installID>:*` — NOT `~jc:<osUser>:*`. A stray
  broad fence reopens the hole. Assert in the isolation test.
- Re-provision ALWAYS re-stamps `wp-config.php` (agent `cache_set`) in the same
  operation, because a new per-install token invalidates the old stamped one. The
  enable path already does both; never split them.
- No plaintext token logged.

---

## Step 0 — Enumerate every consumer of the shared user/fence (gate, no code)

**Depends on:** none. Do this BEFORE Step 3's reap ships.

The security boundary is the **reap** (DELUSER `wp_<osuser>`), not the per-install
provisioning — until the shared user is gone, a compromised sibling holding the
old shared token still reaches everyone's keys. But the reap must not yank a user
that something *else* still authenticates with.

Grep for every consumer of the shared ACL user or the broad fence and confirm the
ONLY thing that opens Redis with the tenant token is the bundled object cache in
each install's `wp-config.php`:

```
grep -rn 'wp_"\s*+\|"wp_\|~jc:\|cacheTenantToken\|cacheInstallToken\|CacheTokenSecret\|redis_password' \
  panel-api panel-agent wp-plugins --include=*.go --include=*.php
```

Check specifically: the page-cache warmup crawler (`applications_cache.go` step 6),
any monitoring/health probe that opens Redis, `jabali repair`, and the object-cache
drop-in. If anything other than the per-install-stamped `wp-config.php` uses the
shared user, the reap will break it — surface that here before Step 3.

**Exit:** written confirmation that only the per-install-stamped drop-in consumes
the tenant credential; no other live consumer of `wp_<osuser>` / `~jc:<osuser>:*`.

---

## Step 1 — Per-install ACL helpers, provision, revoke, token (panel-api)

**Branch:** `sec/jab62-per-install-acl`  •  **Model:** default  •  **Depends on:** none

### Context brief
All in `panel-api/internal/api/applications_cache.go`. Introduce two pure helpers
so the user/fence strings are defined once and unit-testable, then rewrite
provision + revoke + token to be per-install. Do NOT wire the enable paths yet
(Step 2) — this step only changes the callee signatures + helpers and updates the
two call sites minimally to keep the build green.

### Tasks
1. Add helpers (pure, exported-within-package):
   ```go
   // installACLUser is the per-install Redis ACL username (JAB-62). Must match
   // the agent's derivation from p.Prefix exactly.
   func installACLUser(osUser, installID string) string { return "wp_" + osUser + "_" + installID }
   // installACLKeyPattern fences the ACL to ONE install's keyspace.
   func installACLKeyPattern(osUser, installID string) string { return "~jc:" + osUser + ":" + installID + ":*" }
   ```
2. Change token to per-install. Add installID to the HMAC input:
   ```go
   func cacheInstallToken(secret, osUser, installID, salt string) string {
       m := hmac.New(sha256.New, []byte(secret))
       m.Write([]byte("wp-cache-install:" + osUser + ":" + installID + ":" + salt))
       return hex.EncodeToString(m.Sum(nil))
   }
   ```
   Keep `cacheTenantToken` deleted (grep shows only the two enable paths call it).
   New HMAC label (`wp-cache-install:` vs `wp-cache-tenant:`) is intentional — the
   token MUST change so a re-provision rotates it.
3. Rewrite `provisionTenantACL` → `provisionInstallACL(ctx, osUser, installID, token)`:
   use `installACLUser(...)` and `installACLKeyPattern(...)`. Keep the exact
   command allowlist already there (GET/SET/SETEX/…/SCAN/SELECT/PING/AUTH/HELLO —
   NO RANDOMKEY/DBSIZE/KEYS/FLUSH, GH #413) and the `reset` prefix + `ACL SAVE`.
4. Rewrite `revokeTenantRedisACL` → `revokeInstallACL(ctx, rdb, osUser, installID)`:
   `DELUSER installACLUser(osUser, installID)` + `ACL SAVE`. Keep idempotent
   (DELUSER of an absent user is not an error).
5. Update the two call sites (setCacheCore ~207, enableObjectCache ~402) just
   enough to compile with the new signatures — full wiring is Step 2. (In
   practice Step 1 and Step 2 land in the same PR; they're split only so the
   provisioning primitive is reviewed in isolation.)

### Verify
- `go build ./panel-api/...`
- `go test ./panel-api/internal/api/ -run Cache`

### Exit criteria
Helpers exist; provision/revoke/token are per-install; build green.

---

## Step 2 — Wire both enable paths + revoke lifecycle + agent username

**Branch:** same (`sec/jab62-per-install-acl`)  •  **Model:** default  •  **Depends on:** Step 1

### Context brief
Both enable paths must provision per-install and pass the per-install token to the
agent; the agent must stamp the matching username; disable + user-delete must
revoke per-install.

### Tasks
1. **`setCacheCore`** (~137): where it derives `prefix := osUser + ":" + installID`
   and provisions, switch to `token := cacheInstallToken(secret, osUser, installID, salt)`
   and `provisionInstallACL(ctx, osUser, installID, token)`.
2. **`enableObjectCache`** (372): same change (`install.ID` is the installID).
3. **Disable revoke lifecycle** (324-336): DELETE the `CountCacheEnabledByUserID`
   "only when last" coordination — each install now owns its user, so on disable
   just `revokeInstallACL(ctx, Redis, osUser, installID)` unconditionally
   (best-effort, logged, never fails the toggle). Siblings are unaffected because
   they have their own users.
4. **User-delete** (`users.go:742`): replace the single `revokeTenantRedisACL(…,
   username)` with: enumerate the user's application installs and
   `revokeInstallACL(…, username, install.ID)` for each; ALSO `DELUSER
   wp_<username>` (legacy shared user, in case this user predates the migration).
   Best-effort per the existing pattern.
5. **Agent** (`wordpress_cache.go:129`): change the username arg from
   `"wp_"+p.OSUser` to the per-install form derived from `p.Prefix`:
   ```go
   aclUser := "wp_" + strings.ReplaceAll(p.Prefix, ":", "_") // p.Prefix = <osuser>:<installID>
   ```
   Pass `aclUser` into `setWPConfigCacheConstants`. Add a Go test asserting
   `"wp_"+strings.ReplaceAll("<osuser>:<installID>", ":", "_")` ==
   `installACLUser("<osuser>","<installID>")` for representative inputs (keep the
   two derivations from drifting — put the assertion in panel-agent using literal
   expected strings since the two packages can't import each other).

### Verify
- `go build ./panel-api/... ./panel-agent/...`
- `go test ./panel-api/... ./panel-agent/...` (watch for existing cache tests that
  asserted `wp_<osuser>` / `~jc:<osuser>:*` — update them to per-install).

### Exit criteria
New + re-enabled installs get a per-install ACL user, fence, and token; disable
revokes only that install; user-delete revokes all of the user's per-install
users; agent stamps the matching username. Build + tests green.

---

## Step 3 — Cache-doctor migrate-and-reap of legacy shared users

**Branch:** same PR (or a stacked follow-up if Step 2 is already large)
**Model:** default  •  **Depends on:** Step 2

### Context brief
After Steps 1–2 deploy, existing enabled installs still carry the OLD shared
`wp_<osuser>` user with the broad `~jc:<osuser>:*` fence until re-provisioned —
**the vuln persists for the live fleet until migration runs.** The doctor
(`app_cache_doctor_cmd.go`) `--repair` already re-runs the enable path per install,
so it re-provisions each install to its per-install user automatically. The only
new work is reaping the orphaned legacy shared users **in the correct order**.

### Reap ordering (LOAD-BEARING — get this exactly right)
1. For a given osUser, `--repair` re-provisions EVERY one of its cache-enabled
   installs to `wp_<osuser>_<installID>` (+ re-stamps wp-config with the new
   token). This must complete for ALL of that user's enabled installs first.
2. ONLY after every enabled install of `<osuser>` has been re-provisioned, reap
   the legacy shared user: `ACL DELUSER wp_<osuser>` + `ACL SAVE`.
3. NEVER DELUSER `wp_<osuser>` before all its installs are migrated — a
   not-yet-migrated sibling still authenticates with the shared user and would
   lose cache mid-migration (dead cache, not a security hole, but an outage).
4. Idempotent: if `wp_<osuser>` no longer exists (already reaped / fresh install),
   reap is a no-op. Detect presence with `ACL GETUSER wp_<osuser>` (err/nil = absent).

### Tasks
1. In the doctor's `--repair` sweep, group processed installs by osUser. After the
   sweep, for each osUser whose installs were ALL successfully re-provisioned this
   run, `DELUSER wp_<osUser>`. If any install for that osUser failed re-provision,
   SKIP the reap for that osUser (leave the shared user so the un-migrated install
   keeps working) and report it.
2. **Audit the reap.** A `DELUSER` during migration is a security-state change —
   emit a durable audit line per reaped shared user (actor = doctor/operator,
   action = `cache_acl_reap_shared`, target = `wp_<osUser>`), same discipline held
   for JAB-85. A skipped reap is a WARN, not a hard error (next `--repair` retries).
3. Document in the doctor `--help` / command long-description that `--repair`
   migrates legacy per-user ACLs to per-install (JAB-62) and reaps orphans.

### Verify
- `go build ./panel-api/...`
- Dry-run reasoning test (unit) with a fake ACL client capturing DELUSER calls:
  reap fires only after all of a user's installs re-provisioned; skipped when one
  fails.

### Exit criteria
`app-cache doctor --repair` converts a legacy per-user fleet to per-install ACLs
and reaps orphaned shared users without ever removing a still-referenced user.

---

## Step 4 — miniredis isolation test (cross-install access denied)

**Branch:** same PR  •  **Model:** default  •  **Depends on:** Step 1 (helpers), Step 2 (provision)

### Context brief — DO NOT let this test masquerade as an enforcement proof
`github.com/alicebob/miniredis/v2 v2.37.0` is a dependency, but miniredis is an
in-memory fake and **very likely does NOT enforce per-key ACL pattern denial** on
an authed connection — exactly the behavior a fake stubs. **First, empirically
check**: `AUTH` as `wp_<osUser>_<A>`, then `GET jc:<osUser>:<B>:key`. If miniredis
returns the value (or won't auth as the ACL user at all) instead of `NOPERM`, then
a "cross-install GET denied" assertion would either fail spuriously or **pass
trivially and prove nothing** — a green test that proves the wrong thing.

So split what is actually being proven:
- **Unit test (this step) proves ISSUANCE**, which is real and ours to own: the
  SETUSER arg vector fences to `~jc:<osUser>:<installID>:*` (NOT the broad
  `~jc:<osUser>:*`), tokens differ per install, and the allowlist excludes the
  enumeration verbs.
- **The isolation PROPERTY** (A's credential is *denied* B's keys) is Redis's ACL
  enforcement, not ours — it belongs in a **live-Redis/VM smoke**, same category as
  the project's other "live-VM validated" items. Do not claim it in a unit test
  that can't exercise enforcement.

### Tasks
1. Test `installACLUser` / `installACLKeyPattern`: two installs of the same osUser
   produce DIFFERENT users and DIFFERENT fences; the fence contains the installID
   and does NOT reduce to the broad `~jc:<osUser>:*`.
2. Capture the `ACL SETUSER` argument vector (via a fake that records `Do(...)`
   args) for two installs A, B of the same osUser. Assert: A's fence is
   `~jc:<osUser>:<A>:*`, B's is `~jc:<osUser>:<B>:*`; tokens for A and B differ.
3. Assert the allowlist has NO `RANDOMKEY`/`DBSIZE`/`KEYS`/`FLUSHDB` (GH #413 guard).
4. IF the empirical check shows miniredis enforces NOPERM: add the live denial
   assertion too (bonus). If not, `log()`/comment that enforcement is covered by
   the live-Redis smoke below — do NOT silently skip.

### Live-Redis / VM smoke (part of the Done gate, not the unit test)
On a host with real Redis: enable cache on two installs of one OS user, then with
install A's stamped token attempt `GET`/`SCAN` of `jc:<osUser>:<B>:*` and assert
`NOPERM`. After `app-cache doctor --repair`, attempt the SAME with the OLD shared
token and assert it also fails (shared user reaped). This is the actual JAB-62
proof.

### Verify
- `go test ./panel-api/internal/api/ -run 'ACL|Isolation|Cache' -race`
- Live smoke per above before marking Done.

### Exit criteria
Unit test proves per-install ISSUANCE (fence + distinct tokens + allowlist). The
enforcement/denial property is proven on the live-Redis smoke — JAB-62 is marked
Done only after that smoke, including "old shared credential can no longer read a
sibling's keys post-`--repair`."

---

## Rollback

Single revertable PR. If a regression ships:
- Revert the PR → provision reverts to per-user (`wp_<osuser>` / `~jc:<osuser>:*`).
- Existing installs stamped with per-install tokens would then mismatch the
  reverted per-user ACL user → run `app-cache doctor --repair` (post-revert code)
  to re-stamp back to per-user. So: **revert code, then run doctor --repair** to
  reconcile wp-config tokens with whichever ACL scheme is live. Note this in the
  PR description.

## Sequencing summary
- Step 0 (grep gate) before Step 3 ships — no code, but blocks the reap.
- Steps 1+2+4 land together (the isolation fix + issuance test) in `sec/jab62-per-install-acl`.
- Step 3 (doctor reap + audit) same PR or a fast stacked follow-up.
- After merge + release: operator runs `jabali app-cache doctor --repair` on each
  managed host to migrate the live fleet and reap legacy shared users.
- **Mark JAB-62 Done only after the live-Redis smoke** (Step 4) passes BOTH:
  (a) install A's per-install token is denied `jc:<osuser>:<B>:*` (`NOPERM`), and
  (b) post-`--repair`, the OLD shared token is denied a sibling's keys (shared user
  reaped). A green unit test alone is NOT sufficient — it proves issuance, not
  enforcement.
