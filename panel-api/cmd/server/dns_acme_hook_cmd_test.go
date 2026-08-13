package main

import (
	"context"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

type fakeZoneByNameRepo struct {
	repository.DNSZoneRepository
	zones map[string]*models.DNSZone
}

func (f *fakeZoneByNameRepo) FindByName(_ context.Context, name string) (*models.DNSZone, error) {
	if z, ok := f.zones[name]; ok {
		return z, nil
	}
	return nil, repository.ErrNotFound
}

func TestFindZoneForName_WalksParentChain(t *testing.T) {
	repo := &fakeZoneByNameRepo{zones: map[string]*models.DNSZone{
		"example.com": {ID: "Z1", Name: "example.com"},
		"host.tld":    {ID: "Z2", Name: "host.tld"},
	}}
	ctx := context.Background()

	cases := []struct {
		name   string
		wantID string
	}{
		// certbot passes CERTBOT_DOMAIN without the "*." for wildcards, so
		// a *.example.com challenge arrives as "example.com".
		{"example.com", "Z1"},
		{"_acme-challenge.example.com", "Z1"},
		{"preview.host.tld", "Z2"},
		{"deep.sub.preview.host.tld", "Z2"},
	}
	for _, tc := range cases {
		z, err := findZoneForName(ctx, repo, tc.name)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if z.ID != tc.wantID {
			t.Errorf("%s: matched zone %s, want %s", tc.name, z.ID, tc.wantID)
		}
	}
}

func TestFindZoneForName_RefusesForeignZone(t *testing.T) {
	repo := &fakeZoneByNameRepo{zones: map[string]*models.DNSZone{}}
	if _, err := findZoneForName(context.Background(), repo, "elsewhere.net"); err == nil {
		t.Fatal("expected an error for a name with no local zone")
	}
}

func TestAcmeChallengeName(t *testing.T) {
	if got := acmeChallengeName("preview.host.tld"); got != "_acme-challenge.preview.host.tld" {
		t.Errorf("got %q", got)
	}
}

// --- waitForChallengeTXT (JAB-235 follow-up: hoff.co.il propagation race) ---

func TestWaitForChallengeTXT_VisibleImmediately(t *testing.T) {
	lookupNS := func(_ context.Context, _ string) ([]string, bool) {
		return []string{"ns1.example.net", "ns2.example.net"}, true
	}
	lookupTXT := func(_ context.Context, server, name string) ([]string, error) {
		return []string{"other-value", "the-value"}, nil
	}
	if !waitForChallengeTXT(context.Background(), "example.com",
		"_acme-challenge.www.example.com", "the-value",
		lookupNS, lookupTXT, 0, 0) {
		t.Fatal("expected immediate visibility to return true")
	}
}

func TestWaitForChallengeTXT_VisibleAfterRetries(t *testing.T) {
	lookupNS := func(_ context.Context, _ string) ([]string, bool) {
		return []string{"ns1.example.net"}, true
	}
	calls := 0
	lookupTXT := func(_ context.Context, _, _ string) ([]string, error) {
		calls++
		if calls >= 3 {
			return []string{"the-value"}, nil
		}
		return nil, nil
	}
	if !waitForChallengeTXT(context.Background(), "example.com",
		"_acme-challenge.example.com", "the-value",
		lookupNS, lookupTXT, time.Millisecond, time.Second) {
		t.Fatal("expected visibility on the 3rd poll to return true")
	}
	if calls < 3 {
		t.Fatalf("expected >=3 lookups, got %d", calls)
	}
}

func TestWaitForChallengeTXT_TimesOut(t *testing.T) {
	lookupNS := func(_ context.Context, _ string) ([]string, bool) {
		return []string{"ns1.example.net"}, true
	}
	lookupTXT := func(_ context.Context, _, _ string) ([]string, error) {
		return []string{"never-the-right-value"}, nil
	}
	if waitForChallengeTXT(context.Background(), "example.com",
		"_acme-challenge.example.com", "the-value",
		lookupNS, lookupTXT, time.Millisecond, 10*time.Millisecond) {
		t.Fatal("expected timeout to return false")
	}
}

func TestWaitForChallengeTXT_InconclusiveNSFailsOpen(t *testing.T) {
	lookupNS := func(_ context.Context, _ string) ([]string, bool) {
		return nil, false // resolvers unreachable
	}
	lookupTXT := func(_ context.Context, _, _ string) ([]string, error) {
		t.Fatal("lookupTXT must not be called when NS is inconclusive")
		return nil, nil
	}
	if waitForChallengeTXT(context.Background(), "example.com",
		"_acme-challenge.example.com", "the-value",
		lookupNS, lookupTXT, time.Millisecond, time.Second) {
		t.Fatal("expected inconclusive NS to return false (caller proceeds anyway)")
	}
}

func TestWaitForChallengeTXT_CancelledContext(t *testing.T) {
	lookupNS := func(_ context.Context, _ string) ([]string, bool) {
		return []string{"ns1.example.net"}, true
	}
	lookupTXT := func(_ context.Context, _, _ string) ([]string, error) {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitForChallengeTXT(ctx, "example.com",
		"_acme-challenge.example.com", "the-value",
		lookupNS, lookupTXT, time.Hour, time.Hour) {
		t.Fatal("expected cancelled context to return false promptly")
	}
}
