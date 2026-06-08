package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// parseEnvFile must round-trip what panel-api's buildEnvFile writes:
// one KEY=VALUE per line, no quoting, value is everything after the first '='.
// This is the panel↔agent wire contract for preserving install secrets on
// re-render — a drift here silently regenerates secrets.
func TestParseEnvFile_KeyValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	// Includes a value containing '=' (base64url secrets can't, but DSNs and
	// connection strings can), an empty line, and a comment.
	content := "MASTERKEY=x0123456789abcdef\n" +
		"POSTGRES_PASSWORD=p@ss=word/with=eq\n" +
		"\n" +
		"# a comment\n" +
		"EMPTY=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	env, err := parseEnvFile(path)
	if err != nil {
		t.Fatalf("parseEnvFile: %v", err)
	}
	cases := map[string]string{
		"MASTERKEY":         "x0123456789abcdef",
		"POSTGRES_PASSWORD": "p@ss=word/with=eq", // only the FIRST '=' splits
		"EMPTY":             "",
	}
	for k, want := range cases {
		if got, ok := env[k]; !ok || got != want {
			t.Errorf("env[%q] = %q (present=%v), want %q", k, got, ok, want)
		}
	}
	if _, ok := env["# a comment"]; ok {
		t.Error("comment line was parsed as a key")
	}
	if len(env) != 3 {
		t.Errorf("parsed %d keys, want 3: %v", len(env), env)
	}
}

func TestParseEnvFile_Missing(t *testing.T) {
	_, err := parseEnvFile(filepath.Join(t.TempDir(), "nope.env"))
	if !os.IsNotExist(err) {
		t.Errorf("want os.IsNotExist, got %v", err)
	}
}
