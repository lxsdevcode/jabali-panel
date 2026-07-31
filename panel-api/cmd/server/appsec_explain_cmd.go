package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newAppSecExplainCmd answers the question every AppSec false-positive report
// starts with and nothing could previously answer: WHICH rule blocked this?
//
// /var/log/crowdsec.log records only the outcome —
//
//	"crowdsecurity/appsec-native by ip 1.2.3.4 (IT/8075) : 4h ban"
//
// — no rule, no URI, no matched variable. Every FP in the JAB-193 cluster
// (195/196/197/198) stalled on that, and ADR-0124 documents the cost of
// guessing instead: the first triage excluded 901340, which scores nothing.
//
// The data was there the whole time, in `cscli alerts inspect -d` meta. This
// groups it by rule + URI so the pattern behind a complaint is visible, and an
// exclusion can be written against a rule that actually fired.
// crsInfraRules are the CRS rules that appear on almost every block but are
// never the thing to exclude. Naming them inline is the point of this command:
// a raw id list like "901340, 930130, 2546897341, 980170" gives no clue that
// only one of those actually scored.
//
//	901340 — enables request-body inspection. Scores nothing. ADR-0124 exists
//	         because this id was excluded in a first triage, to no effect.
//	949110 — the inbound anomaly threshold rule. It is what returns 403, but it
//	         is an effect of the score, not a detection. Excluding it disables
//	         blocking wholesale.
//	980170 — reports the final score. Bookkeeping.
var crsInfraRules = map[string]string{
	"901340": "body-inspection enabler — scores nothing, never exclude this",
	"949110": "anomaly threshold reached — the blocker, not a detection",
	"980170": "score reporting — bookkeeping",
}

// annotateRules splits an id list into the detections worth acting on and the
// infrastructure rules that ride along on every block.
func annotateRules(ids []string) (detections []string, infra []string) {
	for _, id := range ids {
		if note, ok := crsInfraRules[id]; ok {
			infra = append(infra, id+" ("+note+")")
			continue
		}
		detections = append(detections, id)
	}
	return detections, infra
}

// printInlineBlocks reports 403s that never became a ban.
//
// These are the ones that made JAB-193 hard: an inline denial creates no
// decision and no alert, so `cscli decisions list` and `cscli alerts list` are
// both empty for the affected IP and the operator concludes CrowdSec was not
// involved. crowdsec.log is the only record, and it carries less than an alert
// does — score families and source IP, no rule id and no URI.
//
// Kept in its own section, deliberately: presenting these next to the
// alert-backed rows above would imply a precision they do not have.
func printInlineBlocks(blocks []struct {
	Timestamp string `json:"timestamp"`
	SourceIP  string `json:"source_ip"`
	Scores    string `json:"scores"`
}) {
	if len(blocks) == 0 {
		return
	}
	byIP := map[string][]string{}
	order := []string{}
	for _, b := range blocks {
		if _, seen := byIP[b.SourceIP]; !seen {
			order = append(order, b.SourceIP)
		}
		byIP[b.SourceIP] = append(byIP[b.SourceIP], b.Scores)
	}

	fmt.Printf("\nInline 403s with no alert (%d): these never became a ban, so cscli\n", len(blocks))
	fmt.Println("shows nothing for them. crowdsec.log records no rule id and no URI.")
	for _, ip := range order {
		scores := byIP[ip]
		fmt.Printf("  %-42s %d block(s)   last: %s\n", ip, len(scores), scores[len(scores)-1])
	}
	fmt.Println("\nTo get the URI for these, correlate the IP and timestamp against the")
	fmt.Println("tenant's nginx access log (403 responses). The score family above tells")
	fmt.Println("you which CRS group to look in — sql_injection, rce, php_injection, lfi.")
}

func newAppSecExplainCmd() *cobra.Command {
	var limit int
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Show which CRS rules recently blocked requests (AppSec FP triage)",
		Long: `Groups recent CrowdSec AppSec inband blocks by rule and target URI.

Use this before writing a CRS exclusion. The jabali-before plugin
(ADR-0124) takes surgical ctl:ruleRemoveTargetById directives, and those
are only correct if aimed at the rule that actually scored.

Caution: the rule_name field reports only the FIRST matched id, which on a
real block is routinely 901340 — the CRS body-inspection enabler, which
contributes no score. Exclude that and nothing changes. The scoring rule is
usually another entry in the rule id list.`,
		PreRunE: requireAgent,
		RunE: func(c *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			raw, err := sharedAgent.Call(ctx, "security.crowdsec.appsec.events", map[string]any{
				"limit": limit,
			})
			if err != nil {
				return fmt.Errorf("query appsec events: %w", err)
			}

			var resp struct {
				Events []struct {
					Timestamp  string   `json:"timestamp"`
					SourceIP   string   `json:"source_ip"`
					TargetHost string   `json:"target_host"`
					TargetURI  string   `json:"target_uri"`
					Action     string   `json:"action"`
					RuleName   string   `json:"rule_name"`
					RuleIDs    []string `json:"rule_ids"`
					Country    string   `json:"country"`
					ASNOrg     string   `json:"asn_org"`
				} `json:"events"`
				InlineBlocks []struct {
					Timestamp string `json:"timestamp"`
					SourceIP  string `json:"source_ip"`
					Scores    string `json:"scores"`
				} `json:"inline_blocks"`
				AlertsScanned int `json:"alerts_scanned"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return fmt.Errorf("decode agent response: %w", err)
			}

			if jsonOut {
				out, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Println(string(out))
				return nil
			}

			if len(resp.Events) == 0 && len(resp.InlineBlocks) == 0 {
				fmt.Printf("No AppSec blocks found in the last %d alert(s).\n", resp.AlertsScanned)
				fmt.Println("If a user is reporting a 403, confirm AppSec is in block mode and that the")
				fmt.Println("request actually reached it — an nginx-level deny never becomes an alert.")
				return nil
			}

			type group struct {
				rules  string
				uri    string
				host   string
				count  int
				ips    map[string]bool
				sample string
			}
			groups := map[string]*group{}
			for _, e := range resp.Events {
				key := strings.Join(e.RuleIDs, ",") + "|" + e.TargetHost + "|" + e.TargetURI
				g, ok := groups[key]
				if !ok {
					g = &group{
						rules: strings.Join(e.RuleIDs, ", "),
						uri:   e.TargetURI, host: e.TargetHost,
						ips: map[string]bool{}, sample: e.Timestamp,
					}
					groups[key] = g
				}
				g.count++
				g.ips[e.SourceIP] = true
			}

			ordered := make([]*group, 0, len(groups))
			for _, g := range groups {
				ordered = append(ordered, g)
			}
			sort.Slice(ordered, func(i, j int) bool { return ordered[i].count > ordered[j].count })

			fmt.Printf("AppSec blocks across the last %d alert(s) — %d event(s), %d distinct pattern(s):\n\n",
				resp.AlertsScanned, len(resp.Events), len(ordered))
			for _, g := range ordered {
				det, infra := annotateRules(strings.Split(g.rules, ", "))
				fmt.Printf("  %d block(s), %d distinct source IP(s)\n", g.count, len(g.ips))
				if len(det) > 0 {
					fmt.Printf("    scored by: %s   <- exclude one of THESE\n", strings.Join(det, ", "))
				} else {
					fmt.Printf("    scored by: (none identified — only infrastructure rules matched)\n")
				}
				for _, i := range infra {
					fmt.Printf("    also     : %s\n", i)
				}
				fmt.Printf("    host     : %s\n", g.host)
				fmt.Printf("    uri      : %s\n", g.uri)
				fmt.Printf("    first at : %s\n\n", g.sample)
			}

			fmt.Println("Writing an exclusion:")
			fmt.Println("  - Ignore 901340 if present. It enables body inspection and scores nothing;")
			fmt.Println("    excluding it changes nothing (ADR-0124 was written after that mistake).")
			fmt.Println("  - Target the rule that scored, and scope it to the narrowest ARGS/URI that")
			fmt.Println("    reproduces — internal/appseccfg/appseccfg.go holds the jabali-before rules.")
			fmt.Println("  - Many distinct source IPs on one URI reads as a false positive.")
			fmt.Println("    One IP across many URIs usually reads as an actual attacker.")
			printInlineBlocks(resp.InlineBlocks)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "how many recent AppSec alerts to inspect (max 200)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit raw JSON instead of the grouped report")
	return cmd
}
