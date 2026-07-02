package migrate

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// GH #648: SafeHTTPClient must reject SSRF targets (loopback/link-local/metadata)
// when allowPrivate is false — the plugin pull's first line of defense.
func TestSafeHTTPClient_RejectsSSRF(t *testing.T) {
	c := SafeHTTPClient(false, 5*time.Second)
	for _, target := range []string{
		"http://127.0.0.1:80/",
		"http://169.254.169.254/latest/meta-data/", // cloud metadata
		"http://[::1]:80/",
	} {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
		resp, err := c.Do(req)
		if err == nil {
			resp.Body.Close()
			t.Errorf("SafeHTTPClient reached %s (SSRF not blocked)", target)
		}
	}
}
