package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// forwarderApplyParams is the panel → agent request for forwarder.apply.
//
// Idempotent apply of a mailbox's forwarders. Caller sends the full desired
// state; agent writes x:UserAccount.aliases for type=alias entries and a
// single concatenated x:SieveUserScript per mailbox for type=external
// entries (Stalwart allows only one active sieve script per account —
// schema SieveScript.isActive).
type forwarderApplyParams struct {
	MailboxEmail string              `json:"mailbox_email"`
	Aliases      []forwarderAlias    `json:"aliases"`   // local parts within the mailbox's own domain
	Externals    []forwarderExternal `json:"externals"` // external forward targets
}

// forwarderExternal is one external forward target. KeepCopy controls
// whether a copy is also delivered to the mailbox (Sieve redirect :copy)
// — GH #237's "Keep a copy of forwarded email in this mailbox".
type forwarderExternal struct {
	Target   string `json:"target"`
	KeepCopy bool   `json:"keep_copy"`
}

type forwarderAlias struct {
	LocalPart string `json:"local_part"`
	// DomainID is the Stalwart x:Domain id the alias belongs to. Resolved
	// agent-side from the email's domain.
}

type forwarderApplyResponse struct {
	Ok bool `json:"ok"`
}

func forwarderApplyHandler(ctx context.Context, params json.RawMessage) (any, error) {
	if len(params) == 0 {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "params required"}
	}
	var p forwarderApplyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if _, err := requireEmail(p.MailboxEmail); err != nil {
		return nil, err
	}

	acctID, err := accountIDByEmail(ctx, p.MailboxEmail)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("resolve account: %v", err)}
	}
	if acctID == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeNotFound, Message: "mailbox not yet registered"}
	}

	// Resolve the account's domain id.
	at := strings.LastIndex(p.MailboxEmail, "@")
	domainName := p.MailboxEmail[at+1:]
	domainID, err := domainIDByName(ctx, domainName)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("resolve domain: %v", err)}
	}
	if domainID == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeNotFound, Message: "domain not in Stalwart registry"}
	}

	// 1. Replace account aliases.
	if err := applyAccountAliases(ctx, acctID, domainID, p.Aliases); err != nil {
		return nil, err
	}

	// 2. Replace the concatenated sieve script for external forwards.
	if err := applyExternalSieve(ctx, acctID, p.Externals); err != nil {
		return nil, err
	}

	return forwarderApplyResponse{Ok: true}, nil
}

func applyAccountAliases(ctx context.Context, acctID, domainID string, aliases []forwarderAlias) error {
	entries := make([]map[string]any, 0, len(aliases))
	for _, a := range aliases {
		if a.LocalPart == "" {
			continue
		}
		entries = append(entries, map[string]any{
			"name":     a.LocalPart,
			"domainId": domainID,
			"enabled":  true,
		})
	}
	args := map[string]any{
		"update": map[string]any{
			acctID: map[string]any{
				"aliases": entries,
			},
		},
	}
	// Best-effort: on Stalwart 0.16.7 the legacy `x:Account/User/set`
	// method is gone (returns unknownMethod), AND jabali serves aliases
	// from its SQL directory (queryEmailAliases on email_forwarders), not
	// the principal's `aliases` property — which stays empty by design.
	// The receiving alias and Stalwart's auto-created sending Identity both
	// come from the directory, so this call is unnecessary; we keep it
	// (now via the current `x:Account/set` method) but never fail
	// forwarder.apply on it. Before this, every alias made forwarder.apply
	// error on the dead method (GH #199 investigation).
	var result jmapSetResult
	_ = jmapCall(ctx, "x:Account/set", args, &result)
	return nil
}

// buildExternalSieve renders the concatenated jabali-fwds Sieve script for a
// mailbox's external forwards. GH #237: `redirect :copy` keeps a copy in the
// mailbox, plain `redirect` does not. The "copy" extension is required only
// when at least one target keeps a copy.
func buildExternalSieve(externals []forwarderExternal) string {
	hasCopy := false
	for _, e := range externals {
		if e.KeepCopy {
			hasCopy = true
			break
		}
	}
	var body strings.Builder
	if hasCopy {
		body.WriteString(`require ["copy"];` + "\n")
	}
	for _, e := range externals {
		if e.Target == "" {
			continue
		}
		if e.KeepCopy {
			fmt.Fprintf(&body, "redirect :copy %q;\n", e.Target)
		} else {
			fmt.Fprintf(&body, "redirect %q;\n", e.Target)
		}
	}
	return body.String()
}

func applyExternalSieve(ctx context.Context, acctID string, externals []forwarderExternal) error {
	scriptName := "jabali-fwds"
	if len(externals) == 0 {
		// Destroy the script if it exists.
		args := map[string]any{
			"accountId": acctID,
			"destroy":   []string{scriptName},
		}
		var result jmapSetResult
		_ = jmapCall(ctx, "SieveScript/set", args, &result) // best-effort — may not exist
		return nil
	}
	contents := buildExternalSieve(externals)
	// Upsert + activate.
	args := map[string]any{
		"accountId": acctID,
		"create": map[string]any{
			scriptName: map[string]any{
				"name":     scriptName,
				"isActive": true,
				"contents": contents,
			},
		},
	}
	var result jmapSetResult
	if err := jmapCall(ctx, "x:SieveUserScript/set", args, &result); err != nil {
		return err
	}
	if reason, ok := result.NotCreated[scriptName]; ok {
		// Probably already exists → update instead.
		args = map[string]any{
			"accountId": acctID,
			"update": map[string]any{
				scriptName: map[string]any{
					"contents": contents,
					"isActive": true,
				},
			},
		}
		var result2 jmapSetResult
		if err := jmapCall(ctx, "x:SieveUserScript/set", args, &result2); err != nil {
			return fmt.Errorf("sieve upsert: %w (first NotCreated: %s)", err, string(reason))
		}
		if reason2, ok2 := result2.NotUpdated[scriptName]; ok2 {
			return &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("sieve update refused: %s", string(reason2))}
		}
	}
	return nil
}

func init() {
	Default.Register("forwarder.apply", forwarderApplyHandler)
}
