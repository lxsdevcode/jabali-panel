package dockerapp

import (
	"strings"
	"testing"
)

// TestRender_LinkwardenHealthcheckUsesCurl locks the fix for the "can't
// upgrade" bug: the linkwarden image ships curl, not wget. A wget healthcheck
// returned "wget: not found" on every probe → the container sat unhealthy
// forever → the update watchdog failed to converge within 5m. Guard against a
// revert to wget.
func TestRender_LinkwardenHealthcheckUsesCurl(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	lw, ok := cat.Get("linkwarden")
	if !ok {
		t.Fatal("linkwarden missing from catalog")
	}
	out, err := Render(lw, RenderParams{
		Slug: "linkwarden", Name: "links", Domain: "links.example.com",
		ImageChannel: "ghcr.io/linkwarden/linkwarden:latest",
		DataRoot:     "/var/lib/jabali/docker-apps/linkwarden",
		CPULimit:     "1.5", MemoryLimit: "1g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10000, ContainerPort: 3000, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Check the exact probe commands, not a bare "wget" substring — the
	// template comment legitimately mentions wget while explaining the fix.
	if strings.Contains(out, "wget --spider -q http://127.0.0.1:3000/") {
		t.Errorf("linkwarden still uses the wget probe (not in the image):\n%s", out)
	}
	if !strings.Contains(out, "curl -fsS --max-time 5 -o /dev/null http://127.0.0.1:3000/") {
		t.Errorf("linkwarden healthcheck should use the curl probe (present in the image); got:\n%s", out)
	}
}
