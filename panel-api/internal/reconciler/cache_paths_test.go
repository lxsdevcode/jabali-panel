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
