package hestiacp

import (
	"archive/tar"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeTar writes a tar of the given name→content map to path.
func writeTar(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, data := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// zstdCompress runs `zstd` to produce path+".zst"; skips the test if the CLI
// is unavailable (the parser itself shells out to zstd, so CI has it).
func zstdCompress(t *testing.T, path string) {
	t.Helper()
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd CLI not available")
	}
	if out, err := exec.Command("zstd", "-q", "-f", path).CombinedOutput(); err != nil {
		t.Fatalf("zstd %s: %v (%s)", path, err, out)
	}
}

// TestParseHestiaTarball_ModernLayout covers JAB-27 (db dump), JAB-28 (DNS zone),
// and JAB-30 (SSH authorized_keys in user_dir/.ssh.tar.zst) capture from a
// modern zstd-wrapped v-backup-user tar.
func TestParseHestiaTarball_ModernLayout(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd CLI not available")
	}
	stage := t.TempDir()

	// Nested web archive: web/itflow.app/domain_data.tar.zst -> public_html/index.php
	webTar := filepath.Join(stage, "domain_data.tar")
	writeTar(t, webTar, map[string][]byte{"public_html/index.php": []byte("<?php echo 'hi';")})
	zstdCompress(t, webTar)

	// Nested SSH archive: user_dir/.ssh.tar.zst -> .ssh/authorized_keys (JAB-30)
	sshTar := filepath.Join(stage, "ssh.tar")
	writeTar(t, sshTar, map[string][]byte{".ssh/authorized_keys": []byte("ssh-ed25519 AAAAC3Nz test@host\n")})
	zstdCompress(t, sshTar)

	// DB dump: db/itflowapp_test/itflowapp_test.mysql.sql.zst (JAB-27)
	dbSQL := filepath.Join(stage, "itflowapp_test.mysql.sql")
	if err := os.WriteFile(dbSQL, []byte("CREATE TABLE t(i INT);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	zstdCompress(t, dbSQL)

	dnsZone := []byte("$TTL 14400\nitflow.app. IN SOA ns1.itflow.app. admin.itflow.app. (1 7200 3600 1209600 86400)\nitflow.app. IN A 1.2.3.4\nwww IN CNAME itflow.app.\n")

	// Assemble the outer tar with the modern zstd layout.
	outer := filepath.Join(stage, "itflowapp.2026-07-05_16-10-38.tar")
	writeTar(t, outer, map[string][]byte{
		"web/itflow.app/domain_data.tar.zst":             mustRead(t, webTar+".zst"),
		"user_dir/.ssh.tar.zst":                          mustRead(t, sshTar+".zst"),
		"db/itflowapp_test/itflowapp_test.mysql.sql.zst": mustRead(t, dbSQL+".zst"),
		"dns/itflow.app/conf/itflow.app.db":              dnsZone,
		"dns/itflow.app/hestia/itflow.app.conf":          []byte("DOMAIN='itflow.app'\n"),
		"user.conf":                                      []byte("USER='itflowapp'\n"),
	})

	extractDir := filepath.Join(stage, "extract")
	got, err := ParseHestiaTarball(outer, extractDir)
	if err != nil {
		t.Fatalf("ParseHestiaTarball: %v", err)
	}

	// JAB-27: DB dump hydrated + recorded.
	if len(got.MySQLDumps) != 1 || !strings.HasSuffix(got.MySQLDumps[0], "itflowapp_test.mysql.sql") {
		t.Errorf("MySQLDumps = %v, want one *itflowapp_test.mysql.sql", got.MySQLDumps)
	}
	// JAB-28: DNS zone .db captured, hestia/*.conf ignored.
	if len(got.ZoneFiles) != 1 || !strings.HasSuffix(got.ZoneFiles[0], "dns/itflow.app/conf/itflow.app.db") {
		t.Errorf("ZoneFiles = %v, want one dns/itflow.app/conf/itflow.app.db", got.ZoneFiles)
	}
	// JAB-30: authorized_keys from user_dir/.ssh.tar.zst captured.
	if !strings.HasSuffix(got.SSHKeys, filepath.Join("user_dir", ".ssh", "authorized_keys")) {
		t.Errorf("SSHKeys = %q, want *user_dir/.ssh/authorized_keys", got.SSHKeys)
	}
	if _, err := os.Stat(got.SSHKeys); err != nil {
		t.Errorf("SSHKeys path does not exist: %v", err)
	}
	// Web domain discovered.
	if _, ok := got.DomainDirs["itflow.app"]; !ok {
		t.Errorf("DomainDirs missing itflow.app: %v", got.DomainDirs)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
