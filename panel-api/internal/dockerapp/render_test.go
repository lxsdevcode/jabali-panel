package dockerapp

import (
	"strings"
	"testing"
)

func TestRender_VaultwardenSingleHTTPPort(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	vw, ok := cat.Get("vaultwarden")
	if !ok {
		t.Fatal("vaultwarden missing from catalog")
	}
	out, err := Render(vw, RenderParams{
		Slug:         "vaultwarden",
		Name:         "vault-prod",
		Domain:       "vault.example.com",
		ImageChannel: "vaultwarden/server:latest",
		DataRoot:     "/var/lib/jabali/docker-apps/vaultwarden",
		CPULimit:     "0.5",
		MemoryLimit:  "256m",
		Ports: map[string]RuntimePort{
			"http": {HostPort: 10001, ContainerPort: 80, BindInterface: "127.0.0.1", Protocol: "tcp"},
		},
		Env: map[string]string{
			"SIGNUPS_ALLOWED": "false",
			"ADMIN_TOKEN":     "secret123",
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, needle := range []string{
		"image: vaultwarden/server:latest",
		"container_name: jabali-app-vaultwarden",
		`DOMAIN: "https://vault.example.com"`,
		`"127.0.0.1:10001:80/tcp"`,
		"/var/lib/jabali/docker-apps/vaultwarden/data:/data",
		`SIGNUPS_ALLOWED: "false"`,
		`ADMIN_TOKEN: "secret123"`,
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("rendered compose missing %q. full output:\n%s", needle, out)
		}
	}
}

func TestRender_GiteaTwoPortsBothPublished(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	gitea, ok := cat.Get("gitea")
	if !ok {
		t.Fatal("gitea missing from catalog")
	}
	out, err := Render(gitea, RenderParams{
		Slug:         "gitea",
		Name:         "git",
		Domain:       "git.example.com",
		ImageChannel: "gitea/gitea:1.22",
		DataRoot:     "/var/lib/jabali/docker-apps/gitea",
		CPULimit:     "1.0",
		MemoryLimit:  "512m",
		Ports: map[string]RuntimePort{
			"http": {HostPort: 10010, ContainerPort: 3000, BindInterface: "127.0.0.1", Protocol: "tcp"},
			"ssh":  {HostPort: 222, ContainerPort: 22, BindInterface: "203.0.113.5", Protocol: "tcp"},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, needle := range []string{
		`"127.0.0.1:10010:3000/tcp"`,
		`"203.0.113.5:222:22/tcp"`,
		"/var/lib/jabali/docker-apps/gitea/data:/data",
		"/var/lib/jabali/docker-apps/gitea/config:/etc/gitea",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("rendered compose missing %q. full output:\n%s", needle, out)
		}
	}
}

func TestMaterialiseEnv_GeneratesPassword64WhenMissing(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	vw, _ := cat.Get("vaultwarden")
	env, err := MaterialiseEnv(vw, nil)
	if err != nil {
		t.Fatalf("MaterialiseEnv: %v", err)
	}
	if v, ok := env["SIGNUPS_ALLOWED"]; !ok || v != "false" {
		t.Errorf("SIGNUPS_ALLOWED: expected catalog default 'false'; got %q", v)
	}
	if v, ok := env["ADMIN_TOKEN"]; !ok || len(v) < 40 {
		t.Errorf("ADMIN_TOKEN: expected ~64-char generated secret; got %q (len=%d)", v, len(v))
	}
}

func TestMaterialiseEnv_OverrideWinsOverCatalog(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	vw, _ := cat.Get("vaultwarden")
	env, err := MaterialiseEnv(vw, map[string]string{
		"SIGNUPS_ALLOWED": "true",
		"ADMIN_TOKEN":     "operator-supplied-token",
	})
	if err != nil {
		t.Fatalf("MaterialiseEnv: %v", err)
	}
	if v := env["SIGNUPS_ALLOWED"]; v != "true" {
		t.Errorf("operator override SIGNUPS_ALLOWED ignored; got %q", v)
	}
	if v := env["ADMIN_TOKEN"]; v != "operator-supplied-token" {
		t.Errorf("operator override ADMIN_TOKEN ignored; got %q", v)
	}
}
