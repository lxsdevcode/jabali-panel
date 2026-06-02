package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Pure-unit smoke for the response writer: every router parses the
// first whitespace-separated word, so the body MUST start with the
// status text. Also confirms no leading "0" / "OK\n" / JSON envelope
// got introduced by a future refactor.
func TestDDNSReply_FormatStableForDynDNSClients(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{http.StatusOK, "good 1.2.3.4", "good 1.2.3.4\n"},
		{http.StatusOK, "nochg 1.2.3.4", "nochg 1.2.3.4\n"},
		{http.StatusUnauthorized, "badauth", "badauth\n"},
		{http.StatusNotFound, "nohost", "nohost\n"},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := newTestGinCtx(w)
			ddnsReply(c, tc.status, tc.body)
			if w.Code != tc.status {
				t.Errorf("status = %d, want %d", w.Code, tc.status)
			}
			if w.Body.String() != tc.want {
				t.Errorf("body = %q, want %q", w.Body.String(), tc.want)
			}
			ct := w.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "text/plain") {
				t.Errorf("content-type = %q, want text/plain prefix", ct)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("cache-control = %q, want no-store", w.Header().Get("Cache-Control"))
			}
		})
	}
}
