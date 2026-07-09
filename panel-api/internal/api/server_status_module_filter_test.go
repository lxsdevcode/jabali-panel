package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

type fakeStatusSettingsRepo struct {
	repository.ServerSettingsRepository
	s *models.ServerSettings
}

func (f *fakeStatusSettingsRepo) Get(_ context.Context) (*models.ServerSettings, error) {
	return f.s, nil
}

// The Services card must hide a disabled module's services (mail →
// jabali-stalwart/jabali-webmail, dns → pdns) while always showing core units.
func TestFilterModuleServices(t *testing.T) {
	raw := json.RawMessage(`{"services":[
		{"unit":"nginx.service","active":"active"},
		{"unit":"jabali-stalwart.service","active":"inactive"},
		{"unit":"jabali-webmail.service","active":"inactive"},
		{"unit":"pdns.service","active":"inactive"},
		{"unit":"jabali-kratos.service","active":"active"}
	]}`)

	h := &adminServerStatusHandler{cfg: AdminServerStatusHandlerConfig{
		ServerSettings: &fakeStatusSettingsRepo{s: &models.ServerSettings{MailEnabled: false, DNSEnabled: true}},
	}}
	out := string(h.filterModuleServices(context.Background(), raw))

	for _, hidden := range []string{"jabali-stalwart.service", "jabali-webmail.service"} {
		if strings.Contains(out, hidden) {
			t.Errorf("mail off: %s should be hidden, got %s", hidden, out)
		}
	}
	for _, shown := range []string{"nginx.service", "pdns.service", "jabali-kratos.service"} {
		if !strings.Contains(out, shown) {
			t.Errorf("%s should be shown (core or dns-on), got %s", shown, out)
		}
	}

	// Nil settings repo → no filtering (relay all).
	h2 := &adminServerStatusHandler{cfg: AdminServerStatusHandlerConfig{}}
	if got := string(h2.filterModuleServices(context.Background(), raw)); !strings.Contains(got, "jabali-stalwart.service") {
		t.Errorf("nil settings should relay unchanged, got %s", got)
	}
}
