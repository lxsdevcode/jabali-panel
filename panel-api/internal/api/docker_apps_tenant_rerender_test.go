package api

import (
	"context"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// TestTenantValidateParams covers GH #480: a re-render of a tenant-owned app
// must carry the agent-side tenant compose validation + cap allowlist, while an
// admin app and an unknown slug do not.
func TestTenantValidateParams(t *testing.T) {
	uid := "01TENANT0000000000000000AA"
	uname := "bob"
	users := &fakeUserRepo{user: &models.User{ID: uid, Username: &uname}}
	h := &dockerAppHandler{cfg: DockerAppHandlerConfig{Catalog: tenantCatalog(t), Users: users}}
	ctx := context.Background()

	// Tenant app with a catalog entry + resolvable owner → validate + caps + slice.
	v, caps, cgroup, err := h.tenantValidateParams(ctx, &models.DockerApp{ID: "a1", Slug: "tdemo", UserID: &uid})
	if err != nil || !v {
		t.Fatalf("tenant app must request tenant_validate: v=%v err=%v", v, err)
	}
	if len(caps) == 0 {
		t.Error("tenant app must carry a cap allowlist")
	}
	if cgroup != "jabali-user-bob.slice" {
		t.Errorf("expected owner slice, got %q (#525)", cgroup)
	}

	// Admin app (no owner) → no tenant validation, no error.
	if v2, _, _, err2 := h.tenantValidateParams(ctx, &models.DockerApp{Slug: "tdemo", UserID: nil}); v2 || err2 != nil {
		t.Errorf("admin app must not request tenant_validate (v=%v err=%v)", v2, err2)
	}

	// Tenant app whose slug is not in the catalog → FAIL CLOSED (error), not
	// "no validation needed" (#529).
	if _, _, _, err3 := h.tenantValidateParams(ctx, &models.DockerApp{ID: "a2", Slug: "nope", UserID: &uid}); err3 == nil {
		t.Error("unknown-slug tenant app must fail closed (#529)")
	}
}
