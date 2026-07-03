package commands

import "testing"

// The exclude list is load-bearing (GH #646 invariant 2): if any jabali-managed
// file drops off it, a refresh clobbers the dest's DB creds / Redis integration.
func TestJabaliManagedRefreshExcludes(t *testing.T) {
	must := []string{
		"wp-config.php",
		"wp-content/object-cache.php",
		"wp-content/advanced-cache.php",
		".user.ini",
	}
	have := map[string]bool{}
	for _, e := range jabaliManagedRefreshExcludes {
		have[e] = true
	}
	for _, m := range must {
		if !have[m] {
			t.Errorf("refresh exclude list is missing load-bearing entry %q — a refresh would clobber it", m)
		}
	}
}
