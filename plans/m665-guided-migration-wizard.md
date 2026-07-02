# Blueprint — Guided Migration Wizard + Unified Progress · GH #665

**Objective:** replace the technical `create job → upload secrets → pull-source → import` model with ONE guided wizard (**Choose source → Connect once (+test) → Scan & map → Review → Run & monitor**) and ONE progress screen, for **all** sources: cPanel/WHM, DirectAdmin, HestiaCP, WordPress-SSH (+ WHM-pkgacct upload). Orchestration/UX only — the existing jobs table, secrets, pull-source, and import commands stay.

**This is orchestration + UX over a working engine.** The migration *engine* already exists and is proven: `internal/migrate` Discoverers (cpanel/da/hestia), `migrate pull-source` (kind-dispatched arms), `migrate import`/`import-wp`, `migration_jobs` + `migration_stages`, the secret store + reaper. #665 wraps it in a wizard + a `/events` progress stream. **Do NOT rewrite the engine.**

## What already exists (reuse — verified in this codebase)
- **Discoverers** `internal/migrate/{cpanel,directadmin,hestiacp}/discover.go` — `Connect` + `ListAccounts` + `DescribeAccount` → the **test-connection + scan** data for panels (counts, accounts, domains, DBs, mailboxes). `wordpressssh` has `Connect`/`DiscoverWordPress`/`ScanWordPress`.
- **Migration API** `internal/api/admin_migrations.go` — create, `/secrets` (`migration.secrets_write` writes SSH_PASSWORD/KEY + PLUGIN_TOKEN), `/pull-source` (`migration.pull_source_run` transient unit), `/import`, `/import-wp`. Admin `AdminMigrationsPage` + `CreateMigrationDrawer` + `AdminMigrationDetailPage` + `useMigrationStream` UI.
- **WordPress slice (GH #647/#648, SHIPPED) already implements #665 primitives:** `POST /migrations/:id/verify` = **test-connection** (facts + plugin-version warning); `/scan-wp` = **scan**; `dest_user`/`dest_domain` on the job + `maybeAutoImportWP` = **background run** (pull auto-chains to import); the **Applications-table row** (installing→ready/failed) = a **progress surface**; `migrate.SafeHTTPClient` rebind-safe pull. **Generalize these, don't reinvent.**
- **Stages** `migration_stages` (name/state/bytes/last_error) already exist — the `/events` timeline reads them; the wizard just needs the arms to WRITE more of them.

## Backend endpoint map (issue's shape → existing engine)
| #665 endpoint | Maps to |
|---|---|
| `POST /migrations/draft` `{source_type}` | existing create (state `draft`, per ADR-0095) |
| `PUT /migrations/:id/connection` | existing `/secrets` (host/user/port on the job + secret store) |
| `POST /migrations/:id/test-connection` | **generalize** `/verify`: dispatch by kind → the right Discoverer's `Connect` + a cheap count probe (`ListAccounts`) → `{panel, version, accounts, domains, dbs, mailboxes}` |
| `POST /migrations/:id/scan` | **generalize** `/scan-wp`: Discoverer `ListAccounts` + per-account `DescribeAccount` → the scan-results table rows (source user, main domain, size, notes/warnings) |
| `PUT /migrations/:id/plan` | **NEW**: persist selections (which accounts, dest-owner mapping, import-area toggles) — a `migration_plan` JSON on the job (schema-only additive column) |
| `POST /migrations/:id/run` | existing pull-source → import chain, gated by the plan; writes stages |
| `GET /migrations/:id/events` | **NEW read**: stream/poll `migration_stages` + `last_error` → the progress timeline + live log |

## Steps

### S1 — Migrations landing page (admin) · mockup 01
Nav "Migrations" already routes to `AdminMigrationsPage`. Reshape it: a **"Start a guided migration"** card + **New migration** button; a **Recent migration jobs** table (Source, Host, Accounts, Status, **Next action** = Continue / Add credentials / View progress / Retry, derived from job state). Reuse the existing jobs list query. **Verify:** landing renders real jobs with correct next-action per state.

### S2 — Source cards (wizard 1/4) · mockup 02
Replace the source-kind `Select` with 4 **cards** (cPanel/WHM, DirectAdmin, HestiaCP, WordPress-SSH) + descriptions; selecting → `POST /migrations/draft {source_type}`. **Verify:** each card creates a draft of the right kind.

### S3 — Connect once + Test connection (wizard 2/4) · mockup 03 · ⚠ SECURITY
One step: host, **port**, user, auth toggle (**Password | Private key**), secret → `PUT /connection` (secret internal to the wizard, never a separate action). **Test connection** → `POST /test-connection` → green "Found cPanel 124.0, 42 accounts, 61 domains, 87 DBs, 126 mailboxes" or the exact error. **Generalize `/verify`** to dispatch cpanel/da/hestia via their Discoverers (reuse host-key pinning + `DialTCP` SSRF). Port is a NEW job field (additive column). **Verify:** test-connection returns real counts per panel + errors clearly; secret hits the store, never the client.

### S4 — Scan results + account mapping + import options (wizard 3/4) · mockup 04
Auto-run `POST /scan` after connect. Show summary counts + an **Accounts to migrate** table (Import checkbox, source user, main domain, size, **Destination owner** = Create user / Map to existing / Skip, **Notes/warnings** = OK / PHP 7.4 / No DNS zone). **Import-area** toggles (websites, DBs, mailboxes, DNS, SSL, cron). `PUT /plan` persists it. **Warnings** come from `DescribeAccount` (unsupported PHP, missing DNS zone, custom Apache includes). **Verify:** scan lists real accounts with sizes + warnings; plan round-trips.

### S5 — Review & run (wizard 4/4) · mockup 05
"Ready to migrate N accounts" + **pre-flight checks** (yellow: "DNS not switched automatically") + the pipeline chips (Connect→Discover→Package→Transfer→Restore→Validate) + a **Summary** (source, destination, included areas, estimated transfer, rollback = "source untouched"). **Start migration** → `POST /run`. **Verify:** summary reflects the plan; start kicks the run.

### S6 — Unified progress timeline + live log (mockup 06)
ONE progress screen: job title + Running badge, a **%-complete bar** + phase label, a **stage timeline** (Connected → Exported → Transferred → Restoring DBs+mail → Validating → Final report) with ✓/spinner/pending, a **Live log** panel, **View logs / Pause / Retry**. Backed by `GET /events` reading `migration_stages`. **The engine arms must WRITE these stages** (today only some do — add stage rows at each phase). Hide the pull/import split. **Final report:** migrated accounts/domains/DBs/mailboxes, skipped items, warnings, DNS records to update, links to new Jabali users/domains. **Verify:** timeline advances live through a real migration; failed stage shows "what went wrong + suggested fix".

### S7 — Wording + "Show CLI" accordion
Rename per the issue (`Source kind`→"What are you migrating from?", `Upload secrets`→"Enter SSH credentials", `Pull source`→"Scan source", `Run import`→"Start migration", `Stage rows`→"Progress timeline", `Last error`→"What went wrong + suggested fix"). A **Show CLI** accordion emits the equivalent `jabali migrate …` commands (power users). **Verify:** copy-paste CLI reproduces the wizard's actions.

## Minimum useful release (ship first; per the issue)
Source cards (S2) + connect+secret+test (S3) + scan (S4, even just an **"import all accounts"** checkbox) + review (S5) + start + progress (S6). Perfect per-account mapping (dest-owner, per-area) is a follow-up. **This lands the wizard over the proven engine without the full mapping matrix.**

## Anti-patterns
- Don't rewrite the migration engine — wrap it. The Discoverers + pull/import + stages are the engine.
- Don't fork the WordPress `/verify`/`/scan-wp`/auto-import — **generalize** them (dispatch by kind).
- Secret stays internal to the wizard, in the store, never the client (reuse `migration.secrets_write`).
- `plan`/`port`/progress-detail = additive schema only (migration = schema only; data in the app).
- DNS is never switched automatically — surface it as a pre-flight warning + a suggested-records report.
- Progress must show terminal FAILURE states, not just the happy path (per-stage `last_error`).

## Grounding done 2026-07-03: the WordPress-SSH + plugin slices (GH #647/#648) are SHIPPED and already provide test-connection (`/verify`), scan (`/scan-wp`), background run (auto-import), and a table-row progress surface. #665 = generalize those to the panel Discoverers + build the wizard shell + the `/events` timeline. Mockups in the issue (`00_flow_diagram`…`06_progress`).
