package commands

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Real lines captured from Stalwart 0.16.6 (mx) Log tracer, ansi=false.
const sampleQueued = `2026-06-19T22:11:43Z INFO Queued message for delivery (queue.message-queued) listenerId = "smtp", localPort = 25, remoteIp = 127.0.0.1, remotePort = 47258, queueId = 313790779361329152, from = "probe7@jabali.site", to = ["shuki@jabali.site"], size = 1662, nextRetry = 2026-06-19T22:11:43Z, nextDsn = 2026-06-20T22:11:43Z, expires = 2026-06-22T22:11:43Z`

const sampleMultiRcpt = `2026-06-19T23:00:00Z INFO Queued message for delivery (queue.message-queued) queueId = 9, from = "bob@otherdom.test", to = ["a@jabali.site", "b@jabali.site"], size = 4096, nextRetry = 2026-06-19T23:00:00Z`

const sampleCompleted = `2026-06-19T22:11:43Z INFO Delivery completed (delivery.completed) queueId = 313790779361329152, queueName = "local", from = "probe7@jabali.site", to = ["shuki@jabali.site"], size = 1662, total = 1, elapsed = 0ms`

func TestParseMailLogLine_Queued(t *testing.T) {
	pl, ok := parseMailLogLine(sampleQueued)
	if !ok {
		t.Fatal("expected queue.message-queued line to parse")
	}
	if pl.entry.Timestamp != "2026-06-19T22:11:43Z" {
		t.Errorf("timestamp = %q", pl.entry.Timestamp)
	}
	if pl.entry.From != "probe7@jabali.site" {
		t.Errorf("from = %q", pl.entry.From)
	}
	if pl.entry.To != "shuki@jabali.site" {
		t.Errorf("to = %q", pl.entry.To)
	}
	if pl.entry.Size != 1662 {
		t.Errorf("size = %d", pl.entry.Size)
	}
	if len(pl.recipients) != 1 || pl.recipients[0] != "shuki@jabali.site" {
		t.Errorf("recipients = %v", pl.recipients)
	}
}

func TestParseMailLogLine_MultiRecipient(t *testing.T) {
	pl, ok := parseMailLogLine(sampleMultiRcpt)
	if !ok {
		t.Fatal("expected parse")
	}
	if pl.entry.To != "a@jabali.site, b@jabali.site" {
		t.Errorf("joined to = %q", pl.entry.To)
	}
	if len(pl.recipients) != 2 {
		t.Errorf("recipients = %v", pl.recipients)
	}
}

func TestParseMailLogLine_SkipsNonMessageEvents(t *testing.T) {
	// delivery.completed is NOT the message record we key on.
	if _, ok := parseMailLogLine(sampleCompleted); ok {
		t.Error("delivery.completed should be skipped (not queue.message-queued)")
	}
	if _, ok := parseMailLogLine(`2026-06-19T22:10:31Z INFO Starting Stalwart Server (server.startup) version = "0.16.6"`); ok {
		t.Error("server.startup should be skipped")
	}
	if _, ok := parseMailLogLine(""); ok {
		t.Error("blank line should be skipped")
	}
}

// withMailLogDir points the handler at a temp dir containing one log file.
func withMailLogDir(t *testing.T, filename, content string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := mailLogDir
	mailLogDir = dir
	t.Cleanup(func() { mailLogDir = prev })
}

func callMailLogs(t *testing.T, params map[string]any) mailLogsQueryResponse {
	t.Helper()
	raw, _ := json.Marshal(params)
	out, err := mailLogsQueryHandler(context.Background(), raw)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	resp, ok := out.(mailLogsQueryResponse)
	if !ok {
		t.Fatalf("unexpected response type %T", out)
	}
	return resp
}

func TestMailLogsHandler_ParsesAndScopes(t *testing.T) {
	content := sampleQueued + "\n" + sampleCompleted + "\n" + sampleMultiRcpt + "\n"
	withMailLogDir(t, "delivery.2026-06-19", content)

	// No scope → both queued messages, newest first.
	resp := callMailLogs(t, map[string]any{"limit": 50})
	if resp.Total != 2 {
		t.Fatalf("total = %d, want 2 (delivery.completed must be ignored)", resp.Total)
	}
	if resp.Entries[0].Timestamp < resp.Entries[1].Timestamp {
		t.Error("entries not sorted newest-first")
	}

	// Scope to jabali.site → both still match (recipients are jabali.site).
	resp = callMailLogs(t, map[string]any{"domain_names": []string{"jabali.site"}, "limit": 50})
	if resp.Total != 2 {
		t.Errorf("scoped total = %d, want 2", resp.Total)
	}

	// Scope to a domain that only appears as a SENDER.
	resp = callMailLogs(t, map[string]any{"domain_names": []string{"otherdom.test"}, "limit": 50})
	if resp.Total != 1 {
		t.Errorf("sender-domain scope total = %d, want 1", resp.Total)
	}

	// Scope to an unrelated domain → nothing.
	resp = callMailLogs(t, map[string]any{"domain_names": []string{"nope.example"}, "limit": 50})
	if resp.Total != 0 {
		t.Errorf("unrelated scope total = %d, want 0", resp.Total)
	}
}

func TestMailLogsHandler_FiltersAndPaginates(t *testing.T) {
	content := sampleQueued + "\n" + sampleMultiRcpt + "\n"
	withMailLogDir(t, "delivery.2026-06-19", content)

	// Sender substring filter (panel maps the `sender` query param to
	// sender_prefix; the agent field is sender_prefix).
	resp := callMailLogs(t, map[string]any{"sender_prefix": "bob", "limit": 50})
	if resp.Total != 1 || resp.Entries[0].From != "bob@otherdom.test" {
		t.Errorf("sender filter: total=%d entries=%v", resp.Total, resp.Entries)
	}

	// Recipient substring filter.
	resp = callMailLogs(t, map[string]any{"recipient_prefix": "shuki", "limit": 50})
	if resp.Total != 1 {
		t.Errorf("recipient filter total = %d, want 1", resp.Total)
	}

	// Pagination: limit 1 returns the newest, offset 1 the next.
	page0 := callMailLogs(t, map[string]any{"limit": 1, "offset": 0})
	page1 := callMailLogs(t, map[string]any{"limit": 1, "offset": 1})
	if page0.Total != 2 || page1.Total != 2 {
		t.Fatalf("paginated total = %d/%d, want 2", page0.Total, page1.Total)
	}
	if len(page0.Entries) != 1 || len(page1.Entries) != 1 {
		t.Fatalf("page sizes = %d/%d, want 1", len(page0.Entries), len(page1.Entries))
	}
	if page0.Entries[0].Timestamp == page1.Entries[0].Timestamp {
		t.Error("offset pagination returned the same row")
	}
}

func TestMailLogsHandler_MissingDirIsEmpty(t *testing.T) {
	prev := mailLogDir
	mailLogDir = filepath.Join(t.TempDir(), "does-not-exist")
	t.Cleanup(func() { mailLogDir = prev })
	resp := callMailLogs(t, map[string]any{"limit": 50})
	if resp.Total != 0 || len(resp.Entries) != 0 {
		t.Errorf("missing dir should yield empty, got total=%d", resp.Total)
	}
}
