package dockerapp

import (
	"reflect"
	"testing"
)

func TestTenantCapAllowlist(t *testing.T) {
	cases := []struct {
		name    string
		catalog []string
		want    []string
	}{
		{"no catalog caps -> baseline only", nil, []string{"CHOWN", "SETUID", "SETGID"}},
		{"extra cap appended", []string{"NET_BIND_SERVICE"}, []string{"CHOWN", "SETUID", "SETGID", "NET_BIND_SERVICE"}},
		{"dedup baseline declared explicitly", []string{"CHOWN", "SETUID", "SETGID"}, []string{"CHOWN", "SETUID", "SETGID"}},
		{"case + CAP_ prefix normalised and deduped", []string{"cap_setuid", "Chown"}, []string{"CHOWN", "SETUID", "SETGID"}},
	}
	for _, c := range cases {
		if got := TenantCapAllowlist(c.catalog); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
