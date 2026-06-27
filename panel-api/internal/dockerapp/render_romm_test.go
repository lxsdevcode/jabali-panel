package dockerapp

import "testing"

func TestRender_RomM(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	e, ok := cat.Get("romm")
	if !ok {
		t.Fatal("romm missing from catalog")
	}
	out, err := Render(e, RenderParams{
		Slug: "romm", Name: "games", Domain: "roms.example.com",
		ImageChannel: e.ImageChannel,
		DataRoot:     "/var/lib/jabali/docker-apps/romm",
		CPULimit:     "1.5", MemoryLimit: "1g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10092, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"DB_PASSWD": "dbpw", "MYSQL_ROOT_PASSWORD": "rootpw", "ROMM_AUTH_SECRET_KEY": "authkey"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertNoUnresolved(t, out)
	for _, n := range []string{
		"container_name: jabali-app-romm",
		`ROMM_DB_DRIVER: "mariadb"`,
		`ROMM_AUTH_SECRET_KEY: "authkey"`,
		"/var/lib/jabali/docker-apps/romm/library:/romm/library",
		`"127.0.0.1:10092:8080/tcp"`,
		"mariadb:11@sha256:",
	} {
		mustContain(t, out, n)
	}
}
