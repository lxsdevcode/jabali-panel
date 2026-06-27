package dockerapp

import "testing"

func TestRender_OpenEMR(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	e, ok := cat.Get("openemr")
	if !ok {
		t.Fatal("openemr missing from catalog")
	}
	out, err := Render(e, RenderParams{
		Slug: "openemr", Name: "emr", Domain: "emr.example.com",
		ImageChannel: e.ImageChannel,
		DataRoot:     "/var/lib/jabali/docker-apps/openemr",
		CPULimit:     "1.5", MemoryLimit: "1g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10091, ContainerPort: 80, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"MYSQL_PASS": "dbpw", "MYSQL_ROOT_PASSWORD": "rootpw", "OE_PASS": "adminpw"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertNoUnresolved(t, out)
	for _, n := range []string{
		"container_name: jabali-app-openemr",
		`MYSQL_HOST: "db"`,
		`OE_PASS: "adminpw"`,
		`MYSQL_PASS: "dbpw"`,
		`"127.0.0.1:10091:80/tcp"`,
		"mariadb:11@sha256:",
	} {
		mustContain(t, out, n)
	}
}
