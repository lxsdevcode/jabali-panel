package dockerapp

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// M49 (GH #170): tenant installs get a hardening profile injected into every
// service — cap_drop ALL + the verified allowlist, no-new-privileges, pids cap,
// and cgroup_parent nesting under the tenant's M18 slice.
func TestRender_TenantHardening(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	app, ok := cat.Get("searxng")
	if !ok {
		t.Fatal("searxng missing from catalog")
	}
	out, err := Render(app, RenderParams{
		Slug: "searxng", Name: "search", Domain: "s.example.com",
		ImageChannel: "docker.io/searxng/searxng:2026.6.20-fd42d4fda",
		DataRoot:     "/var/lib/jabali/docker-apps/searxng-u01-search",
		CPULimit:     "1.0", MemoryLimit: "512m",
		Ports: map[string]RuntimePort{"http": {HostPort: 10010, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"SEARXNG_SECRET": "s3cr3t"},
		TenantHardening: &TenantHardening{
			CgroupParent: "jabali-user-alice.slice",
			Caps:         []string{"CHOWN", "SETUID", "SETGID"},
			PIDsLimit:    200,
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Parse the result and assert per-service hardening landed.
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("hardened compose not valid YAML: %v\n%s", err, out)
	}
	svcs := doc["services"].(map[string]any)
	if len(svcs) == 0 {
		t.Fatal("no services after hardening")
	}
	for name, sRaw := range svcs {
		s := sRaw.(map[string]any)
		if got := s["security_opt"]; got == nil || !strings.Contains(out, "no-new-privileges:true") {
			t.Errorf("service %q missing no-new-privileges", name)
		}
		if s["cap_drop"] == nil {
			t.Errorf("service %q missing cap_drop", name)
		}
		if s["cap_add"] == nil {
			t.Errorf("service %q missing cap_add allowlist", name)
		}
		if s["cgroup_parent"] != "jabali-user-alice.slice" {
			t.Errorf("service %q cgroup_parent = %v", name, s["cgroup_parent"])
		}
		// pids must live under deploy.resources.limits.pids, NOT the legacy
		// top-level pids_limit key (compose v5 conflict — GH #284).
		if s["pids_limit"] != nil {
			t.Errorf("service %q must not emit legacy top-level pids_limit: %v", name, s["pids_limit"])
		}
		deploy, _ := s["deploy"].(map[string]any)
		res, _ := deploy["resources"].(map[string]any)
		lim, _ := res["limits"].(map[string]any)
		if lim == nil || lim["pids"] != 200 {
			t.Errorf("service %q deploy.resources.limits.pids = %v, want 200", name, lim["pids"])
		}
	}
	// Secret + the env value must survive the re-marshal.
	if !strings.Contains(out, "s3cr3t") {
		t.Errorf("env value lost in re-marshal:\n%s", out)
	}
}

// Admin installs (nil TenantHardening) must be byte-for-byte the template
// output — no hardening, no YAML round-trip.
func TestRender_NoHardeningWhenNil(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	app, _ := cat.Get("searxng")
	out, err := Render(app, RenderParams{
		Slug: "searxng", Name: "s", Domain: "s.example.com",
		ImageChannel: "docker.io/searxng/searxng:x", DataRoot: "/d",
		CPULimit: "1.0", MemoryLimit: "512m",
		Ports: map[string]RuntimePort{"http": {HostPort: 10010, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"SEARXNG_SECRET": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "cap_drop") || strings.Contains(out, "cgroup_parent") {
		t.Errorf("admin render must not contain hardening:\n%s", out)
	}
}
