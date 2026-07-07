package migrate

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// maxExtractFileBytes caps a single extracted file (loose backstop against a
// degenerate tar entry). The manifest capacity gate is the real limit.
const maxExtractFileBytes = 100 << 30 // 100 GiB

// ExtractTarGz extracts a (optionally gzip'd) tarball into dest with path
// containment: header names with `../`, `/../`, or absolute paths are skipped,
// the joined output path is re-verified to stay under dest (belt-and-suspenders),
// and symlink/hardlink entries are skipped entirely (no write-through). Lifted
// from the migrate CLI so the wordpress_ssh/plugin imports (GH #647/#648) reuse
// one proven containment-safe extractor for the UNTRUSTED source tarball.
func ExtractTarGz(tarPath, dest string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 2)
	if _, err := io.ReadFull(f, buf); err != nil {
		return fmt.Errorf("magic: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	var src io.Reader = f
	if buf[0] == 0x1f && buf[1] == 0x8b {
		gz, gerr := gzip.NewReader(f)
		if gerr != nil {
			return fmt.Errorf("gunzip: %w", gerr)
		}
		defer gz.Close()
		src = gz
	}

	cleanDest := filepath.Clean(dest)
	tr := tar.NewReader(src)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") || filepath.IsAbs(clean) {
			continue // path traversal in the header name
		}
		out := filepath.Join(cleanDest, clean)
		// Belt-and-suspenders: the joined path must remain under dest.
		if out != cleanDest && !strings.HasPrefix(out, cleanDest+string(os.PathSeparator)) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(out, 0o750); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
				return err
			}
			w, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, io.LimitReader(tr, maxExtractFileBytes)); err != nil {
				_ = w.Close()
				return err
			}
			if err := w.Close(); err != nil {
				return err
			}
			// TypeSymlink / TypeLink and everything else: skipped (no write-through).
		}
	}
	return nil
}

// CheckExtractDiskSpace refuses to extract a backup when the destination
// filesystem lacks room for the (estimated) uncompressed tree, so a low-disk
// host fails FAST with a clear message instead of half-extracting and then
// erroring deep in the restore pipeline (JAB-41). Estimate = compressed size ×5
// (conservative gzip/zstd ratio; the multiplier is the headroom). Best-effort:
// if the tarball or filesystem can't be stat'd, it does not block.
func CheckExtractDiskSpace(tarballPath, dest string) error {
	fi, err := os.Stat(tarballPath)
	if err != nil {
		return nil
	}
	needed := uint64(fi.Size()) * 5
	var st syscall.Statfs_t
	if err := syscall.Statfs(dest, &st); err != nil {
		return nil
	}
	free := st.Bavail * uint64(st.Bsize)
	if free < needed {
		return fmt.Errorf("insufficient disk space to extract %s: need ~%d MiB, only %d MiB free on %s (free up space or move staging)",
			filepath.Base(tarballPath), needed>>20, free>>20, dest)
	}
	return nil
}
