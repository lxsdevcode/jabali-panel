package dockerapp

import "testing"

func TestRender_PeerTube(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	e, ok := cat.Get("peertube")
	if !ok {
		t.Fatal("peertube missing from catalog")
	}
	out, err := Render(e, RenderParams{
		Slug: "peertube", Name: "videos", Domain: "tube.example.com",
		ImageChannel: e.ImageChannel,
		DataRoot:     "/var/lib/jabali/docker-apps/peertube",
		CPULimit:     "2.0", MemoryLimit: "2g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10093, ContainerPort: 9000, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"DB_PASSWORD": "dbpw", "PEERTUBE_SECRET": "secret64"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertNoUnresolved(t, out)
	for _, n := range []string{
		"container_name: jabali-app-peertube",
		`PEERTUBE_WEBSERVER_HOSTNAME: "tube.example.com"`,
		`PEERTUBE_SECRET: "secret64"`,
		`PEERTUBE_DB_PASSWORD: "dbpw"`,
		`PEERTUBE_REDIS_HOSTNAME: "redis"`,
		`"127.0.0.1:10093:9000/tcp"`,
		"postgres:17-alpine@sha256:",
		"redis:7-alpine@sha256:",
	} {
		mustContain(t, out, n)
	}
}
