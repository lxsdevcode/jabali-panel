package commands

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/crypto/bcrypt"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// mail_webadmin.go — GH #243 (ADR-0142). Opt-in nginx reverse-proxy exposing
// Stalwart's WebAdmin UI. Stalwart's admin listener stays pinned to
// 127.0.0.1:8446 (M25 invariant); this vhost is the ONLY door, behind:
//   1. TLS,
//   2. nginx basic-auth (a DEDICATED gateway credential, never the Stalwart
//      admin token),
//   3. an optional source-IP allowlist,
//   4. and Stalwart's own admin login underneath.
//
//	mail.webadmin.apply  {enabled, server_name, ssl_cert_path, ssl_key_path,
//	                      allow_cidrs[]}  -> render/remove vhost; on first
//	                      enable, generate + return the gateway credential once.

const (
	stalwartAdminVhostName = "jabali-stalwart-admin.conf"
	stalwartAdminUser      = "stalwart-admin"
)

// stalwartAdminHtpasswd is the nginx basic-auth gateway credential file.
// A var (not const) so tests can redirect it off /etc/nginx.
var stalwartAdminHtpasswd = "/etc/nginx/.jabali-stalwart-admin.htpasswd"

var stalwartAdminVhostTemplate = `# Rendered by panel-agent mail.webadmin.apply (GH #243, ADR-0142).
# Stalwart WebAdmin reverse-proxy. Stalwart stays on 127.0.0.1:8446; this is
# the only externally reachable door, behind TLS + basic-auth{{if .AllowCIDRs}} + IP allowlist{{end}}.
server {
    listen 80;
    listen [::]:80;
    server_name {{.ServerName}};
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    server_name {{.ServerName}};

    ssl_certificate {{.SSLCertPath}};
    ssl_certificate_key {{.SSLKeyPath}};
{{range .AllowCIDRs}}    allow {{.}};
{{end}}{{if .AllowCIDRs}}    deny all;
{{end}}
    auth_basic "Stalwart WebAdmin";
    auth_basic_user_file {{.HtpasswdPath}};

    client_max_body_size 25m;

    location / {
        proxy_pass http://127.0.0.1:8446;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 300s;
    }
}
`

type webadminApplyParams struct {
	Enabled     bool     `json:"enabled"`
	ServerName  string   `json:"server_name"`
	SSLCertPath string   `json:"ssl_cert_path"`
	SSLKeyPath  string   `json:"ssl_key_path"`
	AllowCIDRs  []string `json:"allow_cidrs,omitempty"`
}

type webadminVhostTemplateData struct {
	ServerName   string
	SSLCertPath  string
	SSLKeyPath   string
	AllowCIDRs   []string
	HtpasswdPath string
}

type webadminApplyResponse struct {
	Ok      bool `json:"ok"`
	Changed bool `json:"changed"`
	// GatewayUser/GatewayPassword are returned ONLY when a fresh credential
	// was just generated (first enable). Empty otherwise — the panel shows it
	// once and never stores the plaintext.
	GatewayUser     string `json:"gateway_user,omitempty"`
	GatewayPassword string `json:"gateway_password,omitempty"`
}

func mailWebadminApplyHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p webadminApplyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}

	confPath := filepath.Join(mailVhostSitesAvailable, stalwartAdminVhostName)
	enabledPath := filepath.Join(mailVhostSitesEnabled, stalwartAdminVhostName)

	if !p.Enabled {
		changed := false
		for _, f := range []string{enabledPath, confPath} {
			if _, err := os.Lstat(f); err == nil {
				if err := os.Remove(f); err != nil {
					return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("rm %s: %v", f, err)}
				}
				changed = true
			}
		}
		// Leave the htpasswd in place (cheap, lets a re-enable reuse the
		// existing credential rather than churn it). Reload only on change.
		if changed {
			if err := nginxTestAndReload(ctx); err != nil {
				return nil, err
			}
		}
		return webadminApplyResponse{Ok: true, Changed: changed}, nil
	}

	// enabled=true
	if err := validateDomainNameForShell(p.ServerName); err != nil {
		return nil, err
	}
	if p.SSLCertPath == "" || p.SSLKeyPath == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "ssl_cert_path and ssl_key_path required"}
	}
	for _, c := range p.AllowCIDRs {
		if !validCIDROrIP(c) {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("invalid allow_cidr %q", c)}
		}
	}

	// Generate the gateway credential on first enable (no htpasswd yet).
	var freshUser, freshPass string
	if _, err := os.Stat(stalwartAdminHtpasswd); os.IsNotExist(err) {
		pass, gerr := generateGatewayPassword()
		if gerr != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: gerr.Error()}
		}
		hash, herr := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if herr != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: herr.Error()}
		}
		line := fmt.Sprintf("%s:%s\n", stalwartAdminUser, string(hash))
		if werr := os.WriteFile(stalwartAdminHtpasswd, []byte(line), 0o640); werr != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("write htpasswd: %v", werr)}
		}
		freshUser, freshPass = stalwartAdminUser, pass
	}

	data := webadminVhostTemplateData{
		ServerName:   p.ServerName,
		SSLCertPath:  p.SSLCertPath,
		SSLKeyPath:   p.SSLKeyPath,
		AllowCIDRs:   p.AllowCIDRs,
		HtpasswdPath: stalwartAdminHtpasswd,
	}
	tmpl, err := template.New("swadmin").Parse(stalwartAdminVhostTemplate)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("template: %v", err)}
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("render: %v", err)}
	}

	changed := false
	if cur, _ := os.ReadFile(confPath); !bytes.Equal(cur, rendered.Bytes()) {
		if err := os.WriteFile(confPath, rendered.Bytes(), 0o644); err != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("write vhost: %v", err)}
		}
		changed = true
	}
	if _, err := os.Lstat(enabledPath); os.IsNotExist(err) {
		if err := os.Symlink(confPath, enabledPath); err != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("symlink: %v", err)}
		}
		changed = true
	}
	if changed || freshPass != "" {
		if err := nginxTestAndReload(ctx); err != nil {
			return nil, err
		}
	}
	return webadminApplyResponse{Ok: true, Changed: changed, GatewayUser: freshUser, GatewayPassword: freshPass}, nil
}

// generateGatewayPassword returns a 30-char URL-safe random password.
func generateGatewayPassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// validCIDROrIP accepts a bare IP or a CIDR (the two shapes nginx allow/deny
// take). Rejects anything else so a malformed value can't break nginx -t.
func validCIDROrIP(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

func init() {
	Default.Register("mail.webadmin.apply", mailWebadminApplyHandler)
}
