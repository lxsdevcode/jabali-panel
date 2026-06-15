# ADR-0130: `.htaccess` → Rule Builder converter (typed rules, not raw nginx)

**Date**: 2026-06-15
**Status**: Accepted
**Deciders**: shuki + Claude
**Related**: ADR-0125 (M50 directory privacy — rejected tenant `.htaccess`
files; DB-as-truth), ADR-0023 (M9 nginx/PHP-FPM), the Rule Builder
(`models.NginxRule` + `internal/nginxrules`). Enables cPanel/DA/Hestia
migration fidelity.

## Context

jabali serves via nginx, which has no `.htaccess`. Migrated cPanel /
DirectAdmin / Hestia sites carry `.htaccess` rewrites that **silently break**
on import. ADR-0125 already established the principle: nginx behavior is
DB-as-truth (typed rules + reconciler), never tenant-writable docroot files.

Two off-the-shelf options were considered: `e404/htaccess-for-nginx` (Lua,
per-request) and `lukechilds/htconvert` (JS, redirects-only, one-shot).

## Decision

Build a **Go translator** (`internal/htaccess`) that converts `.htaccess`
into jabali's existing **typed `NginxRule` Rule Builder entries**, not raw
nginx config. The htconvert + e404 mappings are used as MIT-licensed
*reference* for the regex translation; the engine is reimplemented in Go
where the directive compiler already lives.

Three properties define the decision:

1. **Typed rules, not raw directives.** Output is `[]NginxRule` constrained
   to the existing whitelist (`rewrite`, `custom_header`, `ip_access`,
   `php_setting`, `max_upload_size`), compiled by `internal/nginxrules` and
   gated by `nginx -t`. Tenant input can never inject `root`, `proxy_pass`,
   `include`, `load_module`, or escape the domain's server block.
2. **Front-controller is a no-op.** The default vhost `location /` already
   does `try_files $uri $uri/ /index.php?$query_string`, so the canonical
   WP/Drupal/Laravel `RewriteCond !-f/!-d → index.php [L]` block is already
   satisfied. The converter recognizes and **skips** it with an info note.
   This is what makes "full common-set coverage" compatible with
   "typed-rules-only" — the hardest case maps to nothing, deliberately.
3. **Fail-closed.** Any access-control / auth / conditional block that
   cannot be *fully* represented emits **nothing** + a warning, never the
   permissive half (dropping a `RewriteCond` or mis-mapping `Order` could
   invert a deny into an allow). Enforced with negative tests.

The translator is reused by an online "Import from .htaccess" panel action
and by the migration restore pipeline (auto-seed each domain's Rule Builder
from its docroot `.htaccess`, warnings into the migration report).

## Why not the off-the-shelf engines

- **e404 (Lua, per-request)** — reverses ADR-0125 (tenant docroot file as
  live config truth, bypassing DB/audit/reconciler) and puts tenant Lua +
  tenant regex in the **shared** nginx worker's request path → ReDoS / crash
  blast radius across all tenants. Rejected (see the prior analysis).
- **htconvert (JS fork)** — redirects-only, so the hard 80% is ours anyway;
  forking adds a Node hop in a Go control plane for ~50 lines of mapping.
  Adopted as reference, not as a dependency.
- **Emitting nginx `if` for RewriteCond** — the "if is evil" anti-pattern,
  and it would require raw directives, breaking the typed-rules property.
  Rejected: only the front-controller idiom is recognized; other
  `RewriteCond` blocks are dropped fail-closed with a warning.

## Consequences

### Positive
- Migrated sites' redirects/rewrites/headers/php settings carry over as
  first-class, editable Rule Builder entries — DB-as-truth, audited,
  reversible, nginx-`t`-gated. No new request-path attack surface.
- One translator, two consumers (online + migration).

### Negative
- Not 100% Apache coverage. Auth (htpasswd creds aren't in `.htaccess`),
  `+Indexes`, `ErrorDocument`, `DirectoryIndex`, `SetEnv`, mod-specific and
  multi-condition `RewriteCond` blocks are surfaced as warnings, not
  converted. Honest partial conversion with an explicit report beats a
  silent or unsafe full one.
- The converter must be kept in step with the `NginxRule` whitelist as new
  rule types are added.

## Implementation

`internal/htaccess` (`Convert(htaccess, basePath string) Result`,
`Result{Rules, Warnings, Notes}`). Stage-gated build (plans/htaccess-converter.md):
core + golden fixtures + an `nginx -t` integration test first, then a
security-reviewer pass, then API preview endpoint, Rule Builder import UI,
and migration auto-wire. Branch `feat/htaccess-converter`.
