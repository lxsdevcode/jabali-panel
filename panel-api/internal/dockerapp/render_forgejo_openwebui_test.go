package dockerapp

import (
	"strings"
	"testing"
)

// TestRender_Forgejo locks the GH #250 Forgejo app: both ports render when
// SSH is enabled, the data + config volumes mount, and the health probe is
// the gitea-lineage /api/healthz.
func TestRender_Forgejo(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	app, ok := cat.Get("forgejo")
	if !ok {
		t.Fatal("forgejo missing from catalog")
	}
	out, err := Render(app, RenderParams{
		Slug: "forgejo", Name: "git", Domain: "git.example.com",
		ImageChannel: "codeberg.org/forgejo/forgejo:15.0.3",
		DataRoot:     "/var/lib/jabali/docker-apps/forgejo",
		CPULimit:     "1.0", MemoryLimit: "512m",
		Ports: map[string]RuntimePort{
			"http": {HostPort: 10020, ContainerPort: 3000, BindInterface: "127.0.0.1", Protocol: "tcp"},
			"ssh":  {HostPort: 2222, ContainerPort: 22, BindInterface: "203.0.113.5", Protocol: "tcp"},
		},
		Env: map[string]string{"USER_UID": "1000", "USER_GID": "1000"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"codeberg.org/forgejo/forgejo:15.0.3",
		"127.0.0.1:10020:3000",
		"203.0.113.5:2222:22",
		"/api/healthz",
		"/data",
		"/etc/gitea",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered compose missing %q\n%s", want, out)
		}
	}
}

// TestRender_OpenWebUI locks the GH #249 Open WebUI app: the secret env
// interpolates, the public URL uses the install domain, and the port binds
// loopback.
func TestRender_OpenWebUI(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	app, ok := cat.Get("open-webui")
	if !ok {
		t.Fatal("open-webui missing from catalog")
	}
	out, err := Render(app, RenderParams{
		Slug: "open-webui", Name: "ai", Domain: "ai.example.com",
		ImageChannel: "ghcr.io/open-webui/open-webui:0.9.6",
		DataRoot:     "/var/lib/jabali/docker-apps/open-webui",
		CPULimit:     "2.0", MemoryLimit: "2g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10030, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"WEBUI_SECRET_KEY": "k3y-value"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		`WEBUI_SECRET_KEY: "k3y-value"`,
		`WEBUI_URL: "https://ai.example.com/"`,
		"127.0.0.1:10030:8080",
		"/app/backend/data",
		"/health",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered compose missing %q\n%s", want, out)
		}
	}
}
