package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/auth"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// udDockerRepo: only ListByUserID is real.
type udDockerRepo struct {
	repository.DockerAppRepository
	apps []*models.DockerApp
}

func (m *udDockerRepo) ListByUserID(context.Context, string) ([]*models.DockerApp, error) {
	return m.apps, nil
}

// captureAgent records docker_app.delete slugs. Thread-safe (the cascade may
// fire from background goroutines).
type captureAgent struct {
	mu   sync.Mutex
	seen []string
}

func (a *captureAgent) Call(_ context.Context, cmd string, params any) (json.RawMessage, error) {
	if cmd == "docker_app.delete" {
		if m, ok := params.(map[string]any); ok {
			if s, ok := m["slug"].(string); ok {
				a.mu.Lock()
				a.seen = append(a.seen, s)
				a.mu.Unlock()
			}
		}
	}
	return json.RawMessage(`{"ok":true}`), nil
}

func (a *captureAgent) calledDelete(slug string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, s := range a.seen {
		if s == slug {
			return true
		}
	}
	return false
}

func (a *captureAgent) slugs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.seen...)
}

// TestUserDelete_TearsDownDockerApps guards the M49 (GH #170) cascade: deleting
// a tenant must dispatch docker_app.delete for every owned install BEFORE the
// FK drops the rows, else the containers + data trees orphan on the host.
func TestUserDelete_TearsDownDockerApps(t *testing.T) {
	repo := newMemUserRepo()
	admin := makeUser(t, "admin@example.com", true, "adminpassword")
	repo.seed(admin)
	victim := makeUser(t, "victim@example.com", false, "victimpassword")
	uname := "victim"
	victim.Username = &uname
	repo.seed(victim)

	docker := &udDockerRepo{apps: []*models.DockerApp{
		{ID: "01APP00000000000000000001", UserID: &victim.ID, Slug: "memos", InstanceSlug: "memos-victim-notes"},
	}}
	ag := &captureAgent{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	claims := &auth.AccessClaims{UserID: admin.ID, IsAdmin: true}
	g := r.Group("/api/v1", func(c *gin.Context) {
		ginctx.SetClaims(c, claims)
		c.Next()
	})
	api.RegisterUserRoutes(g, api.UserHandlerConfig{
		Repo:       repo,
		Agent:      ag,
		DockerApps: docker,
	})

	rec := doJSON(t, r, http.MethodDelete, "/api/v1/users/"+victim.ID, nil)
	require.Equal(t, http.StatusNoContent, rec.Code)

	if !ag.calledDelete("memos-victim-notes") {
		t.Errorf("expected docker_app.delete for memos-victim-notes; got slugs=%v", ag.slugs())
	}
}
