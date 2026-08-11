package backupfinalizer

// GH #1044: restores run as background goroutines sealed by their own
// MarkFinished. That fails in exactly one way — a panel restart kills the
// goroutine mid-restore and the row stays `running` forever, because the
// finalizer used to skip restore-kind rows entirely on the (now false)
// assumption that the API handler seals them synchronously.
//
// The sweep must be surgical: orphaned restores sealed, in-flight restores
// left alone, and the manifest-tracking path for real backups untouched.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// restoreOrphanJobs extends the shared pagingJobs fake with error capture so
// the seal reason is assertable.
type restoreOrphanJobs struct {
	pagingJobs
	sealedErr map[string]string
}

func (j *restoreOrphanJobs) MarkFinished(_ context.Context, id, _ string, _, _ string,
	_, _ uint64, _, _ json.RawMessage, errText string) error {
	j.finishedIDs = append(j.finishedIDs, id)
	if j.sealedErr == nil {
		j.sealedErr = map[string]string{}
	}
	j.sealedErr[id] = errText
	return nil
}

func startedAgo(d time.Duration) *time.Time {
	t := time.Now().UTC().Add(-d)
	return &t
}

func TestFinalizer_SealsOrphanedRestoreRows(t *testing.T) {
	jobs := &restoreOrphanJobs{pagingJobs: pagingJobs{all: []models.BackupJob{
		// Orphaned: running way past the goroutine's own 60m bound.
		{ID: "restore-old", Kind: models.BackupJobKindAccountRestore,
			Status: models.BackupJobStatusRunning, StartedAt: startedAgo(2 * time.Hour)},
		// In-flight: a long pg restore that is still legitimately running.
		{ID: "restore-young", Kind: models.BackupJobKindAccountRestore,
			Status: models.BackupJobStatusRunning, StartedAt: startedAgo(30 * time.Minute)},
		// System restore orphan gets the same treatment.
		{ID: "sysrestore-old", Kind: models.BackupJobKindSystemRestore,
			Status: models.BackupJobStatusRunning, StartedAt: startedAgo(3 * time.Hour)},
	}}}
	f := &Finalizer{deps: Deps{
		Jobs: jobs,
		Log:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}

	f.tickOnce(context.Background())

	sealed := map[string]bool{}
	for _, id := range jobs.finishedIDs {
		sealed[id] = true
	}
	if !sealed["restore-old"] {
		t.Error("orphaned account_restore row was not sealed — it stays `running` forever after a panel restart")
	}
	if !sealed["sysrestore-old"] {
		t.Error("orphaned system_restore row was not sealed")
	}
	if sealed["restore-young"] {
		t.Error("an in-flight restore inside the goroutine's bound was sealed — a slow-but-alive pg restore just got its row failed under it")
	}
	if err := jobs.sealedErr["restore-old"]; err == "" || !containsStr(err, "interrupted") {
		t.Errorf("seal reason should say the restore was interrupted, got %q", err)
	}
}

func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	}()
}
