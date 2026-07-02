package reconciler

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// TestReconcileSSL_ModeRouting verifies reconcileSSLForDomain dispatches the
// right agent command per ssl_mode (GH #246).
func TestReconcileSSL_ModeRouting(t *testing.T) {
	hasCall := func(ag *fakeAgent, method string) bool {
		for _, c := range ag.calls {
			if c.method == method {
				return true
			}
		}
		return false
	}
	run := func(mode string, cert *models.SSLCertificate) *fakeAgent {
		ag := &fakeAgent{}
		dr := &fakeDomainRepo{domains: map[string]*models.Domain{}}
		sc := newFakeSSLCertRepo()
		ss := &fakeServerSettingsRepo{settings: &models.ServerSettings{Hostname: "host.example.com"}}
		r := New(dr, nil, ag, slog.Default(), Config{}).WithSSLCerts(sc)
		r.serverSettings = ss
		dom := &models.Domain{ID: "d1", Name: "example.com", SSLMode: mode}
		if cert != nil {
			cert.DomainID = "d1"
			sc.byDomain["d1"] = cert
		}
		r.reconcileSSLForDomain(context.Background(), dom)
		return ag
	}

	t.Run("self mode self-signs", func(t *testing.T) {
		ag := run(models.SSLModeSelf, nil)
		require.True(t, hasCall(ag, "ssl.self_sign"), "self mode must call ssl.self_sign")
		require.False(t, hasCall(ag, "ssl.issue"), "self mode must NOT attempt ACME")
	})

	t.Run("none mode revokes issued cert", func(t *testing.T) {
		ag := run(models.SSLModeNone, &models.SSLCertificate{ID: "c1", Status: models.SSLStatusIssued})
		require.True(t, hasCall(ag, "ssl.revoke"), "none mode must revoke an issued cert")
	})

	t.Run("none mode no cert is a no-op", func(t *testing.T) {
		ag := run(models.SSLModeNone, nil)
		require.Empty(t, ag.calls, "none mode with no cert must not call the agent")
	})

	t.Run("custom mode does not touch the cert", func(t *testing.T) {
		ag := run(models.SSLModeCustom, nil)
		require.False(t, hasCall(ag, "ssl.self_sign"))
		require.False(t, hasCall(ag, "ssl.issue"))
		require.False(t, hasCall(ag, "ssl.revoke"), "custom is operator-managed; reconciler must not revoke it")
	})
}

// TestReconcileSSL_NoneClearsEvenIfRevokeFails covers GH #246: switching to
// None must clear the local cert (so the vhost drops :443 + the http->https
// redirect) even when the LE ssl.revoke agent call fails. Previously a failed
// revoke left the cert with its path set, so the site kept redirecting to a
// dead :443.
func TestReconcileSSL_NoneClearsEvenIfRevokeFails(t *testing.T) {
	ag := &fakeAgent{failMethod: "ssl.revoke"}
	dr := &fakeDomainRepo{domains: map[string]*models.Domain{}}
	sc := newFakeSSLCertRepo()
	certPath, keyPath := "/etc/x/cert.pem", "/etc/x/key.pem"
	sc.byDomain["d1"] = &models.SSLCertificate{
		ID: "c1", DomainID: "d1", Status: models.SSLStatusIssued,
		CertPath: &certPath, KeyPath: &keyPath,
	}
	r := New(dr, nil, ag, slog.Default(), Config{}).WithSSLCerts(sc)
	r.serverSettings = &fakeServerSettingsRepo{settings: &models.ServerSettings{Hostname: "host.example.com"}}
	dom := &models.Domain{ID: "d1", Name: "example.com", SSLMode: models.SSLModeNone}

	r.reconcileSSLForDomain(context.Background(), dom)

	if !sc.revoked["c1"] {
		t.Fatal("None mode must MarkRevoked (clear local cert) even when ssl.revoke fails")
	}
}
