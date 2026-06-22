package commands

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestDockerTenantSet_DisableRemovesFlag(t *testing.T) {
	// Point the flag at a temp file by stubbing via a real file (the const path
	// is fixed, so we instead verify the exec seam + os.Remove of a missing
	// file is non-fatal). Disable with no flag present must still succeed.
	called := false
	old := dockerTenantExec
	dockerTenantExec = func(context.Context) error { called = true; return nil }
	t.Cleanup(func() { dockerTenantExec = old })

	raw, err := dockerTenantSetHandler(context.Background(), json.RawMessage(`{"enabled":false}`))
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if r := raw.(dockerTenantSetResponse); r.Enabled || !r.Ok {
		t.Fatalf("disable response wrong: %+v", r)
	}
	if called {
		t.Fatal("disable must NOT run enable-tenant")
	}
}

func TestDockerTenantSet_EnableRunsCLI(t *testing.T) {
	called := false
	old := dockerTenantExec
	dockerTenantExec = func(context.Context) error { called = true; return nil }
	t.Cleanup(func() { dockerTenantExec = old })

	raw, err := dockerTenantSetHandler(context.Background(), json.RawMessage(`{"enabled":true}`))
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if r := raw.(dockerTenantSetResponse); !r.Enabled || !r.Ok {
		t.Fatalf("enable response wrong: %+v", r)
	}
	if !called {
		t.Fatal("enable must run enable-tenant CLI")
	}
}

func TestDockerTenantSet_EnableErrorPropagates(t *testing.T) {
	old := dockerTenantExec
	dockerTenantExec = func(context.Context) error { return os.ErrPermission }
	t.Cleanup(func() { dockerTenantExec = old })
	if _, err := dockerTenantSetHandler(context.Background(), json.RawMessage(`{"enabled":true}`)); err == nil {
		t.Fatal("enable must surface CLI failure")
	}
}
