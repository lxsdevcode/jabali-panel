package migrate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// GH #647/#648: ExtractTarGz must contain an UNTRUSTED source tarball — no
// path-traversal write, no symlink write-through — while extracting legit files.
func TestExtractTarGz_Containment(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	writeFile := func(name, body string) {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	}
	writeFile("wp-config.php", "<?php define('DB_NAME','x');")
	writeFile("wp-content/themes/t/style.css", "body{}")
	writeFile("../escape.txt", "PWNED")             // traversal
	writeFile("a/../../escape2.txt", "PWNED")       // traversal via ..
	writeFile("/abs.txt", "PWNED")                  // absolute
	_ = tw.WriteHeader(&tar.Header{Name: "evil-link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"})
	_ = tw.Close()
	_ = gz.Close()

	dir := t.TempDir()
	src := filepath.Join(dir, "in.tar.gz")
	if err := os.WriteFile(src, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}

	// Legit files landed under dest.
	if b, err := os.ReadFile(filepath.Join(dest, "wp-config.php")); err != nil || string(b) == "" {
		t.Errorf("wp-config.php not extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "wp-content/themes/t/style.css")); err != nil {
		t.Errorf("nested file not extracted: %v", err)
	}
	// Nothing escaped dest.
	for _, escaped := range []string{
		filepath.Join(dir, "escape.txt"),
		filepath.Join(dir, "escape2.txt"),
		filepath.Join(dir, "abs.txt"),
		"/abs.txt",
	} {
		if _, err := os.Stat(escaped); err == nil {
			t.Errorf("path traversal wrote %s", escaped)
		}
	}
	// The symlink was NOT created (no write-through).
	if _, err := os.Lstat(filepath.Join(dest, "evil-link")); err == nil {
		t.Error("symlink entry was created (write-through risk)")
	}
}

// Env-gated: extract a REAL WordPress tarball and confirm the tree lands.
// Skipped in CI. JABALI_EXTRACT_REAL=<tarball path>. GH #647.
func TestExtractTarGz_RealTarball(t *testing.T) {
	src := os.Getenv("JABALI_EXTRACT_REAL")
	if src == "" {
		t.Skip("set JABALI_EXTRACT_REAL=<tarball> to run")
	}
	dest := t.TempDir()
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatalf("ExtractTarGz(real): %v", err)
	}
	for _, want := range []string{"wp-config.php", "wp-includes/version.php", "wp-load.php"} {
		if _, err := os.Stat(filepath.Join(dest, want)); err != nil {
			t.Errorf("real WP tree missing %s: %v", want, err)
		}
	}
	// count extracted files
	n := 0
	filepath.Walk(dest, func(_ string, fi os.FileInfo, _ error) error {
		if fi != nil && fi.Mode().IsRegular() {
			n++
		}
		return nil
	})
	t.Logf("extracted %d regular files under dest", n)
	if n < 100 {
		t.Errorf("suspiciously few files: %d", n)
	}
}
