package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	// mainFuncRe locates the start of main() in install.sh.
	mainFuncRe = regexp.MustCompile(`(?m)^main\(\) \{`)

	// installCallRe matches a call inside main(), with or without the
	// run_if_module <module> prefix, and captures the function name.
	installCallRe = regexp.MustCompile(`(?m)^[[:space:]]+(?:run_if_module [a-z_]+ )?([a-z_][a-z0-9_]*)[[:space:]]*$`)

	// repoInstallRefRe matches a read of the repo's install/ tree, which does
	// not exist until clone_or_update_repo has run.
	repoInstallRefRe = regexp.MustCompile(`\$\{?REPO_DIR\}?/install/`)
)

// TestNoRepoDirReadsBeforeClone pins an ordering rule that install.sh states in
// a comment but had no enforcement behind it:
//
//	install_apparmor and install_aide run AFTER clone_or_update_repo because
//	both functions source profile/unit files from $REPO_DIR/install/; on a
//	fresh install that directory does not exist until the repo is cloned.
//
// Four other functions read the same tree and were called before the clone.
// Each guards its sources with a `_warn` and `return 0`, so a fresh install
// finished green having silently shipped none of them. Confirmed absent on a
// host installed from a release build:
//
//	/etc/logrotate.d/jabali                            (panel, agent and
//	                                                    bulwark logs never
//	                                                    rotate)
//	/etc/systemd/system/jabali-notify@.service         (8 OnFailure drop-ins
//	                                                    reference it; systemd
//	                                                    logged 21 "Unit not
//	                                                    found" errors in one
//	                                                    boot, so no
//	                                                    service-down
//	                                                    notification can fire)
//	/usr/local/bin/jabali-notify-onfailure
//	/etc/crowdsec/acquis.d/jabali-panel.yaml           (panel-api's own logs
//	                                                    are never ingested, so
//	                                                    the JAB-106
//	                                                    kratos_flow_rate_limited
//	                                                    marker is unreachable)
//	/etc/crowdsec/parsers/s01-parse/jabali-panel-kratos.yaml
//	  plus the whole install/crowdsec/stalwart/ tree
//
// A commit already exists that re-runs two of these during `jabali update`,
// which repairs an upgraded host and leaves a fresh one broken. This asserts
// the ordering instead, so the gap cannot reappear via a newly added call.
func TestNoRepoDirReadsBeforeClone(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	script := string(raw)

	loc := mainFuncRe.FindStringIndex(script)
	if loc == nil {
		t.Fatal("could not find main() in install.sh")
	}
	body := script[loc[1]:]

	// Everything in main() up to the clone runs without $REPO_DIR/install/.
	cloneIdx := strings.Index(body, "\n  clone_or_update_repo\n")
	if cloneIdx < 0 {
		t.Fatal("could not find the clone_or_update_repo call in main() — " +
			"if it was renamed this test needs updating, and the ordering rule " +
			"it protects still applies")
	}
	preClone := body[:cloneIdx]
	postClone := body[cloneIdx:]

	// A function called on both sides of the clone is fine: the pre-clone call
	// does whatever does not need the repo, and the post-clone call fills in
	// the rest. install_crowdsec_appsec is deliberately built this way — the
	// early call installs the binary, the later one renders the acquis that
	// lives in $REPO_DIR (GH #109). Its artifacts are present on a fresh host,
	// which is the evidence that the second call really does heal it.
	calledAfterClone := map[string]bool{}
	for _, m := range installCallRe.FindAllStringSubmatch(postClone, -1) {
		calledAfterClone[m[1]] = true
	}

	bodies := installShFunctionBodies(script)

	seen := map[string]bool{}
	checked := 0
	for _, m := range installCallRe.FindAllStringSubmatch(preClone, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		if calledAfterClone[name] {
			continue
		}
		fnBody, ok := bodies[name]
		if !ok {
			// Not a function defined in install.sh (a builtin, a variable
			// assignment that looks like a call, etc.).
			continue
		}
		checked++
		if repoInstallRefRe.MatchString(fnBody) {
			t.Errorf("%s() is called before clone_or_update_repo but reads "+
				"$REPO_DIR/install/. On a fresh install that path does not "+
				"exist yet, and the function warns and returns 0 rather than "+
				"failing — so the install completes green with the files "+
				"missing. Move the call to after clone_or_update_repo.", name)
		}
	}

	if checked == 0 {
		t.Fatal("resolved no install.sh functions from main()'s pre-clone " +
			"section — the call or definition pattern has drifted and this " +
			"test is no longer checking anything")
	}
}

// installShFunctionBodies maps every top-level `name() {` in install.sh to its
// body, ending at the first line that is exactly "}". install.sh consistently
// closes top-level functions in column 0, which makes that terminator reliable
// without needing to balance braces.
func installShFunctionBodies(script string) map[string]string {
	defRe := regexp.MustCompile(`(?m)^([a-z_][a-z0-9_]*)\(\)[[:space:]]*\{$`)
	out := map[string]string{}
	for _, m := range defRe.FindAllStringSubmatchIndex(script, -1) {
		name := script[m[2]:m[3]]
		rest := script[m[1]:]
		if end := strings.Index(rest, "\n}"); end >= 0 {
			out[name] = rest[:end]
		} else {
			out[name] = rest
		}
	}
	return out
}
