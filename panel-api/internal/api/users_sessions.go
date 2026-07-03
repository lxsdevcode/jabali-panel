package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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
	// GH #338 (SSH channel): merge live SSH sessions from the agent.
	if h.cfg.Agent != nil {
		if raw, aerr := h.cfg.Agent.Call(c.Request.Context(), "sessions.ssh.list", nil); aerr == nil {
			var sshResp struct {
				Sessions []struct {
					ID       string `json:"id"`
					User     string `json:"user"`
					RemoteIP string `json:"remote_ip"`
					Since    string `json:"since"`
				} `json:"sessions"`
			}
			if json.Unmarshal(raw, &sshResp) == nil {
				for _, ss := range sshResp.Sessions {
					rows = append(rows, sessionRow{
						ID: ss.ID, Username: ss.User, IP: ss.RemoteIP,
						Channel: "ssh", AuthenticatedAt: ss.Since,
					})
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"sessions": rows})
}

// revokeSession handles DELETE /admin/sessions/:id — deactivate one session.
func (h *userHandler) revokeSession(c *gin.Context) {
	id := c.Param("id")
	c.Set("audit_target_type", "session")
	// GH #338: SSH sessions revoke via the agent (terminate the sshd process);
	// panel/Kratos sessions revoke via Kratos.
	if strings.HasPrefix(id, "ssh:") {
		c.Set("audit_target", id+" (ssh session revoke)")
		if h.cfg.Agent == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_unavailable"})
			return
		}
		pid, perr := strconv.Atoi(strings.TrimPrefix(id, "ssh:"))
		if perr != nil || pid < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_ssh_session"})
			return
		}
		if _, err := h.cfg.Agent.Call(c.Request.Context(), "sessions.ssh.revoke", map[string]any{"pid": pid}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke_failed", "detail": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.Set("audit_target", id+" (panel session revoke)")
	if h.cfg.KratosClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kratos_unavailable"})
		return
	}
	if err := h.cfg.KratosClient.RevokeSession(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke_failed", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
