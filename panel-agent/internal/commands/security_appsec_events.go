// security.crowdsec.appsec.events — surface WHICH CRS rule blocked a request.
//
// JAB-193 and its cluster (195/196/197/198) all stalled on the same thing: an
// operator sees a 403 and has no way to learn which rule fired on which
// variable. /var/log/crowdsec.log only records the outcome —
//
//	"crowdsecurity/appsec-native by ip 1.2.3.4 (IT/8075) : 4h ban on Ip 1.2.3.4"
//
// — with no rule, no URI, no matched variable. Writing a CRS exclusion from
// that is guesswork, and ADR-0124 records what guesswork cost last time: the
// first triage excluded 901340, which is the non-blocking body-inspection
// enabler rather than the rule that scored.
//
// The data does exist. `cscli alerts inspect <id> -d` carries rule_ids,
// rule_name, target_uri, target_host and source_ip in each event's meta. This
// command collects it so `jabali appsec explain` can group it into the shape a
// person needs to write a surgical exclusion.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// appsecScenario is the scenario cscli files AppSec inband blocks under.
const appsecScenario = "crowdsecurity/appsec-native"

// defaultAppsecEventLimit bounds how many alerts we inspect. Each alert costs
// one cscli invocation, and a busy edge box accumulates thousands — the point
// is a triage sample, not an export.
const defaultAppsecEventLimit = 25

type appsecEventsParams struct {
	// Limit caps the alerts inspected (default 25, max 200).
	Limit int `json:"limit,omitempty"`
}

// AppsecEvent is one blocked request, flattened from the alert meta.
type AppsecEvent struct {
	Timestamp  string   `json:"timestamp"`
	SourceIP   string   `json:"source_ip"`
	TargetHost string   `json:"target_host"`
	TargetURI  string   `json:"target_uri"`
	Action     string   `json:"action"`
	RuleName   string   `json:"rule_name"`
	RuleIDs    []string `json:"rule_ids"`
	Country    string   `json:"country"`
	ASNOrg     string   `json:"asn_org"`
}

type appsecEventsResponse struct {
	Events []AppsecEvent `json:"events"`
	// AlertsScanned is how many alerts were inspected, so the caller can say
	// "nothing found in the last N" rather than implying the box is clean.
	AlertsScanned int `json:"alerts_scanned"`
}

func appsecEventsHandler(ctx context.Context, params json.RawMessage) (any, error) {
	limit := defaultAppsecEventLimit
	if len(params) > 0 {
		var p appsecEventsParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, csInvalidArg(fmt.Sprintf("parse params: %v", err))
		}
		if p.Limit > 0 {
			limit = p.Limit
		}
	}
	if limit > 200 {
		limit = 200
	}

	raw, err := runCscliJSON(ctx, "alerts", "list", "--scenario", appsecScenario,
		"--limit", strconv.Itoa(limit))
	if err != nil {
		// No AppSec alerts yet is a normal, healthy state and cscli exits
		// nonzero for it. Report empty rather than an error the operator has
		// to interpret.
		return appsecEventsResponse{Events: []AppsecEvent{}}, nil
	}

	var alerts []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &alerts); err != nil {
		return nil, csInternal("parse cscli alerts list", err)
	}

	out := make([]AppsecEvent, 0, len(alerts))
	for _, a := range alerts {
		evs, err := inspectAppsecAlert(ctx, a.ID)
		if err != nil {
			// One unreadable alert must not sink the whole triage run.
			continue
		}
		out = append(out, evs...)
	}
	return appsecEventsResponse{Events: out, AlertsScanned: len(alerts)}, nil
}

// inspectAppsecAlert flattens one alert's events into AppsecEvent rows.
func inspectAppsecAlert(ctx context.Context, id int) ([]AppsecEvent, error) {
	raw, err := runCscliJSON(ctx, "alerts", "inspect", strconv.Itoa(id), "-d")
	if err != nil {
		return nil, err
	}
	var alert struct {
		Events []struct {
			Timestamp string `json:"timestamp"`
			Meta      []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"meta"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &alert); err != nil {
		return nil, err
	}

	out := make([]AppsecEvent, 0, len(alert.Events))
	for _, ev := range alert.Events {
		m := make(map[string]string, len(ev.Meta))
		for _, kv := range ev.Meta {
			m[kv.Key] = kv.Value
		}
		// Only inband blocks are actionable for FP triage; an out-of-band
		// scenario hit did not 403 anyone.
		if m["log_type"] != "appsec-block" {
			continue
		}
		out = append(out, AppsecEvent{
			Timestamp:  ev.Timestamp,
			SourceIP:   m["source_ip"],
			TargetHost: m["target_host"],
			TargetURI:  m["target_uri"],
			Action:     m["appsec_action"],
			RuleName:   m["rule_name"],
			RuleIDs:    parseRuleIDs(m["rule_ids"]),
			Country:    m["IsoCode"],
			ASNOrg:     m["ASNOrg"],
		})
	}
	return out, nil
}

// parseRuleIDs turns cscli's "[901340 1930792474]" into a slice.
//
// Both entries matter and neither alone is the answer. rule_name reports only
// the FIRST id, and on a real block that is routinely 901340 — the CRS
// body-inspection enabler, which scores nothing. ADR-0124 was written after
// exactly that id was excluded to no effect. Returning the full list is what
// lets the caller show the others, which are the rules that actually scored.
func parseRuleIDs(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return []string{}
	}
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func init() {
	Default.Register("security.crowdsec.appsec.events", appsecEventsHandler)
}
