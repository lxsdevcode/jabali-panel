package reconciler

import (
	"context"
	"os"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// --- minimal embedded-interface fakes (only the methods the reconciler uses) ---

type srFake struct {
	repository.SharedResourceRepository
	res        []models.SharedResource
	grants     map[string][]models.SharedResourceGrant
	tombs      []string
	proj       map[string][2]string
	tombKilled []string
}

func (f *srFake) ListAll(context.Context) ([]models.SharedResource, error) { return f.res, nil }
func (f *srFake) ListGrants(_ context.Context, id string) ([]models.SharedResourceGrant, error) {
	return f.grants[id], nil
}
func (f *srFake) UpdateProjection(_ context.Context, id, host, col string) error {
	if f.proj == nil {
		f.proj = map[string][2]string{}
	}
	f.proj[id] = [2]string{host, col}
	return nil
}
func (f *srFake) ListTombstones(context.Context) ([]string, error) { return f.tombs, nil }
func (f *srFake) DeleteTombstone(_ context.Context, e string) error {
	f.tombKilled = append(f.tombKilled, e)
	return nil
}

type mbFake struct {
	repository.MailboxRepository
	byID map[string]*models.Mailbox
}

func (f *mbFake) FindByID(_ context.Context, id string) (*models.Mailbox, error) {
	return f.byID[id], nil
}

type mgFake struct {
	repository.MailGroupRepository
	members map[string][]string
}

func (f *mgFake) ListMemberEmails(_ context.Context, id string) ([]string, error) {
	return f.members[id], nil
}

type domFake struct {
	repository.DomainRepository
	byID map[string]*models.Domain
}

func (f *domFake) FindByID(_ context.Context, id string) (*models.Domain, error) {
	return f.byID[id], nil
}

func sptr(s string) *string { return &s }

func newSRReconciler(t *testing.T, ag *fakeAgent, sr *srFake, mb *mbFake, mg *mgFake, dom *domFake) *Reconciler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return New(dom, nil, ag, log, Config{Interval: time.Second}).WithSharedResources(sr, mb, mg)
}

func TestReconcileSharedResources_ProjectsAndShares(t *testing.T) {
	email := "teamcal@example.org"
	sr := &srFake{
		res: []models.SharedResource{{
			ID: "res1", DomainID: "dom1", Kind: "calendar", EmailCached: sptr(email), DisplayName: "Team Cal",
		}},
		grants: map[string][]models.SharedResourceGrant{
			"res1": {
				{ResourceID: "res1", GranteeKind: "mailbox", GranteeID: "mb1", Rights: "readwrite"},
				{ResourceID: "res1", GranteeKind: "group", GranteeID: "grp1", Rights: "read"},
			},
		},
	}
	mb := &mbFake{byID: map[string]*models.Mailbox{"mb1": {ID: "mb1", LocalPart: "alice", DomainID: "dom1"}}}
	mg := &mgFake{members: map[string][]string{"grp1": {"bob@example.org"}}}
	dom := &domFake{byID: map[string]*models.Domain{"dom1": {ID: "dom1", Name: "example.org"}}}
	ag := &fakeAgent{}

	r := newSRReconciler(t, ag, sr, mb, mg, dom)
	r.reconcileSharedResources(context.Background())

	// apply was called + projection cached.
	var applied, shared *fakeCall
	for i := range ag.calls {
		switch ag.calls[i].method {
		case "sharedresource.apply":
			applied = &ag.calls[i]
		case "calendar.share_set":
			shared = &ag.calls[i]
		}
	}
	require.NotNil(t, applied, "sharedresource.apply must be called")
	require.Equal(t, [2]string{"hostAcct1", ""}, sr.proj["res1"], "host_account_id cached")
	require.NotNil(t, shared, "calendar.share_set must be called")

	p := shared.params.(map[string]any)
	require.Equal(t, email, p["owner_email"])
	shares := p["shares"].(map[string]string)
	require.Equal(t, "readwrite", shares["alice@example.org"], "mailbox grantee resolved")
	require.Equal(t, "read", shares["bob@example.org"], "group grantee expanded to members")
}

func TestReconcileSharedResources_TombstoneGC(t *testing.T) {
	sr := &srFake{tombs: []string{"gone@example.org"}}
	ag := &fakeAgent{}
	r := newSRReconciler(t, ag, sr, &mbFake{}, &mgFake{}, &domFake{})
	r.reconcileSharedResources(context.Background())

	var destroyed bool
	for _, c := range ag.calls {
		if c.method == "sharedresource.destroy" {
			destroyed = true
			require.Equal(t, "gone@example.org", c.params.(map[string]any)["email"])
		}
	}
	require.True(t, destroyed, "tombstone host destroyed")
	require.Equal(t, []string{"gone@example.org"}, sr.tombKilled, "tombstone cleared on success")
}

func TestReconcileSharedResources_TombstoneRetainedOnAgentFailure(t *testing.T) {
	sr := &srFake{tombs: []string{"gone@example.org"}}
	ag := &fakeAgent{failMethod: "sharedresource.destroy"}
	r := newSRReconciler(t, ag, sr, &mbFake{}, &mgFake{}, &domFake{})
	r.reconcileSharedResources(context.Background())
	require.Empty(t, sr.tombKilled, "tombstone kept when destroy fails (retry next pass)")
}
