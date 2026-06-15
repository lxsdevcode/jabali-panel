package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/auth"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// ImpersonateHeader carries an active impersonation grant id (ADR-0128).
const ImpersonateHeader = "X-Jabali-Act-As"

// userByID is the narrow slice of the user repository the override needs
// (repository.UserRepository satisfies it). Keeping it narrow lets tests fake
// it with one method.
type userByID interface {
	FindByID(ctx context.Context, id string) (*models.User, error)
}

// ResolveImpersonation applies an admin "act-as" override (ADR-0128). It is
// mounted AFTER RequireKratosSession and is a no-op unless the
// X-Jabali-Act-As header is present.
//
// Security (see ADR-0128): the override acts ONLY when the *real* Kratos-cookie
// claims are admin AND the grant belongs to that admin AND the target exists
// AND is non-admin. On success it replaces the effective claims with the
// target user's (IsAdmin=false, scoped to the target), keeping ImpersonatedBy
// = the real admin so the audit trail records the real actor. A non-admin (or
// unauthenticated) request that carries the header is ignored — it grants
// nothing, so the caller simply gets their own data. An admin whose grant is
// invalid/expired/not-theirs gets 403 impersonation_invalid so the SPA drops
// back to the admin view instead of silently acting as admin.
func ResolveImpersonation(
	grants repository.ImpersonationGrantRepository,
	users userByID,
	now func() time.Time,
) gin.HandlerFunc {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return func(c *gin.Context) {
		// The impersonation-management endpoints must always run as the
		// REAL admin (so an admin can create/list/exit grants even while an
		// X-Jabali-Act-As header is set) — never override claims for them.
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/admin/impersonation") {
			c.Next()
			return
		}
		gid := c.GetHeader(ImpersonateHeader)
		if gid == "" {
			c.Next()
			return
		}
		real := ginctx.Claims(c)
		if real == nil || !real.IsAdmin {
			// Only a real admin can impersonate. Ignore the header silently —
			// a tenant who sends it still gets exactly their own scope.
			c.Next()
			return
		}
		fail := func() {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "impersonation_invalid",
				"message": "impersonation grant is invalid, expired, or not yours",
			})
			c.Abort()
		}
		grant, err := grants.FindActive(c.Request.Context(), gid, now())
		if err != nil || grant == nil || grant.AdminUserID != real.UserID {
			fail()
			return
		}
		target, err := users.FindByID(c.Request.Context(), grant.TargetUserID)
		if err != nil || target == nil || target.IsAdmin {
			// Target gone or (defensively) an admin — never act as another
			// admin. Refuse rather than fall back to admin scope.
			fail()
			return
		}
		ginctx.SetClaims(c, &auth.AccessClaims{
			UserID:         target.ID,
			Email:          target.Email,
			IsAdmin:        false,
			Source:         real.Source,
			ImpersonatedBy: real.UserID,
		})
		c.Next()
	}
}
