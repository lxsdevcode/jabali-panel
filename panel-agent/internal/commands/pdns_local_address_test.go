package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// helperPath is the shipped script that pdns.service runs as ExecStartPre.
const helperPath = "install/systemd/pdns-local-address"

func repoRootT(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

// TestPDNSLocalAddressDropInRunsPrivileged pins the '+' on the ExecStartPre
// line, which is easy to lose in a reformat and expensive to lose in
// production.
//
// Debian's pdns.service runs User=pdns with ProtectSystem=full. Without the
// prefix the helper runs unprivileged against a read-only /etc, exits 1, and
// systemd refuses to start the service at all — so a change meant to keep DNS
// up after an address change instead becomes a second, unconditional way to
// take it down. Confirmed on a live host: plain ExecStartPre left pdns in
// activating with "Control process exited, code=exited, status=1/FAILURE";
// adding '+' brought it to active on the same config.
func TestPDNSLocalAddressDropInRunsPrivileged(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(repoRootT(t), "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	script := string(raw)

	// Scope to the installer function: the drop-in interpolates $dst rather
	// than spelling the helper path out, so matching on the path would silently
	// find nothing.
	const fn = "install_pdns_local_address_helper() {"
	start := strings.Index(script, fn)
	if start < 0 {
		t.Fatalf("could not find %s in install.sh", fn)
	}
	body := script[start:]
	if end := strings.Index(body, "\n}\n"); end > 0 {
		body = body[:end]
	}

	idx := strings.Index(body, "ExecStartPre=")
	if idx < 0 {
		t.Fatal("install_pdns_local_address_helper writes no ExecStartPre line")
	}
	line := body[idx:]
	if end := strings.IndexByte(line, '\n'); end > 0 {
		line = line[:end]
	}
	if !strings.HasPrefix(line, "ExecStartPre=+") {
		t.Errorf("pdns drop-in has %q; it needs the '+' prefix. "+
			"pdns.service runs User=pdns with ProtectSystem=full, so without it "+
			"the helper cannot write /etc/powerdns and its nonzero exit blocks "+
			"pdns from starting at all.", line)
	}
}

// TestPDNSLocalAddressHelperIsExecutableAndValid keeps the shipped helper
// syntactically sound and executable — install.sh copies it with mode 0755 and
// systemd execs it directly, so a syntax error surfaces as pdns failing to
// start rather than as anything resembling a script problem.
func TestPDNSLocalAddressHelperIsExecutableAndValid(t *testing.T) {
	t.Parallel()

	p := filepath.Join(repoRootT(t), helperPath)
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat %s: %v", helperPath, err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Errorf("%s is not executable (mode %v)", helperPath, fi.Mode())
	}
	if out, err := exec.Command("bash", "-n", p).CombinedOutput(); err != nil {
		t.Errorf("bash -n %s failed: %v\n%s", helperPath, err, out)
	}
}

// TestPDNSLocalAddressHelperRewritesStaleAddress drives the helper against a
// throwaway config to pin the behaviour that matters: a local-address= naming
// an address this host does not have is replaced, and loopback:5300 — the port
// pdns-recursor forwards into (ADR-0047) — survives.
//
// The original failure was a host restored from an image onto a new address.
// pdns cannot bind the old one, treats that as fatal, and burns its restart
// limit: 27 attempts, then authoritative DNS stays down until an operator
// re-runs install.sh by hand.
func TestPDNSLocalAddressHelperRewritesStaleAddress(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("ip"); err != nil {
		t.Skip("no `ip` binary; helper enumerates interfaces via iproute2")
	}

	dir := t.TempDir()
	conf := filepath.Join(dir, "01-jabali-mysql.conf")
	const stale = "local-address=127.0.0.1:5300,203.0.113.99:53,[::1]:5300"
	body := "launch=gmysql\n" + stale + "\ngmysql-dnssec=yes\n"
	if err := os.WriteFile(conf, []byte(body), 0o640); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cmd := exec.Command("bash", filepath.Join(repoRootT(t), helperPath))
	cmd.Env = append(os.Environ(), "JABALI_PDNS_CONF="+conf)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper failed: %v\n%s", err, out)
	}

	got, err := os.ReadFile(conf)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	text := string(got)

	if strings.Contains(text, "203.0.113.99") {
		t.Errorf("stale address survived the rewrite:\n%s", text)
	}
	if !strings.Contains(text, "127.0.0.1:5300") || !strings.Contains(text, "[::1]:5300") {
		t.Errorf("loopback:5300 must survive — pdns-recursor forwards into it "+
			"(ADR-0047). got:\n%s", text)
	}
	// Everything else in the file must be untouched; the helper edits one line.
	for _, keep := range []string{"launch=gmysql", "gmysql-dnssec=yes"} {
		if !strings.Contains(text, keep) {
			t.Errorf("helper dropped unrelated line %q:\n%s", keep, text)
		}
	}
	if n := strings.Count(text, "local-address="); n != 1 {
		t.Errorf("expected exactly one local-address= line, got %d:\n%s", n, text)
	}
}

// TestPDNSLocalAddressHelperLeavesUnmanagedHostsAlone covers the case where a
// host does not run the jabali PowerDNS layout at all: the helper must be a
// no-op rather than inventing a config file.
func TestPDNSLocalAddressHelperLeavesUnmanagedHostsAlone(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "absent.conf")
	cmd := exec.Command("bash", filepath.Join(repoRootT(t), helperPath))
	cmd.Env = append(os.Environ(), "JABALI_PDNS_CONF="+missing)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper should exit 0 when the config is absent: %v\n%s", err, out)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("helper created %s; it must not fabricate a config", missing)
	}
}
