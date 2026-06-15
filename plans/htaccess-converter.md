# Blueprint: `.htaccess` → Rule Builder converter

**Status**: Stages 1–5 implemented on `feat/htaccess-converter`. Stage 5
(migration auto-wire) is correct but DORMANT — it lives in `ImportDomains`,
which the cP/DA/Hestia restore-stage runner does not yet call (those import
functions are scaffolding without a live caller); it activates when the
restore stage is wired.
**ADR**: 0130. **Fixes**: the migration gap where Apache `.htaccess` rewrites
silently break on nginx (raised on GH #133 follow-up; cPanel/DA/Hestia docs).

## Goal

Convert an Apache `.htaccess` file into jabali's existing **typed
`NginxRule` Rule Builder entries** (`models.NginxRule`, compiled by
`internal/nginxrules`) — NOT raw nginx. Tenant input becomes a constrained,
whitelisted, nginx-`t`-gated set of typed rules. Two consumers share one
core translator:

1. **Online action** — operator pastes / picks a `.htaccess`, previews the
   generated rules + warnings, saves into `domain.NginxRules`.
2. **Migration auto-wire** — cPanel/DA/Hestia restore reads each domain's
   docroot `.htaccess`, runs the converter, populates that domain's Rule
   Builder, and attaches the warnings to the migration report.

## Why a Go translator, not a fork

Decided (operator): port the mapping logic from htconvert (MIT, redirects
only) + e404/htaccess-for-nginx (directive map) as **reference**, implement
in Go. The control-plane directive compiler already lives in Go
(`internal/nginxrules`, `domain_create.go`); a JS fork would add a Node hop
in the apply path for ~50 lines of regex mapping and still leave the hard
80% (RewriteRule/Cond/auth/deny) to us. See ADR-0130.

## The load-bearing design decision: front-controller is a no-op

The default vhost `location /` already does
`try_files $uri $uri/ /index.php?$query_string;`
(`domain_create.go:172-174`). So the canonical WP/Drupal/Laravel block:

```
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule . /index.php [L]
```

is **already satisfied**. The converter RECOGNIZES this idiom and SKIPS it
with an informational note — it never translates it. This is what keeps
"full common set" compatible with "typed-rules-only": the hardest, most
common case maps to nothing, by design.

## Safety invariants (non-negotiable, tested)

1. **Fail-closed.** If any access-control / auth / conditional block cannot
   be *fully* represented, emit **nothing** for that block + a warning —
   never the permissive half. Dropping a `RewriteCond` while keeping its
   `RewriteRule`, or mis-mapping `Order`, can invert a deny into an allow.
   Negative tests assert the rule is **dropped**, not partially emitted.
2. **Typed rules only.** Output is `[]NginxRule` from the existing
   whitelist (`rewrite`, `custom_header`, `ip_access`, `php_setting`,
   `max_upload_size`). The converter never emits raw directives, `root`,
   `proxy_pass` from tenant input, `include`, etc.
3. **basePath aware.** Apache matches RewriteRule relative to the
   `.htaccess` directory (+ `RewriteBase`); nginx matches the full URI.
   Signature is `Convert(htaccess string, basePath string) Result` — a
   subdir `.htaccess` rewrites wrong without it.
4. **Order preserved.** Emit rules in source order; honor `[L]`.
5. **No silent partial.** Every unconverted line produces a `Warning`
   (kind + line + reason). The caller surfaces them; nothing is dropped
   invisibly.

## Directive mapping (v1 "full common set")

| Apache | → | jabali |
|---|---|---|
| front-controller idiom (`!-f`+`!-d`→index.php `[L]`) | skip | info note "already handled by default routing" |
| `Redirect [code] /prefix URL` (prefix+append) | `rewrite` | `^/prefix(.*)$ → URL$1`, flag permanent/redirect |
| `RedirectMatch [code] regex URL` | `rewrite` | regex→replacement, flag permanent/redirect |
| `RewriteRule pat sub [R=code,L]` (external) | `rewrite` | flag permanent (301) / redirect (302) |
| `RewriteRule pat sub [L]` (internal, non-FC) | `rewrite` | flag last (only if no unrepresentable Cond) |
| `Header [always] set Name Val` | `custom_header` | Name/Value/Always |
| `php_value name val` / `php_flag name on\|off` | `php_setting` | Name=Value (on→On, off→Off) |
| `Allow/Deny` + `Order` | `ip_access` | allow_list/deny_list — **fail-closed on ambiguous Order** |
| `Options -Indexes` | info | already nginx default (autoindex off) |
| `AuthType Basic` + `AuthUserFile` | warn | "use Directory Privacy" — htpasswd creds aren't in `.htaccess`, can't migrate |
| `Options +Indexes`, `DirectoryIndex`, `ErrorDocument`, `SetEnv`, `mod_*`, complex `RewriteCond` | warn | unmapped |

`RewriteCond` handling: only the file-controller idiom is recognized. Any
other `RewriteCond` makes its `RewriteRule` block **unrepresentable** →
drop block + warning (fail-closed). nginx `if` is deliberately NOT emitted
(the "if is evil" anti-pattern + breaks typed-rules-only).

## Build order (stage-gated; prove core before wiring)

1. **Core translator** `internal/htaccess` + golden-file tests with real
   fixtures (stock WP, Drupal, Joomla, Laravel, redirect pack, an
   access-control pack). Assert emitted rules AND warnings. **+ one
   integration test that runs the compiled output through `nginx -t`** on a
   fixture vhost (the real gate). ← **this stage**
2. **security-reviewer pass** on the core (output is tenant-influenced).
3. **API** — `POST /api/v1/domains/:id/htaccess/preview` → `{rules,
   warnings}` (no mutation); apply via the existing domain-update path that
   writes `NginxRules` (reuses nginx -t gate + reconciler + audit).
4. **UI** — Rule Builder "Import from .htaccess" action: paste or pick a
   docroot file (File Manager) → preview rules + warnings → save.
5. **Migration auto-wire** — restore pipeline reads docroot `.htaccess`
   per domain → `Convert` → seed `NginxRules` → warnings into the report.

## Known limitations (recorded, not silent)

- **Docroot-level access control is refused (security).** A whole-site
  (`basePath "/"`) `allow_list`/`deny_list` would compile to `location / {…}`,
  which makes `writeVhost` treat the root as overridden and drop BOTH the
  default `location /` AND `location ~ \.php$` (`domain_create.go:473` feeds
  the compiled rules to `directivesOverrideRoot`; template lines 171/220).
  An allow_list would then serve PHP as SOURCE to the allowed clients. The
  converter refuses any docroot access rule except a pure `Deny from all`
  (everything 403s, no routing needed) and emits a security warning instead.
  Verified by code inspection + unit tests; a full live PATCH round-trip is
  NOT yet run (needs an authenticated session).
- **Subdir `ip_access` does not gate `.php` files.** A `location /sub/ {
  allow…; deny…; }` is a prefix match; nginx's regex `location ~ \.php$`
  wins for `/sub/x.php`, so the IP restriction does not cover PHP under that
  subdir. This is the existing `ip_access` rule-type behavior, inherited (not
  introduced) by the converter; faithful conversion, imperfect Apache parity.

- **Docroot-level access rule → `location /`.** A `basePath "/"` access rule
  (e.g. a docroot `Deny from all`) compiles to `location / { deny all; }`.
  Verified on real nginx (10.0.3.14): the compiled directives pass `nginx -t`
  in a production-like vhost, but injecting them *alongside* the template's
  default `location /` is a `duplicate location "/"` emerg. **Stage 3's apply
  path MUST route converted rules through `NginxRules` so the existing
  `directivesOverrideRoot` (domain_create.go:824, which already folds in the
  compiled rules) omits the default `location /`.** This is the single most
  important apply-path check.
- **Apache 2.4 `Require` + legacy `Order/Allow/Deny` in one file.**
  `flushAccess` resolves `Require` and early-returns, ignoring any legacy
  `Order/Allow/Deny` lines in the same scope. Rare; the `Require` form is
  usually the more-restrictive winner. Surfaced here, not silently lost.
- **`nginx -t` integration gate** (`nginxgate_test.go`) skips when nginx is
  absent (the Go-only CI job). Syntax was proven manually on 10.0.3.14; the
  full vhost-template integration belongs to Stage 3.

## Result type (core)

```go
type Result struct {
    Rules    []models.NginxRule
    Warnings []Warning // {Line int, Source string, Reason string}
    Notes    []string  // informational (e.g. front-controller skipped)
}
func Convert(htaccess, basePath string) Result
```
