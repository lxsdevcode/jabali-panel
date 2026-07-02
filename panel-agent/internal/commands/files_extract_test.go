package commands

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/filesafe"
)

func testExtractor(dir string) *extractor {
	sc := filesafe.NewScopeForTest("u", "u", dir)
	ed, err := sc.NewExtractDir(dir, os.Getuid(), os.Getgid())
	if err != nil {
		panic(err)
	}
	return &extractor{ed: ed}
}

func TestSafeDestRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	ex := testExtractor(dir)

	// Normal entries stay inside.
	for _, name := range []string{"a.txt", "sub/b.txt", "./c.txt"} {
		got, err := ex.safeDest(name)
		if err != nil {
			t.Errorf("%q should be safe, got %v", name, err)
		}
		// safeDest now returns a cleaned RELATIVE path under destDir.
		if filepath.IsAbs(got) || got == ".." || strings.HasPrefix(got, "../") {
			t.Errorf("%q should be a safe relative path, got %q", name, got)
		}
	}

	// Escapes (traversal AND absolute paths) must be rejected.
	for _, name := range []string{"../evil.txt", "../../etc/passwd", "a/../../b", `..\..\win.txt`, "/etc/passwd"} {
		if _, err := ex.safeDest(name); err == nil {
			t.Errorf("%q must be rejected", name)
		}
	}
}

func TestUnzipExtractsAndRejectsZipSlip(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "dest")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	// Good archive.
	good := filepath.Join(work, "good.zip")
	writeZip(t, good, map[string]string{
		"hello.txt":     "hi",
		"sub/world.txt": "earth",
	})
	ex := testExtractor(dest)
	goodF, err := os.Open(good)
	if err != nil {
		t.Fatal(err)
	}
	defer goodF.Close()
	if err := ex.unzip(goodF); err != nil {
		t.Fatalf("good zip failed: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "hello.txt")); string(b) != "hi" {
		t.Errorf("hello.txt not extracted")
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "sub/world.txt")); string(b) != "earth" {
		t.Errorf("sub/world.txt not extracted")
	}
	if ex.extracted != 2 {
		t.Errorf("expected 2 files, got %d", ex.extracted)
	}

	// Zip-slip archive must error AND not write outside dest.
	evil := filepath.Join(work, "evil.zip")
	writeZip(t, evil, map[string]string{"../pwned.txt": "owned"})
	ex2 := testExtractor(dest)
	evilF, err := os.Open(evil)
	if err != nil {
		t.Fatal(err)
	}
	defer evilF.Close()
	if err := ex2.unzip(evilF); err == nil {
		t.Errorf("zip-slip entry must be rejected")
	}
	if _, err := os.Stat(filepath.Join(work, "pwned.txt")); err == nil {
		t.Fatalf("zip-slip wrote outside the destination!")
	}
}

func TestUntarSkipsSymlinks(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "d")
	os.Mkdir(dest, 0o755)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	// regular file
	body := []byte("data")
	tw.WriteHeader(&tar.Header{Name: "ok.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	tw.Write(body)
	// symlink pointing outside — must be skipped, not created
	tw.WriteHeader(&tar.Header{Name: "evil-link", Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink})
	tw.Close()

	ex := testExtractor(dest)
	if err := ex.untar(&buf); err != nil {
		t.Fatalf("untar failed: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "ok.txt")); string(b) != "data" {
		t.Errorf("regular file not extracted")
	}
	if _, err := os.Lstat(filepath.Join(dest, "evil-link")); err == nil {
		t.Errorf("symlink entry must be skipped, not created")
	}
	if ex.extracted != 1 || ex.skipped != 1 {
		t.Errorf("expected extracted=1 skipped=1, got extracted=%d skipped=%d", ex.extracted, ex.skipped)
	}
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// Gitea #423 (write side, reopened): a tenant pre-plants a SYMLINK directory
// component inside their own (tenant-owned) destDir pointing out of scope, then
// extracts an archive whose regular-file entry lands under it. The per-entry
// write must be refused (skipped), never followed — root must not write through
// the planted parent symlink.
func TestUnzip_RejectsWriteThroughPlantedParentSymlink(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "dest")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(work, "victim")
	if err := os.Mkdir(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	// Tenant-planted symlink dir inside their own destDir -> outside.
	if err := os.Symlink(victim, filepath.Join(dest, "sub")); err != nil {
		t.Fatal(err)
	}

	z := filepath.Join(work, "evil.zip")
	writeZip(t, z, map[string]string{"sub/pwn": "owned"})
	ex := testExtractor(dest)
	zf, err := os.Open(z)
	if err != nil {
		t.Fatal(err)
	}
	defer zf.Close()
	if err := ex.unzip(zf); err != nil {
		t.Fatalf("unzip should skip the unsafe entry, not hard-error: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(victim, "pwn")); err == nil {
		t.Fatalf("root wrote through the planted parent symlink into the victim — ESCAPE")
	}
	if ex.skipped == 0 {
		t.Errorf("the unsafe entry should have been counted as skipped")
	}
}
