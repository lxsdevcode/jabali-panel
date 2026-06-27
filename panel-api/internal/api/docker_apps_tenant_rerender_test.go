package api

import (
	"testing"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// TestTenantValidateParams covers GH #480: a re-render of a tenant-owned app
// must carry the agent-side tenant compose validation + cap allowlist, while an
// admin app and an unknown slug do not.
func TestTenantValidateParams(t *testing.T) {
	h := &dockerAppHandler{cfg: DockerAppHandlerConfig{Catalog: tenantCatalog(t)}}
	uid := "01TENANT0000000000000000AA"

	// Tenant app with a catalog entry → validate + caps.
	v, caps := h.tenantValidateParams(&models.DockerApp{Slug: "tdemo", UserID: &uid})
	if !v {
		t.Error("tenant app must request tenant_validate on re-render")
	}
	if len(caps) == 0 {
		t.Error("tenant app must carry a cap allowlist")
	}

	// Admin app (no owner) → no tenant validation.
	if v2, _ := h.tenantValidateParams(&models.DockerApp{Slug: "tdemo", UserID: nil}); v2 {
		t.Error("admin app must not request tenant_validate")
	}

	// Tenant app whose slug is not in the catalog → no validation (handled by
	// the on-disk-compose fallback path).
	if v3, _ := h.tenantValidateParams(&models.DockerApp{Slug: "nope", UserID: &uid}); v3 {
		t.Error("unknown slug must not request tenant_validate")
	}
}
