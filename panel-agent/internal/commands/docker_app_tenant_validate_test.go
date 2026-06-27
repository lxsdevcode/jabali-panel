package commands

import "testing"

const root = "/var/lib/jabali/docker-apps/memos-u01-notes"

func cfg(svc string) []byte { return []byte(`{"services":{` + svc + `}}`) }

func TestValidateTenantCompose_OK(t *testing.T) {
	j := cfg(`"memos":{"cap_add":["CHOWN","SETUID"],"volumes":[{"type":"bind","source":"` + root + `/data","target":"/data"},{"type":"volume","source":"vol","target":"/x"}]}`)
	if err := validateTenantCompose(j, []string{"CHOWN", "SETUID", "SETGID"}, root); err != nil {
		t.Fatalf("expected clean, got %v", err)
	}
}

func TestValidateTenantCompose_RejectsPrivileged(t *testing.T) {
	j := cfg(`"memos":{"privileged":true}`)
	if err := validateTenantCompose(j, nil, root); err == nil {
		t.Fatal("privileged service must be rejected")
	}
}

func TestValidateTenantCompose_RejectsForeignCap(t *testing.T) {
	j := cfg(`"memos":{"cap_add":["NET_ADMIN"]}`)
	if err := validateTenantCompose(j, []string{"CHOWN"}, root); err == nil {
		t.Fatal("cap outside allowlist must be rejected")
	}
}

func TestValidateTenantCompose_CapPrefixNormalised(t *testing.T) {
	// "CAP_CHOWN" in config vs "chown" in allowlist must still match.
	j := cfg(`"memos":{"cap_add":["CAP_CHOWN"]}`)
	if err := validateTenantCompose(j, []string{"chown"}, root); err != nil {
		t.Fatalf("normalised cap should match: %v", err)
	}
}

func TestValidateTenantCompose_RejectsHostBindMount(t *testing.T) {
	j := cfg(`"memos":{"volumes":[{"type":"bind","source":"/var/run/docker.sock","target":"/sock"}]}`)
	if err := validateTenantCompose(j, nil, root); err == nil {
		t.Fatal("host bind-mount outside data tree must be rejected")
	}
}

func TestValidateTenantCompose_RejectsParentEscape(t *testing.T) {
	// A source that prefix-matches the root string but escapes it.
	j := cfg(`"memos":{"volumes":[{"type":"bind","source":"` + root + `-evil/data","target":"/data"}]}`)
	if err := validateTenantCompose(j, nil, root); err == nil {
		t.Fatal("sibling-dir escape (root-evil) must be rejected")
	}
}

func TestValidateTenantCompose_NoServices(t *testing.T) {
	if err := validateTenantCompose([]byte(`{"services":{}}`), nil, root); err == nil {
		t.Fatal("empty services must error")
	}
}

// Host-namespace / device escapes (GH #450).
func TestValidateTenantCompose_RejectsHostEscapes(t *testing.T) {
	cases := map[string]string{
		"network_mode_host":     `"memos":{"network_mode":"host"}`,
		"pid_host":              `"memos":{"pid":"host"}`,
		"ipc_host":              `"memos":{"ipc":"host"}`,
		"uts_host":              `"memos":{"uts":"host"}`,
		"userns_host":           `"memos":{"userns_mode":"host"}`,
		"network_container":     `"memos":{"network_mode":"container:other"}`,
		"pid_container":         `"memos":{"pid":"container:other"}`,
		"devices":               `"memos":{"devices":["/dev/sda:/dev/sda"]}`,
		"device_cgroup_rules":   `"memos":{"device_cgroup_rules":["c 1:3 rmw"]}`,
		"volumes_from":          `"memos":{"volumes_from":["other"]}`,
		"security_opt_seccomp":  `"memos":{"security_opt":["seccomp:unconfined"]}`,
		"security_opt_apparmor": `"memos":{"security_opt":["apparmor:unconfined"]}`,
	}
	for name, svc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validateTenantCompose(cfg(svc), []string{"CHOWN"}, root); err == nil {
				t.Fatalf("%s must be rejected", name)
			}
		})
	}
}

func TestValidateTenantCompose_AllowsInjectedSecurityOpt(t *testing.T) {
	// The panel injects no-new-privileges:true — must pass (both : and = forms).
	for _, so := range []string{"no-new-privileges:true", "no-new-privileges=true"} {
		j := cfg(`"memos":{"security_opt":["` + so + `"]}`)
		if err := validateTenantCompose(j, []string{"CHOWN"}, root); err != nil {
			t.Fatalf("injected security_opt %q should pass: %v", so, err)
		}
	}
}

func TestValidateTenantCompose_AllowsBenignNamespaceModes(t *testing.T) {
	// none/private/service refs are not host escapes.
	j := cfg(`"memos":{"network_mode":"none","ipc":"private"}`)
	if err := validateTenantCompose(j, []string{"CHOWN"}, root); err != nil {
		t.Fatalf("benign namespace modes should pass: %v", err)
	}
}
