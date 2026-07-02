package migrate

import (
	"context"
	"net"
	"net/http"
	"time"
)

// SafeHTTPClient returns an *http.Client whose every TCP connection is SSRF- and
// DNS-rebind-safe: the Transport's DialContext re-resolves + re-validates the
// host on EACH dial (ValidateHost) and connects through the rebind-safe Dialer
// (its Control hook re-checks the peer IP against the validated set). Reuses the
// exact primitives DialTCP uses — do NOT hand-roll SSRF for the plugin pull
// (GH #648). allowPrivate mirrors the migration_allow_private_hosts toggle.
func SafeHTTPClient(allowPrivate bool, timeout time.Duration) *http.Client {
	tr := &http.Transport{
		Proxy: nil, // never honor env proxies for a migration pull
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			v, err := ValidateHost(ctx, host, allowPrivate)
			if err != nil {
				return nil, err
			}
			d := v.Dialer()
			d.Timeout = 30 * time.Second
			return d.DialContext(ctx, network, addr)
		},
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}
