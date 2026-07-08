package modules

import (
	"sort"
	"testing"
)

func keysOf(m map[string]bool) []string {
	var out []string
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestResolve_PullsRequirements(t *testing.T) {
	// Enabling docker_apps must pull in docker.
	got := keysOf(Resolve(map[string]bool{"docker_apps": true}, "docker_apps"))
	if !eq(got, []string{"docker", "docker_apps"}) {
		t.Errorf("docker_apps should pull docker, got %v", got)
	}
}

func TestResolve_TransitiveAndStable(t *testing.T) {
	got := keysOf(Resolve(map[string]bool{"dns": true, "mail": true}, ""))
	if !eq(got, []string{"dns", "mail"}) {
		t.Errorf("no spurious pulls, got %v", got)
	}
}

func TestDependentsOf(t *testing.T) {
	// docker_apps depends on docker → unticking docker should surface it.
	deps := DependentsOf("docker", map[string]bool{"docker": true, "docker_apps": true})
	if len(deps) != 1 || deps[0] != "docker_apps" {
		t.Errorf("expected [docker_apps], got %v", deps)
	}
}

func TestProfiles_ValidKeys(t *testing.T) {
	for _, p := range Profiles {
		for _, k := range p.Modules {
			if _, ok := byKey(k); !ok {
				t.Errorf("profile %q references unknown module %q", p.Key, k)
			}
		}
	}
}
