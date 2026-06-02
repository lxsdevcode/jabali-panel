package api

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// newTestGinCtx returns a gin.Context backed by the given recorder,
// with a minimal request stub. Tests use this to drive helpers that
// take a *gin.Context without setting up a full router.
func newTestGinCtx(w *httptest.ResponseRecorder) (*gin.Context, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	c, e := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/nic/update", nil)
	return c, e
}
