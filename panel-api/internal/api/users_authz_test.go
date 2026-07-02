package api_test

import (
	"context"
	"net/http"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/auth"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsers_Patch_OwnerCannotSetPackageID covers GH #481: a non-admin owner
// cannot move themselves between hosting packages — package_id is ignored.
func TestUsers_Patch_OwnerCannotSetPackageID(t *testing.T) {
	t.Parallel()
	repo := newMemUserRepo()
	owner := makeUser(t, "u@example.com", false, "password01")
	repo.seed(owner)
	pkgRepo := newMemPackageRepo()
	pkg := &models.HostingPackage{ID: ids.NewULID(), Name: "Premium"}
	pkgRepo.seed(pkg)

	r := buildRouterWithPackages(repo, pkgRepo, &auth.AccessClaims{UserID: owner.ID})
	rec := doJSON(t, r, http.MethodPatch, "/api/v1/users/"+owner.ID, map[string]any{
		"package_id": pkg.ID,
	})
	require.Equal(t, http.StatusOK, rec.Code)
	after, err := repo.FindByID(context.Background(), owner.ID)
	require.NoError(t, err)
	assert.Nil(t, after.PackageID, "non-admin owner must not be able to assign a package")
}

// TestUsers_Patch_OwnerPasswordRequiresCurrent covers GH #482: a non-admin
// owner changing their own password must supply a matching current_password;
// admins are exempt.
func TestUsers_Patch_OwnerPasswordRequiresCurrent(t *testing.T) {
	t.Parallel()

	t.Run("missing current_password rejected", func(t *testing.T) {
		repo := newMemUserRepo()
		owner := makeUser(t, "u@example.com", false, "password01")
		repo.seed(owner)
		r := buildRouter(repo, &auth.AccessClaims{UserID: owner.ID})
		rec := doJSON(t, r, http.MethodPatch, "/api/v1/users/"+owner.ID, map[string]any{
			"password": "brandnewpass9",
		})
		require.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "current_password_required")
	})

	t.Run("wrong current_password rejected", func(t *testing.T) {
		repo := newMemUserRepo()
		owner := makeUser(t, "u@example.com", false, "password01")
		repo.seed(owner)
		r := buildRouter(repo, &auth.AccessClaims{UserID: owner.ID})
		rec := doJSON(t, r, http.MethodPatch, "/api/v1/users/"+owner.ID, map[string]any{
			"password":         "brandnewpass9",
			"current_password": "wrongpassword",
		})
		require.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "current_password_invalid")
	})

	t.Run("admin exempt from current_password", func(t *testing.T) {
		repo := newMemUserRepo()
		admin := makeUser(t, "admin@example.com", true, "adminpassword")
		target := makeUser(t, "u@example.com", false, "password01")
		repo.seed(admin)
		repo.seed(target)
		r := buildRouter(repo, &auth.AccessClaims{UserID: admin.ID, IsAdmin: true})
		rec := doJSON(t, r, http.MethodPatch, "/api/v1/users/"+target.ID, map[string]any{
			"password": "resetbyadmin9",
		})
		// Admin is past the re-auth gate; the request proceeds to the Kratos
		// stage (no identity in the mem repo => 409), NOT a 403 re-auth error.
		assert.NotEqual(t, http.StatusForbidden, rec.Code)
		assert.NotContains(t, rec.Body.String(), "current_password")
	})
}
