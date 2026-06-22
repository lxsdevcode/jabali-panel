// mail_logs_detail.go — GH #261 mail-log message detail (delivery trail).
//
// The Stalwart file tracer carries no subject/body (that needs JMAP x:Trace),
// but the full log DOES carry the per-message delivery trail keyed by queueId:
// queued → attempt → connect → STARTTLS → delivered / DSN with the remote
// server's response code + hostname + text, or the failure reason. That trail
// is the "why didn't my mail arrive" view this command returns.
//
// Scoped exactly like mail.logs_query: a non-admin caller passes domain_names
// and the trail is returned ONLY when the message's sender or a recipient is in
// that scope — so a tenant can't read another tenant's delivery metadata by
// guessing a queueId.
package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type mailLogsDetailRequest struct {
	QueueID     string   `json:"queue_id"`
	DomainNames []string `json:"domain_names"`
}

type mailLogDetailEvent struct {
	Timestamp string `json:"timestamp"`
	Event     string `json:"event"`             // e.g. "delivery.completed"
	Message   string `json:"message"`           // human text Stalwart logged
	Detail    string `json:"detail,omitempty"`  // remote code/hostname/text/reason
}

type mailLogsDetailResponse struct {
	QueueID string               `json:"queue_id"`
	From    string               `json:"from"`
	To      string               `json:"to"`
	Events  []mailLogDetailEvent `json:"events"`
}

var (
	reMailDetailQIDNumeric = regexp.MustCompile(`^\d{1,30}$`)
	reMailDetailEvent      = regexp.MustCompile(`\(([a-z]+\.[a-z-]+)\)`)
	// Message = the text between the log level and the "(event)" tag.
	reMailDetailMessage = regexp.MustCompile(`^\S+\s+\w+\s+(.*?)\s+\([a-z]+\.[a-z-]+\)`)
	reMailDetailCode     = regexp.MustCompile(`\bcode = (\d+)`)
	reMailDetailHostname = regexp.MustCompile(`\bhostname = "([^"]*)"`)
	reMailDetailDetails  = regexp.MustCompile(`\bdetails = "([^"]*)"`)
	reMailDetailReason   = regexp.MustCompile(`\breason = "([^"]*)"`)
)

const mailLogDetailMaxEvents = 200

// mailLogAllFiles lists every tracer file in mailLogDir (the delivery.* tracer
// AND the full stalwart.log.* tracer), newest first. The delivery trail's
// connect/STARTTLS/DSN events live in stalwart.log, not the narrow delivery
// tracer, so the detail view must read both.
func mailLogAllFiles() []string {
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
		if strings.HasPrefix(name, "delivery") || strings.HasPrefix(name, "stalwart.log") {
			files = append(files, filepath.Join(mailLogDir, name))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}

func mailLogDetailField(line string) string {
	parts := []string{}
	if m := reMailDetailCode.FindStringSubmatch(line); m != nil {
		parts = append(parts, "code "+m[1])
	}
	if m := reMailDetailHostname.FindStringSubmatch(line); m != nil && m[1] != "" {
		parts = append(parts, m[1])
	}
	if m := reMailDetailDetails.FindStringSubmatch(line); m != nil && m[1] != "" {
		parts = append(parts, m[1])
	}
	if m := reMailDetailReason.FindStringSubmatch(line); m != nil && m[1] != "" {
		parts = append(parts, m[1])
	}
	return strings.Join(parts, " — ")
}

func mailLogsDetailHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var req mailLogsDetailRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, mwInvalidArg("malformed JSON body")
	}
	if !reMailDetailQIDNumeric.MatchString(req.QueueID) {
		return nil, mwInvalidArg("queue_id must be numeric")
	}
	needle := "queueId = " + req.QueueID

	out := mailLogsDetailResponse{QueueID: req.QueueID, Events: []mailLogDetailEvent{}}
	var recipients []string

	for _, f := range mailLogAllFiles() {
		if err := ctx.Err(); err != nil {
			break
		}
		fh, err := os.Open(f) //nolint:gosec // fixed agent-owned log dir
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			// Exact-token match so queueId 31 doesn't match 314…; the id is
			// followed by ", " or end-of-fields.
			idx := strings.Index(line, needle)
			if idx < 0 {
				continue
			}
			after := line[idx+len(needle):]
			if after != "" && after[0] != ',' && after[0] != ' ' {
				continue
			}
			// Capture from/to once for the scope check + header.
			if out.From == "" {
				if m := reMailLogFrom.FindStringSubmatch(line); m != nil {
					out.From = m[1]
				}
			}
			if len(recipients) == 0 {
				if m := reMailLogTo.FindStringSubmatch(line); m != nil {
					for _, p := range strings.Split(m[1], ",") {
						p = strings.Trim(strings.TrimSpace(p), `"`)
						if p != "" {
							recipients = append(recipients, p)
						}
					}
				}
			}
			ev := ""
			if m := reMailDetailEvent.FindStringSubmatch(line); m != nil {
				ev = m[1]
			}
			msg := ev
			if m := reMailDetailMessage.FindStringSubmatch(line); m != nil {
				msg = m[1]
			}
			ts := line
			if sp := strings.IndexByte(line, ' '); sp > 0 {
				ts = line[:sp]
			}
			out.Events = append(out.Events, mailLogDetailEvent{
				Timestamp: ts,
				Event:     ev,
				Message:   msg,
				Detail:    mailLogDetailField(line),
			})
			if len(out.Events) >= mailLogDetailMaxEvents {
				fh.Close()
				goto done
			}
		}
		fh.Close()
	}
done:
	out.To = strings.Join(recipients, ", ")

	// Scope: a non-admin caller (domain_names set) may only see a trail whose
	// message involves one of its domains. Reuse the same scoping as the list.
	if len(req.DomainNames) > 0 {
		pl := parsedMailLine{entry: mailLogEntry{From: out.From}, recipients: recipients}
		if !mailLineInScope(pl, req.DomainNames) {
			return mailLogsDetailResponse{QueueID: req.QueueID, Events: []mailLogDetailEvent{}}, nil
		}
	}

	// Oldest-first so the trail reads top-to-bottom in time order.
	sort.SliceStable(out.Events, func(i, j int) bool {
		return out.Events[i].Timestamp < out.Events[j].Timestamp
	})
	return out, nil
}

func init() {
	Default.Register("mail.logs_detail", mailLogsDetailHandler)
}
