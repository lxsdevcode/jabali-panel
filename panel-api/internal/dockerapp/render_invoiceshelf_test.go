package dockerapp

import "testing"

// InvoiceShelf — Laravel app + bundled MariaDB. Verifies the compose template
// renders end-to-end with generated secrets (APP_KEY/DB_PASSWORD), the
// reverse-proxy port binding, the https APP_URL, and the per-domain
// SESSION_DOMAIN / SANCTUM_STATEFUL_DOMAINS.
func TestRender_InvoiceShelf(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	a, ok := cat.Get("invoiceshelf")
	if !ok {
		t.Fatal("invoiceshelf missing from catalog")
	}
	out, err := Render(a, RenderParams{
		Slug: "invoiceshelf", Name: "billing", Domain: "invoice.example.com",
		ImageChannel: "invoiceshelf/invoiceshelf:2.4.1",
		DataRoot:     "/var/lib/jabali/docker-apps/invoiceshelf",
		CPULimit:     "1.0", MemoryLimit: "768m",
		Ports: map[string]RuntimePort{"http": {HostPort: 10090, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env: map[string]string{
			"DB_PASSWORD":         "dbpass",
			"MYSQL_ROOT_PASSWORD": "rootpass",
			"APP_KEY":             "abcdefghijklmnopqrstuvwxyz012345",
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertNoUnresolved(t, out)
	for _, n := range []string{
		"image: invoiceshelf/invoiceshelf:2.4.1",
		"image: mariadb:11",
		`APP_URL: "https://invoice.example.com"`,
		`APP_KEY: "abcdefghijklmnopqrstuvwxyz012345"`,
		`DB_CONNECTION: "mariadb"`,
		`DB_PASSWORD: "dbpass"`,
		`MYSQL_ROOT_PASSWORD: "rootpass"`,
		`SESSION_DOMAIN: "invoice.example.com"`,
		`SANCTUM_STATEFUL_DOMAINS: "invoice.example.com"`,
		`"127.0.0.1:10090:8080/tcp"`,
		"/var/lib/jabali/docker-apps/invoiceshelf/storage:/var/www/html/storage",
		"/var/lib/jabali/docker-apps/invoiceshelf/db:/var/lib/mysql",
	} {
		mustContain(t, out, n)
	}
}
