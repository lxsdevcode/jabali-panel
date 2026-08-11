package api

// GH #1044: restores moved out of the HTTP request into background job
// runners. The runners own the row-sealing now, so their semantics are the
// contract: an agent failure seals `failed`, a partial stage set seals
// `partial`, success persists the outcome the drawer renders — and every
// seal happens on a FRESH context, because the request context these used
// to borrow is long dead by the time a big restore finishes.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

type sealCapture struct {
	repository.BackupJobRepository
	mu       sync.Mutex
	status   string
	errText  string
	manifest json.RawMessage
	warnings json.RawMessage
	sealed   chan struct{}
}

func newSealCapture() *sealCapture { return &sealCapture{sealed: make(chan struct{})} }

func (r *sealCapture) MarkFinished(_ context.Context, _, status string, _, _ string,
	_, _ uint64, manifest, warnings json.RawMessage, errText string) error {
	r.mu.Lock()
	r.status, r.errText, r.manifest, r.warnings = status, errText, manifest, warnings
	r.mu.Unlock()
	close(r.sealed)
	return nil
}

func (r *sealCapture) wait(t *testing.T) {
	t.Helper()
	select {
	case <-r.sealed:
	case <-time.After(5 * time.Second):
		t.Fatal("job was never sealed — a restore that finishes without MarkFinished leaves the row `running` forever")
	}
}

type restoreAgent struct {
	reply json.RawMessage
	err   error
}

func (a restoreAgent) Call(context.Context, string, any) (json.RawMessage, error) {
	return a.reply, a.err
}

func TestRunAccountRestoreJob_AgentFailureSealsFailed(t *testing.T) {
	jobs := newSealCapture()
	h := &backupHandler{cfg: BackupHandlerConfig{
		Jobs:  jobs,
		Agent: restoreAgent{err: errors.New("restic: repo locked")},
	}}
	h.runAccountRestoreJob("job-1", &models.BackupDestination{ID: "d1", Kind: "local"}, map[string]any{})
	jobs.wait(t)
	if jobs.status != models.BackupJobStatusFailed {
		t.Errorf("status = %q, want failed", jobs.status)
	}
	if !strings.Contains(jobs.errText, "repo locked") {
		t.Errorf("the AGENT failure should be the recorded reason, got %q", jobs.errText)
	}
}

func TestRunAccountRestoreJob_PartialStagesSealPartial(t *testing.T) {
	jobs := newSealCapture()
	h := &backupHandler{cfg: BackupHandlerConfig{
		Jobs: jobs,
		Agent: restoreAgent{reply: json.RawMessage(
			`{"job_id":"job-1","stages":[{"name":"home","status":"ok"},{"name":"db","status":"failed","error":"pg_restore: boom"}]}`)},
	}}
	h.runAccountRestoreJob("job-1", &models.BackupDestination{ID: "d1", Kind: "local"}, map[string]any{})
	jobs.wait(t)
	if jobs.status != models.BackupJobStatusPartial {
		t.Errorf("status = %q, want partial — one failed stage out of two", jobs.status)
	}
	if jobs.errText == "" || !strings.Contains(jobs.errText, "pg_restore") {
		t.Errorf("the failed stage's error should surface on the row, got %q", jobs.errText)
	}
}

func TestRunSelectiveRestoreJob_PersistsOutcomeForTheDrawer(t *testing.T) {
	jobs := newSealCapture()
	h := &meBackupHandler{cfg: MeBackupsHandlerConfig{
		Jobs: jobs,
		Agent: restoreAgent{reply: json.RawMessage(
			`{"applied":["db → alice_pg (postgres)"],"skipped":[],"warnings":["mail: 1 message already existed"]}`)},
	}}
	h.runSelectiveRestoreJob("job-2", "snap", "alice",
		meRestoreSelectiveRequest{Databases: []string{"alice_pg"}, Overwrite: true}, nil)
	jobs.wait(t)
	if jobs.status != models.BackupJobStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", jobs.status)
	}
	// The drawer polls the job and renders warnings_json — an empty blob
	// here means the tenant sees "restored" with no detail of WHAT.
	var out selectiveRestoreOutcome
	if err := json.Unmarshal(jobs.warnings, &out); err != nil {
		t.Fatalf("warnings_json is not the outcome shape: %v (%s)", err, jobs.warnings)
	}
	if len(out.Applied) != 1 || !strings.Contains(out.Applied[0], "alice_pg") {
		t.Errorf("applied list lost between agent and row: %#v", out)
	}
	if len(out.Warnings) != 1 {
		t.Errorf("agent warnings lost: %#v", out.Warnings)
	}
}

func TestRunSelectiveRestoreJob_AgentFailureIsGenericOnTheRow(t *testing.T) {
	jobs := newSealCapture()
	h := &meBackupHandler{cfg: MeBackupsHandlerConfig{
		Jobs:  jobs,
		Agent: restoreAgent{err: errors.New("restic: /var/lib secret path leaked")},
	}}
	h.runSelectiveRestoreJob("job-3", "snap", "alice",
		meRestoreSelectiveRequest{Databases: []string{"x"}, Overwrite: true}, nil)
	jobs.wait(t)
	if jobs.status != models.BackupJobStatusFailed {
		t.Fatalf("status = %q, want failed", jobs.status)
	}
	// JAB-114: agent errors carry host internals. The tenant-visible row
	// must NOT echo them.
	if strings.Contains(jobs.errText, "/var/lib") {
		t.Errorf("agent internals leaked onto the tenant-visible row: %q", jobs.errText)
	}
}
