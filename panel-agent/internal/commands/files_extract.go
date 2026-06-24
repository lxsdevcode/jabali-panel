package commands

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/filesafe"
)

// files.extract — unpack an archive (.zip / .tar / .tar.gz / .tgz / .tar.bz2 /
// .gz) into a directory inside the user's scope. Every destination path is
// re-validated against the scope (zip-slip defense); symlink/hardlink/device
// entries are skipped (they could escape the scope); total entry count and
// uncompressed size are capped (decompression-bomb defense); extracted files
// are chowned to the hosting user.
//
// Security notes:
//   - The archive itself is resolved + opened through filesafe.Scope.
//   - Each entry name is cleaned and joined to the (scoped) destination, then
//     the result is re-Resolved so a "../../etc/passwd" or absolute-path entry
//     is rejected, not written.
//   - Files are created O_NOFOLLOW so a pre-planted symlink at the destination
//     can't redirect the write outside the docroot.
const (
	maxExtractEntries   = 100_000
	maxExtractTotalSize = int64(10) << 30 // 10 GiB uncompressed, total
	maxExtractFileSize  = int64(5) << 30  // 5 GiB per file
)

type filesExtractParams struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Path     string `json:"path"`           // archive to extract (scope-relative)
	Dest     string `json:"dest,omitempty"` // target dir; default = archive's parent
}

type filesExtractResult struct {
	Dest      string `json:"dest"`
	Extracted int    `json:"extracted"`
	Skipped   int    `json:"skipped"` // symlinks/unsafe entries not written
}

func filesExtractHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p filesExtractParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("failed to parse params: %v", err)}
	}
	if p.Username == "" || p.Path == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "username and path are required"}
	}

	homeDir := fmt.Sprintf("/home/%s", p.Username)
	scope, err := filesafe.NewScope(p.UserID, p.Username, []string{homeDir})
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("failed to create scope: %v", err)}
	}

	cleanArchive, err := scope.Clean(p.Path)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("path validation failed: %v", err)}
	}

	// Destination directory: explicit dest, else the archive's parent.
	destRel := p.Dest
	if strings.TrimSpace(destRel) == "" {
		destRel = filepath.Dir(p.Path)
	}
	destDir, err := scope.Clean(destRel)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("destination validation failed: %v", err)}
	}
	// Verify the destination is an existing directory via the escape-proof stat,
	// not an os.Stat(string) that re-resolves a swapped parent.
	if di, derr := scope.StatInScope(destDir); derr != nil || !di.IsDir {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "destination is not a directory"}
	}

	uid, gid := hostingIDs(p.Username)

	ex := &extractor{scope: scope, destDir: destDir, uid: uid, gid: gid}

	// SECURITY (Gitea #423): open the archive through the escape-proof fd. The
	// previous scope.Open(p.Path) / zip.OpenReader(path) re-resolved the path and
	// followed a tenant-planted PARENT symlink, letting a tenant read another
	// tenant's / root's archive. OpenInScope (RESOLVE_BENEATH) cannot be
	// redirected out of the docroot.
	f, err := scope.OpenInScope(cleanArchive, os.O_RDONLY, 0)
	if err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeNotFound, Message: fmt.Sprintf("failed to open archive: %v", err)}
	}
	defer f.Close()

	lower := strings.ToLower(p.Path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		err = ex.unzip(f)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		var gz *gzip.Reader
		if gz, err = gzip.NewReader(f); err == nil {
			defer gz.Close()
			err = ex.untar(gz)
		}
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		err = ex.untar(bzip2.NewReader(f))
	case strings.HasSuffix(lower, ".tar"):
		err = ex.untar(f)
	case strings.HasSuffix(lower, ".gz"):
		// Single-file gunzip -> drop the .gz suffix.
		err = ex.gunzipSingle(f, strings.TrimSuffix(filepath.Base(p.Path), filepath.Ext(p.Path)))
	default:
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "unsupported archive type (supported: .zip, .tar, .tar.gz, .tgz, .tar.bz2, .gz)"}
	}
	if err != nil {
		if ae, ok := err.(*agentwire.AgentError); ok {
			return nil, ae
		}
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("extract failed: %v", err)}
	}

	return filesExtractResult{Dest: destRel, Extracted: ex.extracted, Skipped: ex.skipped}, nil
}

// extractor carries the scope + running counters/limits across one archive.
type extractor struct {
	scope     *filesafe.Scope
	destDir   string
	uid, gid  int
	extracted int
	skipped   int
	total     int64
}

// safeDest cleans an archive entry name and resolves it inside destDir,
// rejecting absolute paths and "../" escapes. Returns the validated absolute
// destination path.
func (ex *extractor) safeDest(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	if name == "" || name == "." {
		return "", &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "empty archive entry name"}
	}
	escape := func() (string, error) {
		return "", &agentwire.AgentError{Code: agentwire.CodePermissionDenied, Message: fmt.Sprintf("archive entry escapes destination: %q", name)}
	}
	// Reject absolute paths and any ".." traversal outright (rather than
	// silently re-rooting) so a malicious archive surfaces an error.
	if strings.HasPrefix(name, "/") || filepath.IsAbs(name) {
		return escape()
	}
	cleaned := filepath.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return escape()
	}
	joined := filepath.Join(ex.destDir, cleaned)
	// Defense-in-depth: ensure the cleaned join is still under destDir.
	rel, err := filepath.Rel(ex.destDir, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return escape()
	}
	return joined, nil
}

func (ex *extractor) bump(n int64) error {
	ex.total += n
	if ex.total > maxExtractTotalSize {
		return &agentwire.AgentError{Code: agentwire.CodeFailedPrecondition, Message: "archive exceeds the uncompressed size limit"}
	}
	return nil
}

func (ex *extractor) mkdirAll(dir string, mode os.FileMode) error {
	if mode == 0 || mode&0o700 == 0 {
		mode = 0o755
	}
	if err := os.MkdirAll(dir, mode); err != nil {
		return err
	}
	_ = os.Chown(dir, ex.uid, ex.gid)
	return nil
}

// writeFile streams r into dir/dest (O_NOFOLLOW final component), capped at the
// per-file + total limits, then chowns to the hosting user.
func (ex *extractor) writeFile(dest string, mode os.FileMode, r io.Reader) error {
	if err := ex.mkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	// O_NOFOLLOW: never write THROUGH a symlink planted at the destination.
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, mode.Perm())
	if err != nil {
		// A symlink at dest (EEXIST/ELOOP) is treated as an unsafe entry.
		ex.skipped++
		return nil
	}
	defer out.Close()

	limited := io.LimitReader(r, maxExtractFileSize+1)
	n, err := io.Copy(out, limited)
	if err != nil {
		return err
	}
	if n > maxExtractFileSize {
		_ = os.Remove(dest)
		return &agentwire.AgentError{Code: agentwire.CodeFailedPrecondition, Message: "an archived file exceeds the per-file size limit"}
	}
	if err := ex.bump(n); err != nil {
		_ = os.Remove(dest)
		return err
	}
	_ = out.Chown(ex.uid, ex.gid)
	ex.extracted++
	return nil
}

func (ex *extractor) countGuard() error {
	if ex.extracted+ex.skipped >= maxExtractEntries {
		return &agentwire.AgentError{Code: agentwire.CodeFailedPrecondition, Message: "archive has too many entries"}
	}
	return nil
}

func (ex *extractor) unzip(f *os.File) error {
	// Random-access zip read straight from the escape-proof fd (no path reopen).
	fi, err := f.Stat()
	if err != nil {
		return &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("stat archive: %v", err)}
	}
	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("not a valid zip: %v", err)}
	}
	for _, zf := range zr.File {
		if err := ex.countGuard(); err != nil {
			return err
		}
		dest, err := ex.safeDest(zf.Name)
		if err != nil {
			return err
		}
		info := zf.FileInfo()
		switch {
		case info.IsDir():
			if err := ex.mkdirAll(dest, info.Mode()); err != nil {
				return err
			}
		case info.Mode()&os.ModeSymlink != 0, info.Mode()&os.ModeType != 0 && !info.Mode().IsRegular():
			ex.skipped++ // symlink / device / fifo — never materialize
		default:
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			err = ex.writeFile(dest, info.Mode(), rc)
			rc.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (ex *extractor) untar(r io.Reader) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("not a valid tar: %v", err)}
		}
		if err := ex.countGuard(); err != nil {
			return err
		}
		dest, err := ex.safeDest(hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := ex.mkdirAll(dest, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := ex.writeFile(dest, os.FileMode(hdr.Mode), tr); err != nil {
				return err
			}
		default:
			// Symlink, hardlink, char/block device, fifo — skip (unsafe).
			ex.skipped++
		}
	}
}

func (ex *extractor) gunzipSingle(r io.Reader, outName string) error {
	if outName == "" {
		outName = "extracted"
	}
	gz, err := gzip.NewReader(r)
	if err != nil {
		return &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("not a valid gzip: %v", err)}
	}
	defer gz.Close()
	dest, err := ex.safeDest(outName)
	if err != nil {
		return err
	}
	return ex.writeFile(dest, 0o644, gz)
}

// hostingIDs returns the uid + the www-data gid for the hosting user (matching
// the ownership the other files.* handlers set). Falls back to the user's own
// gid if www-data isn't present.
func hostingIDs(username string) (int, int) {
	u, err := user.Lookup(username)
	if err != nil {
		return 0, 0
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	if g, err := user.LookupGroup("www-data"); err == nil {
		if wgid, err := strconv.Atoi(g.Gid); err == nil {
			gid = wgid
		}
	}
	return uid, gid
}

func init() {
	Default.Register("files.extract", filesExtractHandler)
}
