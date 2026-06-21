package commands

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMailWebadminApply_RenderAndGate(t *testing.T) {
	sa, se := t.TempDir(), t.TempDir()
	oldA, oldE := mailVhostSitesAvailable, mailVhostSitesEnabled
	mailVhostSitesAvailable, mailVhostSitesEnabled = sa, se
	defer func() { mailVhostSitesAvailable, mailVhostSitesEnabled = oldA, oldE }()
	stalwartAdminHtpasswd = filepath.Join(t.TempDir(), "htpasswd")

	oldReload := nginxTestAndReload
	nginxTestAndReload = func(context.Context) error { return nil }
	defer func() { nginxTestAndReload = oldReload }()

	params, _ := json.Marshal(map[string]any{
		"enabled":       true,
		"server_name":   "admin.mail.mx.example.com",
		"ssl_cert_path": "/etc/jabali/tls/panel.crt",
		"ssl_key_path":  "/etc/jabali/tls/panel.key",
		"allow_cidrs":   []string{"203.0.113.0/24", "198.51.100.5"},
	})
	res, err := mailWebadminApplyHandler(context.Background(), params)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	resp := res.(webadminApplyResponse)
	if resp.GatewayPassword == "" || resp.GatewayUser != "stalwart-admin" {
		t.Fatalf("first enable must return a fresh gateway credential: %+v", resp)
	}

	conf, _ := os.ReadFile(filepath.Join(sa, stalwartAdminVhostName))
	cs := string(conf)
	for _, want := range []string{
		"server_name admin.mail.mx.example.com;",
		"proxy_pass http://127.0.0.1:8446;", // Stalwart stays loopback
		"auth_basic", "auth_basic_user_file",
		"allow 203.0.113.0/24;", "allow 198.51.100.5;", "deny all;",
		"return 301 https://",
	} {
		if !strings.Contains(cs, want) {
			t.Errorf("vhost missing %q\n%s", want, cs)
		}
	}
	// symlink enabled
	if _, err := os.Lstat(filepath.Join(se, stalwartAdminVhostName)); err != nil {
		t.Error("vhost not symlinked into sites-enabled")
	}
	// htpasswd written, bcrypt
	hp, _ := os.ReadFile(stalwartAdminHtpasswd)
	if !strings.HasPrefix(string(hp), "stalwart-admin:$2") {
		t.Errorf("htpasswd not bcrypt: %q", string(hp))
	}

	// disable removes the vhost
	dp, _ := json.Marshal(map[string]any{"enabled": false})
	if _, err := mailWebadminApplyHandler(context.Background(), dp); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sa, stalwartAdminVhostName)); !os.IsNotExist(err) {
		t.Error("disable must remove the vhost conf")
	}
}

func TestMailWebadminApply_RejectsBadCIDR(t *testing.T) {
	stalwartAdminHtpasswd = filepath.Join(t.TempDir(), "htpasswd")
	params, _ := json.Marshal(map[string]any{
		"enabled": true, "server_name": "admin.mail.x.com",
		"ssl_cert_path": "/c", "ssl_key_path": "/k",
		"allow_cidrs": []string{"not-a-cidr"},
	})
	if _, err := mailWebadminApplyHandler(context.Background(), params); err == nil {
		t.Fatal("bad CIDR must be rejected")
	}
}

func TestMailWebadminApply_Regenerate(t *testing.T) {
	sa, se := t.TempDir(), t.TempDir()
	oldA, oldE := mailVhostSitesAvailable, mailVhostSitesEnabled
	mailVhostSitesAvailable, mailVhostSitesEnabled = sa, se
	defer func() { mailVhostSitesAvailable, mailVhostSitesEnabled = oldA, oldE }()
	stalwartAdminHtpasswd = filepath.Join(t.TempDir(), "htpasswd")
	oldReload := nginxTestAndReload
	nginxTestAndReload = func(context.Context) error { return nil }
	defer func() { nginxTestAndReload = oldReload }()

	base := map[string]any{"enabled": true, "server_name": "admin.mx.example.com", "ssl_cert_path": "/c", "ssl_key_path": "/k"}
	p1, _ := json.Marshal(base)
	r1, _ := mailWebadminApplyHandler(context.Background(), p1)
	first := r1.(webadminApplyResponse).GatewayPassword
	if first == "" {
		t.Fatal("first enable should mint a credential")
	}
	// Re-apply without regenerate → no new credential.
	r2, _ := mailWebadminApplyHandler(context.Background(), p1)
	if r2.(webadminApplyResponse).GatewayPassword != "" {
		t.Fatal("re-apply must NOT mint a new credential")
	}
	// Regenerate → a fresh, different credential.
	base["regenerate"] = true
	p3, _ := json.Marshal(base)
	r3, _ := mailWebadminApplyHandler(context.Background(), p3)
	second := r3.(webadminApplyResponse).GatewayPassword
	if second == "" || second == first {
		t.Fatalf("regenerate must mint a NEW credential: first=%q second=%q", first, second)
	}
}
