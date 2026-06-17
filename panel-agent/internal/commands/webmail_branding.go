package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// webmail_branding.go — apply server-wide branding to Bulwark webmail
// (GH #200). Bulwark is a single instance serving every domain, so its
// branding (logo + app/company name) is global env config — there's no
// per-domain branding. We mirror the panel's operator branding (the logo
// the admin uploaded in Server Settings + the brand text) onto Bulwark:
// copy the logo into Bulwark's public/branding dir and set the
// APP_NAME / LOGIN_COMPANY_NAME / *_LOGO_*_URL keys in bulwark.env, then
// restart jabali-webmail. With no operator branding it falls back to
// jabali's own mail logo + "Jabali Mail" (the install ships the default
// jabali-mail-{light,dark}.svg).

const (
	panelBrandingDir      = "/var/lib/jabali-panel/branding"
	webmailPublicBranding = "/opt/jabali-webmail/public/branding"
	webmailEnvFile        = "/etc/jabali-panel/bulwark.env"
)

// brandingManagedKeys are the bulwark.env keys this command owns. They are
// stripped + rewritten on every apply so the result is deterministic.
var brandingManagedKeys = []string{
	"APP_NAME",
	"LOGIN_COMPANY_NAME",
	"LOGIN_LOGO_LIGHT_URL",
	"LOGIN_LOGO_DARK_URL",
	"APP_LOGO_LIGHT_URL",
	"APP_LOGO_DARK_URL",
}

type webmailBrandingApplyParams struct {
	BrandText string `json:"brand_text"`
}

func webmailBrandingApplyHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p webmailBrandingApplyParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
		}
	}
	// Bulwark not installed on this host → nothing to do (idempotent).
	if _, err := os.Stat(webmailPublicBranding); err != nil {
		return okBody{Ok: true}, nil
	}

	lightURL := brandingLogoURL(ctx, "light")
	darkURL := brandingLogoURL(ctx, "dark")

	appName := strings.TrimSpace(p.BrandText)
	if appName == "" {
		appName = "Jabali Mail"
	}
	company := strings.TrimSpace(p.BrandText)
	if company == "" {
		if h, err := os.Hostname(); err == nil {
			company = h
		} else {
			company = "Jabali Mail"
		}
	}

	changed, err := setWebmailEnvKeys(map[string]string{
		"APP_NAME":             appName,
		"LOGIN_COMPANY_NAME":   company,
		"LOGIN_LOGO_LIGHT_URL": lightURL,
		"LOGIN_LOGO_DARK_URL":  darkURL,
		"APP_LOGO_LIGHT_URL":   lightURL,
		"APP_LOGO_DARK_URL":    darkURL,
	})
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: err.Error()}
	}
	if changed {
		if out, sErr := runSystemctl(ctx, "restart", "jabali-webmail.service"); sErr != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("restart jabali-webmail: %v (%s)", sErr, bytesTrim(out))}
		}
	}
	return okBody{Ok: true}, nil
}

// brandingLogoURL returns the Bulwark /branding URL for a variant: the
// operator's uploaded panel logo (copied into Bulwark's public dir) when
// present, else jabali's default mail logo. Empty string only if neither
// exists (shouldn't happen — install ships the defaults).
func brandingLogoURL(ctx context.Context, variant string) string {
	// Operator logo: /var/lib/jabali-panel/branding/{variant}.{svg,png}
	for _, ext := range []string{".svg", ".png"} {
		src := filepath.Join(panelBrandingDir, variant+ext)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(webmailPublicBranding, "jabali-brand-"+variant+ext)
			if err := copyBrandingFile(src, dst); err == nil {
				// Remove a stale copy at the other extension.
				other := ".png"
				if ext == ".png" {
					other = ".svg"
				}
				_ = os.Remove(filepath.Join(webmailPublicBranding, "jabali-brand-"+variant+other))
				return "/branding/jabali-brand-" + variant + ext
			}
		}
	}
	// No operator logo → jabali default (shipped by install_bulwark). Drop
	// any stale operator copy so a cleared logo reverts cleanly.
	_ = os.Remove(filepath.Join(webmailPublicBranding, "jabali-brand-"+variant+".svg"))
	_ = os.Remove(filepath.Join(webmailPublicBranding, "jabali-brand-"+variant+".png"))
	def := filepath.Join(webmailPublicBranding, "jabali-mail-"+variant+".svg")
	if _, err := os.Stat(def); err == nil {
		return "/branding/jabali-mail-" + variant + ".svg"
	}
	return ""
}

func copyBrandingFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		return err
	}
	if u, err := user.Lookup("jabali-webmail"); err == nil {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		_ = os.Chown(dst, uid, gid)
	}
	return nil
}

// setWebmailEnvKeys rewrites the managed branding keys in bulwark.env:
// strips every existing occurrence, appends the new values. Returns
// whether the file content changed.
func setWebmailEnvKeys(kv map[string]string) (bool, error) {
	orig, err := os.ReadFile(webmailEnvFile)
	if err != nil {
		return false, fmt.Errorf("read bulwark.env: %w", err)
	}
	managed := map[string]bool{}
	for _, k := range brandingManagedKeys {
		managed[k] = true
	}
	var keep []string
	for _, line := range strings.Split(string(orig), "\n") {
		drop := false
		for k := range managed {
			if strings.HasPrefix(line, k+"=") {
				drop = true
				break
			}
		}
		if !drop {
			keep = append(keep, line)
		}
	}
	body := strings.TrimRight(strings.Join(keep, "\n"), "\n") + "\n"
	body += "\n# Branding (managed by webmail.branding.apply — GH #200)\n"
	for _, k := range brandingManagedKeys {
		if v := kv[k]; v != "" {
			body += k + "=" + v + "\n"
		}
	}
	if body == string(orig) {
		return false, nil
	}
	tmp := webmailEnvFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o640); err != nil {
		return false, fmt.Errorf("write bulwark.env: %w", err)
	}
	if u, err := user.Lookup("jabali-webmail"); err == nil {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		_ = os.Chown(tmp, uid, gid)
	}
	if err := os.Rename(tmp, webmailEnvFile); err != nil {
		return false, fmt.Errorf("rename bulwark.env: %w", err)
	}
	return true, nil
}

func init() {
	Default.Register("webmail.branding.apply", webmailBrandingApplyHandler)
}
