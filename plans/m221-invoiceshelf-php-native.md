# Blueprint: PHP-native InvoiceShelf 1-click (GH #221)

**Status:** DRAFT (feasibility de-risked via research, 2026-06-20)
**Issue:** #221 — johnnyq asks for a PHP-native InvoiceShelf alongside the Docker app
**Target ADR:** 0144

## Why (johnnyq's case, accepted)

The Docker InvoiceShelf shipped (`5fe54c46`). A PHP-native variant is worth it:
less resource overhead than a container + bundled MariaDB, easier panel-driven
backup, and Flarum already proves a Laravel-class app can be a native 1-click.

## What the research established

- **Prebuilt release exists:** `InvoiceShelf.zip` (v2.4.1, 30 MB) bundles `vendor/`
  and built front-end assets — NO composer / npm needed (Flarum-equivalent).
- **Layout:** single top-level `InvoiceShelf/` wrapper (extract with strip-1);
  Laravel webroot is `public/`; ships `.env.example` + `artisan`.
- **No headless installer:** admin + company are created by a **web wizard**
  (`resources/scripts/admin/views/installation/Step0…Step6`), exactly like the
  Docker app ("complete the one-time setup wizard on first visit"). So — unlike
  Flarum's `php flarum install -f` — we cannot create the admin from the CLI.
- **Env contract (from the shipped Docker app):** `APP_ENV=production`,
  `APP_DEBUG=false`, `APP_URL`, `APP_KEY` (32 chars ok), `DB_CONNECTION=mariadb`,
  `DB_HOST/PORT/DATABASE/USERNAME/PASSWORD`, `CACHE_STORE=file`,
  `SESSION_DRIVER=file`, `SESSION_DOMAIN`, `SANCTUM_STATEFUL_DOMAINS`.
- **Webroot mechanism already in jabali:** the domain **document_root override**
  (`validateDocumentRoot`, vhost `RootOverriddenOmitsDefaultLocations`) can point
  the site root at `<install>/public` — no custom nginx hacks needed.

## Design

Model on Flarum (`apps/flarum.go` + `commands/flarum_install.go`), with three
deltas: no admin CLI (wizard finishes it), a `public/` webroot, and Laravel
`.env` + key + migrate.

1. **Descriptor** `panel-api/internal/apps/invoiceshelf.go`:
   - `RequiresDB: true`, `AgentInstallCmd: "invoiceshelf.install"`,
     `AgentDeleteCmd: "app.delete"`, clone empty (hidden) for v1.
   - `InstallParamSchema`: `site_title`, `admin_email` only — **no admin_user /
     admin_password** (wizard creates the admin). Post-install reveal text =
     "open the site and complete the setup wizard."
   - `DefaultSubdirectory: ""` (apex; subdir deferred — SPA base-path is fiddly).
   - Register in `apps/registry.go`.

2. **Agent installer** `panel-agent/internal/commands/invoiceshelf_install.go`:
   - Pin `invoiceshelfVersion = "2.4.1"` + release-asset URL + **SHA-256** (bump
     together). Download (10-min client) → verify sha → extract strip-1.
   - Write `.env` from `.env.example` with the env contract above; generate
     `APP_KEY` via `php artisan key:generate --force` (per-user `phpCLIFor`).
   - `php artisan migrate --force` + `php artisan storage:link`. (Confirm on the
     VM the wizard tolerates a pre-migrated DB; if it insists on migrating
     itself, skip migrate and let the wizard do it — Spike A.)
   - Perms: `storage/` + `bootstrap/cache/` writable by the per-user pool.
   - Sensitive-path hardening snippet (deny `.env`, `/storage/`, `/vendor/`,
     `artisan`) like `flarumNginxConf`.

3. **Webroot wiring (the one framework touch):** the panel-api app-install
   handler sets the domain's `document_root` to `<install>/public` so the vhost
   serves Laravel's front controller. (Alternative if that coupling is unwanted:
   a self-contained nginx snippet with `root <install>/public;` inside `location
   /` + the front controller — but the document_root override is the blessed
   path and renders cleanly.) **Decide in review.**

## Verification (live on mx — required, wizard is the risk)

- Install at an apex test domain → site loads the wizard → complete
  language/requirements/domain/account/company → create one invoice + PDF.
- Requirements step passes (PHP extensions present in jabali's pool: bcmath,
  ctype, json, mbstring, openssl, pdo, tokenizer, xml, gd/imagick, zip, fileinfo,
  intl — verify against the per-user pool; add any missing to install.sh).
- Asset URLs resolve under the apex (SPA + Vite manifest).
- `app.delete` removes files + drops the DB (db_id path, same as Flarum/phpBB).
- Disabled/again-install idempotency.

## Open questions for review

- document_root wiring vs self-contained nginx snippet (lean: document_root).
- Pre-migrate vs let the wizard migrate (Spike A on the VM).
- Subdir support (defer to v2 — SPA base path).

## Steps

1. ADR-0144 + this blueprint.
2. Descriptor + registry registration.
3. Agent installer (download/verify/extract/.env/key/migrate/perms/nginx).
4. document_root wiring in the app-install handler.
5. PHP-extension audit for the per-user pool (install.sh if gaps).
6. Live wizard validation on mx + delete round-trip.
7. Memory + ADR ACCEPTED after smoke.
