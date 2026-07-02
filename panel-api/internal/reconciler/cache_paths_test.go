package reconciler

import (
	"reflect"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// GH #601: the reconciler↔agent seam. cachePathsFromInstalls must emit exactly
// the shape the agent's buildCacheGate consumes ("/", "/blog", "/shop/eu") —
// this is the contract the two isolated unit-test suites both assume. If the
// trim logic drifts, this fails instead of silently under-caching.
func TestCachePathsFromInstalls(t *testing.T) {
	inst := func(subdir string) models.ApplicationInstall {
		return models.ApplicationInstall{Subdirectory: subdir}
	}
	cases := []struct {
		name string
		in   []models.ApplicationInstall
		want []string
	}{
		{"root only", []models.ApplicationInstall{inst("")}, []string{"/"}},
		{"root + blog", []models.ApplicationInstall{inst(""), inst("blog")}, []string{"/", "/blog"}},
		{"leading/trailing slash normalized", []models.ApplicationInstall{inst("/shop/")}, []string{"/shop"}},
		{"nested subdir", []models.ApplicationInstall{inst("shop/eu")}, []string{"/shop/eu"}},
		{"dedupe same prefix", []models.ApplicationInstall{inst("blog"), inst("/blog")}, []string{"/blog"}},
		{"two subdirs", []models.ApplicationInstall{inst("blog"), inst("shop")}, []string{"/blog", "/shop"}},
	}
	for _, c := range cases {
		if got := cachePathsFromInstalls(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: cachePathsFromInstalls = %v, want %v", c.name, got, c.want)
		}
	}
}

// GH #616: the bypass-paths seam — cacheBypassPathsFromInstalls merges every
// install's url_exclusions (from cache_settings JSON), deduped, so the agent
// gets exactly one sanitized set to fold into the gate.
func TestCacheBypassPathsFromInstalls(t *testing.T) {
	withEx := func(ex string) models.ApplicationInstall {
		return models.ApplicationInstall{CacheSettings: []byte(`{"url_exclusions":` + ex + `}`)}
	}
	cases := []struct {
		name string
		in   []models.ApplicationInstall
		want []string
	}{
		{"none configured", []models.ApplicationInstall{{}}, []string{}},
		{"single", []models.ApplicationInstall{withEx(`["/private"]`)}, []string{"/private"}},
		{"merge + dedupe across installs",
			[]models.ApplicationInstall{withEx(`["/private","/x"]`), withEx(`["/x","/y"]`)},
			[]string{"/private", "/x", "/y"}},
		{"blank entries skipped", []models.ApplicationInstall{withEx(`["","/a"," "]`)}, []string{"/a"}},
	}
	for _, c := range cases {
		if got := cacheBypassPathsFromInstalls(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: = %v, want %v", c.name, got, c.want)
		}
	}
}
