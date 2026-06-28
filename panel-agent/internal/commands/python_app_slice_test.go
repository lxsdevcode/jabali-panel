package commands

import (
	"strings"
	"testing"
)

// TestRenderPythonUnit_TenantSlice asserts the rendered unit places the app in
// the owner's M18 user slice (ADR-0131, Gitea #490) so package cgroup limits
// apply, and that User/Group are the tenant.
func TestRenderPythonUnit_TenantSlice(t *testing.T) {
	p := pythonAppApplyParams{
		AppID:    "01ABC",
		Username: "alice",
		AppType:  "wsgi",
	}
	unit := renderPythonUnit(p, "/home/alice/app", "/home/alice/app/.env", "/usr/bin/true")
	if !strings.Contains(unit, "Slice=jabali-user-alice.slice") {
		t.Errorf("unit missing tenant slice:\n%s", unit)
	}
	if !strings.Contains(unit, "User=alice") {
		t.Errorf("unit missing User=alice")
	}
}
