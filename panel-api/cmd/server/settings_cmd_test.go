package main

import (
	"testing"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// TestSettingsRegistryApply covers Gitea #539 criterion #4: unknown keys and
// invalid values are rejected; valid values apply and flag nginx side effects.
func TestSettingsRegistryApply(t *testing.T) {
	t.Run("unknown key rejected", func(t *testing.T) {
		if _, ok := settableKeys["does_not_exist"]; ok {
			t.Fatal("unexpected key in registry")
		}
	})

	cases := []struct {
		name    string
		key     string
		val     string
		wantErr bool
		check   func(t *testing.T, s *models.ServerSettings)
	}{
		{name: "bool true", key: "webmail_enabled", val: "true", check: func(t *testing.T, s *models.ServerSettings) {
			if !s.WebmailEnabled {
				t.Fatal("want WebmailEnabled true")
			}
		}},
		{name: "bool off alias", key: "postgres_enabled", val: "off", check: func(t *testing.T, s *models.ServerSettings) {
			if s.PostgresEnabled {
				t.Fatal("want PostgresEnabled false")
			}
		}},
		{name: "bool invalid", key: "webmail_enabled", val: "maybe", wantErr: true},
		{name: "nginx str", key: "nginx_client_max_body_size", val: "50m", check: func(t *testing.T, s *models.ServerSettings) {
			if s.NginxClientMaxBodySize != "50m" {
				t.Fatalf("want 50m got %q", s.NginxClientMaxBodySize)
			}
		}},
		{name: "nginx worker_connections valid", key: "nginx_worker_connections", val: "1024", check: func(t *testing.T, s *models.ServerSettings) {
			if s.NginxWorkerConnections != 1024 {
				t.Fatalf("want 1024 got %d", s.NginxWorkerConnections)
			}
		}},
		{name: "nginx worker_connections invalid", key: "nginx_worker_connections", val: "lots", wantErr: true},
		{name: "dns preset valid", key: "dns_user_record_policy_preset", val: "hosting-safe"},
		{name: "dns preset invalid", key: "dns_user_record_policy_preset", val: "nope", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nginxDirty = false
			def, ok := settableKeys[tc.key]
			if !ok {
				t.Fatalf("key %q not in registry", tc.key)
			}
			s := &models.ServerSettings{ID: 1}
			err := def.apply(s, tc.val)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error for %s=%s", tc.key, tc.val)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, s)
			}
		})
	}
}

// TestSettingsNginxDirtyFlag verifies nginx-key setters flip nginxDirty so the
// nginx.tunables.apply side effect fires, while non-nginx keys do not.
func TestSettingsNginxDirtyFlag(t *testing.T) {
	nginxDirty = false
	s := &models.ServerSettings{ID: 1}
	if err := settableKeys["webmail_enabled"].apply(s, "true"); err != nil {
		t.Fatal(err)
	}
	if nginxDirty {
		t.Fatal("non-nginx key must not set nginxDirty")
	}
	if err := settableKeys["nginx_gzip"].apply(s, "off"); err != nil {
		t.Fatal(err)
	}
	if !nginxDirty {
		t.Fatal("nginx key must set nginxDirty")
	}
	nginxDirty = false
}
