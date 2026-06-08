package dockerapp

import (
	"strings"
	"testing"
)

// TestRender_ZitadelIdP renders the zitadel catalog entry end-to-end so the
// template's {{ index .Env … }} / {{ .Domain }} / port range can't silently
// regress to "<no value>" or fail to parse. Also locks the two behaviours
// the entry depends on for correctness behind the nginx vhost: the https
// external-domain wiring and the password-policy suffix on the first admin.
func TestRender_ZitadelIdP(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	z, ok := cat.Get("zitadel")
	if !ok {
		t.Fatal("zitadel missing from catalog")
	}
	out, err := Render(z, RenderParams{
		Slug:         "zitadel",
		Name:         "id",
		Domain:       "id.example.com",
		ImageChannel: "ghcr.io/zitadel/zitadel:v4.15.0",
		DataRoot:     "/var/lib/jabali/docker-apps/zitadel",
		CPULimit:     "1.0",
		MemoryLimit:  "1g",
		Ports: map[string]RuntimePort{
			"http": {HostPort: 10042, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"},
		},
		Env: map[string]string{
			"MASTERKEY":         "x0123456789abcdef0123456789abcd", // 32 chars
			"POSTGRES_PASSWORD": "pgsecret",
			"ADMIN_PASSWORD":    "baseadminpw",
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "<no value>") {
		t.Fatalf("template left an unresolved variable:\n%s", out)
	}
	for _, needle := range []string{
		"image: ghcr.io/zitadel/zitadel:v4.15.0",
		"container_name: jabali-app-zitadel",
		"command: start-from-init --masterkeyFromEnv --tlsMode external",
		`ZITADEL_MASTERKEY: "x0123456789abcdef0123456789abcd"`,
		`ZITADEL_EXTERNALDOMAIN: "id.example.com"`,
		`ZITADEL_EXTERNALSECURE: "true"`,
		`ZITADEL_TLS_ENABLED: "false"`,
		`ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_REQUIRED: "false"`,
		`ZITADEL_DATABASE_POSTGRES_USER_PASSWORD: "pgsecret"`,
		// admin password carries the policy-guaranteeing suffix
		`ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD: "baseadminpwAa1!"`,
		`ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORDCHANGEREQUIRED: "true"`,
		`"127.0.0.1:10042:8080/tcp"`,
		"/var/lib/jabali/docker-apps/zitadel/postgres:/var/lib/postgresql/data",
		"image: postgres:17-alpine",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("rendered compose missing %q.\nfull output:\n%s", needle, out)
		}
	}
}

// TestZitadelMasterkeyIsExactly32 guards the hard ZITADEL constraint: the
// masterkey must be exactly 32 chars. password32 (base64url of 24 bytes)
// renders to exactly 32 — if the generator scheme ever changes, this breaks
// loudly here instead of at a tenant's failed container init.
func TestZitadelMasterkeyIsExactly32(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	z, ok := cat.Get("zitadel")
	if !ok {
		t.Fatal("zitadel missing from catalog")
	}
	env, err := MaterialiseEnv(z, nil)
	if err != nil {
		t.Fatalf("MaterialiseEnv: %v", err)
	}
	if got := len(env["MASTERKEY"]); got != 32 {
		t.Errorf("ZITADEL_MASTERKEY length = %d, want exactly 32 (zitadel rejects any other length)", got)
	}
}
