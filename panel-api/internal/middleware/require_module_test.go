package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func TestModuleFlags(t *testing.T) {
	on := &models.ServerSettings{DNSEnabled: true, MailEnabled: true, SecurityEnabled: true, QuotaEnabled: true, APIEnabled: true}
	off := &models.ServerSettings{}
	for _, f := range []ModuleFlag{ModuleDNS, ModuleMail, ModuleSecurity, ModuleQuota, ModuleAPI} {
		if !f(on) {
			t.Error("flag should be true when enabled")
		}
		if f(off) {
			t.Error("flag should be false when disabled")
		}
	}
}

// guard mirrors the middleware body against a pre-populated cache so we can
// assert the 409-on-disabled + fail-open-on-nil behavior without a live repo.
func guardWith(s *models.ServerSettings, flag ModuleFlag) int {
	gin.SetMode(gin.TestMode)
	cache := &moduleSettingsCache{cached: s, fetched: time.Unix(1, 0), ttl: time.Hour, nowFunc: func() time.Time { return time.Unix(2, 0) }}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	ss := cache.get(c)
	if ss != nil && !flag(ss) {
		c.AbortWithStatus(http.StatusConflict)
	}
	return w.Code
}

func TestModuleGuard_Behavior(t *testing.T) {
	if guardWith(&models.ServerSettings{MailEnabled: false}, ModuleMail) != http.StatusConflict {
		t.Error("disabled module should 409")
	}
	if guardWith(&models.ServerSettings{MailEnabled: true}, ModuleMail) == http.StatusConflict {
		t.Error("enabled module should pass")
	}
	if guardWith(nil, ModuleMail) == http.StatusConflict {
		t.Error("nil settings must fail open (module on)")
	}
}
