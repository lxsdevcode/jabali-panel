package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// mailbox_display_name.go — set a mailbox's display name (GH #197).
//
// The display name is the Stalwart Account's `description`. Stalwart derives
// the default JMAP Identity name from it, which is what Bulwark webmail shows
// as the "From" display name (verified on .14: x:Account/set description ->
// Identity/get name follows). We set it over JMAP (admin) rather than via the
// SQL directory: the directory config is create-only + skipped when Stalwart
// is already serving, so it would never converge on an existing install — the
// JMAP path works on fresh AND existing installs.

// setAccountDescription resolves the registry account for email and sets its
// description (the display name). Empty displayName clears it. Best-effort: a
// missing account (mailbox never authed / not yet registered) is reported so
// the caller can decide, but callers generally treat it as non-fatal.
func setAccountDescription(ctx context.Context, email, displayName string) error {
	id, err := accountIDByEmail(ctx, email)
	if err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("setAccountDescription: no registry account for %q", email)
	}
	args := map[string]any{
		"update": map[string]any{
			id: map[string]any{"description": displayName},
		},
	}
	var result jmapSetResult
	if err := jmapCall(ctx, "x:Account/set", args, &result); err != nil {
		return err
	}
	if reason, ok := result.NotUpdated[id]; ok {
		return &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("x:Account/set description refused: %s", string(reason)),
		}
	}
	return nil
}

// mailboxSetDisplayNameParams is the request shape for
// mailbox.set_display_name.
type mailboxSetDisplayNameParams struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func mailboxSetDisplayNameHandler(ctx context.Context, params json.RawMessage) (any, error) {
	if len(params) == 0 {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "params required"}
	}
	var p mailboxSetDisplayNameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if _, err := requireEmail(p.Email); err != nil {
		return nil, err
	}
	if err := setAccountDescription(ctx, p.Email, p.DisplayName); err != nil {
		return nil, err
	}
	return okBody{Ok: true}, nil
}

func init() {
	Default.Register("mailbox.set_display_name", mailboxSetDisplayNameHandler)
}
