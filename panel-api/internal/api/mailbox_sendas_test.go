package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/auth"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// --- fakes ---

type saMailboxRepo struct {
	repository.MailboxRepository
	byID    map[string]*models.Mailbox
	byEmail map[string]*models.Mailbox
}

func (r *saMailboxRepo) FindByID(_ context.Context, id string) (*models.Mailbox, error) {
	if m, ok := r.byID[id]; ok {
		return m, nil
	}
	return nil, repository.ErrNotFound
}
func (r *saMailboxRepo) FindByEmail(_ context.Context, e string) (*models.Mailbox, error) {
	if m, ok := r.byEmail[e]; ok {
		return m, nil
	}
	return nil, repository.ErrNotFound
}

type saDomainRepo struct {
	repository.DomainRepository
	byID map[string]*models.Domain
}

func (r *saDomainRepo) FindByID(_ context.Context, id string) (*models.Domain, error) {
	if d, ok := r.byID[id]; ok {
		return d, nil
	}
	return nil, repository.ErrNotFound
}

type saDelegRepo struct {
	repository.MailboxSendDelegationRepository
	rows   []*models.MailboxSendDelegation
	exists bool
}

func (r *saDelegRepo) Create(_ context.Context, d *models.MailboxSendDelegation) error {
	r.rows = append(r.rows, d)
	return nil
}
func (r *saDelegRepo) ExistsPair(_ context.Context, _, _ string) (bool, error) { return r.exists, nil }
func (r *saDelegRepo) ListByDelegate(_ context.Context, _ string) ([]models.MailboxSendDelegation, error) {
	return nil, nil
}
func (r *saDelegRepo) ListAllPairs(_ context.Context) ([]models.SendAsPair, error) { return nil, nil }

type saAgent struct{ calls []string }

func (a *saAgent) Call(_ context.Context, cmd string, _ any) (json.RawMessage, error) {
	a.calls = append(a.calls, cmd)
	return json.RawMessage(`{}`), nil
}

func saRouter(cfg MailboxSendAsHandlerConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ginctx.SetClaims(c, &auth.AccessClaims{UserID: "u1", IsAdmin: true})
		c.Next()
	})
	RegisterMailboxSendAsRoutes(r.Group(""), cfg)
	return r
}

func saFixture() (MailboxSendAsHandlerConfig, *saAgent) {
	dom := &models.Domain{ID: "dom1", Name: "x.test", UserID: "u1"}
	otherDom := &models.Domain{ID: "dom2", Name: "y.test", UserID: "u1"}
	support := &models.Mailbox{ID: "mbSupport", DomainID: "dom1", LocalPart: "support", EmailCached: "support@x.test"}
	sales := &models.Mailbox{ID: "mbSales", DomainID: "dom1", LocalPart: "sales", EmailCached: "sales@x.test"}
	extern := &models.Mailbox{ID: "mbExtern", DomainID: "dom2", LocalPart: "z", EmailCached: "z@y.test"}
	agent := &saAgent{}
	cfg := MailboxSendAsHandlerConfig{
		Mailboxes: &saMailboxRepo{
			byID:    map[string]*models.Mailbox{"mbSupport": support, "mbSales": sales, "mbExtern": extern},
			byEmail: map[string]*models.Mailbox{"support@x.test": support, "sales@x.test": sales, "z@y.test": extern},
		},
		Domains:         &saDomainRepo{byID: map[string]*models.Domain{"dom1": dom, "dom2": otherDom}},
		SendDelegations: &saDelegRepo{},
		Agent:           agent,
	}
	return cfg, agent
}

func saPost(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/mailboxes/mbSupport/send-as", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSendAsAdd_Success(t *testing.T) {
	cfg, agent := saFixture()
	r := saRouter(cfg)
	w := saPost(r, `{"grantor_mailbox_id":"mbSales"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	deleg := cfg.SendDelegations.(*saDelegRepo)
	if len(deleg.rows) != 1 || deleg.rows[0].GrantorMailboxID != "mbSales" {
		t.Fatalf("delegation row not created: %+v", deleg.rows)
	}
	if len(agent.calls) != 1 || agent.calls[0] != "mail.sendas.reconcile" {
		t.Fatalf("reconcile not called: %v", agent.calls)
	}
}

func TestSendAsAdd_SelfRejected(t *testing.T) {
	cfg, agent := saFixture()
	r := saRouter(cfg)
	w := saPost(r, `{"grantor_mailbox_id":"mbSupport"}`) // delegate == grantor
	if w.Code != http.StatusBadRequest {
		t.Fatalf("self-delegation status %d, want 400", w.Code)
	}
	if len(agent.calls) != 0 {
		t.Fatalf("reconcile must NOT run on rejected add")
	}
}

func TestSendAsAdd_CrossDomainRejected(t *testing.T) {
	cfg, _ := saFixture()
	r := saRouter(cfg)
	w := saPost(r, `{"grantor_mailbox_id":"mbExtern"}`) // grantor in dom2
	if w.Code != http.StatusBadRequest {
		t.Fatalf("cross-domain status %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestSendAsAdd_DuplicateRejected(t *testing.T) {
	cfg, _ := saFixture()
	cfg.SendDelegations.(*saDelegRepo).exists = true
	r := saRouter(cfg)
	w := saPost(r, `{"grantor_mailbox_id":"mbSales"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate status %d, want 409", w.Code)
	}
}
