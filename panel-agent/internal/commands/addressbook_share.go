package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// addressbook_share.go — M52 (ADR-0133). Converge a shared address book's
// JMAP AddressBook.shareWith, mirroring calendar_share.go / mailbox_share.go.
//
// AddressBooks use yet another shareWith rights vocabulary (verified live on
// mx): mayRead, mayWrite, mayDelete, mayShare (NOT the calendar mayReadItems
// keys — Stalwart rejects cross-type keys as invalidProperties). The panel
// sends a coarse level (read / readwrite / admin) matching
// shared_resource_grants.rights.

// jmapCapContacts is the JMAP capability AddressBook/* methods require.
const jmapCapContacts = "urn:ietf:params:jmap:contacts"

type addressbookShareSetParams struct {
	OwnerEmail string            `json:"owner_email"`
	Shares     map[string]string `json:"shares"` // targetEmail → "read"|"readwrite"|"admin"
}

type addressbookShareSetResponse struct {
	Ok bool `json:"ok"`
}

// addressbookACLForLevel maps a coarse grant level to AddressBook shareWith
// keys. Unknown levels degrade to read-only.
func addressbookACLForLevel(level string) map[string]bool {
	acl := map[string]bool{"mayRead": true}
	switch level {
	case "readwrite":
		acl["mayWrite"] = true
	case "admin":
		acl["mayWrite"] = true
		acl["mayShare"] = true
		acl["mayDelete"] = true
	}
	return acl
}

func addressbookShareSetHandler(ctx context.Context, params json.RawMessage) (any, error) {
	if len(params) == 0 {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "params required"}
	}
	var p addressbookShareSetParams
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
		return nil, &agentwire.AgentError{Code: agentwire.CodeNotFound, Message: "shared address book host not yet registered"}
	}

	targetIDs := make(map[string]map[string]bool, len(p.Shares))
	for targetEmail, level := range p.Shares {
		tid, err := accountIDByEmail(ctx, targetEmail)
		if err != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("resolve target %s: %v", targetEmail, err)}
		}
		if tid == "" {
			continue
		}
		targetIDs[tid] = addressbookACLForLevel(level)
	}

	abID, err := defaultAddressBookID(ctx, acctID)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("resolve address book: %v", err)}
	}
	if abID == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeNotFound, Message: "shared address book host has no address book yet"}
	}

	args := map[string]any{
		"accountId": acctID,
		"update": map[string]any{
			abID: map[string]any{
				"shareWith": targetIDs,
			},
		},
	}
	var result jmapSetResult
	if err := jmapCallWith(ctx, jmapCapContacts, "AddressBook/set", args, &result); err != nil {
		return nil, err
	}
	if reason, ok := result.NotUpdated[abID]; ok {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("AddressBook/set refused: %s", string(reason))}
	}
	return addressbookShareSetResponse{Ok: true}, nil
}

// defaultAddressBookID resolves the host account's default address book id.
func defaultAddressBookID(ctx context.Context, accountID string) (string, error) {
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
	if err := jmapCallWith(ctx, jmapCapContacts, "AddressBook/get", args, &result); err != nil {
		return "", err
	}
	if len(result.List) == 0 {
		return "", nil
	}
	for _, a := range result.List {
		if a.IsDefault {
			return a.ID, nil
		}
	}
	return result.List[0].ID, nil
}

func init() {
	Default.Register("addressbook.share_set", addressbookShareSetHandler)
}
