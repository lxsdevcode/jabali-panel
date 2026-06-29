package dockerapp

import (
	"strings"
	"testing"
)

// TestRender_SMTPMapping_AllApps renders every smtp:true app with operator
// SMTP_* set and asserts the app-specific SMTP env mapping appears (GH #322).
// MaterialiseEnv supplies each app's generated secrets so the render is valid;
// we add the SMTP overrides on top. A wrong/regressed mapping fails loudly here
// instead of at a tenant's silently-broken email.
func TestRender_SMTPMapping_AllApps(t *testing.T) {
	cat, _ := LoadDir(repoCatalogDir(t))
	smtp := map[string]string{
		"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "587", "SMTP_USER": "relayuser",
		"SMTP_PASSWORD": "relaypw", "SMTP_FROM": "noreply@example.com",
		"SMTP_FROM_NAME": "Acme", "SMTP_TLS": "true",
	}
	// app -> substrings that MUST appear when SMTP is supplied.
	cases := map[string][]string{
		"gitea":         {`GITEA__mailer__SMTP_ADDR: "smtp.example.com"`, `GITEA__mailer__USER: "relayuser"`},
		"forgejo":       {`FORGEJO__mailer__SMTP_ADDR: "smtp.example.com"`, `FORGEJO__mailer__PASSWD: "relaypw"`},
		"grafana":       {`GF_SMTP_HOST: "smtp.example.com:587"`, `GF_SMTP_FROM_ADDRESS: "noreply@example.com"`},
		"vaultwarden":   {`SMTP_SECURITY: "starttls"`, `SMTP_USERNAME: "relayuser"`},
		"ghost":         {`mail__options__host: "smtp.example.com"`, `mail__options__auth__user: "relayuser"`},
		"n8n":           {`N8N_EMAIL_MODE: "smtp"`, `N8N_SMTP_HOST: "smtp.example.com"`},
		"mealie":        {`SMTP_FROM_EMAIL: "noreply@example.com"`, `SMTP_AUTH_STRATEGY: "TLS"`},
		"joplin":        {`MAILER_ENABLED: "true"`, `MAILER_HOST: "smtp.example.com"`},
		"snipe-it":      {`MAIL_HOST: "smtp.example.com"`, `MAIL_FROM_ADDR: "noreply@example.com"`},
		"plausible":     {`SMTP_HOST_ADDR: "smtp.example.com"`, `MAILER_EMAIL: "noreply@example.com"`},
		"peertube":      {`PEERTUBE_SMTP_HOSTNAME: "smtp.example.com"`, `PEERTUBE_SMTP_FROM: "noreply@example.com"`},
		"linkwarden":    {`EMAIL_SERVER: "smtp://relayuser:relaypw@smtp.example.com:587"`},
		"rocketchat":    {`OVERWRITE_SETTING_SMTP_Host: "smtp.example.com"`, `OVERWRITE_SETTING_From_Email: "noreply@example.com"`},
		"nextcloud":     {`SMTP_HOST: "smtp.example.com"`, `SMTP_SECURE: "tls"`},
		"paperless-ngx": {`PAPERLESS_EMAIL_HOST: "smtp.example.com"`},
		"bigcapital":    {`MAIL_HOST: "smtp.example.com"`, `MAIL_FROM_ADDRESS: "noreply@example.com"`},
		"zitadel":       {`ZITADEL_DEFAULTINSTANCE_SMTPCONFIGURATION_SMTP_HOST: "smtp.example.com:587"`},
	}
	for app, wants := range cases {
		e, ok := cat.Get(app)
		if !ok {
			t.Errorf("%s missing from catalog", app)
			continue
		}
		env, err := MaterialiseEnv(e, smtp)
		if err != nil {
			t.Errorf("%s MaterialiseEnv: %v", app, err)
			continue
		}
		out, err := Render(e, RenderParams{
			Slug: app, Name: "t", Domain: "t.example.com", ImageChannel: "img",
			DataRoot: "/d", CPULimit: "1.0", MemoryLimit: "1g", Env: env,
		})
		if err != nil {
			t.Errorf("%s Render: %v", app, err)
			continue
		}
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s: unresolved template var:\n%s", app, out)
		}
		for _, w := range wants {
			if !strings.Contains(out, w) {
				t.Errorf("%s: missing %q", app, w)
			}
		}
	}
}
