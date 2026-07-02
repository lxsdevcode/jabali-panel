package commands

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
)

func TestCalendarACLForLevel(t *testing.T) {
	read := calendarACLForLevel("read")
	if !read["mayReadItems"] || !read["mayReadFreeBusy"] {
		t.Errorf("read must grant read perms: %v", read)
	}
	if read["mayWriteAll"] || read["mayShare"] || read["mayDelete"] {
		t.Errorf("read must NOT grant write/share/delete: %v", read)
	}

	rw := calendarACLForLevel("readwrite")
	for _, k := range []string{"mayReadItems", "mayWriteAll", "mayWriteOwn", "mayUpdatePrivate", "mayRSVP"} {
		if !rw[k] {
			t.Errorf("readwrite missing %s: %v", k, rw)
		}
	}
	if rw["mayShare"] || rw["mayDelete"] {
		t.Errorf("readwrite must NOT grant share/delete: %v", rw)
	}

	admin := calendarACLForLevel("admin")
	for _, k := range []string{"mayReadItems", "mayWriteAll", "mayShare", "mayDelete"} {
		if !admin[k] {
			t.Errorf("admin missing %s: %v", k, admin)
		}
	}

	// Unknown level degrades to read-only.
	unknown := calendarACLForLevel("bogus")
	if unknown["mayWriteAll"] || len(unknown) != 2 {
		t.Errorf("unknown level must be read-only, got %v", unknown)
	}
}

func TestCalendarShareSetHandler_Validation(t *testing.T) {
	if _, err := calendarShareSetHandler(context.Background(), json.RawMessage(``)); err == nil {
		t.Error("empty params should error")
	}
	// Bad owner email errors before any JMAP call.
	_, err := calendarShareSetHandler(context.Background(), json.RawMessage(`{"owner_email":"not-an-email"}`))
	if err == nil {
		t.Fatal("invalid owner_email should error")
	}
	var ae *agentwire.AgentError
	if !errors.As(err, &ae) || ae.Code != agentwire.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestCalendarShareSetRegistered(t *testing.T) {
	if _, ok := Default.handlers["calendar.share_set"]; !ok {
		t.Error("calendar.share_set not registered")
	}
}
