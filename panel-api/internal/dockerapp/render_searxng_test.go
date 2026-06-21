package dockerapp

import (
	"strings"
	"testing"
)

// TestRender_SearXNG locks the GH #251 SearXNG app: the secret env interpolates,
// the base URL uses the install domain, and the healthcheck hits /healthz.
func TestRender_SearXNG(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	app, ok := cat.Get("searxng")
	if !ok {
		t.Fatal("searxng missing from catalog")
	}
	out, err := Render(app, RenderParams{
		Slug: "searxng", Name: "search", Domain: "search.example.com",
		ImageChannel: "docker.io/searxng/searxng:2026.6.20-fd42d4fda",
		DataRoot:     "/var/lib/jabali/docker-apps/searxng",
		CPULimit:     "1.0", MemoryLimit: "512m",
		Ports: map[string]RuntimePort{"http": {HostPort: 10010, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"SEARXNG_SECRET": "s3cr3t-value"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		`SEARXNG_SECRET: "s3cr3t-value"`,
		`SEARXNG_BASE_URL: "https://search.example.com/"`,
		"/healthz",
		"127.0.0.1:10010:8080",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered compose missing %q\n%s", want, out)
		}
	}
}
