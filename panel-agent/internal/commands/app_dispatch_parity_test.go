package commands

import "testing"

// TestEveryInstallerHasDeleter guards the #227 bug class: dokuwiki shipped
// an installer but no deleter, so deleting a DokuWiki install failed with
// `unknown app_type "dokuwiki" for app.delete`. Any app the user can
// install, they must be able to delete — assert the two dispatch tables
// stay in sync at the package-init level.
func TestEveryInstallerHasDeleter(t *testing.T) {
	installers := snapshotInstallers()
	deleters := snapshotDeleters()
	for appType := range installers {
		if _, ok := deleters[appType]; !ok {
			t.Errorf("app_type %q has an installer but no deleter (RegisterAppDeleter missing) — delete will fail with \"unknown app_type\"", appType)
		}
	}
}
