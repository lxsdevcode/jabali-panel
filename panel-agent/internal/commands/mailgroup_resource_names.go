package commands

import (
	"context"
	"strings"
)

// renameGroupResources renames a resource group's auto-created shared
// collections (calendar + address book) to the group's display name. Stalwart
// auto-creates them named "Personal (<address>)" for any principal (GH #350) —
// on a shared group that reads as every member's own "Personal", which is
// wrong. This projects the group's name onto them instead.
//
// Best-effort: the group principal already exists when this runs, so a naming
// hiccup must never fail the apply (same reflex as setAccountDescription). For
// distribution groups (no calendar/address book) the default-id lookups return
// "" and this no-ops. FileNode is created on demand (no default node to rename
// at provision time), so it's left to the share/create path.
func renameGroupResources(ctx context.Context, groupEmail, displayName string) {
	name := strings.TrimSpace(displayName)
	if name == "" {
		return
	}
	acctID, err := accountIDByEmail(ctx, groupEmail)
	if err != nil || acctID == "" {
		return
	}
	if calID, err := defaultCalendarID(ctx, acctID); err == nil && calID != "" {
		_ = jmapSetCollectionName(ctx, jmapCapCalendars, "Calendar/set", acctID, calID, name)
	}
	if abID, err := defaultAddressBookID(ctx, acctID); err == nil && abID != "" {
		_ = jmapSetCollectionName(ctx, jmapCapContacts, "AddressBook/set", acctID, abID, name)
	}
}

// jmapSetCollectionName updates the `name` of one collection via a JMAP */set.
func jmapSetCollectionName(ctx context.Context, capability, method, accountID, id, name string) error {
	args := map[string]any{
		"accountId": accountID,
		"update": map[string]any{
			id: map[string]any{"name": name},
		},
	}
	var result jmapSetResult
	return jmapCallWith(ctx, capability, method, args, &result)
}
