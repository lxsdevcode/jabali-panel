package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// calendar_share.go — M52 (ADR-0133). Converge a shared calendar's
// JMAP Calendar.shareWith, mirroring mailbox_share.go for the Mailbox case.
//
// A shared calendar is one collection on its own host principal (the M52
// one-principal-per-resource model). This command writes the whole shareWith
// map for that principal's default calendar in one idempotent replace.
//
// Calendars use a DIFFERENT shareWith rights vocabulary than mailboxes
// (verified live on mx): mayReadFreeBusy, mayReadItems, mayWriteAll,
// mayWriteOwn, mayUpdatePrivate, mayRSVP, mayShare, mayDelete. The panel
// sends a coarse level (read / readwrite / admin) matching
// shared_resource_grants.rights; calendarACLForLevel maps it to those keys.

// jmapCapCalendars is the JMAP capability that Calendar/* methods require in
// the request `using` list (jmapUsing alone doesn't advertise it).
const jmapCapCalendars = "urn:ietf:params:jmap:calendars"

type calendarShareSetParams struct {
	// OwnerEmail is the shared-calendar host principal's address.
	OwnerEmail string `json:"owner_email"`
	// Shares maps each target principal's email → grant level
	// ("read" | "readwrite" | "admin"). Absent targets are revoked
	// (whole-map replace).
	Shares map[string]string `json:"shares"`
}

type calendarShareSetResponse struct {
	Ok bool `json:"ok"`
}

// calendarACLForLevel maps a coarse grant level to the exact Stalwart
// Calendar shareWith permission keys. Unknown levels degrade to read-only.
func calendarACLForLevel(level string) map[string]bool {
	acl := map[string]bool{
		"mayReadFreeBusy": true,
		"mayReadItems":    true,
	}
	switch level {
	case "readwrite":
		acl["mayWriteAll"] = true
		acl["mayWriteOwn"] = true
		acl["mayUpdatePrivate"] = true
		acl["mayRSVP"] = true
	case "admin":
		acl["mayWriteAll"] = true
		acl["mayWriteOwn"] = true
		acl["mayUpdatePrivate"] = true
		acl["mayRSVP"] = true
		acl["mayShare"] = true
		acl["mayDelete"] = true
	}
	return acl
}

func calendarShareSetHandler(ctx context.Context, params json.RawMessage) (any, error) {
	if len(params) == 0 {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "params required"}
	}
	var p calendarShareSetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if _, err := requireEmail(p.OwnerEmail); err != nil {
		return nil, err
	}

	acctID, err := accountIDByEmail(ctx, p.OwnerEmail)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("resolve account: %v", err)}
	}
	if acctID == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeNotFound, Message: "shared calendar host not yet registered"}
	}

	// Resolve target principals by email → id, mapping each level to its ACL.
	targetIDs := make(map[string]map[string]bool, len(p.Shares))
	for targetEmail, level := range p.Shares {
		tid, err := accountIDByEmail(ctx, targetEmail)
		if err != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("resolve target %s: %v", targetEmail, err)}
		}
		if tid == "" {
			continue // not yet registered; reconciler retries on a later pass
		}
		targetIDs[tid] = calendarACLForLevel(level)
	}

	calID, err := defaultCalendarID(ctx, acctID)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("resolve calendar: %v", err)}
	}
	if calID == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeNotFound, Message: "shared calendar host has no calendar yet"}
	}

	args := map[string]any{
		"accountId": acctID,
		"update": map[string]any{
			calID: map[string]any{
				"shareWith": targetIDs,
			},
		},
	}
	var result jmapSetResult
	if err := jmapCallWith(ctx, jmapCapCalendars, "Calendar/set", args, &result); err != nil {
		return nil, err
	}
	if reason, ok := result.NotUpdated[calID]; ok {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("Calendar/set refused: %s", string(reason))}
	}
	return calendarShareSetResponse{Ok: true}, nil
}

// defaultCalendarID resolves the host account's default calendar id. In the
// one-principal-per-resource model the host has exactly one calendar; prefer
// the isDefault flag, fall back to the first listed.
func defaultCalendarID(ctx context.Context, accountID string) (string, error) {
	args := map[string]any{
		"accountId":  accountID,
		"properties": []string{"isDefault"},
	}
	var result struct {
		List []struct {
			ID        string `json:"id"`
			IsDefault bool   `json:"isDefault"`
		} `json:"list"`
	}
	if err := jmapCallWith(ctx, jmapCapCalendars, "Calendar/get", args, &result); err != nil {
		return "", err
	}
	if len(result.List) == 0 {
		return "", nil
	}
	for _, c := range result.List {
		if c.IsDefault {
			return c.ID, nil
		}
	}
	return result.List[0].ID, nil
}

func init() {
	Default.Register("calendar.share_set", calendarShareSetHandler)
}
