package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// errorPagesDir is the shared docroot the per-domain vhosts point their
// error_page directives at (location = /jabali-err-NNN.html { internal;
// root /var/www/jabali-errors; }). The pages are global — the page_template
// error bodies carry no per-domain placeholders — so one set serves every
// vhost.
const errorPagesDir = "/var/www/jabali-errors"

// syncErrorPagesParams carries the three branded error bodies the
// reconciler converges from the editable page_template rows. An empty
// field means "leave the existing file untouched" (the reconciler only
// sends bodies it has).
type syncErrorPagesParams struct {
	Error404 string `json:"error_404"`
	Error403 string `json:"error_403"`
	Error500 string `json:"error_500"`
}

type syncErrorPagesResponse struct {
	Written []string `json:"written"`
}

// syncErrorPagesHandler writes the shared branded error pages to
// errorPagesDir. Files are root-owned 0644 (nginx reads them as www-data;
// they are internal-only, reachable solely via error_page). Idempotent —
// the reconciler gates the call behind a content hash, but writing the
// same bytes again is harmless. Atomic per file (tmp + rename) so nginx
// never reads a half-written page.
func syncErrorPagesHandler(_ context.Context, raw json.RawMessage) (any, error) {
	var p syncErrorPagesParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: fmt.Sprintf("parse: %v", err),
		}
	}
	if err := os.MkdirAll(errorPagesDir, 0o755); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("mkdir %s: %v", errorPagesDir, err),
		}
	}

	files := []struct {
		name    string
		content string
	}{
		{"jabali-err-404.html", p.Error404},
		{"jabali-err-403.html", p.Error403},
		{"jabali-err-500.html", p.Error500},
	}
	var written []string
	for _, f := range files {
		if f.content == "" {
			continue
		}
		dst := filepath.Join(errorPagesDir, f.name)
		if err := writeFileAtomic(dst, []byte(f.content), 0o644); err != nil {
			return nil, &agentwire.AgentError{
				Code:    agentwire.CodeInternal,
				Message: fmt.Sprintf("write %s: %v", dst, err),
			}
		}
		written = append(written, dst)
	}
	return syncErrorPagesResponse{Written: written}, nil
}

// writeFileAtomic writes content to dst via a temp file + rename so a
// reader never observes a partial file.
func writeFileAtomic(dst string, content []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-"+filepath.Base(dst)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if the rename already moved it
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

func init() {
	Default.Register("page_templates.sync_error_pages", syncErrorPagesHandler)
}
