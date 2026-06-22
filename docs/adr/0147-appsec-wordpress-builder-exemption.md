# ADR-0147: AppSec exemption for WordPress page-builder endpoints (cookie-gated)

**Date:** 2026-06-22
**Status:** Accepted — live-verified on 10.0.3.14
**Owner:** shuki
**Issue:** GH #404

## Context

CrowdSec AppSec (OWASP CRS) false-positive-bans legitimate WordPress admins. A
page builder (Elementor) saving a page POSTs arbitrary HTML/CSS/URLs inside
JSON, which is indistinguishable from injection at the WAF: CRS 942550 (JSON
SQLi) + 932370 (RCE) + 911100 (method) push the inbound anomaly score past 949110
→ AppSec issues a ~4h **ban on the editor's own IP** → every subsequent request
(save, `wp-json`, `admin-ajax`) 403s → admin locked out. Hit on `malki-ins.co.il`
during migration ops.

The endpoints: `/wp-json/elementor/*`, `/wp-admin/admin-ajax.php`,
`/wp-admin/post.php`. The issue proposed a blanket path-allow on these plus
`/wp-json/wp/v2/`.

## Decision

A blanket path-allow is **rejected** for `admin-ajax.php` and `wp/v2`: the WAF
cannot see authentication, and both have genuine **unauthenticated** surface
(`wp_ajax_nopriv_*` handlers; public WP REST endpoints). Allowing them outright
strips the WAF from a real attack surface. jabali's own config philosophy
(`appseccfg.go`) is "prefer narrow over path-allow," but a per-CRS-rule-ID
exclusion is whack-a-mole against a builder that posts arbitrary markup.

The resolution: **localize by auth-signal, not by rule.** New
`Opts.WordPressAllowlist` (default-on, rendered by `jabali appsec render-config`
into the managed `jabali-appsec.yaml`, so it survives `jabali update`) adds one
`on_match` filter:

```yaml
- filter: req.URL.Path startsWith "/wp-json/elementor/"
       || (req.URL.Path == "/wp-admin/admin-ajax.php" && any(req.Cookies(), {.Name startsWith "wordpress_logged_in_"}))
       || (req.URL.Path == "/wp-admin/post.php"       && any(req.Cookies(), {.Name startsWith "wordpress_logged_in_"}))
  apply: [ CancelEvent(), CancelAlert(), SetRemediation("allow") ]
```

- `/wp-json/elementor/` — plain allow. Plugin-namespaced REST with its own
  permission callbacks; the defensible analog of the existing `/api/v1/` +
  webmail-host allowlist (ADR-0102).
- `/wp-admin/{admin-ajax,post}.php` — exempt **only** when a WordPress login
  cookie (`wordpress_logged_in_<hash>`) is present. The cookieless `nopriv`
  surface stays fully CRS-inspected.
- `/wp-json/wp/v2/` and `/wp-login.php` — **not** exempted (public surface).

Both `appseccfg.Render` callers (the `render-config` CLI used by install.sh +
`jabali update`, and the panel-agent re-render in `security_crowdsec.go`) set
`WordPressAllowlist: true`, so a reconciler re-render never drops the exemption
(cross-boundary single-source, [[feedback_cross_boundary_contracts]]).

### Expr notes (validated live, not assumed)

- `req` in `on_match` is a Go `http.Request`; `req.URL.Path`, `req.Cookies()`,
  `req.Header` are available in the inband phase.
- The infix `contains` operator does **not** do string-substring in CrowdSec's
  expr (compiles, never matches) — `any(req.Cookies(), {.Name startsWith …})` is
  used instead. It matches by cookie **name** (robust; a value containing the
  string can't spoof it) and reuses the proven `startsWith` helper.

## Consequences

- Verified on 10.0.3.14 against live CrowdSec (`crowdsec -t` + the appsec
  endpoint): deny non-exempt SQLi; allow elementor; **deny** cookieless
  admin-ajax; allow admin-ajax/post.php only with the WP login cookie; deny a
  junk (`PHPSESSID`) cookie. Front-end + `wp-login` keep full CRS.
- **Residual** (stated honestly): cookie *presence* ≠ *validation* — an attacker
  can add the header, so this is not a hard auth boundary. It is strictly better
  than blanket allow: drive-by / automated scanning (the bulk of WAF value) on
  the nopriv surface stays inspected, and a targeted attacker who already has a
  valid login cookie is an authenticated admin anyway.
- Default-on; `WordPressAllowlist` is the flag to wire a Security-tab toggle to
  later (not built — out of v1 scope).
