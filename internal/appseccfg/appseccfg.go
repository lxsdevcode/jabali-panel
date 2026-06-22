// Package appseccfg is the single source of the
// crowdsecurity/jabali-appsec AppSec config (ADR-0083 shape;
// ADR-0102 follow-up). It replaces two hand-written copies — the
// bash heredoc in install.sh and the Go string template in
// panel-agent security_crowdsec.go (which fully regenerates the file
// on every geoblock Apply). Both now call Render so the schema, the
// ADR-0102 admin-API allowlist, and the geoblock pre_eval cannot
// drift.
//
// The inband-rule set is host-dependent (install.sh presence-gates it
// by stat'ing /etc/crowdsec/appsec-rules/*; the agent uses its fixed
// set) so it is a parameter, not baked in — only the template is
// single-sourced.
package appseccfg

import (
	"sort"
	"strings"
)

// Opts is everything a caller must supply. Inband is the presence-gated
// rule list (caller decides which files exist). Mode is
// "off"|"allow"|"deny"; Countries are ISO-3166-1 alpha-2 codes used by
// the geoblock pre_eval for allow/deny modes.
type Opts struct {
	Mode           string
	Countries      []string
	Inband         []string
	AdminAllowlist bool
	// WordPressAllowlist (GH #404) exempts the WordPress page-builder
	// endpoints that CRS false-positive-bans on legitimate admin edits
	// (Elementor save -> 942550 JSON-SQLi / 932370 RCE / 911100 method ->
	// 949110 anomaly block -> ~4h ban on the editor's IP). A page builder
	// POSTs arbitrary HTML/CSS/URLs in JSON, indistinguishable from
	// injection at the WAF, so a per-rule-ID exclusion is whack-a-mole.
	// Instead: plain-allow the plugin-namespaced /wp-json/elementor/ REST
	// (own permission callbacks, like our /api/v1/ bypass), and for the
	// dual-purpose /wp-admin/{admin-ajax,post}.php gate the exemption on
	// the WordPress login cookie so the UNAUTHENTICATED nopriv surface stays
	// fully inspected. Cookie presence != validation (an attacker can add
	// the header), so not a hard boundary, but it keeps drive-by/automated
	// scanning -- the bulk of WAF value -- inspected.
	WordPressAllowlist bool
	// WebmailHosts is the explicit allowlist of FQDNs exempted from
	// CrowdSec AppSec. These are the dedicated webmail/autodiscover
	// vhosts served exclusively by Bulwark + Stalwart, both of which
	// auth-gate their own traffic — CRS rule 911100 ("Method not
	// allowed by policy") otherwise 403s Bulwark's PUT /api/auth/
	// session, which the SPA needs to hydrate the impersonation
	// session after the JWT exchange.
	//
	// Caller MUST populate from a managed source (the panel-api
	// reconciler writes /var/lib/jabali-panel/webmail-hosts.list from the
	// domain repo every pass; render-config + agent both read that
	// file). MUST NOT be derived from the request — `req.Host
	// startsWith "mail."` is bypassable via arbitrary Host headers
	// reaching the default_server (which serves phpMyAdmin).
	WebmailHosts []string
}

// allowExpr/denyExpr reproduce panel-agent renderAppSecGeoblockRule
// EXACTLY (byte parity is the point of single-sourcing):
//
//	no countries  → allow `""`         deny ``
//	N countries    → allow `"A", "B", ""`  deny `"A", "B"`
func geoExpr(codes []string, allow bool) string {
	if len(codes) == 0 {
		if allow {
			return `""`
		}
		return ""
	}
	q := make([]string, len(codes))
	for i, c := range codes {
		q[i] = `"` + c + `"`
	}
	j := strings.Join(q, ", ")
	if allow {
		return j + `, ""`
	}
	return j
}

// sanitizeWebmailHosts dedupes, lowercases, and rejects anything that
// would break the YAML literal or the seclang expr. Valid hostnames are
// strictly [a-z0-9.-], 1-253 chars; anything else is dropped. Returns
// a stable-sorted slice for deterministic output (write-on-diff stays
// stable even when the source list comes back in random order).
func sanitizeWebmailHosts(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, h := range in {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || len(h) > 253 {
			continue
		}
		valid := true
		for _, r := range h {
			if !(r == '.' || r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		// Reject empty labels, leading/trailing dots and double dots.
		if strings.HasPrefix(h, ".") || strings.HasSuffix(h, ".") || strings.Contains(h, "..") {
			continue
		}
		if _, dup := seen[h]; dup {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// CRSPluginBeforePath is the jabali-owned CRS "before" plugin file.
// crowdsecurity/crs globs crs-plugins/*/*-before.conf (via its
// seclang_files_rules) and runs them BEFORE the CRS detection rules
// (REQUEST-933 PHP-injection, REQUEST-949 anomaly blocking), which is
// exactly where a targeted exclusion must sit. The jabali/ subdir is
// not hub-managed, so it survives crowdsec hub updates; render-config
// rewrites it on every install + `jabali update`.
const CRSPluginBeforePath = "/var/lib/crowdsec/data/crs-plugins/jabali/jabali-before.conf"

// CRSPluginBefore returns the body of the jabali CRS "before" plugin —
// targeted CrowdSec AppSec / CRS false-positive exclusions that must run
// before the detection + anomaly-scoring rules.
//
// Jabali CRS-plugin SecRule id range: 9,599,000–9,599,999.
//
// Keep every exclusion as NARROW as the evidence: prefer
// ctl:ruleRemoveTargetById=<rule>;ARGS:<arg> (drop one rule's
// inspection of one argument) over ruleRemoveById (kills the rule
// everywhere) or a path-allow (kills the whole WAF for that path).
func CRSPluginBefore() string {
	return `# Managed by jabali — CRS "before" exclusion plugin.
# DO NOT hand-edit. Written by ` + "`jabali appsec render-config`" + `.
# Loaded by crowdsecurity/crs via the crs-plugins/*/*-before.conf glob,
# BEFORE REQUEST-933 (PHP injection) + REQUEST-949 (anomaly blocking).
#
# 933120 vs WordPress admin search: editing a custom post type issues
# /wp-admin/edit.php?...&_wp_http_referer=<double-URL-encoded URL>. The
# nested URL-inside-a-URL trips CRS 933120 (PHP injection) → php_injection
# score 10 >= anomaly threshold 5 → 949110 blocks → CrowdSec AppSec 4h
# ban. False positive on legit logged-in admin traffic.
#
# Drop ONLY 933120's inspection of ONLY the _wp_http_referer arg, ONLY
# under /wp-admin/. Body inspection, every other rule, and 933120 on
# every other arg/path all stay active.
SecRule REQUEST_URI "@beginsWith /wp-admin/"     "id:9599100,phase:1,pass,nolog,ctl:ruleRemoveTargetById=933120;ARGS:_wp_http_referer"
`
}

// Render returns the full jabali-appsec.yaml body. Deterministic:
// header(+inband) → on_match (ADR-0102) → pre_eval (geoblock).
func Render(o Opts) string {
	mode := o.Mode
	if mode == "" {
		mode = "off"
	}
	csv := strings.Join(o.Countries, ",")

	var b strings.Builder
	b.WriteString("# Managed by jabali — M27 AppSec config.\n")
	b.WriteString("# DO NOT hand-edit. Set via the admin Security → CrowdSec tab OR\n")
	b.WriteString("# POST /api/v1/admin/security/crowdsec/appsec/geoblock.\n")
	b.WriteString("# jabali-mode: " + mode + "\n")
	b.WriteString("# jabali-countries: " + csv + "\n")
	b.WriteString("name: crowdsecurity/jabali-appsec\n")
	b.WriteString("default_remediation: ban\n")
	b.WriteString("inband_rules:\n")
	for _, r := range o.Inband {
		b.WriteString(" - " + r + "\n")
	}

	// ADR-0102 (amended 2026-05-19): the ENTIRE panel API (/api/v1/)
	// is Kratos-session-gated, same-origin SPA control plane — not
	// public web attack surface. The CRS generic ruleset false-
	// positives on legitimate REST: rule 911100 "Method not allowed
	// by policy" blocks every PATCH/PUT/DELETE (DNS records, etc.),
	// and body-inspection flags JSON/ULID payloads. Narrowing to
	// /api/v1/admin/ left the SPA's own mutations (e.g.
	// PATCH /api/v1/dns/records/:id) WAF-blocked with an opaque 403.
	// Exempt the whole prefix; public vhosts keep full AppSec.
	//
	// M6.6 follow-up (2026-06-01): the webmail vhosts (mail.<dom> /
	// autoconfig.<dom>) are served exclusively by Bulwark + Stalwart,
	// both of which auth-gate their own traffic. CRS rule 911100
	// blocks Bulwark's PUT /api/auth/session — the SPA's session
	// rehydration call AFTER /api/auth/impersonate sets cookies — so
	// every webmail SSO landed on a logged-out SPA (symptom: "SSO not
	// working" even though the JWT was accepted and impersonation
	// cookies were set). JMAP batch POSTs also trip body-inspector
	// false-positives. Allowlist these hostnames with EXPLICIT
	// equality, never a `startsWith` prefix — req.Host is a client-
	// controlled header and nginx falls back to default_server (which
	// serves phpMyAdmin) when no vhost matches.
	hosts := sanitizeWebmailHosts(o.WebmailHosts)
	if o.AdminAllowlist || o.WordPressAllowlist || len(hosts) > 0 {
		b.WriteString("on_match:\n")
	}
	if o.AdminAllowlist {
		b.WriteString(` - filter: req.URL.Path startsWith "/api/v1/"
   apply:
    - CancelEvent()
    - CancelAlert()
    - SetRemediation("allow")
`)
	}
	if o.WordPressAllowlist {
		b.WriteString(` - filter: req.URL.Path startsWith "/wp-json/elementor/" || (req.URL.Path == "/wp-admin/admin-ajax.php" && any(req.Cookies(), {.Name startsWith "wordpress_logged_in_"})) || (req.URL.Path == "/wp-admin/post.php" && any(req.Cookies(), {.Name startsWith "wordpress_logged_in_"}))
   apply:
    - CancelEvent()
    - CancelAlert()
    - SetRemediation("allow")
`)
	}
	if len(hosts) > 0 {
		parts := make([]string, len(hosts))
		for i, h := range hosts {
			parts[i] = `req.Host == "` + h + `"`
		}
		b.WriteString(" - filter: " + strings.Join(parts, " || ") + "\n")
		b.WriteString("   apply:\n")
		b.WriteString("    - CancelEvent()\n")
		b.WriteString("    - CancelAlert()\n")
		b.WriteString("    - SetRemediation(\"allow\")\n")
	}

	// Geoblock pre_eval (ADR-0060). off = inert (no block).
	switch mode {
	case "allow":
		// allow-list: drop everything NOT in the set; the trailing ""
		// keeps requests whose GeoIP lookup yields no country (local /
		// private ranges) reachable.
		b.WriteString(`pre_eval:
 - filter: IsInBand == true && GeoIPEnrich(req.RemoteAddr)?.Country.IsoCode not in [` + geoExpr(o.Countries, true) + `]
   apply:
    - DropRequest("Forbidden Country (jabali allow-list)")
`)
	case "deny":
		b.WriteString(`pre_eval:
 - filter: IsInBand == true && GeoIPEnrich(req.RemoteAddr)?.Country.IsoCode in [` + geoExpr(o.Countries, false) + `]
   apply:
    - DropRequest("Forbidden Country (jabali deny-list)")
`)
	}
	return b.String()
}
