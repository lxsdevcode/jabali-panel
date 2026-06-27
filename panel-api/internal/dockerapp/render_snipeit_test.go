package dockerapp

import "testing"

func TestRender_SnipeIT(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	e, ok := cat.Get("snipe-it")
	if !ok {
		t.Fatal("snipe-it missing from catalog")
	}
	out, err := Render(e, RenderParams{
		Slug: "snipe-it", Name: "assets", Domain: "assets.example.com",
		ImageChannel: e.ImageChannel,
		DataRoot:     "/var/lib/jabali/docker-apps/snipe-it",
		CPULimit:     "1.5", MemoryLimit: "768m",
		Ports: map[string]RuntimePort{"http": {HostPort: 10090, ContainerPort: 80, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"APP_KEY": "base64:AAAA", "DB_PASSWORD": "dbpw", "MYSQL_ROOT_PASSWORD": "rootpw"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertNoUnresolved(t, out)
	for _, n := range []string{
		"container_name: jabali-app-snipe-it",
		`APP_URL: "https://assets.example.com"`,
		`APP_KEY: "base64:AAAA"`,
		`DB_PASSWORD: "dbpw"`,
		`MYSQL_ROOT_PASSWORD: "rootpw"`,
		`"127.0.0.1:10090:80/tcp"`,
		"/var/lib/jabali/docker-apps/snipe-it/db:/var/lib/mysql",
		"mariadb:11@sha256:",
	} {
		mustContain(t, out, n)
	}
}
