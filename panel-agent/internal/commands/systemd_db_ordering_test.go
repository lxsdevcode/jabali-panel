package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// jabaliCLIExecRe matches an ExecStart that shells out to the jabali CLI.
// Every such unit opens its own MariaDB connection during startup, which is
// what makes boot ordering load-bearing for it.
var jabaliCLIExecRe = regexp.MustCompile(`(?m)^ExecStart=.*?/usr/local/bin/jabali\s`)

var afterLineRe = regexp.MustCompile(`(?m)^After=(.*)$`)

// TestCLIUnitsAreOrderedAfterMariaDB guards a boot race that is invisible on a
// fast host and reliable on a slow one.
//
// `After=jabali-panel.service` looks like it implies the database is up, and it
// does not: these units run /usr/local/bin/jabali, which dials
// /var/run/mysqld/mysqld.sock itself. On a 2 GB / 2 vCPU host mariadb took 44
// seconds to accept connections, and jabali-backup-retention.service — fired at
// boot by its own Persistent=true catch-up — died with
//
//	open db: open mariadb: dial unix /var/run/mysqld/mysqld.sock:
//	connect: no such file or directory
//
// It carries Restart=no, so that run was simply lost. A host powered off
// overnight can therefore miss retention indefinitely while showing nothing
// worse than one failed unit per boot.
//
// The check is written against the whole directory rather than a fixed list so
// that a unit added later cannot quietly reintroduce it.
func TestCLIUnitsAreOrderedAfterMariaDB(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	dir := filepath.Join(root, "install", "systemd")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".service") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		text := string(body)
		if !jabaliCLIExecRe.MatchString(text) {
			continue
		}
		checked++

		m := afterLineRe.FindStringSubmatch(text)
		if m == nil {
			t.Errorf("%s runs the jabali CLI but has no After= line at all; "+
				"it needs After=mariadb.service or it will race the database at boot",
				e.Name())
			continue
		}
		if !strings.Contains(m[1], "mariadb.service") {
			t.Errorf("%s runs the jabali CLI but is only ordered %q. "+
				"The CLI opens its own DB connection, so ordering after "+
				"jabali-panel.service does not imply mysqld.sock exists — "+
				"add mariadb.service to After=.", e.Name(), strings.TrimSpace(m[1]))
		}
	}

	// A rename or a path change that stops matching jabaliCLIExecRe would
	// otherwise turn this into a test that silently checks nothing.
	if checked == 0 {
		t.Fatal("matched no units invoking /usr/local/bin/jabali — the ExecStart " +
			"pattern has drifted and this test is no longer checking anything")
	}
}

// TestPanelUnitOrdersAfterMariaDB covers the same race for panel-api itself,
// whose unit is generated inline by install.sh rather than shipped as a file.
//
// main() pings the database and exits(1) on failure, so without the ordering
// panel-api hard-crashes on every boot of a slow host —
//
//	Error: ping: dial unix /var/run/mysqld/mysqld.sock: no such file
//
// — and comes back only because Restart=on-failure catches it. That is
// recoverable, which is precisely why it can sit unnoticed for a long time.
func TestPanelUnitOrdersAfterMariaDB(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	sh, err := os.ReadFile(filepath.Join(root, "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	idx := strings.Index(string(sh), "Description=Jabali Panel API")
	if idx < 0 {
		t.Fatal("could not find the panel unit block in install.sh — " +
			"the Description= line moved and this test needs updating")
	}
	// The [Unit] section ends at the next section header.
	rest := string(sh)[idx:]
	if end := strings.Index(rest, "[Service]"); end > 0 {
		rest = rest[:end]
	}

	m := afterLineRe.FindStringSubmatch(rest)
	if m == nil {
		t.Fatal("panel unit has no After= line")
	}
	if !strings.Contains(m[1], "mariadb.service") {
		t.Errorf("panel unit is ordered %q with no mariadb.service; "+
			"panel-api pings the DB at startup and exits nonzero if it is not "+
			"reachable, so it will crash once on every boot of a host where "+
			"mariadb is slow to come up", strings.TrimSpace(m[1]))
	}
}
