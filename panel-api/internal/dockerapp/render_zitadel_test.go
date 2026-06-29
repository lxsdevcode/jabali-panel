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
		`ZITADEL_FIRSTINSTANCE_ORG_HUMAN_EMAIL_VERIFIED: "true"`,
		`ZITADEL_FIRSTINSTANCE_ORG_HUMAN_EMAIL_ADDRESS: "admin@id.example.com"`,
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

// TestRender_ZitadelSMTP_OmittedWhenUnset confirms the GH #322 SMTP block is a
// no-op when the operator supplies no SMTP_HOST: no SMTPCONFIGURATION lines and
// no unresolved template var.
func TestRender_ZitadelSMTP_OmittedWhenUnset(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	z, _ := cat.Get("zitadel")
	out, err := Render(z, RenderParams{
		Slug: "zitadel", Name: "id", Domain: "id.example.com",
		ImageChannel: "img", DataRoot: "/d", CPULimit: "1.0", MemoryLimit: "1g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10042, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env:   map[string]string{"MASTERKEY": "x0123456789abcdef0123456789abcd", "POSTGRES_PASSWORD": "p", "ADMIN_PASSWORD": "a"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "SMTPCONFIGURATION") {
		t.Errorf("SMTP block should be omitted when SMTP_HOST unset:\n%s", out)
	}
	if strings.Contains(out, "<no value>") {
		t.Errorf("unresolved template var:\n%s", out)
	}
}

// TestRender_ZitadelSMTP_WiredWhenSet confirms operator-supplied SMTP_* env
// (GH #322) maps to ZITADEL's DefaultInstance SMTP config, including the
// host:port join and the sender-address-match opt-out.
func TestRender_ZitadelSMTP_WiredWhenSet(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	z, _ := cat.Get("zitadel")
	out, err := Render(z, RenderParams{
		Slug: "zitadel", Name: "id", Domain: "id.example.com",
		ImageChannel: "img", DataRoot: "/d", CPULimit: "1.0", MemoryLimit: "1g",
		Ports: map[string]RuntimePort{"http": {HostPort: 10042, ContainerPort: 8080, BindInterface: "127.0.0.1", Protocol: "tcp"}},
		Env: map[string]string{
			"MASTERKEY": "x0123456789abcdef0123456789abcd", "POSTGRES_PASSWORD": "p", "ADMIN_PASSWORD": "a",
			"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "2525", "SMTP_USER": "relayuser",
			"SMTP_PASSWORD": "relaypw", "SMTP_FROM": "noreply@example.com", "SMTP_FROM_NAME": "Acme", "SMTP_TLS": "false",
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, needle := range []string{
		`ZITADEL_DEFAULTINSTANCE_SMTPCONFIGURATION_SMTP_HOST: "smtp.example.com:2525"`,
		`ZITADEL_DEFAULTINSTANCE_SMTPCONFIGURATION_SMTP_USER: "relayuser"`,
		`ZITADEL_DEFAULTINSTANCE_SMTPCONFIGURATION_SMTP_PASSWORD: "relaypw"`,
		`ZITADEL_DEFAULTINSTANCE_SMTPCONFIGURATION_TLS: "false"`,
		`ZITADEL_DEFAULTINSTANCE_SMTPCONFIGURATION_FROM: "noreply@example.com"`,
		`ZITADEL_DEFAULTINSTANCE_SMTPCONFIGURATION_FROMNAME: "Acme"`,
		`ZITADEL_DEFAULTINSTANCE_DOMAINPOLICY_SMTPSENDERADDRESSMATCHESINSTANCEDOMAIN: "false"`,
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("missing %q\n%s", needle, out)
		}
	}
}

// TestZitadelSMTPEnvAutoInjected confirms `smtp: true` adds the standard SMTP_*
// env (operator-supplied: empty + no generator) with SMTP_PASSWORD marked secret.
func TestZitadelSMTPEnvAutoInjected(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	z, _ := cat.Get("zitadel")
	names := map[string]EnvVar{}
	for _, e := range z.Env {
		names[e.Name] = e
	}
	for _, want := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_FROM_NAME", "SMTP_TLS"} {
		if _, ok := names[want]; !ok {
			t.Errorf("smtp:true did not inject %s", want)
		}
	}
	if !names["SMTP_PASSWORD"].Secret {
		t.Error("SMTP_PASSWORD should be marked secret")
	}
	if names["SMTP_HOST"].Generate != "" || names["SMTP_HOST"].Value != "" {
		t.Error("SMTP_HOST should be operator-supplied (no value/generate)")
	}
}
