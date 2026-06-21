package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
)

func newSpeedtestRouter(isAdmin bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(injectAdminClaims(isAdmin))
	api.RegisterAdminSpeedtestRoutes(v1)
	return r
}

func TestSpeedtest_RBAC(t *testing.T) {
	r := newSpeedtestRouter(false)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/speedtest/ping", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSpeedtest_Ping(t *testing.T) {
	r := newSpeedtestRouter(true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/speedtest/ping", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.Bytes())
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
}

func TestSpeedtest_DownloadSize(t *testing.T) {
	r := newSpeedtestRouter(true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/speedtest/download?bytes=131072", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 131072, rec.Body.Len())
	assert.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
}

func TestSpeedtest_DownloadCapped(t *testing.T) {
	// A request well past the cap is clamped, not honored verbatim.
	r := newSpeedtestRouter(true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/speedtest/download?bytes=999999999999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 100<<20, rec.Body.Len())
}

func TestSpeedtest_UploadDiscards(t *testing.T) {
	r := newSpeedtestRouter(true)
	body := bytes.NewReader([]byte(strings.Repeat("x", 4096)))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/speedtest/upload", body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}
