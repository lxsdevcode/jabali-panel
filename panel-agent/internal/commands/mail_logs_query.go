package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// mail_logs_query.go — GH #239 mail delivery log viewer.
//
// Stalwart's "message delivery history" (the x:Trace JMAP method the first
// implementation used) is an ENTERPRISE-only feature — the community
// edition jabali ships returns `forbidden`, which the old handler silently
// swallowed, so the Mail > Logs tab was always empty.
//
// Logging/tracing to a file IS a community feature, so we read mail logs
// from a Stalwart Log file tracer instead. install/stalwart/apply-plan.json
// provisions a Log tracer (id jabali-delivery-log) that writes — with an
// include-only event filter, so the file stays small — one line per message
// to /var/log/stalwart/delivery.<date>:
//
//	2026-06-19T22:11:43Z INFO Queued message for delivery (queue.message-queued) \
//	  listenerId = "smtp", ..., from = "a@x", to = ["b@y", "c@z"], size = 1662, ...
//
// The systemd unit's LogsDirectory=stalwart guarantees the directory exists
// and is writable inside the ProtectSystem=strict sandbox (the original
// breakage: the dir was absent AND not in ReadWritePaths, so every Stalwart
// file tracer silently wrote nothing).
//
// We parse the `queue.message-queued` lines — one per accepted message, with
// from / to[] / size — into the {timestamp, from, to, size} rows the panel
// already renders. The agent runs as root, so it reads the
// jabali-mail:jabali-mail 0750 log files directly.

const (
	mailLogPrefix  = "delivery"
	mailLogEventID = "(queue.message-queued)"
	// mailLogMaxScan bounds memory: at most this many matching lines are
	// parsed across all log files before filtering. Newest files are read
	// first, so the cap drops only the oldest history.
	mailLogMaxScan = 50000
)

// mailLogDir is the Stalwart Log tracer output directory. A var (not const)
// so tests can point it at a temp dir.
var mailLogDir = "/var/log/stalwart"

// mailLogsQueryParams — panel → agent.
type mailLogsQueryParams struct {
	FromDate        *string  `json:"from_date"` // RFC 3339 or YYYY-MM-DD
	ToDate          *string  `json:"to_date"`
	SenderPrefix    *string  `json:"sender_prefix"`    // case-insensitive substring on From
	RecipientPrefix *string  `json:"recipient_prefix"` // case-insensitive substring on any recipient
	DomainNames     []string `json:"domain_names"`     // scope — From or a recipient must match ONE
	Limit           int      `json:"limit"`
	Offset          int      `json:"offset"`
}

type mailLogEntry struct {
	Timestamp string `json:"timestamp"`
	From      string `json:"from"`
	To        string `json:"to"` // recipients joined with ", "
	Size      int    `json:"size"`
}

type mailLogsQueryResponse struct {
	Entries []mailLogEntry `json:"entries"`
	Total   int            `json:"total"`
}

var (
	reMailLogFrom = regexp.MustCompile(`\bfrom = "([^"]*)"`)
	reMailLogTo   = regexp.MustCompile(`\bto = \[([^\]]*)\]`)
	reMailLogSize = regexp.MustCompile(`\bsize = (\d+)`)
)

// parsedMailLine is the parse result with recipients kept split for accurate
// per-recipient domain-scope matching (the wire entry joins them for display).
type parsedMailLine struct {
	entry      mailLogEntry
	recipients []string
	ts         time.Time
}

// parseMailLogLine extracts a message record from a single Stalwart Log
// tracer line. Returns ok=false for any non-message line (lifecycle events,
// blank lines) so callers can skip them.
func parseMailLogLine(line string) (parsedMailLine, bool) {
	if !strings.Contains(line, mailLogEventID) {
		return parsedMailLine{}, false
	}
	sp := strings.IndexByte(line, ' ')
	if sp <= 0 {
		return parsedMailLine{}, false
	}
	tsStr := line[:sp]
	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		return parsedMailLine{}, false
	}

	from := ""
	if m := reMailLogFrom.FindStringSubmatch(line); m != nil {
		from = m[1]
	}

	var recipients []string
	if m := reMailLogTo.FindStringSubmatch(line); m != nil {
		for _, p := range strings.Split(m[1], ",") {
			p = strings.Trim(strings.TrimSpace(p), `"`)
			if p != "" {
				recipients = append(recipients, p)
			}
		}
	}

	size := 0
	if m := reMailLogSize.FindStringSubmatch(line); m != nil {
		size, _ = strconv.Atoi(m[1])
	}

	return parsedMailLine{
		entry: mailLogEntry{
			Timestamp: tsStr,
			From:      from,
			To:        strings.Join(recipients, ", "),
			Size:      size,
		},
		recipients: recipients,
		ts:         ts,
	}, true
}

// mailLogFiles returns the delivery log files newest-first. Matches the
// tracer prefix exactly so the sibling stalwart.log tracer's files are not
// read. Missing directory (logging not yet provisioned) → empty, no error.
func mailLogFiles() []string {
	dents, err := os.ReadDir(mailLogDir)
	if err != nil {
		return nil
	}
	var files []string
	for _, d := range dents {
		if d.IsDir() {
			continue
		}
		name := d.Name()
		if name == mailLogPrefix || strings.HasPrefix(name, mailLogPrefix+".") {
			files = append(files, filepath.Join(mailLogDir, name))
		}
	}
	// Dated suffixes sort lexically == chronologically; reverse = newest first.
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}

// parseDateBound parses a from/to filter value as RFC3339 or YYYY-MM-DD.
func parseDateBound(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func mailLogsQueryHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p mailLogsQueryParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
		}
	}
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	if p.Offset < 0 {
		p.Offset = 0
	}

	var fromBound, toBound time.Time
	if p.FromDate != nil {
		if t, ok := parseDateBound(*p.FromDate); ok {
			fromBound = t
		}
	}
	if p.ToDate != nil {
		if t, ok := parseDateBound(*p.ToDate); ok {
			// Date-only "to" means the whole day is included.
			if len(*p.ToDate) <= 10 {
				t = t.Add(24*time.Hour - time.Second)
			}
			toBound = t
		}
	}

	senderSub := ""
	if p.SenderPrefix != nil {
		senderSub = strings.ToLower(strings.TrimSpace(*p.SenderPrefix))
	}
	recipientSub := ""
	if p.RecipientPrefix != nil {
		recipientSub = strings.ToLower(strings.TrimSpace(*p.RecipientPrefix))
	}

	matched := make([]mailLogEntry, 0, 128)
	scanned := 0

files:
	for _, f := range mailLogFiles() {
		if err := ctx.Err(); err != nil {
			break
		}
		fh, err := os.Open(f) //nolint:gosec // fixed log dir, agent-owned
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if scanned >= mailLogMaxScan {
				fh.Close()
				break files
			}
			pl, ok := parseMailLogLine(sc.Text())
			if !ok {
				continue
			}
			scanned++
			if !fromBound.IsZero() && pl.ts.Before(fromBound) {
				continue
			}
			if !toBound.IsZero() && pl.ts.After(toBound) {
				continue
			}
			if senderSub != "" && !strings.Contains(strings.ToLower(pl.entry.From), senderSub) {
				continue
			}
			if recipientSub != "" && !strings.Contains(strings.ToLower(pl.entry.To), recipientSub) {
				continue
			}
			if len(p.DomainNames) > 0 && !mailLineInScope(pl, p.DomainNames) {
				continue
			}
			matched = append(matched, pl.entry)
		}
		fh.Close()
	}

	// Newest first. Files are already newest-first and Stalwart appends in
	// time order, so a stable sort by timestamp desc gives a global order.
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].Timestamp > matched[j].Timestamp
	})

	total := len(matched)
	start := p.Offset
	if start > total {
		start = total
	}
	end := start + p.Limit
	if end > total {
		end = total
	}
	page := matched[start:end]
	if page == nil {
		page = []mailLogEntry{}
	}
	return mailLogsQueryResponse{Entries: page, Total: total}, nil
}

// mailLineInScope reports whether the message's sender or any recipient
// belongs to one of the caller's domains.
func mailLineInScope(pl parsedMailLine, domains []string) bool {
	for _, d := range domains {
		if containsDomain(pl.entry.From, d) {
			return true
		}
		for _, r := range pl.recipients {
			if containsDomain(r, d) {
				return true
			}
		}
	}
	return false
}

// containsDomain reports whether addr's domain part equals domain exactly.
func containsDomain(addr, domain string) bool {
	if addr == "" || domain == "" {
		return false
	}
	at := strings.LastIndexByte(addr, '@')
	if at < 0 || at == len(addr)-1 {
		return false
	}
	return strings.EqualFold(addr[at+1:], domain)
}

func init() {
	Default.Register("mail.logs_query", mailLogsQueryHandler)
}
