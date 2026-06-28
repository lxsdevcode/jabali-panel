package dockerapp

import "testing"

func TestRender_ERPNext(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	e, ok := cat.Get("erpnext")
	if !ok {
		t.Fatal("erpnext missing from catalog")
	}
	out, err := Render(e, RenderParams{
		Slug: "erpnext", Name: "erp", Domain: "erp.example.com",
		ImageChannel: e.ImageChannel,
		DataRoot:     "/var/lib/jabali/docker-apps/erpnext",
		CPULimit:     "2.0", MemoryLimit: "2g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10110, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"ADMIN_PASSWORD": "adminpw", "DB_ROOT_PASSWORD": "rootpw"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertNoUnresolved(t, out)
	for _, n := range []string{
		// frontend is the published service (container == slug)
		"container_name: jabali-app-erpnext",
		// the one-shot site bootstrap + its generated secrets
		"container_name: jabali-app-erpnext-create-site",
		`ADMIN_PASSWORD: "adminpw"`,
		`DB_ROOT_PASSWORD: "rootpw"`,
		"condition: service_completed_successfully",
		// reverse-proxied http bind
		`"127.0.0.1:10110:8080/tcp"`,
		// shared bench volume + db data
		"/var/lib/jabali/docker-apps/erpnext/sites:/home/frappe/frappe-bench/sites",
		"/var/lib/jabali/docker-apps/erpnext/db-data:/var/lib/mysql",
		// every sidecar image digest-pinned
		"mariadb:11.8@sha256:",
		"redis:7-alpine@sha256:",
	} {
		mustContain(t, out, n)
	}
}
