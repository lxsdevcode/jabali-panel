package dockerapp

import (
	"path/filepath"
	"runtime"
	"testing"
)

// repoCatalogDir returns the absolute path to install/docker-apps/
// regardless of where `go test` is invoked from. The runtime caller
// trick gives us the directory of this file in the source tree.
func repoCatalogDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller(0) failed")
	}
	// catalog_test.go lives at panel-api/internal/dockerapp/, so the
	// repo root is four directories up.
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "install", "docker-apps")
}

func TestCatalogLoad_RealCatalogParsesCleanly(t *testing.T) {
	root := repoCatalogDir(t)
	cat, errs := LoadDir(root)
	for _, e := range errs {
		t.Errorf("entry-load error: %v", e)
	}
	if cat.Len() < 3 {
		t.Errorf("expected at least 3 catalog entries (vaultwarden + uptime-kuma + gitea); got %d: %+v", cat.Len(), cat.All())
	}
}

func TestCatalogLoad_ExpectedEntries(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	for _, slug := range []string{"vaultwarden", "uptime-kuma", "gitea"} {
		e, ok := cat.Get(slug)
		if !ok {
			t.Errorf("catalog missing expected entry %q", slug)
			continue
		}
		if e.Slug != slug {
			t.Errorf("entry %q has wrong slug field: %q", slug, e.Slug)
		}
		if e.Name == "" {
			t.Errorf("entry %q has no display name", slug)
		}
		if e.ComposeTemplate() == "" {
			t.Errorf("entry %q has empty compose template", slug)
		}
		if len(e.Ports) == 0 {
			t.Errorf("entry %q declares no ports", slug)
		}
	}
}

func TestCatalogLoad_GiteaDeclaresTwoPorts(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	gitea, ok := cat.Get("gitea")
	if !ok {
		t.Skip("gitea entry missing; covered by sibling test")
	}
	if len(gitea.Ports) != 2 {
		t.Errorf("expected gitea to declare http+ssh (2 ports); got %d: %+v", len(gitea.Ports), gitea.Ports)
	}
	// http should be loopback+reverse_proxy=true; ssh should be public+reverse_proxy=false.
	for _, p := range gitea.Ports {
		switch p.Name {
		case "http":
			if !p.DefaultReverseProxy || p.DefaultBind != "loopback" {
				t.Errorf("gitea http port: expected loopback+reverse_proxy=true; got bind=%q rp=%v", p.DefaultBind, p.DefaultReverseProxy)
			}
		case "ssh":
			if p.DefaultReverseProxy || p.DefaultBind != "public" {
				t.Errorf("gitea ssh port: expected public+reverse_proxy=false; got bind=%q rp=%v", p.DefaultBind, p.DefaultReverseProxy)
			}
		default:
			t.Errorf("unexpected gitea port %q", p.Name)
		}
	}
}

func TestEntry_Validate_RejectsBadSlug(t *testing.T) {
	cases := []struct {
		name string
		slug string
		ok   bool
	}{
		{"valid", "vaultwarden", true},
		{"valid with dash", "uptime-kuma", true},
		{"too short", "a", false},
		{"starts with digit", "9abc", false},
		{"starts with dash", "-abc", false},
		{"contains uppercase", "Vaultwarden", false},
		{"contains underscore", "uptime_kuma", false},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := Entry{
				Slug:         tc.slug,
				Name:         "x",
				Version:      "0.1",
				Description:  "x",
				ImageChannel: "x:1@sha256:0000000000000000000000000000000000000000000000000000000000000000",
				Volumes:      []Volume{{Name: "data", ContainerPath: "/data"}},
				Ports: []PortSpec{
					{Name: "http", ContainerPort: 80, Protocol: "tcp", DefaultEnabled: true, DefaultBind: "loopback", DefaultReverseProxy: true},
				},
			}
			err := e.validate()
			if (err == nil) != tc.ok {
				t.Errorf("slug %q: got err=%v want ok=%v", tc.slug, err, tc.ok)
			}
		})
	}
}

func TestEntry_Validate_RejectsReverseProxyOnPublicBind(t *testing.T) {
	e := Entry{
		Slug:         "demo",
		Name:         "x",
		Version:      "0.1",
		Description:  "x",
		ImageChannel: "x:1@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Volumes:      []Volume{{Name: "data", ContainerPath: "/data"}},
		Ports: []PortSpec{
			{Name: "http", ContainerPort: 80, Protocol: "tcp", DefaultEnabled: true, DefaultBind: "public", DefaultReverseProxy: true},
		},
	}
	if err := e.validate(); err == nil {
		t.Error("expected reverse_proxy=true + public bind to be rejected")
	}
}

func TestEntry_Validate_RejectsReverseProxyOnUDP(t *testing.T) {
	e := Entry{
		Slug:         "demo",
		Name:         "x",
		Version:      "0.1",
		Description:  "x",
		ImageChannel: "x:1@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Volumes:      []Volume{{Name: "data", ContainerPath: "/data"}},
		Ports: []PortSpec{
			{Name: "udp1", ContainerPort: 53, Protocol: "udp", DefaultEnabled: true, DefaultBind: "loopback", DefaultReverseProxy: true},
		},
	}
	if err := e.validate(); err == nil {
		t.Error("expected reverse_proxy=true + UDP to be rejected")
	}
}
