package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// systemAptCheckResponse is the wire shape for system.apt_check.
type systemAptCheckResponse struct {
	Packages       []aptUpgradablePackage `json:"packages"`
	Total          int                    `json:"total"`
	SecurityTotal  int                    `json:"security_total"`
	InstalledTotal int                    `json:"installed_total"`
}

type aptUpgradablePackage struct {
	Name    string `json:"name"`
	Current string `json:"current"`
	New     string `json:"new"`
	Source  string `json:"source"`
	// Security is true when the upgrade candidate comes from a *-security
	// suite (e.g. "stable-security"). Coarse severity, zero extra apt calls.
	Security bool `json:"security"`
}

// systemAptCheckHandler runs `apt-get update` then `apt list --upgradable`
// and parses the column-stable output. Every apt invocation includes
// `-o DPkg::Lock::Timeout=60` because unattended-upgrades.timer runs
// nightly and holds the dpkg lock; without the timeout, ops see a
// cryptic crash. LC_ALL=C pins the column header locale.
//
// This command takes NO user-controlled parameters — apt is invoked with
// fixed args only. Any future params should be type-tagged enums, not
// passed through as strings.
func systemAptCheckHandler(ctx context.Context, _ json.RawMessage) (any, error) {
	if err := runApt(ctx, "update", "-qq"); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("apt-get update: %v", err)}
	}
	out, err := aptList(ctx)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("apt list: %v", err)}
	}
	pkgs := parseAptUpgradable(string(out))
	sec := 0
	for _, p := range pkgs {
		if p.Security {
			sec++
		}
	}
	return systemAptCheckResponse{
		Packages:       pkgs,
		Total:          len(pkgs),
		SecurityTotal:  sec,
		InstalledTotal: countInstalledPackages(ctx),
	}, nil
}

// countInstalledPackages returns the number of installed dpkg packages, for
// the "X of N packages" stat. Best-effort: 0 on error (the UI just omits it).
func countInstalledPackages(ctx context.Context) int {
	out, err := exec.CommandContext(ctx, "dpkg-query", "-f", "${db:Status-Abbrev}\n", "-W").Output()
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "ii") {
			n++
		}
	}
	return n
}

func runApt(ctx context.Context, args ...string) error {
	full := append([]string{"-o", "DPkg::Lock::Timeout=60"}, args...)
	cmd := exec.CommandContext(ctx, "apt-get", full...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func aptList(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "apt", "list", "--upgradable")
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// parseAptUpgradable reads `apt list --upgradable` output, format example:
//
//	Listing...
//	curl/stable 8.5.0-2 amd64 [upgradable from: 8.4.0-2]
//	libc6/stable 2.36-9+deb12u4 amd64 [upgradable from: 2.36-9+deb12u3]
var aptListLine = regexp.MustCompile(`^([A-Za-z0-9.+\-]+)/(\S+)\s+(\S+)\s+(\S+)\s+\[upgradable from:\s*(\S+?)\]$`)

func parseAptUpgradable(out string) []aptUpgradablePackage {
	var pkgs []aptUpgradablePackage
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Listing") || strings.HasPrefix(line, "WARNING:") {
			continue
		}
		m := aptListLine.FindStringSubmatch(line)
		if len(m) != 6 {
			continue
		}
		pkgs = append(pkgs, aptUpgradablePackage{
			Name:     m[1],
			Source:   m[2],
			New:      m[3],
			Current:  m[5],
			Security: strings.Contains(strings.ToLower(m[2]), "security"),
		})
	}
	return pkgs
}

func init() {
	Default.Register("system.apt_check", systemAptCheckHandler)
}
