package api

import (
	"strings"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// The bulk handler builds one child migration job per selected account. Three
// separate bugs have now been caused by the same thing: a field that lives on
// the discovery draft, is needed by the pull, and was simply not written into
// the child struct literal.
//
//	the SSH secret       — needed migration.secrets_clone
//	the preserve plan    — GH #633/#634; passwords silently reset without it
//	the SSH port         — GH #429; pull dialled 22 and timed out
//	the host-key pin     — every transfer connection ran trust-on-first-use
//
// Each was invisible: an omitted field is the zero value, and for columns
// carrying `gorm:"default:N"` the zero value quietly becomes N. Nothing fails,
// nothing logs, and the symptom appears far from the cause.
//
// The literal made the wrong thing the default. Writing a field was opt-IN, so
// forgetting one meant dropping it. newBulkChildJob inverts that: it starts
// from a copy of the draft, so a field added to MigrationJob later is inherited
// unless someone deliberately clears it. Dropping now takes an explicit act,
// which is the direction the last four bugs argue for.
//
// Go cannot enforce exhaustiveness on a struct literal, so the second half of
// the guard is TestBulkChildJobFieldsAreClassified: every exported field of
// MigrationJob must be listed as inherited or per-child, and a new field fails
// the build until someone decides which it is.

// childJobInputs is everything the caller has to supply per child. Naming them
// explicitly keeps the call site honest about what varies per account and what
// merely rides along from the draft.
type childJobInputs struct {
	ID       string
	BatchID  *string
	Account  string
	State    string
	TargetID *string

	// SourceKind and SourceHost come from the request rather than the draft
	// because the bulk endpoint accepts them directly and validates them; the
	// draft's copies are the same values.
	SourceKind string
	SourceHost string

	// SourcePort is resolved by the caller: the draft's value when there is a
	// draft, otherwise the request's, otherwise 22. Passed in rather than read
	// here so the precedence lives in one visible place.
	SourcePort int
}

// newBulkChildJob builds one child from the discovery draft. A nil draft is the
// legacy path — a caller with no draft to inherit from — and yields a job with
// only the explicitly supplied fields set.
func newBulkChildJob(draft *models.MigrationJob, in childJobInputs) *models.MigrationJob {
	row := &models.MigrationJob{}
	if draft != nil {
		// Copy first, override after. This is what makes inheritance the
		// default: a field added to MigrationJob arrives here automatically.
		*row = *draft
	}

	// ── per-child: identity and batch membership ──────────────────────────
	row.ID = in.ID
	row.BatchID = in.BatchID
	row.SourceUser = in.Account
	row.State = in.State
	row.TargetUserID = in.TargetID

	// ── per-child: request-supplied, validated by the handler ─────────────
	row.SourceKind = in.SourceKind
	row.SourceHost = in.SourceHost
	row.SourcePort = in.SourcePort

	// ── per-child: must NOT carry over from the draft ─────────────────────
	// ManifestJSON is the draft's own discovery OUTPUT — each child discovers
	// its own account, and inheriting it would describe the wrong subscription.
	row.ManifestJSON = nil
	// Lifecycle fields belong to this child's own run. GORM stamps CreatedAt /
	// UpdatedAt on insert; clearing them keeps a copied draft from carrying a
	// stale StartedAt into a job that has not started.
	row.LastError = nil
	row.EndedAt = nil
	row.StartedAt = time.Time{}
	row.CreatedAt = time.Time{}
	row.UpdatedAt = time.Time{}
	// Destination binding is resolved per account later in the flow.
	row.DestUser = nil
	row.DestDomain = nil

	// Everything not touched above is inherited on purpose: Mode,
	// ExpectedHostKey, SourcePath and PlanJSON.
	return row
}

// resolveChildTarget maps an account to an existing target user when the
// operator picked one; empty means "auto-create a new Jabali user".
func resolveChildTarget(accountTargets map[string]string, acct string) *string {
	if accountTargets == nil {
		return nil
	}
	if t := strings.TrimSpace(accountTargets[acct]); t != "" {
		return &t
	}
	return nil
}
