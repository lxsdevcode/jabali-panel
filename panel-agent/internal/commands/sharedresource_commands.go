package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// sharedresource_commands.go — M52 (ADR-0133). Provision / tear down the
// Stalwart host principal that backs a shared resource.
//
// In the one-principal-per-resource model each shared resource (calendar /
// address book / file folder / mailbox) is hosted by its own @type:Group
// principal, which auto-owns the collections. The `*.share_set` commands then
// converge that collection's shareWith. The host is NOT made a mail recipient
// here — the SQL directory's queryRecipient resolves delivery from the
// mailboxes / mail_groups / forwarders tables, not the registry — so a
// calendar/addressbook/file host never receives mail.

type sharedResourceApplyParams struct {
	Email       string `json:"email"`        // host principal address (name@domain)
	DisplayName string `json:"display_name"` // shown as the collection / From name
	Kind        string `json:"kind"`         // mailbox | calendar | addressbook | files (advisory)
}

type sharedResourceApplyResponse struct {
	Ok            bool   `json:"ok"`
	HostAccountID string `json:"host_account_id"`
}

func sharedResourceApplyHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p sharedResourceApplyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	email, err := requireEmail(p.Email)
	if err != nil {
		return nil, err
	}

	hostID, err := ensureGroupInRegistry(ctx, email, strings.TrimSpace(p.DisplayName))
	if err != nil {
		return nil, err
	}
	// Best-effort display name (drives the collection / From name); the host
	// already exists at this point so a hiccup must not fail the apply.
	if dn := strings.TrimSpace(p.DisplayName); dn != "" {
		_ = setAccountDescription(ctx, email, dn)
	}
	return sharedResourceApplyResponse{Ok: true, HostAccountID: hostID}, nil
}

type sharedResourceDestroyParams struct {
	Email string `json:"email"`
}

type sharedResourceDestroyResponse struct {
	Ok bool `json:"ok"`
}

func sharedResourceDestroyHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p sharedResourceDestroyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if _, err := requireEmail(p.Email); err != nil {
		return nil, err
	}

	hostID, err := accountIDByEmail(ctx, p.Email)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("resolve host: %v", err)}
	}
	if hostID == "" {
		// Already gone — destroy is idempotent.
		return sharedResourceDestroyResponse{Ok: true}, nil
	}

	// Destroying the host principal removes its collections and every
	// shareWith grant pointing at it in one step.
	args := map[string]any{"destroy": []string{hostID}}
	var result jmapSetResult
	if err := jmapCall(ctx, "x:Account/set", args, &result); err != nil {
		return nil, err
	}
	if reason, ok := result.NotDestroyed[hostID]; ok {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("x:Account/set destroy refused: %s", string(reason))}
	}
	return sharedResourceDestroyResponse{Ok: true}, nil
}

func init() {
	Default.Register("sharedresource.apply", sharedResourceApplyHandler)
	Default.Register("sharedresource.destroy", sharedResourceDestroyHandler)
}
