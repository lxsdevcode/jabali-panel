// render.go — render a catalog Entry's compose.yml.tmpl into an
// installable docker-compose YAML. Owned by panel-api; the agent
// receives the already-rendered text + the supporting files
// (volumes + ports + env) and never touches the catalog itself.
package dockerapp

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"text/template"
)

// RenderParams holds everything the compose template needs.
// Field names match the template-variable contract documented in
// install/docker-apps/README.md.
type RenderParams struct {
	Slug         string
	Name         string
	Domain       string
	ImageChannel string
	DataRoot     string
	CPULimit     string
	MemoryLimit  string
	PIDsLimit    int
	// Ports maps the catalog port name (e.g. "http", "ssh") to the
	// runtime binding chosen at install time. Disabled ports are
	// omitted from the map; templates can safely range over the map.
	Ports map[string]RuntimePort
	// Env is the materialised env-var set: catalog defaults + any
	// auto-generated secrets (e.g. ADMIN_TOKEN for vaultwarden).
	Env map[string]string
}

// RuntimePort is the resolved binding for one published port.
// BindInterface is either "127.0.0.1" (loopback) or the operator-
// chosen managed-IP address (public bind).
type RuntimePort struct {
	HostPort      int
	ContainerPort int
	BindInterface string
	Protocol      string
}

// Render returns the docker-compose YAML for the given Entry and
// runtime parameters. Errors surface template parse failures, which
// indicate a malformed catalog entry — the agent NEVER sees an
// unrenderable template because catalog loading runs validate()
// before exposing the entry.
func Render(entry Entry, params RenderParams) (string, error) {
	tmpl, err := template.New(entry.Slug).Parse(entry.ComposeTemplate())
	if err != nil {
		return "", fmt.Errorf("parse compose template for %q: %w", entry.Slug, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute compose template for %q: %w", entry.Slug, err)
	}
	return buf.String(), nil
}

// MaterialiseEnv resolves the catalog's env declarations into a
// concrete name->value map. Catalog defaults are used as-is;
// generate:'password32'/'password64' entries get a fresh random
// secret per install.
func MaterialiseEnv(entry Entry, overrides map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(entry.Env))
	for _, e := range entry.Env {
		if v, ok := overrides[e.Name]; ok {
			out[e.Name] = v
			continue
		}
		if e.Value != "" {
			out[e.Name] = e.Value
			continue
		}
		if e.Generate != "" {
			secret, err := generateSecret(e.Generate)
			if err != nil {
				return nil, fmt.Errorf("generate %s: %w", e.Name, err)
			}
			out[e.Name] = secret
			continue
		}
		// Declared but no value, no generator, no override — leave
		// empty; the compose template can decide whether to omit.
		out[e.Name] = ""
	}
	return out, nil
}

func generateSecret(scheme string) (string, error) {
	var nbytes int
	switch scheme {
	case "password32":
		nbytes = 24 // base64-rounded to ~32 chars
	case "password64":
		nbytes = 48 // base64-rounded to ~64 chars
	default:
		return "", fmt.Errorf("unknown secret scheme %q", scheme)
	}
	buf := make([]byte, nbytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
