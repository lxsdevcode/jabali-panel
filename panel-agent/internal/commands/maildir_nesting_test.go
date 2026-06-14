package commands

import (
	"strings"
	"testing"
)

// TestMaildirSegmentName — one Maildir++ path segment per mailbox:
// canonical role name for roled folders, sanitized name for custom.
func TestMaildirSegmentName(t *testing.T) {
	cases := []struct {
		mb   jmapMailbox
		want string
	}{
		{jmapMailbox{Role: "inbox", Name: "Inbox"}, ""},
		{jmapMailbox{Role: "sent", Name: "Sent Items"}, "Sent"},
		{jmapMailbox{Role: "drafts", Name: "Drafts"}, "Drafts"},
		{jmapMailbox{Role: "archive", Name: "Archive"}, "Archive"},
		{jmapMailbox{Name: "Receipts"}, "Receipts"},
		// Internal dot escaped so it can't be read as a hierarchy
		// separator on import.
		{jmapMailbox{Name: "v1.2"}, "v1_2"},
		// Slash in a single name (NOT real nesting) sanitized to _.
		{jmapMailbox{Name: "Work/2026"}, "Work_2026"},
		// Dots (leading or internal) are escaped to "_" so they can't be
		// read as Maildir++ hierarchy separators.
		{jmapMailbox{Name: "..hidden"}, "__hidden"},
	}
	for _, c := range cases {
		if got := maildirSegmentName(c.mb); got != c.want {
			t.Errorf("maildirSegmentName(%+v) = %q, want %q", c.mb, got, c.want)
		}
	}
}

// TestMaildirSubdirForMailbox — the nesting fix (defect 1): a child
// mailbox must encode its FULL parent path as Maildir++ ".A.B.C", not
// just the leaf, and the importer's split must reproduce the segments.
func TestMaildirSubdirForMailbox(t *testing.T) {
	inbox := jmapMailbox{ID: "i", Name: "Inbox", Role: "inbox"}
	archive := jmapMailbox{ID: "a", Name: "Archive", Role: "archive"}
	y2026 := jmapMailbox{ID: "y", Name: "2026", ParentID: "a"}
	q1 := jmapMailbox{ID: "q", Name: "Q1", ParentID: "y"}
	receipts := jmapMailbox{ID: "r", Name: "Receipts"}
	// A custom folder whose parent is INBOX is top-level in Maildir++.
	underInbox := jmapMailbox{ID: "u", Name: "Newsletters", ParentID: "i"}
	byID := map[string]jmapMailbox{
		"i": inbox, "a": archive, "y": y2026, "q": q1, "r": receipts, "u": underInbox,
	}

	cases := []struct {
		mb   jmapMailbox
		want string
	}{
		{inbox, ""},
		{archive, ".Archive"},
		{y2026, ".Archive.2026"},
		{q1, ".Archive.2026.Q1"},
		{receipts, ".Receipts"},
		{underInbox, ".Newsletters"},
	}
	for _, c := range cases {
		got := maildirSubdirForMailbox(c.mb, byID)
		if got != c.want {
			t.Errorf("maildirSubdirForMailbox(%q) = %q, want %q", c.mb.Name, got, c.want)
		}
		// Round-trip: the importer splits ".A.B.C" → [A,B,C]. The leaf
		// segment must equal the mailbox's own segment name.
		if got != "" {
			segs := strings.Split(strings.TrimPrefix(got, "."), ".")
			if leaf := segs[len(segs)-1]; leaf != maildirSegmentName(c.mb) {
				t.Errorf("round-trip %q: leaf %q != segment %q", got, leaf, maildirSegmentName(c.mb))
			}
		}
	}
}

// TestMaildirSubdirForMailbox_BrokenParent — a dangling parentId must
// not loop or panic; the chain stops and encodes what it can.
func TestMaildirSubdirForMailbox_BrokenParent(t *testing.T) {
	orphan := jmapMailbox{ID: "o", Name: "Lost", ParentID: "missing"}
	byID := map[string]jmapMailbox{"o": orphan}
	if got := maildirSubdirForMailbox(orphan, byID); got != ".Lost" {
		t.Errorf("orphan parent: got %q, want %q", got, ".Lost")
	}
}
