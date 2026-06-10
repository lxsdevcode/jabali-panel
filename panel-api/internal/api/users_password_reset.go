package api

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/kratosclient"
)

// resetPassword handles POST /admin/users/:id/password/reset (M54).
//
// Under username-only login the email is a plain trait, so Kratos self-service
// recovery does not exist (spike-proven). The only working reset is an admin
// directly setting a fresh strong password and handing it to the user. This
// generates a temp password, bcrypts it, sets it on the Kratos identity, and
// returns the plaintext EXACTLY ONCE (reveal-once, like mailbox / db-user /
// automation-token secrets). Admin-only (route registration).
func (h *userHandler) resetPassword(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id required"})
		return
	}
	user, err := h.cfg.Repo.FindByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.KratosIdentityID == nil || *user.KratosIdentityID == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  "user_has_no_kratos_identity",
			"detail": "User predates Kratos migration. Run `jabali admin rebuild-kratos`, then re-attempt.",
		})
		return
	}
	if h.cfg.KratosClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kratos not available"})
		return
	}

	temp, err := generateTempPassword(18)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password generation failed"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(temp), h.cfg.BcryptCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash failed"})
		return
	}
	kctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.cfg.KratosClient.SetPassword(kctx, *user.KratosIdentityID, string(hash)); err != nil {
		if errors.Is(err, kratosclient.ErrIdentityNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "kratos identity not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "kratos_error", "detail": "failed to set password"})
		return
	}

	// reveal-once: plaintext returned now, never persisted, never logged.
	c.JSON(http.StatusOK, gin.H{"password": temp})
}

// generateTempPassword returns an n-char password from an unambiguous
// alphanumeric alphabet (no 0/O/1/l/I) via crypto/rand.
func generateTempPassword(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf), nil
}
