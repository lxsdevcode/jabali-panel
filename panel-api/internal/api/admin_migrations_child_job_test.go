package api

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// inheritedFromDraft — fields a bulk child takes from the discovery draft.
// Every one of these has been, or could have been, a silent-drop bug.
var inheritedFromDraft = map[string]string{
	"Mode":            "import vs refresh is a property of the whole migration, not one account",
	"ExpectedHostKey": "an empty pin makes every transfer connection accept any host key",
	"SourcePort":      "GH #429 — the pull dials this; 22 is a timeout on a custom-port source",
	"SourcePath":      "wordpress_ssh control-input; harmless and more correct to carry over",
	"PlanJSON":        "GH #633/#634 — without it preserve.credentials is lost and passwords reset",
}

// perChild — fields the child must own rather than inherit. The reason matters:
// several of these would be actively wrong if copied.
var perChild = map[string]string{
	"ID":           "new identity",
	"BatchID":      "set to the new batch",
	"SourceKind":   "supplied and validated by the request",
	"SourceHost":   "supplied and validated by the request",
	"SourceUser":   "this is the account being migrated — the whole point of the fan-out",
	"TargetUserID": "resolved per account from account_targets",
	"State":        "children start pending or draft depending on secret inheritance",
	"ManifestJSON": "the draft's discovery OUTPUT describes a different subscription",
	"LastError":    "belongs to this child's own run",
	"StartedAt":    "this child has not started",
	"EndedAt":      "this child has not ended",
	"CreatedAt":    "stamped on insert",
	"UpdatedAt":    "stamped on insert",
	"DestUser":     "destination binding is resolved per account later",
	"DestDomain":   "destination binding is resolved per account later",
}

// TestBulkChildJobFieldsAreClassified is the exhaustiveness check Go will not
// give us on a struct literal.
//
// Four fields have been silently dropped from bulk children over time — the SSH
// secret, the preserve plan (GH #633/#634), the SSH port (GH #429) and the
// host-key pin. Each was invisible: an omitted field is the zero value, and for
// a column with `gorm:"default:N"` that zero quietly becomes N.
//
// newBulkChildJob now copies the draft and clears what must not carry over, so
// a new field is inherited by default rather than dropped. This test closes the
// other half: adding a field to MigrationJob fails here until someone states
// which side it belongs on, with a reason.
func TestBulkChildJobFieldsAreClassified(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(models.MigrationJob{})
	var unclassified, both []string

	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue // unexported
		}
		_, inh := inheritedFromDraft[f.Name]
		_, own := perChild[f.Name]
		switch {
		case inh && own:
			both = append(both, f.Name)
		case !inh && !own:
			unclassified = append(unclassified, f.Name)
		}
	}

	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Errorf("MigrationJob field(s) %s are not classified for bulk children.\n"+
			"Add each to inheritedFromDraft (carried over from the discovery "+
			"draft) or perChild (owned by the child) in "+
			"admin_migrations_child_job_test.go, with a reason.\n"+
			"This is deliberate friction: four fields have already been lost by "+
			"nobody making this decision.", strings.Join(unclassified, ", "))
	}
	for _, name := range both {
		t.Errorf("%s is in both inheritedFromDraft and perChild — pick one", name)
	}
}

// TestNewBulkChildJob_InheritsAndClears exercises the constructor against a
// fully-populated draft, so a field that stops being copied (or starts being
// copied when it should not be) shows up here rather than in production.
func TestNewBulkChildJob_InheritsAndClears(t *testing.T) {
	t.Parallel()

	plan := `{"preserve":{"credentials":true}}`
	manifest := `{"accounts":["someone-else"]}`
	srcPath := "/var/www/wp"
	lastErr := "previous failure"
	destUser := "olduser"
	destDomain := "old.example.com"
	oldTarget := "OLDTARGET"
	oldBatch := "OLDBATCH"
	ended := time.Unix(1, 0).UTC()

	draft := &models.MigrationJob{
		ID: "DRAFT01", BatchID: &oldBatch,
		SourceKind: "plesk", SourceHost: "plesk01", SourceUser: "__discovery__",
		Mode: "refresh", ExpectedHostKey: "SHA256:pinned", SourcePort: 8404,
		SourcePath: &srcPath, PlanJSON: &plan, ManifestJSON: &manifest,
		TargetUserID: &oldTarget, State: models.MigrationStateDraft,
		LastError: &lastErr, EndedAt: &ended,
		StartedAt: time.Unix(2, 0).UTC(),
		DestUser:  &destUser, DestDomain: &destDomain,
	}

	newBatch := "NEWBATCH"
	target := "TARGET9"
	got := newBulkChildJob(draft, childJobInputs{
		ID: "CHILD01", BatchID: &newBatch, Account: "shop.example.com",
		State: models.MigrationStatePending, TargetID: &target,
		SourceKind: "plesk", SourceHost: "plesk01", SourcePort: 8404,
	})

	// Inherited.
	if got.Mode != "refresh" {
		t.Errorf("Mode = %q, want the draft's %q", got.Mode, "refresh")
	}
	if got.ExpectedHostKey != "SHA256:pinned" {
		t.Errorf("ExpectedHostKey = %q, want the draft's pin", got.ExpectedHostKey)
	}
	if got.SourcePort != 8404 {
		t.Errorf("SourcePort = %d, want 8404", got.SourcePort)
	}
	if got.PlanJSON == nil || *got.PlanJSON != plan {
		t.Errorf("PlanJSON = %v, want the draft's plan", got.PlanJSON)
	}
	if got.SourcePath == nil || *got.SourcePath != srcPath {
		t.Errorf("SourcePath = %v, want the draft's path", got.SourcePath)
	}

	// Per-child.
	if got.ID != "CHILD01" || got.SourceUser != "shop.example.com" {
		t.Errorf("identity not overridden: id=%q user=%q", got.ID, got.SourceUser)
	}
	if got.BatchID == nil || *got.BatchID != newBatch {
		t.Errorf("BatchID = %v, want the new batch", got.BatchID)
	}
	if got.TargetUserID == nil || *got.TargetUserID != target {
		t.Errorf("TargetUserID = %v, want the per-account target", got.TargetUserID)
	}
	if got.State != models.MigrationStatePending {
		t.Errorf("State = %q, want pending", got.State)
	}
	// The dangerous ones: inheriting these would describe the wrong account or
	// resurrect a finished run.
	if got.ManifestJSON != nil {
		t.Errorf("ManifestJSON must not be inherited — it is the draft's own "+
			"discovery output and describes a different subscription; got %v",
			*got.ManifestJSON)
	}
	if got.LastError != nil {
		t.Errorf("LastError must not carry over; got %v", *got.LastError)
	}
	if got.EndedAt != nil {
		t.Errorf("EndedAt must not carry over; got %v", *got.EndedAt)
	}
	if !got.StartedAt.IsZero() {
		t.Errorf("StartedAt must be zero on a job that has not started; got %v", got.StartedAt)
	}
	if got.DestUser != nil || got.DestDomain != nil {
		t.Errorf("destination binding must be resolved per account, not inherited: user=%v domain=%v",
			got.DestUser, got.DestDomain)
	}
}

// TestNewBulkChildJob_NilDraft covers the legacy path — no discovery draft to
// inherit from — where only the supplied inputs should be set.
func TestNewBulkChildJob_NilDraft(t *testing.T) {
	t.Parallel()

	batch := "B1"
	got := newBulkChildJob(nil, childJobInputs{
		ID: "CHILD01", BatchID: &batch, Account: "shop.example.com",
		State:      models.MigrationStateDraft,
		SourceKind: "plesk", SourceHost: "plesk01", SourcePort: 2222,
	})

	if got.SourcePort != 2222 || got.SourceUser != "shop.example.com" {
		t.Errorf("supplied inputs not applied: port=%d user=%q", got.SourcePort, got.SourceUser)
	}
	if got.PlanJSON != nil || got.ExpectedHostKey != "" || got.Mode != "" {
		t.Errorf("nothing should be inherited without a draft: plan=%v pin=%q mode=%q",
			got.PlanJSON, got.ExpectedHostKey, got.Mode)
	}
}
