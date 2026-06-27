package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDrupalVendoredLockPinsDrush guards the reviewed Drupal+Drush lock the
// installer ships (GH #459): both assets must be present + valid, and the lock
// must pin an exact drush/drush version (not a range), so `composer install`
// is reproducible.
func TestDrupalVendoredLockPinsDrush(t *testing.T) {
	if len(drupalComposerJSON) == 0 || len(drupalComposerLock) == 0 {
		t.Fatal("vendored composer.json/composer.lock must be embedded")
	}
	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(drupalComposerLock, &lock); err != nil {
		t.Fatalf("composer.lock is not valid JSON: %v", err)
	}
	var drushVer string
	for _, p := range lock.Packages {
		if p.Name == "drush/drush" {
			drushVer = p.Version
		}
	}
	if drushVer == "" {
		t.Fatal("composer.lock does not pin drush/drush")
	}
	if strings.ContainsAny(drushVer, "^~*") {
		t.Fatalf("drush version %q must be an exact pin, not a range", drushVer)
	}
}
