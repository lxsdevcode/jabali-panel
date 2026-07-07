package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/agentwire"
)

// migration.http_probe — post-migration health check. For each migrated
// domain, fetch the homepage THROUGH THE LOCAL nginx (curl --resolve pins
// the name to 127.0.0.1) so the real vhost + the user's PHP-FPM pool run.
// A migrated WP site that survived on the source via a persistent object
// cache can fatal on jabali (no object cache) over malformed option data —
// this surfaces it (a 500) at migration time instead of from the customer.
//
// Retries 502/503/connection-refused a few times because the per-user FPM
// pool converges asynchronously just after restore; a genuine app crash
// (500) is reported immediately.

type migrationHTTPProbeParams struct {
	Domains []string `json:"domains"`
}

type httpProbeResult struct {
	Domain string `json:"domain"`
	Status int    `json:"status"`
	OK     bool   `json:"ok"`
	Note   string `json:"note,omitempty"`
}

type migrationHTTPProbeResponse struct {
	Results []httpProbeResult `json:"results"`
}

// probeDomainRe bounds what reaches curl's --resolve / URL args.
var probeDomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}\.)+[a-z]{2,63}$`)

// probeTargetIP is the IP curl --resolve pins the migrated domain to so the
// request hits THIS box's nginx (the domain's real DNS may still point at
// the source server mid-migration). It is the box's primary IPv4 — nginx
// vhosts bind a managed IP (M24), not 127.0.0.1, so resolving to loopback
// gets connection-refused. Computed once.
var probeTargetIP = func() string {
	out, err := exec.Command("ip", "-4", "route", "get", "1.1.1.1").Output()
	if err == nil {
		// "1.1.1.1 via X dev Y src <IP> ..." — take the token after "src".
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "src" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	// JAB-31 follow-up: a box with no default route to 1.1.1.1 (isolated test
	// VM) previously fell through to 127.0.0.1, which every managed-IP vhost
	// (M24 `listen <IP>:443`) refuses → false connection-refused. Fall back to
	// the first non-loopback global IPv4 instead.
	if hi, herr := exec.Command("hostname", "-I").Output(); herr == nil {
		for _, tok := range strings.Fields(string(hi)) {
			if strings.Contains(tok, ".") && !strings.HasPrefix(tok, "127.") {
				return tok
			}
		}
	}
	return "127.0.0.1"
}()

// nginxSitesDir is a var so tests can point vhostListenIP at a fixture dir.
var nginxSitesDir = "/etc/nginx/sites-available"

// vhostListenRe extracts the IPv4 from a rendered `listen <IP>:443` directive.
var vhostListenRe = regexp.MustCompile(`(?m)^\s*listen\s+(\d{1,3}(?:\.\d{1,3}){3}):443\b`)

// vhostListenIP returns the specific IPv4 a domain's rendered nginx vhost binds
// (M24 per-domain listen IP), or "" when it listens on all addresses. The probe
// must curl --resolve to THIS ip: a vhost bound to a managed IP refuses a
// connection to any other address, which produced false 0-status health
// failures inside a `done` migration (JAB-31 follow-up).
func vhostListenIP(domain string) string {
	b, err := os.ReadFile(filepath.Join(nginxSitesDir, domain+".conf"))
	if err != nil {
		return ""
	}
	if m := vhostListenRe.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

var runProbeCurl = func(ctx context.Context, domain, ip string) int {
	args := []string{
		"-s", "-o", "/dev/null", "-w", "%{http_code}",
		"-k", "-L", "--max-time", "15",
		"--resolve", domain + ":443:" + ip,
		"--resolve", domain + ":80:" + ip,
		"https://" + domain + "/",
	}
	out, _ := exec.CommandContext(ctx, "curl", args...).Output()
	code, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return code // 0 on connection failure / timeout
}

var probeSleep = func() { time.Sleep(5 * time.Second) }

func probeOneDomain(ctx context.Context, domain string) httpProbeResult {
	// Resolve to the domain's actual vhost listen IP; fall back to the box's
	// primary IPv4 when the vhost listens on all addresses (JAB-31 follow-up).
	ip := vhostListenIP(domain)
	if ip == "" {
		ip = probeTargetIP
	}
	var code int
	for attempt := 0; attempt < 3; attempt++ {
		code = runProbeCurl(ctx, domain, ip)
		// 0 = couldn't connect; 502/503 = upstream (FPM) not up yet.
		// Those are transient just after restore — give the pool time.
		if code != 0 && code != 502 && code != 503 {
			break
		}
		if attempt < 2 {
			probeSleep()
		}
	}
	res := httpProbeResult{Domain: domain, Status: code, OK: code >= 200 && code < 400}
	switch {
	case code == 0:
		res.Note = "no response (connection refused / timeout)"
	case code >= 500:
		res.Note = "server error — likely a crashing app (e.g. WP fatal on imported data)"
	case code >= 400:
		res.Note = "client error"
	}
	return res
}

func migrationHTTPProbeHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p migrationHTTPProbeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	results := make([]httpProbeResult, 0, len(p.Domains))
	for _, d := range p.Domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if !probeDomainRe.MatchString(d) {
			results = append(results, httpProbeResult{Domain: d, Status: 0, OK: false, Note: "skipped: invalid domain"})
			continue
		}
		results = append(results, probeOneDomain(ctx, d))
	}
	return migrationHTTPProbeResponse{Results: results}, nil
}

func init() {
	Default.Register("migration.http_probe", migrationHTTPProbeHandler)
}
