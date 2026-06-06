package commands

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

func TestMailSANHostnames_Order(t *testing.T) {
	got := mailSANHostnames("Example.COM")
	want := []string{"mail.example.com", "autoconfig.example.com", "autodiscover.example.com", "mta-sts.example.com"}
	assert.Equal(t, want, got, "order must be stable + lowercased")
}

func TestSSLMailIssue_RejectsEmptyDomain(t *testing.T) {
	raw, _ := json.Marshal(sslMailIssueParams{Domain: "  "})
	_, err := sslMailIssueHandler(context.Background(), raw)
	require.Error(t, err)
	var aerr *agentwire.AgentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, agentwire.CodeInvalidArgument, aerr.Code)
}

func TestSSLMailIssue_RejectsInvalidDomain(t *testing.T) {
	raw, _ := json.Marshal(sslMailIssueParams{Domain: "../etc/passwd"})
	_, err := sslMailIssueHandler(context.Background(), raw)
	require.Error(t, err)
	var aerr *agentwire.AgentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, agentwire.CodeInvalidArgument, aerr.Code)
}

func TestSSLMailIssue_RequiresPublicIPWhenNotSkipped(t *testing.T) {
	raw, _ := json.Marshal(sslMailIssueParams{Domain: "example.com"})
	_, err := sslMailIssueHandler(context.Background(), raw)
	require.Error(t, err)
	var aerr *agentwire.AgentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, agentwire.CodeInvalidArgument, aerr.Code)
}

func TestSSLMailIssue_SkipDNS_StillRequiresEmail(t *testing.T) {
	raw, _ := json.Marshal(sslMailIssueParams{
		Domain:  "example.com",
		SkipDNS: true,
	})
	_, err := sslMailIssueHandler(context.Background(), raw)
	require.Error(t, err)
	var aerr *agentwire.AgentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, agentwire.CodeInvalidArgument, aerr.Code)
}

func TestScanMailSANDNS_AllNonResolvableReturnsFalse(t *testing.T) {
	// A bunch of guaranteed-NX hostnames. Even if a flaky network
	// returns some glue address, none of these can ever equal the
	// canary publicIP below, so the func must return false.
	sans := []string{
		"mail.this-domain-cannot-exist-jabali-test.invalid",
		"autoconfig.this-domain-cannot-exist-jabali-test.invalid",
		"autodiscover.this-domain-cannot-exist-jabali-test.invalid",
		"mta-sts.this-domain-cannot-exist-jabali-test.invalid",
	}
	_, ok := scanMailSANDNS(context.Background(), sans, "203.0.113.42")
	assert.False(t, ok, "no SAN should match the canary IP")
}
