package commands

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

// JAB-191: content that is not valid UTF-8 must come back base64, not as a JSON
// string. encoding/json substitutes U+FFFD for every invalid byte, so the
// string path silently hands the caller a corrupted file with no error.
// A latin1 PHP/HTML file sniffs as text/plain, so MIME detection alone misses it.
func TestFilesRead_NonUTF8RoundTripsExactly(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot resolve current user")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}

	cases := []struct {
		name string
		body []byte
	}{
		{"latin1_text", []byte("caf\xe9 latin1\n")},
		{"latin1_php", []byte("<?php $x = \"\xe9\"; ?>\n")},
		{"stray_invalid_bytes", []byte("a\xffb\xfec\n")},
		{"valid_utf8_stays_text", []byte("café ünïcode\n")},
		{"ascii", []byte("plain ascii\n")},
		{"empty", []byte("")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			// The handler scopes reads to /home/<user>; skip when TempDir is elsewhere.
			if rel, rerr := filepath.Rel(home, dir); rerr != nil || len(rel) > 2 && rel[:2] == ".." {
				t.Skip("temp dir outside the scoped home")
			}
			path := filepath.Join(dir, "f.txt")
			if werr := os.WriteFile(path, tc.body, 0o644); werr != nil {
				t.Fatal(werr)
			}

			params, _ := json.Marshal(map[string]any{
				"username": u.Username,
				"path":     path,
			})
			out, herr := filesReadHandler(context.Background(), params)
			if herr != nil {
				t.Fatalf("handler: %v", herr)
			}
			resp, ok := out.(*filesReadResponse)
			if !ok {
				t.Fatalf("unexpected response type %T", out)
			}

			var got []byte
			if resp.IsBinary {
				got, err = base64.StdEncoding.DecodeString(resp.ContentB64)
				if err != nil {
					t.Fatalf("decode base64: %v", err)
				}
			} else {
				got = []byte(resp.Content)
			}

			if string(got) != string(tc.body) {
				t.Errorf("content corrupted on read\n got: % x\nwant: % x", got, tc.body)
			}
		})
	}
}

// A truncated read can slice the final rune mid-sequence. That is a cut, not
// corruption, and must not flip an ordinary UTF-8 text file to the binary path.
func TestValidUTF8Body_TruncatedRuneIsNotCorruption(t *testing.T) {
	t.Parallel()
	// "hé" is 68 c3 a9; slicing to 2 bytes leaves a lead byte with no
	// continuation -- a genuinely half-read rune. (Cutting the last byte of
	// "héllo" would leave "héll", which is still perfectly valid UTF-8.)
	full := []byte("hé")
	cut := full[:2]

	if validUTF8Body(cut, false) {
		t.Error("a mid-rune cut must be invalid when the read was NOT truncated")
	}
	if !validUTF8Body(cut, true) {
		t.Error("a mid-rune cut must be tolerated when the read WAS truncated")
	}
	if !validUTF8Body(full, false) {
		t.Error("intact UTF-8 must be valid")
	}
	if validUTF8Body([]byte("a\xffb"), true) {
		t.Error("a genuinely invalid byte must stay invalid even when truncated")
	}
}
