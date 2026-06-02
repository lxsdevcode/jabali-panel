package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyTarballChecksum_OK(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "data.tar.gz")
	body := []byte("hello world")
	if err := os.WriteFile(tarPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	hexSum := hex.EncodeToString(sum[:])
	sumPath := tarPath + ".sha256"
	if err := os.WriteFile(sumPath, []byte(hexSum+"  data.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyTarballChecksum(tarPath, sumPath); err != nil {
		t.Errorf("expected OK, got %v", err)
	}
}

func TestVerifyTarballChecksum_Mismatch(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "data.tar.gz")
	if err := os.WriteFile(tarPath, []byte("real content"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrong := sha256.Sum256([]byte("different bytes"))
	sumPath := tarPath + ".sha256"
	if err := os.WriteFile(sumPath, []byte(hex.EncodeToString(wrong[:])+"  data.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyTarballChecksum(tarPath, sumPath)
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Errorf("want sha256 mismatch, got %v", err)
	}
}

func TestVerifyTarballChecksum_MalformedSumFile(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "data.tar.gz")
	if err := os.WriteFile(tarPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sumPath := tarPath + ".sha256"
	if err := os.WriteFile(sumPath, []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyTarballChecksum(tarPath, sumPath)
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Errorf("want malformed error, got %v", err)
	}
}

func TestParseManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "MANIFEST")
	body := `short_sha=abc1234
full_sha=abc1234deadbeef
build_time=2026-06-02T00:00:00Z
go_version=go1.24.1
# a comment line
node_version=v20.10.0
release_format=v1
`
	if err := os.WriteFile(manifest, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := parseManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if got["short_sha"] != "abc1234" {
		t.Errorf("short_sha = %q, want abc1234", got["short_sha"])
	}
	if got["full_sha"] != "abc1234deadbeef" {
		t.Errorf("full_sha = %q", got["full_sha"])
	}
	if got["go_version"] != "go1.24.1" {
		t.Errorf("go_version = %q", got["go_version"])
	}
	// Comment must be ignored.
	if _, ok := got["# a comment line"]; ok {
		t.Error("comment line should not appear as key")
	}
}

func TestPickAssetURL(t *testing.T) {
	rel := &releaseResponse{
		TagName: "release-abc1234",
		Assets: []releaseAsset{
			{Name: "jabali-release-abc1234.tar.gz", BrowserDownloadURL: "https://x/tar"},
			{Name: "jabali-release-abc1234.tar.gz.sha256", BrowserDownloadURL: "https://x/sum"},
		},
	}
	if got := pickAssetURL(rel, "jabali-release-abc1234.tar.gz"); got != "https://x/tar" {
		t.Errorf("tar URL = %q", got)
	}
	if got := pickAssetURL(rel, "missing.bin"); got != "" {
		t.Errorf("missing asset should return empty string, got %q", got)
	}
}

func TestFetchLatestMainSHA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/branches/main") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"commit":{"id":"abc1234deadbeefcafebabe1234567890abcdef12"}}`))
	}))
	defer srv.Close()
	sha, err := fetchLatestMainSHA(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if sha != "abc1234deadbeefcafebabe1234567890abcdef12" {
		t.Errorf("sha = %q", sha)
	}
}

func TestFetchReleaseByTag_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()
	_, err := fetchReleaseByTag(context.Background(), srv.URL, "release-deadbeef")
	if err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestExtractTarball(t *testing.T) {
	// Create a real tar.gz with one file, then extract via the
	// production code path that shells out to `tar`. Skip if tar
	// missing on the test host (unlikely in CI).
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not on PATH")
	}
	dir := t.TempDir()
	// Stage source.
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "bin/hello"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "MANIFEST"), []byte("short_sha=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(dir, "out.tar.gz")
	if err := exec.Command("tar", "-C", src, "-czf", tarPath, "bin", "MANIFEST").Run(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "extract")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarball(context.Background(), tarPath, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "bin/hello")); err != nil {
		t.Errorf("expected bin/hello extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "MANIFEST")); err != nil {
		t.Errorf("expected MANIFEST extracted: %v", err)
	}
}
