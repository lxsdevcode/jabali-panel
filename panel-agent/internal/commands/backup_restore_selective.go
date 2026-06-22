// backup_restore_selective.go — GH #267 Wave 2 (tenant selective restore).
//
// DESTRUCTIVE, fail-closed, DATABASES ONLY. Restores a chosen subset of an
// account backup's databases into the owner's existing DBs by reusing the exact
// proven apply path (applyAccountRestore) — but with the manifest stage list
// FILTERED to only the requested db stages, so home / mail / meta are never in
// the apply set and cannot be touched. That filtering is the whole selectivity
// guarantee: the negative round-trip (restore one DB, assert the others + home
// are byte-unchanged) is the ship gate.
//
// Why DB-only (see plans/m267-tenant-selective-restore.md):
//   - home rsync uses --delete (removes live files absent from the backup) —
//     deferred to a later wave with explicit overwrite-flag design.
//   - mail is NOT auto-appliable (stalwart-cli over a running spool corrupts
//     RocksDB) — the admin path refuses too; safe path is JMAP import (future).
//   - DNS managed records re-derive from domain config; custom records aren't
//     captured.
//
// Safety:
//   - overwrite MUST be true to write anything (DB restore is drop+reload). With
//     overwrite=false the handler applies NOTHING and reports what would change.
//   - target_username (server-derived by the panel from the caller) must equal
//     the manifest owner — defense-in-depth against a mismatched manifest.
//   - reuses the global LOCK_NB restore flock so it can't race an admin restore.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/backup"
)

type backupRestoreSelectiveParams struct {
	JobID              string   `json:"job_id"`
	ManifestSnapshotID string   `json:"manifest_snapshot_id"`
	TargetUsername     string   `json:"target_username"`
	Databases          []string `json:"databases"`
	Overwrite          bool     `json:"overwrite"`
	// Restic dest (optional; empty = default local repo, like materialize).
	RepoURL        string   `json:"repo_url,omitempty"`
	CredentialsRef string   `json:"credentials_ref,omitempty"`
	PasswordFile   string   `json:"password_file,omitempty"`
	ExtraOptions   []string `json:"extra_options,omitempty"`
}

type backupRestoreSelectiveResult struct {
	JobID    string   `json:"job_id"`
	Applied  []string `json:"applied"`
	Skipped  []string `json:"skipped"`
	Warnings []string `json:"warnings"`
}

func backupRestoreSelectiveHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var p backupRestoreSelectiveParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, bkInvalidArg("malformed JSON body")
	}
	if !ulidRE.MatchString(p.JobID) {
		return nil, bkInvalidArg("job_id must be a 26-char ULID")
	}
	if p.ManifestSnapshotID == "" {
		return nil, bkInvalidArg("manifest_snapshot_id required")
	}
	if p.TargetUsername == "" {
		return nil, bkInvalidArg("target_username required")
	}
	if len(p.Databases) == 0 {
		return nil, bkInvalidArg("databases must be non-empty (DB-only restore in v1)")
	}

	out := backupRestoreSelectiveResult{JobID: p.JobID}

	// Global restore flock — same lock the admin restore uses, so a tenant
	// selective restore can't run concurrently with any other restore.
	if err := os.MkdirAll(filepath.Dir(restoreLockPath), 0o750); err != nil {
		return nil, bkInternal("mkdir lock dir", err)
	}
	lf, err := os.OpenFile(restoreLockPath, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return nil, bkInternal("open lock file", err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, ErrRestoreLocked
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)

	cfg, cerr := bkResticConfigWithPassword(p.RepoURL, p.CredentialsRef, p.PasswordFile, p.ExtraOptions)
	if cerr != nil {
		return nil, bkInternal("restic config", cerr)
	}
	c := backup.New(cfg)

	manifestBytes, err := c.Dump(ctx, p.ManifestSnapshotID, "manifest.json")
	if err != nil {
		return nil, bkInternal("read manifest", err)
	}
	manifest, err := backup.AccountManifestFromBytes(manifestBytes)
	if err != nil {
		return nil, bkInternal("parse manifest", err)
	}

	// Defense-in-depth: the manifest's owner must match the server-derived
	// target. The panel already forces target_username from the caller and
	// 404s a cross-user job, but never apply onto a username the snapshot
	// wasn't taken for.
	if manifest.User.Username != p.TargetUsername {
		return nil, bkInvalidArg(fmt.Sprintf(
			"manifest owner %q != target_username %q — refusing",
			manifest.User.Username, p.TargetUsername))
	}

	// Build the requested set; filter the manifest to ONLY db stages whose
	// db name was requested. Anything requested but not in the backup is a
	// warning, not a failure.
	want := map[string]bool{}
	for _, d := range p.Databases {
		want[d] = true
	}
	inManifest := map[string]bool{}
	var filtered []backup.ManifestStage
	for _, st := range manifest.Stages {
		if st.Name != backup.StageDB || st.Status != backup.StageStatusOK || len(st.Items) == 0 {
			continue
		}
		db := st.Items[0]
		inManifest[db] = true
		if want[db] {
			filtered = append(filtered, st)
		}
	}
	for _, d := range p.Databases {
		if !inManifest[d] {
			out.Warnings = append(out.Warnings, fmt.Sprintf("database %q is not in this backup — skipped", d))
		}
	}
	if len(filtered) == 0 {
		out.Warnings = append(out.Warnings, "no requested database matched the backup — nothing to do")
		return out, nil
	}

	// Fail-closed: DB restore drops + reloads. Do nothing unless the caller
	// explicitly opted into overwrite.
	if !p.Overwrite {
		for _, st := range filtered {
			out.Skipped = append(out.Skipped, st.Items[0])
		}
		out.Warnings = append(out.Warnings,
			"overwrite=false: nothing applied. Restoring a database drops and reloads it; "+
				"re-send with overwrite=true to replace the selected database(s).")
		return out, nil
	}

	// Restic-restore ONLY the filtered db stages into a per-job staging dir,
	// then hand the SAME filtered list to applyAccountRestore — which walks
	// exactly the stages it is given, so only these databases are written.
	stagingRoot := filepath.Join("/var/lib/jabali-backups/restore-staging", p.JobID+"-sel")
	defer os.RemoveAll(stagingRoot)

	var stageResults []backupRestoreStage
	for _, st := range filtered {
		target := filepath.Join(stagingRoot, st.Name)
		if err := os.MkdirAll(target, 0o750); err != nil {
			stageResults = append(stageResults, backupRestoreStage{
				Name: st.Name, Status: backup.StageStatusFailed,
				Error: fmt.Sprintf("mkdir staging: %v", err),
			})
			continue
		}
		rerr := c.Restore(ctx, backup.RestoreOpts{SnapshotID: st.SnapshotID, Target: target})
		sr := backupRestoreStage{Name: st.Name, Status: backup.StageStatusOK}
		if rerr != nil {
			sr.Status = backup.StageStatusFailed
			sr.Error = rerr.Error()
		}
		stageResults = append(stageResults, sr)
	}

	applied, warnings := applyAccountRestore(ctx, stagingRoot, p.TargetUsername, manifest.User, filtered, stageResults)
	out.Applied = applied
	out.Warnings = append(out.Warnings, warnings...)
	return out, nil
}

func init() {
	Default.Register("backup.restore_selective", backupRestoreSelectiveHandler)
}
