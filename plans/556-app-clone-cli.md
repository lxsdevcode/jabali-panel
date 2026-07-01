# GH #556 part 3 — `jabali app clone` (design, ready to implement)

**Status:** designed, not implemented. `app scan` (pre-existing) + `app cache`
(shipped `20d89b6c`) are done. This is the last part.

**Why deferred to fresh context:** faithful extraction of the ~235-line
`wordPressHandler.clone` (wordpress.go:690–925) — high-blast-radius shared web
code + an async goroutine that makes the characterization test harder than
cache's. Compact-first.

## Approach (same pattern as `app cache` extraction, `20d89b6c`)

1. **Characterization test FIRST** (`applications_test.go`), locking the web
   `POST /applications/:id/clone` behavior via `applicationsRouter`:
   - source not found → 404 `source_install_not_found`
   - dest domain not found → 404 `domain_not_found`
   - dest domain cross-user → 403 `forbidden`
   - collision at (dest domain, src subdir) → 409 `install_exists`
   - success → 202 + a "cloning" install row created + dest DB row created.
     (Do NOT assert the async goroutine's terminal status — racy. Only the
     synchronous provisioning + 202.)
   Mocks needed: mockWordPressInstallRepo (add FindByDomainAndSubdirectory if
   missing), mockDatabaseRepo/UserRepo/GrantRepo Create/Delete, mockAgent.

2. **Extract** `cloneCore(ctx, sourceInstallID, destDomainID string, isAdmin bool,
   actorUserID string, async bool) (*createWordPressResponse, error)`:
   - Move wordpress.go:707–925 verbatim; `targetUserID := actorUserID`.
   - Replace each `c.JSON(status, {error:X}); return` with `return <sentinel>`:
     `errCloneSourceNotFound` (404), `errCloneDomainNotFound` (404),
     `errCloneForbidden` (403), `errCloneInstallExists` (409),
     `errCloneUserNotProvisioned` (409). Agent failures → a `cloneAgentError`
     wrapping the detail (→ 502 `agent_failed` + `detail`). Others → 500.
   - The kick: `if async { go createCloneAndKickAgent(ctx, ...) } else {
     createCloneAndKickAgent(ctx, ...) }`. It already detaches to
     `context.Background()` + 5-min timeout, so sync-blocking is safe and the
     web goroutine still survives.
   - Return `*createWordPressResponse` (built as today) + nil.

3. **Wrapper** `clone(c *gin.Context)`: parse claims/id/body (401/400
   `invalid_request`+detail), call `cloneCore(..., claims.IsAdmin, claims.UserID,
   true)`, map sentinels→status (incl. `errors.As` for `*cloneAgentError` →
   502 + detail), success → `c.JSON(202, resp)`.

4. **Exported entry** `api.CloneApplication(ctx, cfg, sourceInstallID,
   destDomainID string, isAdmin bool, actorUserID string, async bool)
   (*createWordPressResponse, error)` → `(&wordPressHandler{cfg}).cloneCore(...)`.

5. **CLI** `panel-api/cmd/server/app_clone_cmd.go`, `jabali app clone
   <src-install-id> --to-domain <dest-domain-id>`:
   - `requireDBAndAgent`; `cfg := buildAppDeps()`.
   - Resolve dest domain → owner `U` (domainRepoFromDB().FindByID). Clone AS the
     owner: `api.CloneApplication(ctx, cfg, src, destDomainID, false, U.UserID,
     false)` (async=false → blocks until the clone finishes; process stays
     alive). Long cobra timeout (≥6 min) since the kick runs up to 5 min.
   - Print the resulting install ID + final status; `cliAuditOK`.
   - Wire `newAppCloneCmd()` into `newAppCmd()` (app_cmd.go).

6. Regenerate golden (`go test ./panel-api/cmd/server -run TestCLIReferenceGolden
   -update`), `gofmt`, build, `go test ./panel-api/...`, commit + push both
   remotes, `jabali update` on 10.0.3.14, live-verify a clone (a WP install must
   exist — none currently on the host; may need to install one first).

## Ownership note

The web `clone` ties the clone to `claims.UserID` and 403s if the caller doesn't
own the dest domain — so an operator CLI passing `isAdmin=true` would always
403. The CLI instead resolves the dest-domain owner and clones as them
(same code path as a tenant cloning their own site, within one account). No web
behavior change.
