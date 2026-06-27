package dockerapp

import "testing"

// TestEveryCatalogEntryHasTags ensures the catalog tag filter (GH #448) has
// something to filter on: every docker-app catalog entry must carry at least
// one category tag.
func TestEveryCatalogEntryHasTags(t *testing.T) {
	cat, errs := LoadDir(repoCatalogDir(t))
	if len(errs) > 0 {
		t.Fatalf("catalog load errors: %v", errs)
	}
	for _, e := range cat.All() {
		if len(e.Tags) == 0 {
			t.Errorf("catalog entry %q has no tags", e.Slug)
		}
	}
}
