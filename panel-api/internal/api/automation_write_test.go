package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func writeGuardRouter(tok *models.AutomationToken, scope string, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if tok != nil {
			c.Set("jabali_automation_token", tok)
		}
		c.Next()
	})
	r.POST("/w", requireWriteScope(scope), handler)
	return r
}

func doWritePost(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/w", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRequireWriteScope(t *testing.T) {
	ok := func(c *gin.Context) { autoOK(c, "done") }

	// read:* token → write endpoint denied 403 scope_denied.
	rd := &models.AutomationToken{ID: "t1", Scopes: models.AutomationScopes{"read:*"}, WritesEnabled: true}
	w := doWritePost(writeGuardRouter(rd, "write:services", ok))
	if w.Code != http.StatusForbidden {
		t.Fatalf("read token on write: want 403, got %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["ok"] != false || body["error"] != "scope_denied" {
		t.Fatalf("want {ok:false,error:scope_denied}, got %v", body)
	}

	// Correct scope + writes enabled → allowed.
	wr := &models.AutomationToken{ID: "t2", Scopes: models.AutomationScopes{"write:services"}, WritesEnabled: true}
	if code := doWritePost(writeGuardRouter(wr, "write:services", ok)).Code; code != http.StatusOK {
		t.Fatalf("write token: want 200, got %d", code)
	}

	// Scope held but writes_enabled=false → 403 scope_denied (master switch).
	off := &models.AutomationToken{ID: "t3", Scopes: models.AutomationScopes{"write:services"}, WritesEnabled: false}
	if code := doWritePost(writeGuardRouter(off, "write:services", ok)).Code; code != http.StatusForbidden {
		t.Fatalf("writes disabled: want 403, got %d", code)
	}
}

func TestBindSigned(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Good JSON in the signed-body slot parses.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("jabali_automation_signed_body", []byte(`{"scope":"all"}`))
	var v struct {
		Scope string `json:"scope"`
	}
	if !bindSigned(c, &v) || v.Scope != "all" {
		t.Fatalf("bindSigned should parse signed bytes, got %q", v.Scope)
	}
	// Malformed → 422, returns false.
	w := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w)
	c2.Set("jabali_automation_signed_body", []byte(`{not json`))
	if bindSigned(c2, &v) {
		t.Fatal("bindSigned should reject malformed JSON")
	}
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed body: want 422, got %d", w.Code)
	}
}

// SignedBody accessor returns the exact stashed bytes (validated == signed).
func TestSignedBodyAccessor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("jabali_automation_signed_body", []byte("abc"))
	if string(middleware.SignedBody(c)) != "abc" {
		t.Fatal("SignedBody must return the stashed signed bytes")
	}
}

func TestWriteRateLimiter(t *testing.T) {
	l := newWriteRateLimiter(60, 2) // burst 2
	if !l.allow("tok") || !l.allow("tok") {
		t.Fatal("first two writes should pass the burst")
	}
	if l.allow("tok") {
		t.Fatal("third immediate write should be rate-limited")
	}
	// A different token has its own bucket.
	if !l.allow("other") {
		t.Fatal("a different token must not share the bucket")
	}
}
