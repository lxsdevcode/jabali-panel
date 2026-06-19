package dockerapp

import "testing"

// Bigcapital — multi-service accounting platform (envoy proxy + NestJS
// server + React webapp + MariaDB + Redis + MinIO + Gotenberg + one-shot
// migration/createbucket). Verifies the template renders end-to-end with
// generated secrets, the inline envoy + db-grant configs, the https
// BASE_URL, the S3/MinIO wiring, and the migration command path.
func TestRender_Bigcapital(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	a, ok := cat.Get("bigcapital")
	if !ok {
		t.Fatal("bigcapital missing from catalog")
	}
	out, err := Render(a, RenderParams{
		Slug: "bigcapital", Name: "acct", Domain: "books.example.com",
		ImageChannel: "bigcapitalhq/server:v0.25.20",
		DataRoot:     "/var/lib/jabali/docker-apps/bigcapital",
		CPULimit:     "2.0", MemoryLimit: "2g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10092, ContainerPort: 80, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env: map[string]string{
			"DB_PASSWORD":         "dbpass",
			"DB_ROOT_PASSWORD":    "rootpass",
			"JWT_SECRET":          "jwtsecret",
			"MINIO_ROOT_PASSWORD": "miniopass",
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertNoUnresolved(t, out)
	for _, n := range []string{
		"image: bigcapitalhq/server:v0.25.20",
		"image: bigcapitalhq/webapp:v0.25.20",
		"image: envoyproxy/envoy:v1.30-latest",
		"image: minio/minio:latest",
		"image: gotenberg/gotenberg:7",
		"image: mariadb:11",
		"image: redis:7",
		// inline configs
		"envoy_config:",
		"cluster: dynamic_server",
		"db_grant:",
		"GRANT ALL PRIVILEGES ON *.* TO 'bigcapital'@'%'",
		"target: /etc/envoy/envoy.yaml",
		"target: /docker-entrypoint-initdb.d/10-grant.sql",
		// one-shot ordering
		"condition: service_completed_successfully",
		"cd packages/server && node dist/cli.js system:migrate:latest",
		"mc mb --ignore-existing bc/bigcapital",
		// wiring
		`BASE_URL: "https://books.example.com"`,
		`JWT_SECRET: "jwtsecret"`,
		`S3_ENDPOINT: "http://minio:9000"`,
		`S3_SECRET_ACCESS_KEY: "miniopass"`,
		`MYSQL_PASSWORD: "dbpass"`,
		`"127.0.0.1:10092:80/tcp"`,
		"/var/lib/jabali/docker-apps/bigcapital/db:/var/lib/mysql",
		"/var/lib/jabali/docker-apps/bigcapital/minio:/data",
	} {
		mustContain(t, out, n)
	}
}
