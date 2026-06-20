package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// domainDisclaimerApplyParams is the panel → agent request (one domain at a
// time; the reconciler/API call it per domain).
//
// GH #233: the disclaimer never worked. Two stacked bugs, both fixed here:
//
//  1. The SMTP DATA-stage script binding (`MtaStageData.script`) was left at
//     its default `false`, so Stalwart ran NO sieve at the DATA stage — the
//     per-domain SieveSystemScript objects existed but were never invoked.
//  2. The sieve matched Content-Type with `:is "text/plain"`, which never
//     fires against the real `text/plain; charset="utf-8"` header.
//
// The model is also reworked: instead of one SieveSystemScript per domain
// (selected by a data-stage expression — which fails because script-name
// lookup chokes on the dots in a domain name), we keep ONE global script
// named `jabali-disclaimer` (dot-free, so the binding resolves) that holds a
// marker-delimited section per domain. Each apply rebuilds the global script
// with that domain's section spliced in or out, binds MtaStageData.script to
// the constant `'jabali-disclaimer'`, and reloads settings. All verified
// end-to-end on the test server (Stalwart 0.16.6).
type domainDisclaimerApplyParams struct {
	DomainName string `json:"domain_name"`
	Enabled    bool   `json:"enabled"`
	Text       string `json:"text"`
}

type domainDisclaimerApplyResponse struct {
	Ok bool `json:"ok"`
}

const (
	// disclaimerScriptName is the single global system sieve script holding
	// every domain's disclaimer section. Dot-free so the data-stage binding
	// expression resolves it (Stalwart script-name lookup fails on dots).
	disclaimerScriptName = "jabali-disclaimer"
	// dataStageDisclaimerExpr binds the DATA stage to the global script.
	dataStageDisclaimerExpr = "'jabali-disclaimer'"
)

// disclaimerDomainRe is a strict DNS-name guard. The agent is a privileged
// trust boundary: even though the panel validates domains at create time, a
// malformed domain reaching this handler could break out of the sieve string
// literal (`"`, `\`) or pollute another domain's marker section (`\n`, `#`)
// — i.e. Sieve-script injection / cross-domain section pollution. Reject
// anything that isn't a plain lowercase FQDN before it touches the script.
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

	existingID, existingContents, err := getSieveSystemScript(ctx, disclaimerScriptName)
	if err != nil {
		return nil, err
	}

	// Parse the current global script into per-domain sections, drop this
	// domain's, then re-add it if enabled.
	sections := parseDisclaimerSections(existingContents)
	delete(sections, p.DomainName)
	if p.Enabled && strings.TrimSpace(p.Text) != "" {
		sections[p.DomainName] = renderDisclaimerSection(p.DomainName, p.Text)
	}

	changed := false

	if len(sections) == 0 {
		// No disclaimers left → destroy the global script. The binding can
		// stay (a missing script is a safe DATA-stage no-op).
		if existingID != "" {
			var result jmapSetResult
			if err := jmapCall(ctx, "x:SieveSystemScript/set", map[string]any{"destroy": []string{existingID}}, &result); err != nil {
				return nil, fmt.Errorf("disclaimer sieve destroy: %w", err)
			}
			changed = true
		}
	} else {
		body := buildDisclaimerScript(sections)
		if existingID == "" {
			var result jmapSetResult
			args := map[string]any{"create": map[string]any{disclaimerScriptName: map[string]any{
				"name": disclaimerScriptName, "isActive": true, "contents": body,
			}}}
			if err := jmapCall(ctx, "x:SieveSystemScript/set", args, &result); err != nil {
				return nil, err
			}
			if reason, bad := result.NotCreated[disclaimerScriptName]; bad {
				return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("disclaimer sieve create refused: %s", string(reason))}
			}
			changed = true
		} else if existingContents != body {
			// Update BY ID (name-keyed updates silently no-op).
			var result jmapSetResult
			args := map[string]any{"update": map[string]any{existingID: map[string]any{
				"contents": body, "isActive": true,
			}}}
			if err := jmapCall(ctx, "x:SieveSystemScript/set", args, &result); err != nil {
				return nil, fmt.Errorf("disclaimer sieve update: %w", err)
			}
			if reason, bad := result.NotUpdated[existingID]; bad {
				return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("disclaimer sieve update refused: %s", string(reason))}
			}
			changed = true
		}

		bound, err := ensureDataStageDisclaimerBinding(ctx)
		if err != nil {
			return nil, err
		}
		changed = changed || bound
	}

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

const (
	disclaimerSectBegin = "# jabali-disclaimer-begin "
	disclaimerSectEnd   = "# jabali-disclaimer-end "
)

// parseDisclaimerSections extracts the per-domain sections from an existing
// global disclaimer script, keyed by domain. The require header and anything
// outside the begin/end markers is discarded (regenerated on build).
func parseDisclaimerSections(contents string) map[string]string {
	out := map[string]string{}
	if contents == "" {
		return out
	}
	lines := strings.Split(contents, "\n")
	var curDomain string
	var cur []string
	for _, ln := range lines {
		switch {
		case strings.HasPrefix(ln, disclaimerSectBegin):
			curDomain = strings.TrimSpace(strings.TrimPrefix(ln, disclaimerSectBegin))
			cur = nil
		case strings.HasPrefix(ln, disclaimerSectEnd):
			if curDomain != "" {
				out[curDomain] = strings.Join(cur, "\n")
			}
			curDomain = ""
			cur = nil
		default:
			if curDomain != "" {
				cur = append(cur, ln)
			}
		}
	}
	return out
}

// buildDisclaimerScript assembles the global script from per-domain sections:
// one require header, then each domain's marker-wrapped section in sorted
// order (stable output → idempotent content compare).
func buildDisclaimerScript(sections map[string]string) string {
	domains := make([]string, 0, len(sections))
	for d := range sections {
		domains = append(domains, d)
	}
	// Insertion sort keeps output stable without importing sort just for this.
	for i := 1; i < len(domains); i++ {
		for j := i; j > 0 && domains[j-1] > domains[j]; j-- {
			domains[j-1], domains[j] = domains[j], domains[j-1]
		}
	}
	var b strings.Builder
	b.WriteString(`require ["envelope","variables","mime","foreverypart","extracttext","replace"];` + "\n")
	for _, d := range domains {
		b.WriteString(disclaimerSectBegin + d + "\n")
		b.WriteString(sections[d])
		if !strings.HasSuffix(sections[d], "\n") {
			b.WriteString("\n")
		}
		b.WriteString(disclaimerSectEnd + d + "\n")
	}
	return b.String()
}

// renderDisclaimerSection builds the per-domain body (no require line — that
// lives once at the top of the global script). Appends the disclaimer to
// text/plain and text/html parts.
//
// GH #233: the Content-Type match uses `:contains`, NOT `:is` — the real
// header is `text/plain; charset="utf-8"`, so an exact `:is "text/plain"`
// never fires.
func renderDisclaimerSection(domain, text string) string {
	plainText := sieveEscape(text)
	htmlText := sieveEscape(htmlEscape(text))
	var b strings.Builder
	fmt.Fprintf(&b, "if envelope :domain \"from\" \"%s\" {\n", sieveEscape(domain))
	b.WriteString("  foreverypart {\n")
	b.WriteString("    if header :mime :contenttype :contains \"Content-Type\" \"text/plain\" {\n")
	b.WriteString("      extracttext \"jabali_orig\";\n")
	fmt.Fprintf(&b, "      replace \"${jabali_orig}\\n\\n-- \\n%s\\n\";\n", plainText)
	b.WriteString("    } elsif header :mime :contenttype :contains \"Content-Type\" \"text/html\" {\n")
	b.WriteString("      extracttext \"jabali_orig\";\n")
	fmt.Fprintf(&b, "      replace \"${jabali_orig}<hr><p>%s</p>\";\n", htmlText)
	b.WriteString("    }\n")
	b.WriteString("  }\n")
	b.WriteString("}")
	return b.String()
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

// ensureDataStageDisclaimerBinding sets MtaStageData.script to the global
// disclaimer script if it isn't already. Returns whether a write happened.
// Patches ONLY the `script` field.
func ensureDataStageDisclaimerBinding(ctx context.Context) (bool, error) {
	var gr jmapGetResult
	if err := jmapCall(ctx, "x:MtaStageData/get", map[string]any{"ids": []string{"singleton"}}, &gr); err != nil {
		return false, fmt.Errorf("disclaimer binding get: %w", err)
	}
	if len(gr.List) > 0 {
		var cur struct {
			Script struct {
				Else string `json:"else"`
			} `json:"script"`
		}
		if err := json.Unmarshal(gr.List[0], &cur); err == nil && cur.Script.Else == dataStageDisclaimerExpr {
			return false, nil
		}
	}
	args := map[string]any{"update": map[string]any{"singleton": map[string]any{
		"script": map[string]any{"match": map[string]any{}, "else": dataStageDisclaimerExpr},
	}}}
	var sr jmapSetResult
	if err := jmapCall(ctx, "x:MtaStageData/set", args, &sr); err != nil {
		return false, fmt.Errorf("disclaimer binding set: %w", err)
	}
	if reason, bad := sr.NotUpdated["singleton"]; bad {
		return false, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("disclaimer binding refused: %s", string(reason))}
	}
	return true, nil
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

// sieveEscape escapes a string for inclusion inside a sieve double-quoted
// string literal: backslash first, then double-quote, then newline (as the
// literal \n escape). Sieve strings do not process \t or other escapes.
func sieveEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// htmlEscape escapes HTML-special characters so operator-typed text can't
// inject markup into the HTML-part disclaimer.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `<`, `&lt;`)
	s = strings.ReplaceAll(s, `>`, `&gt;`)
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	return s
}

func init() {
	Default.Register("domain.disclaimer_apply", domainDisclaimerApplyHandler)
}
