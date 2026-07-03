package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// users_sessions.go — GH #338. Admin "Active Sessions" view: list live panel
// (Kratos) sessions with user + source IP + channel, and revoke one. Backed by
// the Kratos admin sessions API (kratosclient.ListActiveSessions / RevokeSession).

// sessionRow is the UI-facing shape (flattened from the Kratos session).
type sessionRow struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	Username        string `json:"username"`
	IsAdmin         bool   `json:"is_admin"`
	IP              string `json:"ip"`
	UserAgent       string `json:"user_agent"`
	Channel         string `json:"channel"`
	AAL             string `json:"aal"`
	AuthenticatedAt string `json:"authenticated_at"`
	ExpiresAt       string `json:"expires_at"`
}

// listSessions handles GET /admin/sessions — all active panel sessions.
func (h *userHandler) listSessions(c *gin.Context) {
	if h.cfg.KratosClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kratos_unavailable"})
		return
	}
	sessions, err := h.cfg.KratosClient.ListActiveSessions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list_sessions_failed", "detail": err.Error()})
		return
	}
	rows := make([]sessionRow, 0, len(sessions))
	for _, s := range sessions {
		r := sessionRow{
			ID: s.ID, AAL: s.AAL, Channel: "panel",
			AuthenticatedAt: s.AuthenticatedAt, ExpiresAt: s.ExpiresAt,
		}
		if s.Identity != nil {
			r.Email = s.Identity.GetTraitEmail()
			r.Username = s.Identity.GetTraitUsername()
			r.IsAdmin = s.Identity.GetTraitIsAdmin()
		}
		if len(s.Devices) > 0 {
			r.IP = s.Devices[0].IPAddress
			r.UserAgent = s.Devices[0].UserAgent
		}
		rows = append(rows, r)
	}
	c.JSON(http.StatusOK, gin.H{"sessions": rows})
}

// revokeSession handles DELETE /admin/sessions/:id — deactivate one session.
func (h *userHandler) revokeSession(c *gin.Context) {
	if h.cfg.KratosClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kratos_unavailable"})
		return
	}
	id := c.Param("id")
	c.Set("audit_target", id+" (panel session revoke)")
	c.Set("audit_target_type", "session")
	if err := h.cfg.KratosClient.RevokeSession(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke_failed", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
