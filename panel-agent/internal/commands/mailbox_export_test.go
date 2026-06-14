package commands

import "testing"

// roleToMaildirSubdir must be the inverse of maildirSubfolderRole for
// the roled folders, so an export→import round-trip lands each Stalwart
// folder back in the same role.
func TestRoleToMaildirSubdir(t *testing.T) {
	cases := []struct {
		role, name, want string
	}{
		{"inbox", "Inbox", ""},
		{"sent", "Sent Items", ".Sent"},
		{"drafts", "Drafts", ".Drafts"},
		{"junk", "Junk Mail", ".Junk"},
		{"trash", "Deleted Items", ".Trash"},
		{"archive", "Archive", ".Archive"},
		{"", "Receipts", ".Receipts"},          // custom folder → name
		{"", "Work/2026", ".Work_2026"},        // path sep sanitized
		{"", ".hidden", ".hidden"},             // leading dots stripped then re-added
		{"INBOX", "Inbox", ""},                 // case-insensitive role
	}
	for _, c := range cases {
		got := roleToMaildirSubdir(c.role, c.name)
		if got != c.want {
			t.Errorf("roleToMaildirSubdir(%q,%q) = %q, want %q", c.role, c.name, got, c.want)
		}
	}

	// Round-trip: each role subdir's stripped name maps back to the role
	// the importer assigns.
	for _, rt := range []struct{ role, sub string }{
		{"sent", ".Sent"}, {"drafts", ".Drafts"}, {"junk", ".Junk"},
		{"trash", ".Trash"}, {"archive", ".Archive"},
	} {
		name := rt.sub[1:] // strip leading dot
		if back := maildirSubfolderRole(name); back != rt.role {
			t.Errorf("round-trip %q: maildirSubfolderRole(%q)=%q, want %q", rt.sub, name, back, rt.role)
		}
	}
}
