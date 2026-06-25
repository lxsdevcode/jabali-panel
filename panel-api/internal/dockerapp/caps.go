package dockerapp

import "strings"

// TenantBaselineCaps are the Linux capabilities every tenant container gets
// back after the M49 hardening's cap_drop:ALL, regardless of what the catalog
// entry declares. They cover the near-universal self-hosted-image entrypoint
// pattern of "chown the data dir, then drop from root to a runtime user" via
// su-exec / gosu / s6:
//
//   - CHOWN  — fix bind-mount ownership at startup
//   - SETUID — su-exec/gosu setuid() to the runtime user
//   - SETGID — the matching setgid()/setgroups()
//
// Without these such images crash-loop ("setgroups: Operation not permitted").
// They are safe to grant unconditionally under docker userns-remap (which
// tenant docker requires): capabilities are confined to the container's
// unprivileged user namespace and cannot affect the host or another tenant.
// (GH #284 — every tenant_installable catalog app shipped with no tenant_caps,
// so all su-exec/gosu apps were unrunnable.)
var TenantBaselineCaps = []string{"CHOWN", "SETUID", "SETGID"}

// TenantCapAllowlist returns the effective capability set for a tenant install:
// the baseline union the catalog-declared extras, normalised (upper-case, no
// CAP_ prefix) and de-duplicated, baseline first then extras in declared order.
// Used for BOTH the rendered cap_add and the agent-side validation allowlist so
// the two never disagree.
func TenantCapAllowlist(catalogCaps []string) []string {
	out := make([]string, 0, len(TenantBaselineCaps)+len(catalogCaps))
	seen := make(map[string]bool)
	add := func(c string) {
		c = strings.ToUpper(strings.TrimSpace(c))
		c = strings.TrimPrefix(c, "CAP_")
		if c == "" || seen[c] {
			return
		}
		seen[c] = true
		out = append(out, c)
	}
	for _, c := range TenantBaselineCaps {
		add(c)
	}
	for _, c := range catalogCaps {
		add(c)
	}
	return out
}
