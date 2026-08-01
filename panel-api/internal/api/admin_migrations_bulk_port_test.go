package api

import (
	"context"
	"net/http"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// TestBulkCreate_ChildrenInheritDraftSourcePort guards GH #429.
//
// Reported symptom: the wizard connects and discovers subscriptions fine on a
// custom SSH port, then the import fails seconds later with
//
//	pull-source: plesk.Connect: tcp dial <host>:22: i/o timeout
//
// Discovery runs against the DRAFT job, which does hold the custom port, so
// everything up to that point confirms the connection is configured. bulkCreate
// then built each child without SourcePort, the `default:22` column tag turned
// that zero value into 22, and the auto-kicked pull-source dialled 22.
//
// This is the third thing children have to inherit explicitly, after the SSH
// secret and the preserve plan (GH #633/#634) — same handler, same shape of
// bug, found twice before this one.
//
// Only multi-account sources are affected. submitDraft reuses the discovery job
// rather than creating children, so single-account cPanel migrations on a
// custom port always worked, which is part of why this survived.
func TestBulkCreate_ChildrenInheritDraftSourcePort(t *testing.T) {
	h, jobs := newIMAPHandler(nil)

	draft := &models.MigrationJob{
		ID: "DRAFT01", SourceKind: "plesk", SourceHost: "plesk01",
		SourceUser: "__discovery__", SourcePort: 8404,
	}
	if err := jobs.Create(context.Background(), draft); err != nil {
		t.Fatal(err)
	}

	body := `{"source_kind":"plesk","source_host":"plesk01",` +
		`"accounts":["shop.example.com","blog.example.com"],"source_job_id":"DRAFT01"}`
	w := postJSON(h.bulkCreate, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("bulkCreate status=%d body=%s", w.Code, w.Body.String())
	}

	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	children := 0
	for id, j := range jobs.byID {
		if id == "DRAFT01" {
			continue
		}
		children++
		if j.SourcePort != 8404 {
			t.Errorf("child %s (%s) has SourcePort=%d, want 8404 inherited from "+
				"the draft — pull-source dials this port, so 22 here means the "+
				"import fails with a connection timeout on a host that just "+
				"discovered successfully", id, j.SourceUser, j.SourcePort)
		}
	}
	if children != 2 {
		t.Fatalf("expected 2 child jobs, got %d", children)
	}
}

// TestBulkCreate_ExplicitSourcePortUsedWithoutDraft covers the caller that has
// no discovery draft to inherit from — the legacy path where source_job_id is
// absent and children land as drafts for manual resume.
func TestBulkCreate_ExplicitSourcePortUsedWithoutDraft(t *testing.T) {
	h, jobs := newIMAPHandler(nil)

	body := `{"source_kind":"plesk","source_host":"plesk01",` +
		`"accounts":["shop.example.com"],"source_port":2222}`
	w := postJSON(h.bulkCreate, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("bulkCreate status=%d body=%s", w.Code, w.Body.String())
	}

	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	for _, j := range jobs.byID {
		if j.SourcePort != 2222 {
			t.Errorf("child %s has SourcePort=%d, want the explicitly supplied 2222",
				j.SourceUser, j.SourcePort)
		}
	}
}

// TestBulkCreate_DefaultsTo22WhenNothingSupplied pins that the fix did not turn
// the common case into a surprise: no draft, no explicit port, still 22.
func TestBulkCreate_DefaultsTo22WhenNothingSupplied(t *testing.T) {
	h, jobs := newIMAPHandler(nil)

	body := `{"source_kind":"plesk","source_host":"plesk01","accounts":["shop.example.com"]}`
	w := postJSON(h.bulkCreate, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("bulkCreate status=%d body=%s", w.Code, w.Body.String())
	}

	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	for _, j := range jobs.byID {
		if j.SourcePort != 22 {
			t.Errorf("child %s has SourcePort=%d, want the 22 default",
				j.SourceUser, j.SourcePort)
		}
	}
}

// TestBulkCreate_DraftPortWinsOverExplicit pins the precedence. The draft is
// what discovery actually connected with, so it is the value the operator has
// already seen succeed; a stale or defaulted body field must not override it.
func TestBulkCreate_DraftPortWinsOverExplicit(t *testing.T) {
	h, jobs := newIMAPHandler(nil)

	draft := &models.MigrationJob{
		ID: "DRAFT01", SourceKind: "plesk", SourceHost: "plesk01",
		SourceUser: "__discovery__", SourcePort: 8404,
	}
	if err := jobs.Create(context.Background(), draft); err != nil {
		t.Fatal(err)
	}

	body := `{"source_kind":"plesk","source_host":"plesk01",` +
		`"accounts":["shop.example.com"],"source_job_id":"DRAFT01","source_port":22}`
	w := postJSON(h.bulkCreate, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("bulkCreate status=%d body=%s", w.Code, w.Body.String())
	}

	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	for id, j := range jobs.byID {
		if id == "DRAFT01" {
			continue
		}
		if j.SourcePort != 8404 {
			t.Errorf("child has SourcePort=%d, want the draft's 8404 to win over "+
				"the body's 22 — the wizard sends no port here, so a defaulted "+
				"body value must not undo discovery", j.SourcePort)
		}
	}
}
