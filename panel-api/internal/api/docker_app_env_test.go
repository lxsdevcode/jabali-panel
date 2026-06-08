package api

import "testing"

func TestValidateEnvKV(t *testing.T) {
	cases := []struct {
		name, k, v string
		wantErr    bool
	}{
		{"ok plain", "ADMIN_PASSWORD", "s3cr3t-Token_09", false},
		{"ok empty value", "ALLOW_REGISTRATION", "", false},
		{"bad key space", "ADMIN PASSWORD", "x", true},
		{"bad key dash", "ADMIN-PASSWORD", "x", true},
		{"empty key", "", "x", true},
		{"newline value", "K", "line1\nline2", true},
		{"cr value", "K", "a\rb", true},
		{"nul value", "K", "a\x00b", true},
		{"too long", "K", string(make([]byte, maxEnvValueLen+1)), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateEnvKV(tc.k, tc.v)
			if (got != "") != tc.wantErr {
				t.Errorf("validateEnvKV(%q,...) = %q, wantErr=%v", tc.k, got, tc.wantErr)
			}
		})
	}
}
