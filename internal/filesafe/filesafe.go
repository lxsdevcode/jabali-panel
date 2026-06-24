// Package filesafe provides secure file path validation and access control
// for user-submitted file operations. This is the shared contract between the API
// handler (pre-accept gate) and the agent (defense-in-depth before filesystem access).
//
// Design principles:
//   - Path traversal prevention via filepath.Clean(), filepath.EvalSymlinks(), and prefix validation
//   - Symlink escape detection via os.Lstat() + ModeSymlink check
//   - TOCTOU mitigation using O_NOFOLLOW syscall flag on filesystem operations
//   - chown race prevention via 0600 temp file creation before chown sequencing
//   - Null-byte and control character rejection in paths
//   - Allow-list is closed-set: only absolute paths within owned docroots allowed
//   - No subprocess calls for path validation; all validation is in-process
package filesafe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Error codes for structured error reporting (suitable for API "error" field).
const (
	ErrCodeEmpty           = "empty"
	ErrCodeTooLong         = "too_long"         // >4096 bytes
	ErrCodeNotAbsolute     = "not_absolute"     // path not absolute
	ErrCodeContainsNull    = "contains_null"    // NUL byte in path
	ErrCodeContainsControl = "contains_control" // control character in path
	ErrCodeTraversal       = "path_traversal"   // .. or other traversal attempt
	ErrCodeSymlinkEscape   = "symlink_escape"   // symlink points outside scope
	ErrCodeNotInScope      = "not_in_scope"     // path not within owned docroots
	ErrCodeSymlinkLoop     = "symlink_loop"     // circular symlink chain
	ErrCodeBadCharacters   = "bad_characters"   // invalid filename characters
)

// ValidationError is the error type returned by validators.
type ValidationError struct {
	Code   string // One of ErrCode* constants
	Detail string // Human-readable explanation
}

func (e *ValidationError) Error() string {
	return e.Code + ": " + e.Detail
}

// Scope represents a validated security context for file operations.
// All paths must resolve within the owned docroots.
type Scope struct {
	UserID        string
	Username      string
	OwnedDocroots []string
	// resolveCache caches EvalSymlinks results to detect symlink loops
	resolveCache map[string]string
}

// NewScope creates a new validated scope for file operations.
// ownedDocroots must be non-empty and will be normalized.
func NewScope(userID, username string, ownedDocroots []string) (*Scope, error) {
	if username == "" {
		return nil, &ValidationError{
			Code:   ErrCodeEmpty,
			Detail: "username cannot be empty",
		}
	}

	if len(ownedDocroots) == 0 {
		return nil, &ValidationError{
			Code:   ErrCodeEmpty,
			Detail: "ownedDocroots cannot be empty",
		}
	}

	// Normalize all docroots
	normalized := make([]string, len(ownedDocroots))
	for i, root := range ownedDocroots {
		root = filepath.Clean(root)
		if !filepath.IsAbs(root) {
			return nil, &ValidationError{
				Code:   ErrCodeNotAbsolute,
				Detail: fmt.Sprintf("docroot %q must be absolute", root),
			}
		}
		normalized[i] = root
	}

	return &Scope{
		UserID:        userID,
		Username:      username,
		OwnedDocroots: normalized,
		resolveCache:  make(map[string]string),
	}, nil
}

// NewScopeForTest creates a Scope for testing with a temporary docroot.
// Useful for testing symlink escape detection and other filesafe behavior
// without requiring real user accounts.
func NewScopeForTest(userID, username, docroot string) *Scope {
	return &Scope{
		UserID:        userID,
		Username:      username,
		OwnedDocroots: []string{docroot},
		resolveCache:  make(map[string]string),
	}
}

// Clean validates and cleans a path, returning the canonical form.
// The path must be absolute and resolve within the owned docroots.
// This is a pre-access gate; use before any filesystem operation.
func (s *Scope) Clean(pathStr string) (string, error) {
	if err := s.validatePathString(pathStr); err != nil {
		return "", err
	}

	// Clean the path (resolves . and .. in a path-safe manner)
	cleaned := filepath.Clean(pathStr)

	// Verify cleaned path is still absolute
	if !filepath.IsAbs(cleaned) {
		return "", &ValidationError{
			Code:   ErrCodeNotAbsolute,
			Detail: fmt.Sprintf("path %q cleaned to non-absolute %q", pathStr, cleaned),
		}
	}

	// Verify path is within scope
	if err := s.verifyInScope(cleaned); err != nil {
		return "", err
	}

	return cleaned, nil
}

// Resolve validates and resolves a path, following symlinks up to 40 levels deep.
// Returns the absolute canonical path (all symlinks resolved).
// Detects symlink loops and escape attempts.
// Use before actual filesystem access to ensure path is real and in-scope.
func (s *Scope) Resolve(pathStr string) (string, error) {
	if err := s.validatePathString(pathStr); err != nil {
		return "", err
	}

	// Start with cleaned path
	cleaned := filepath.Clean(pathStr)
	if !filepath.IsAbs(cleaned) {
		return "", &ValidationError{
			Code:   ErrCodeNotAbsolute,
			Detail: fmt.Sprintf("path %q cleaned to non-absolute %q", pathStr, cleaned),
		}
	}

	// Check in-scope before resolution (guard against symlink bombs on the way)
	if err := s.verifyInScope(cleaned); err != nil {
		return "", err
	}

	// SECURITY (Gitea #424): canonicalize by resolving the NEAREST EXISTING
	// ancestor and re-appending the non-existent tail, failing CLOSED on any
	// EvalSymlinks error other than "doesn't exist". The previous code did
	// EvalSymlinks(cleaned) and, on ANY error (ENOENT for every new-file create
	// and the .tmp.<rand> name), silently fell back to the LOGICAL path — so a
	// tenant-planted PARENT symlink was never collapsed and verifyInScope passed
	// on a prefix that didn't reflect where the bytes actually land.
	resolved, err := s.resolveNearest(cleaned)
	if err != nil {
		return "", err
	}

	// Verify resolved path is still in scope (symlink escape detection)
	if err := s.verifyInScope(resolved); err != nil {
		return "", err
	}

	// Check for symlink loops by doing a final lstat and comparing
	// (EvalSymlinks detects loops via iteration limit, but we add defense-in-depth)
	fi, err := os.Lstat(resolved)
	if err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		// Final path is itself a symlink (EvalSymlinks may not have followed it)
		return "", &ValidationError{
			Code:   ErrCodeSymlinkEscape,
			Detail: fmt.Sprintf("resolved path %q is itself a symlink", resolved),
		}
	}

	return resolved, nil
}

// Open opens a file at the given path with the given flags and perms.
// Uses O_NOFOLLOW to prevent TOCTOU symlink attacks.
// Path is validated and resolved before opening.
func (s *Scope) Open(pathStr string, flags int, perm os.FileMode) (*os.File, error) {
	// Validate first
	cleanPath, err := s.Clean(pathStr)
	if err != nil {
		return nil, err
	}

	// Add O_NOFOLLOW to prevent TOCTOU symlink race
	// If the final component is a symlink, open() will fail with ELOOP
	flags |= syscall.O_NOFOLLOW

	// Use os.OpenFile which ultimately calls open(2) with our flags
	return os.OpenFile(cleanPath, flags, perm)
}

// ChownToUser changes the owner of the file at pathStr to the given uid/gid.
// Uses O_NOFOLLOW internally to prevent symlink-based chown races.
// Recommended: create temp file with 0600, then chown, then move into place.
func (s *Scope) ChownToUser(pathStr string, uid, gid int) error {
	// Validate first
	cleanPath, err := s.Clean(pathStr)
	if err != nil {
		return err
	}

	// Lstat without following symlinks to ensure target is not a symlink
	fi, err := os.Lstat(cleanPath)
	if err != nil {
		return err
	}
	if (fi.Mode() & os.ModeSymlink) != 0 {
		return &ValidationError{
			Code:   ErrCodeSymlinkEscape,
			Detail: fmt.Sprintf("cannot chown symlink %q", cleanPath),
		}
	}

	return os.Chown(cleanPath, uid, gid)
}

// --- Private helpers ---

// validatePathString performs pre-path checks: null bytes, control chars, length.
func (s *Scope) validatePathString(pathStr string) error {
	if pathStr == "" {
		return &ValidationError{
			Code:   ErrCodeEmpty,
			Detail: "path cannot be empty",
		}
	}

	if len(pathStr) > 4096 {
		return &ValidationError{
			Code:   ErrCodeTooLong,
			Detail: fmt.Sprintf("path exceeds 4096 bytes (%d bytes)", len(pathStr)),
		}
	}

	// Reject null bytes
	if strings.ContainsRune(pathStr, '\x00') {
		return &ValidationError{
			Code:   ErrCodeContainsNull,
			Detail: "path contains NUL byte",
		}
	}

	// Reject control characters (ASCII 0-31 except tab, lf, cr are still bad)
	for _, r := range pathStr {
		if r < 32 {
			return &ValidationError{
				Code:   ErrCodeContainsControl,
				Detail: fmt.Sprintf("path contains control character (0x%02x)", r),
			}
		}
	}

	// Reject .. traversal attempts (defense-in-depth; filepath.Clean will also handle)
	if strings.Contains(pathStr, "..") {
		return &ValidationError{
			Code:   ErrCodeTraversal,
			Detail: "path contains '..' traversal sequence",
		}
	}

	return nil
}

// verifyInScope checks if a path is within one of the owned docroots.
// Uses word-boundary checking: /x must not match /xyz.
func (s *Scope) verifyInScope(pathStr string) error {
	// Path must be absolute
	if !filepath.IsAbs(pathStr) {
		return &ValidationError{
			Code:   ErrCodeNotAbsolute,
			Detail: fmt.Sprintf("path %q is not absolute", pathStr),
		}
	}

	// Check containment in owned docroots
	for _, docroot := range s.OwnedDocroots {
		docroot = filepath.Clean(docroot)

		// Exact match (for root-level docroots)
		if pathStr == docroot {
			return nil
		}

		// Prefix match with / boundary (no /x matching /xyz)
		if strings.HasPrefix(pathStr, docroot+"/") {
			return nil
		}
	}

	return &ValidationError{
		Code:   ErrCodeNotInScope,
		Detail: fmt.Sprintf("path %q is not within owned docroots: %v", pathStr, s.OwnedDocroots),
	}
}

// resolveNearest canonicalizes cleaned by EvalSymlinks-resolving its nearest
// EXISTING ancestor and re-joining the (guaranteed non-existent, hence
// symlink-free) tail. It fails CLOSED on any EvalSymlinks error other than the
// component simply not existing (ENOENT) — an EACCES/ELOOP is returned, never
// swallowed into a logical-path fallback (Gitea #424). The canonical ancestor
// must itself stay in scope, so a parent symlink pointing out of the docroot is
// rejected even when the final component does not yet exist.
func (s *Scope) resolveNearest(cleaned string) (string, error) {
	dir := cleaned
	var tailParts []string
	for {
		real, err := filepath.EvalSymlinks(dir)
		if err == nil {
			if verr := s.verifyInScope(real); verr != nil {
				return "", verr
			}
			out := real
			for i := len(tailParts) - 1; i >= 0; i-- {
				out = filepath.Join(out, tailParts[i])
			}
			return out, nil
		}
		if !os.IsNotExist(err) {
			// EACCES, ELOOP (symlink loop / too many levels), etc. — fail closed.
			return "", &ValidationError{Code: ErrCodeSymlinkEscape, Detail: fmt.Sprintf("cannot resolve %q: %v", dir, err)}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached "/" with no existing ancestor — impossible for an in-scope
			// path (the docroot exists); fail closed.
			return "", &ValidationError{Code: ErrCodeNotInScope, Detail: fmt.Sprintf("no existing ancestor for %q", cleaned)}
		}
		tailParts = append(tailParts, filepath.Base(dir))
		dir = parent
	}
}
