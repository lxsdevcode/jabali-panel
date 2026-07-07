package hestiacp

import (
	"os"
	"path/filepath"
	"regexp"
)

// GH #327 (johnnyq): HestiaCP's user.conf carries the account's Contact Name
// (FNAME / LNAME). Carry it into the created jabali user's name so the migrated
// account keeps its display name instead of a blank one.

var (
	hestiaFNameRe = regexp.MustCompile(`(?m)^\s*FNAME=['"]?([^'"\n]*)['"]?`)
	hestiaLNameRe = regexp.MustCompile(`(?m)^\s*LNAME=['"]?([^'"\n]*)['"]?`)
)

// PeekUserName reads <extractDir>/user.conf (the top-level Hestia account file)
// and returns the FNAME / LNAME contact name. Best-effort: returns "","" when
// the file is absent or the fields are empty.
func PeekUserName(extractDir string) (first, last string) {
	if extractDir == "" {
		return "", ""
	}
	b, err := os.ReadFile(filepath.Join(extractDir, "user.conf"))
	if err != nil {
		return "", ""
	}
	if m := hestiaFNameRe.FindSubmatch(b); m != nil {
		first = string(m[1])
	}
	if m := hestiaLNameRe.FindSubmatch(b); m != nil {
		last = string(m[1])
	}
	return first, last
}
