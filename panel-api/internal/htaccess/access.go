package htaccess

import (
	"net"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// flushAccess resolves the buffered Apache access-control directives (Order /
// Allow / Deny, and the Apache 2.4 Require form) into at most one ip_access
// rule, FAIL-CLOSED: anything ambiguous or not fully representable emits no
// rule plus a security warning, never a permissive half.
//
// Apache 2.2 semantics:
//   - `Order Allow,Deny`  -> default DENY; only explicitly Allowed pass
//     -> nginx allow_list (allow X…; deny all)
//   - `Order Deny,Allow`  -> default ALLOW; only explicitly Denied blocked
//     -> nginx deny_list (deny X…; allow all)
//
// Mixing Allow and Deny under one Order can't be expressed by nginx's flat
// allow/deny list, so it is dropped fail-closed.
func flushAccess(res *Result, st *state) {
	if len(st.access) == 0 {
		return
	}

	// Split the buffered lines by kind.
	var order string
	var allows, denies []string
	var requires [][]string
	firstLine := st.access[0].num
	for _, a := range st.access {
		switch a.directive {
		case "order":
			order = strings.ToLower(strings.Join(a.args, ""))
		case "allow":
			allows = append(allows, ipTokens(a.args)...)
		case "deny":
			denies = append(denies, ipTokens(a.args)...)
		case "require":
			requires = append(requires, a.args)
		}
	}

	// Apache 2.4 Require — handle the common, unambiguous forms; defer to the
	// classic Order path only if no Require lines are present.
	if len(requires) > 0 {
		flushRequire(res, st, requires, firstLine)
		return
	}

	src := "Order/Allow/Deny block"
	allDenied := containsAll(denies)  // `Deny from all`
	allAllowed := containsAll(allows) // `Allow from all`
	specificAllows := !allAllowed && len(allows) > 0
	specificDenies := !allDenied && len(denies) > 0

	switch order {
	case "allow,deny":
		// Default DENY; Allow is read first, Deny last and Deny WINS.
		switch {
		case allDenied:
			// `Deny from all` wins over any Allow -> deny everyone.
			res.addRule(denyAllRule(st.base))
		case len(allows) == 0:
			// Default-deny with no Allow -> nobody passes (any specific Deny
			// is redundant). Deny everyone.
			res.addRule(denyAllRule(st.base))
		case specificDenies:
			// Allow X but also Deny Y under default-deny: a flat nginx list
			// can't express "allow X minus Y". Drop fail-closed.
			res.warnSec(firstLine, src, "mixed Allow and specific Deny under `Order Allow,Deny` can't be expressed as one nginx allow/deny list — NOT applied; recreate it in the panel")
		case allAllowed:
			res.addNote("`Allow from all` under `Order Allow,Deny` allows everyone — no rule needed")
		default:
			ips, ok := cidrList(allows)
			if !ok {
				res.warnSec(firstLine, src, "Allow list has a hostname or unparseable entry — NOT applied (an omitted entry would silently change access)")
				return
			}
			res.addRule(allowListRule(st.base, ips))
		}

	case "deny,allow":
		// Default ALLOW; Deny is read first, Allow last and Allow WINS.
		switch {
		case allAllowed:
			res.addNote("`Allow from all` wins under `Order Deny,Allow` — allows everyone, no rule needed")
		case allDenied:
			if specificAllows {
				// `Deny from all` then `Allow from X` -> whitelist X.
				ips, ok := cidrList(allows)
				if !ok {
					res.warnSec(firstLine, src, "Allow list has a hostname or unparseable entry — NOT applied (an omitted allow would silently change access)")
					return
				}
				res.addRule(allowListRule(st.base, ips))
			} else {
				// `Deny from all` with no Allow -> deny everyone.
				res.addRule(denyAllRule(st.base))
			}
		case specificAllows && specificDenies:
			// default-allow, deny Y, allow X: Allow wins on overlap; a flat
			// list can't express it. Drop fail-closed.
			res.warnSec(firstLine, src, "mixed specific Allow and Deny under `Order Deny,Allow` can't be expressed as one nginx allow/deny list — NOT applied; recreate it in the panel")
		case specificDenies:
			ips, ok := cidrList(denies)
			if !ok {
				res.warnSec(firstLine, src, "Deny list has a hostname or unparseable entry — NOT applied (an omitted deny would silently grant access)")
				return
			}
			res.addRule(denyListRule(st.base, ips))
		default:
			res.addNote("`Order Deny,Allow` with no effective Deny — allows everyone, no rule needed")
		}

	default:
		// No Order, or an unrecognized one: refuse to guess the default
		// action — guessing wrong inverts the policy.
		res.warnSec(firstLine, src, "Allow/Deny without a clear `Order` — can't determine the default action, NOT applied (fail-closed)")
	}
}

// denyAllRule denies everyone for base. allow_list with no IPs compiles to a
// bare `deny all;`.
func denyAllRule(base string) models.NginxRule {
	return models.NginxRule{Type: "ip_access", Mode: "allow_list", Path: base, IPs: nil}
}

// allowListRule allows only ips (deny everyone else).
func allowListRule(base string, ips []string) models.NginxRule {
	return models.NginxRule{Type: "ip_access", Mode: "allow_list", Path: base, IPs: ips}
}

// denyListRule denies ips (allow everyone else).
func denyListRule(base string, ips []string) models.NginxRule {
	return models.NginxRule{Type: "ip_access", Mode: "deny_list", Path: base, IPs: ips}
}

// flushRequire handles the Apache 2.4 `Require` forms we can represent.
func flushRequire(res *Result, st *state, requires [][]string, lineNum int) {
	src := "Require block"
	var allowIPs []string
	for _, r := range requires {
		if len(r) == 0 {
			continue
		}
		switch strings.ToLower(r[0]) {
		case "all":
			if len(r) >= 2 && strings.EqualFold(r[1], "granted") {
				res.addNote("`Require all granted` is the default — no rule needed")
				continue
			}
			if len(r) >= 2 && strings.EqualFold(r[1], "denied") {
				// Deny everyone: allow_list with no IPs => `deny all;`.
				res.addRule(models.NginxRule{Type: "ip_access", Mode: "allow_list", Path: st.base, IPs: nil})
				continue
			}
			res.warnSec(lineNum, src, "unrecognized `Require all …` — dropped (fail-closed)")
			return
		case "ip":
			ips, ok := cidrList(r[1:])
			if !ok {
				res.warnSec(lineNum, src, "`Require ip` with an unparseable address — dropped (fail-closed)")
				return
			}
			allowIPs = append(allowIPs, ips...)
		case "valid-user", "user", "group":
			res.warnSec(lineNum, src, "`Require valid-user`/user/group is HTTP auth — set it up under the panel's Directory Privacy (htpasswd credentials are not stored in .htaccess and can't be migrated)")
			return
		default:
			res.warnSec(lineNum, src, "unsupported `Require "+r[0]+"` — dropped (fail-closed)")
			return
		}
	}
	if len(allowIPs) > 0 {
		res.addRule(models.NginxRule{Type: "ip_access", Mode: "allow_list", Path: st.base, IPs: allowIPs})
	}
}

// ipTokens drops a leading "from" keyword (Apache: `Allow from 1.2.3.4`).
func ipTokens(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.EqualFold(a, "from") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func containsAll(toks []string) bool {
	for _, t := range toks {
		if strings.EqualFold(t, "all") {
			return true
		}
	}
	return false
}

// cidrList validates every token as an IP or CIDR. Returns ok=false if ANY
// token is not (so the caller drops the whole block fail-closed rather than
// emit a partial list). "all" is skipped (handled separately by the caller).
func cidrList(toks []string) ([]string, bool) {
	var out []string
	for _, t := range toks {
		if strings.EqualFold(t, "all") {
			continue
		}
		if ip := net.ParseIP(t); ip != nil {
			out = append(out, t)
			continue
		}
		if _, _, err := net.ParseCIDR(t); err == nil {
			out = append(out, t)
			continue
		}
		// Apache allows partial-IP and bare-domain matches; nginx allow/deny
		// needs a real IP/CIDR. Treat anything else as unrepresentable.
		return nil, false
	}
	return out, true
}
