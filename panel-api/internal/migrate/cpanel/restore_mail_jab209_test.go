package cpanel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JAB-209. A cPanel restore reported
//
//	mailboxes: count=1 messages_found=74168 messages_pushed=74167
//
// with zero occurrences of "error" or "fail" in the entire restore log, exit 0,
// and no degraded flag — then deleted the staging tree, so the message could
// not even be identified afterwards.
//
// One in 74k is survivable. The pattern is not: a silent, unlogged gap means a
// LARGER loss would present identically, and no operator can honestly tell a
// customer their mail migrated.

func TestCountMaildirMessages_ExcludesTmp(t *testing.T) {
	t.Parallel()

	// tmp/ was counted as findable while the importer only ever walks cur/ and
	// new/, so every stale tmp/ file inflated messages_found by one and looked
	// exactly like a lost message. Per the Maildir spec tmp/ holds deliveries
	// in progress — an interrupted one is not mail the user has.
	md := t.TempDir()
	for _, sub := range []string{"cur", "new", "tmp"} {
		if err := os.MkdirAll(filepath.Join(md, sub), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(sub, name string) {
		if err := os.WriteFile(filepath.Join(md, sub, name), []byte("From: a@b\r\n\r\nx"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("cur", "1700000000.M1:2,S")
	write("cur", "1700000001.M2:2,S")
	write("new", "1700000002.M3")
	write("tmp", "1700000003.M4") // interrupted delivery

	count, _, tmpFound := countMaildirMessagesDetailed(md)

	if count != 3 {
		t.Errorf("count = %d, want 3 (cur+new only) — counting tmp/ is what made "+
			"found exceed pushed with nothing logged", count)
	}
	if tmpFound != 1 {
		t.Errorf("tmpFound = %d, want 1 — tmp/ must still be visible to the "+
			"caller, just not folded into the pushable total", tmpFound)
	}
}

func TestReconcileMailCounts(t *testing.T) {
	t.Parallel()

	t.Run("no gap says nothing", func(t *testing.T) {
		if got := reconcileMailCounts(100, 100, nil); got != nil {
			t.Errorf("expected silence when everything imported, got %v", got)
		}
	})

	t.Run("a fully explained gap is stated but not alarming", func(t *testing.T) {
		msgs := reconcileMailCounts(10, 8, []string{
			"deduped:2:/srv/mail/x — message(s) already present with the same Message-ID",
		})
		if len(msgs) != 1 {
			t.Fatalf("want one line, got %v", msgs)
		}
		if strings.Contains(msgs[0], "WARNING") {
			t.Errorf("a gap that is entirely deduplication must not read as loss: %s", msgs[0])
		}
		if !strings.Contains(msgs[0], "all accounted for") {
			t.Errorf("should say the gap is understood: %s", msgs[0])
		}
	})

	t.Run("an unexplained gap warns loudly and says not to trust the migration", func(t *testing.T) {
		// This is the reported case: 74168 found, 74167 pushed, nothing skipped.
		msgs := reconcileMailCounts(74168, 74167, nil)
		if len(msgs) != 1 {
			t.Fatalf("want one line, got %v", msgs)
		}
		got := msgs[0]
		if !strings.Contains(got, "WARNING") {
			t.Errorf("an unexplained missing message MUST warn — silence here is "+
				"the whole bug: %s", got)
		}
		if !strings.Contains(got, "UNEXPLAINED") {
			t.Errorf("should distinguish unexplained from deduped: %s", got)
		}
		if !strings.Contains(got, "do not report this mailbox as fully migrated") {
			t.Errorf("the operator needs to be told what NOT to claim: %s", got)
		}
		if !strings.Contains(got, "before deleting the source") {
			t.Errorf("staging is deleted on completion, so the guidance has to "+
				"mention the source is the remaining copy: %s", got)
		}
	})

	t.Run("partly explained still warns about the remainder", func(t *testing.T) {
		// 3 missing, 2 deduped -> 1 genuinely unaccounted for.
		msgs := reconcileMailCounts(10, 7, []string{
			"deduped:2:/srv/mail/x — message(s) already present with the same Message-ID",
		})
		got := msgs[0]
		if !strings.Contains(got, "WARNING") || !strings.Contains(got, "1 of those are UNEXPLAINED") {
			t.Errorf("a partly-explained gap must still surface the remainder: %s", got)
		}
	})

	t.Run("oversized messages count as explained", func(t *testing.T) {
		msgs := reconcileMailCounts(10, 9, []string{
			"oversized:/srv/mail/x/cur/huge:120000000",
		})
		if strings.Contains(msgs[0], "WARNING") {
			t.Errorf("an oversized message is a reported decision, not a silent "+
				"drop: %s", msgs[0])
		}
	})
}
