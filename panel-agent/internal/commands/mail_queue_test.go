package commands

import (
	"strings"
	"testing"
)

// Contract test (feedback_schema_enumerate_kinds_not_names): pin the
// Stalwart 0.16 QueuedMessage field KINDS, not just names. Golden
// mirrors the authoritative `stalwart-cli describe QueuedMessage`
// schema (recipients=map, returnPath=string, size=number,
// nextRetry=datetime|null, priority=number, id=string). The mx queue
// was empty at pin time so the cli --json envelope shape is tolerated
// both as a bare array and an {items|data:[]} wrapper.
func TestParseQueuedMessages_Contract(t *testing.T) {
	cases := []string{
		`[{"id":"abc123","returnPath":"bounce@ex.com","size":4096,"priority":10,"nextRetry":"2026-05-18T10:00:00Z","receivedFromIp":"203.0.113.9","recipients":{"a@d1.com":{"status":"scheduled"},"b@d2.com":{"status":"failed"}}}]`,
		`{"items":[{"id":"abc123","returnPath":"bounce@ex.com","size":4096,"priority":10,"nextRetry":null,"recipients":{"a@d1.com":{"status":"scheduled"}}}]}`,
		`{"data":[]}`,
		`[]`,
		``,
	}
	for i, in := range cases {
		out, err := parseQueuedMessages([]byte(in))
		if err != nil {
			t.Fatalf("case %d: unexpected err: %v", i, err)
		}
		if i == 0 {
			if len(out) != 1 {
				t.Fatalf("case0: want 1 msg, got %d", len(out))
			}
			m := out[0]
			if m.ID != "abc123" || m.From != "bounce@ex.com" || m.Size != 4096 {
				t.Errorf("case0 map wrong: %+v", m)
			}
			if len(m.Recipients) != 2 {
				t.Errorf("case0 recipients = %v, want 2 (map keys)", m.Recipients)
			}
			if m.NextRetry != "2026-05-18T10:00:00Z" {
				t.Errorf("case0 next_retry = %q", m.NextRetry)
			}
		}
		if i == 1 && (len(out) != 1 || out[0].NextRetry != "") {
			t.Errorf("case1 (items wrapper, null nextRetry): %+v", out)
		}
		if (i == 2 || i == 3 || i == 4) && len(out) != 0 {
			t.Errorf("case %d: empty/blank must yield 0, got %d", i, len(out))
		}
	}
}

func TestSanitizeQueueID(t *testing.T) {
	ok := []string{"abc123", "01KRSHK2XDJS7", "msg_4-5A"}
	for _, s := range ok {
		if err := sanitizeQueueID(s); err != nil {
			t.Errorf("sanitizeQueueID(%q) rejected valid id: %v", s, err)
		}
	}
	bad := []string{"", "a b", "a;rm -rf", "$(x)", "a/b", "a`b`", strings.Repeat("x", 200)}
	for _, s := range bad {
		if err := sanitizeQueueID(s); err == nil {
			t.Errorf("sanitizeQueueID(%q) accepted unsafe id", s)
		}
	}
}
