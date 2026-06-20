package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// domainDisclaimerApplyParams is the panel → agent request (one domain at a
// time; the reconciler/API call it per domain).
//
// GH #233 / ADR-0143: the disclaimer is now applied by the jabali-mailhook
// MTA-hook service, NOT by a Sieve script. Sieve could not append to HTML
// without corrupting it (`replace` downgrades the part to text/plain;
// `extracttext` de-tags HTML). The hook rewrites the MIME body in Go,
// preserving markup, encodings, boundaries, and attachments.
//
// The hook reads the per-domain disclaimer from the panel DB live, so this
// handler no longer renders or stores anything per domain — it only ensures
// the global MtaHook is registered with Stalwart and tears down any leftover
// Sieve disclaimer from the pre-ADR-0143 implementation. The panel API remains
// the single writer of domains.disclaimer_text / disclaimer_enabled.
type domainDisclaimerApplyParams struct {
	DomainName string `json:"domain_name"`
	Enabled    bool   `json:"enabled"`
	Text       string `json:"text"`
}

type domainDisclaimerApplyResponse struct {
	Ok bool `json:"ok"`
}

const (
	// mailhookURL is the loopback MTA-hook endpoint Stalwart calls at the
	// DATA stage (jabali-mailhook service, ADR-0143).
	mailhookURL = "http://127.0.0.1:8462/"
	// mailhookTokenPath holds the Bearer token shared with the hook service.
	mailhookTokenPath = "/etc/jabali-panel/mailhook.token"
	// legacyDisclaimerScriptName is the pre-ADR-0143 global Sieve script that
	// must be removed on upgrade.
	legacyDisclaimerScriptName = "jabali-disclaimer"
	legacyDisclaimerExpr       = "'jabali-disclaimer'"
)

// disclaimerDomainRe is a strict DNS-name guard kept as input validation at
// this privileged trust boundary.
var disclaimerDomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?)+$`)

func domainDisclaimerApplyHandler(ctx context.Context, params json.RawMessage) (any, error) {
	if len(params) == 0 {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "params required"}
	}
	var p domainDisclaimerApplyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if p.DomainName == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "domain_name required"}
	}
	if !disclaimerDomainRe.MatchString(p.DomainName) {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "invalid domain_name"}
	}

	changed := false

	// Tear down any leftover Sieve disclaimer from the pre-ADR-0143 design.
	removed, err := removeLegacyDisclaimerSieve(ctx)
	if err != nil {
		return nil, err
	}
	changed = changed || removed

	// Ensure the global MTA hook is registered. The hook (not this handler)
	// reads the per-domain disclaimer text from the DB, so registration is
	// global and idempotent — no per-domain state here.
	created, err := ensureMtaHook(ctx)
	if err != nil {
		return nil, err
	}
	changed = changed || created

	// Reload only when something changed — the reconciler calls this every
	// tick for every domain, and an unconditional reload would be a
	// settings-reload storm (per-tick-idempotent-loops rule).
	if changed {
		if err := reloadStalwartSettings(ctx); err != nil {
			return nil, err
		}
	}
	return domainDisclaimerApplyResponse{Ok: true}, nil
}

// ensureMtaHook registers the jabali-mailhook MTA hook with Stalwart if a hook
// with our URL isn't already present. Returns whether a hook was created.
func ensureMtaHook(ctx context.Context) (bool, error) {
	present, err := mtaHookExists(ctx)
	if err != nil {
		return false, err
	}
	if present {
		return false, nil
	}

	token, err := readMailhookToken()
	if err != nil {
		return false, err
	}

	hook := map[string]any{
		"@type":  "MtaHook",
		"url":    mailhookURL,
		"stages": map[string]any{"data": true},
		"enable": map[string]any{"@type": "Expression", "else": "true"},
		"httpAuth": map[string]any{
			"@type":       "Bearer",
			"bearerToken": map[string]any{"@type": "Value", "secret": token},
		},
		// Fail open: never bounce mail because the disclaimer hook errors.
		"tempFailOnError":   false,
		"timeout":           30000,
		"maxResponseSize":   52428800,
		"allowInvalidCerts": false,
	}
	args := map[string]any{"create": map[string]any{"jabali-mailhook": hook}}
	var res jmapSetResult
	if err := jmapCall(ctx, "x:MtaHook/set", args, &res); err != nil {
		return false, fmt.Errorf("mta hook create: %w", err)
	}
	if reason, bad := res.NotCreated["jabali-mailhook"]; bad {
		return false, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("mta hook create refused: %s", string(reason))}
	}
	return true, nil
}

// mtaHookExists reports whether an MtaHook with our URL is already registered.
func mtaHookExists(ctx context.Context) (bool, error) {
	var qr jmapQueryResult
	if err := jmapCall(ctx, "x:MtaHook/query", map[string]any{}, &qr); err != nil {
		return false, fmt.Errorf("mta hook query: %w", err)
	}
	if len(qr.IDs) == 0 {
		return false, nil
	}
	var gr jmapGetResult
	if err := jmapCall(ctx, "x:MtaHook/get", map[string]any{"ids": qr.IDs}, &gr); err != nil {
		return false, fmt.Errorf("mta hook get: %w", err)
	}
	for _, raw := range gr.List {
		var h struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(raw, &h); err != nil {
			continue
		}
		if h.URL == mailhookURL {
			return true, nil
		}
	}
	return false, nil
}

// removeLegacyDisclaimerSieve destroys the pre-ADR-0143 global Sieve script and
// unbinds it from the DATA stage. Idempotent; returns whether anything changed.
func removeLegacyDisclaimerSieve(ctx context.Context) (bool, error) {
	changed := false

	id, _, err := getSieveSystemScript(ctx, legacyDisclaimerScriptName)
	if err != nil {
		return false, err
	}
	if id != "" {
		var res jmapSetResult
		if err := jmapCall(ctx, "x:SieveSystemScript/set", map[string]any{"destroy": []string{id}}, &res); err != nil {
			return false, fmt.Errorf("legacy disclaimer sieve destroy: %w", err)
		}
		changed = true
	}

	// Unbind the DATA-stage script if it still points at the legacy script.
	var gr jmapGetResult
	if err := jmapCall(ctx, "x:MtaStageData/get", map[string]any{"ids": []string{"singleton"}}, &gr); err != nil {
		return changed, fmt.Errorf("disclaimer binding get: %w", err)
	}
	if len(gr.List) > 0 {
		var cur struct {
			Script struct {
				Else string `json:"else"`
			} `json:"script"`
		}
		if err := json.Unmarshal(gr.List[0], &cur); err == nil && cur.Script.Else == legacyDisclaimerExpr {
			args := map[string]any{"update": map[string]any{"singleton": map[string]any{
				"script": map[string]any{"match": map[string]any{}, "else": "false"},
			}}}
			var sr jmapSetResult
			if err := jmapCall(ctx, "x:MtaStageData/set", args, &sr); err != nil {
				return changed, fmt.Errorf("disclaimer unbind: %w", err)
			}
			changed = true
		}
	}
	return changed, nil
}

// getSieveSystemScript returns the id + contents of the named system sieve
// script, or ("", "", nil) if it doesn't exist.
func getSieveSystemScript(ctx context.Context, name string) (string, string, error) {
	var qr jmapQueryResult
	if err := jmapCall(ctx, "x:SieveSystemScript/query", map[string]any{"filter": map[string]any{"name": name}}, &qr); err != nil {
		return "", "", err
	}
	if len(qr.IDs) == 0 {
		return "", "", nil
	}
	id := qr.IDs[0]
	var gr jmapGetResult
	if err := jmapCall(ctx, "x:SieveSystemScript/get", map[string]any{"ids": []string{id}}, &gr); err != nil {
		return "", "", err
	}
	for _, raw := range gr.List {
		var s struct {
			ID       string `json:"id"`
			Contents string `json:"contents"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		if s.ID == id {
			return id, s.Contents, nil
		}
	}
	return id, "", nil
}

// reloadStalwartSettings asks Stalwart to reload its settings so a config
// write takes effect without a service restart.
func reloadStalwartSettings(ctx context.Context) error {
	args := map[string]any{"create": map[string]any{"reload": map[string]any{"@type": "ReloadSettings"}}}
	var sr jmapSetResult
	if err := jmapCall(ctx, "x:Action/set", args, &sr); err != nil {
		return fmt.Errorf("reload settings: %w", err)
	}
	return nil
}

func readMailhookToken() (string, error) {
	path := os.Getenv("MAILHOOK_TOKEN_PATH")
	if path == "" {
		path = mailhookTokenPath
	}
	b, err := os.ReadFile(path) //nolint:gosec // operator-owned secret path
	if err != nil {
		return "", fmt.Errorf("read mailhook token %s: %w", path, err)
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("mailhook token %s is empty", path)
	}
	return tok, nil
}

func init() {
	Default.Register("domain.disclaimer_apply", domainDisclaimerApplyHandler)
}
