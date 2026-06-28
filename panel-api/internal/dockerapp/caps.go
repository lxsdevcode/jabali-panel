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

// tenantForbiddenCaps are capabilities NEVER granted to a tenant container,
// even if a catalog entry declares them in tenant_caps (Gitea #515). The
// catalog is the policy boundary, so a single metadata mistake must not be able
// to hand a tenant a host-affecting capability. These either escape userns
// confinement, touch the host kernel/network/devices, or defeat the sandbox.
var tenantForbiddenCaps = map[string]bool{
	"SYS_ADMIN": true, "SYS_MODULE": true, "SYS_RAWIO": true, "SYS_PTRACE": true,
	"SYS_BOOT": true, "SYS_TIME": true, "NET_ADMIN": true, "MAC_ADMIN": true,
	"MAC_OVERRIDE": true, "DAC_READ_SEARCH": true, "LINUX_IMMUTABLE": true,
	"AUDIT_CONTROL": true, "BPF": true, "PERFMON": true, "SYSLOG": true,
	"SYS_PACCT": true, "WAKE_ALARM": true, "BLOCK_SUSPEND": true,
}

// normalizeCapName upper-cases + strips the CAP_ prefix + trims.
func normalizeCapName(c string) string {
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(c)), "CAP_")
}

// IsForbiddenTenantCap reports whether a capability may never be granted to a
// tenant container. Used by catalog validation (reject at load) and the
// allowlist builder (drop at runtime). Accepts CAP_-prefixed or bare names.
func IsForbiddenTenantCap(c string) bool {
	return tenantForbiddenCaps[normalizeCapName(c)]
}

// TenantCapAllowlist returns the effective capability set for a tenant install:
// the baseline union the catalog-declared extras, normalised (upper-case, no
// CAP_ prefix) and de-duplicated, baseline first then extras in declared order.
// Used for BOTH the rendered cap_add and the agent-side validation allowlist so
// the two never disagree.
func TenantCapAllowlist(catalogCaps []string) []string {
	out := make([]string, 0, len(TenantBaselineCaps)+len(catalogCaps))
	seen := make(map[string]bool)
	add := func(c string) {
		c = normalizeCapName(c)
		// Drop forbidden caps even if the catalog declared them (Gitea #515) —
		// defence in depth behind catalog-load validation.
		if c == "" || seen[c] || tenantForbiddenCaps[c] {
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
