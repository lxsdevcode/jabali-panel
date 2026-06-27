// Shared atomic-write helper for M22 SSO files (ADR-0040).
//
// Called by every per-CMS create_sso_file handler. The helper owns the
// atomic-rename / chown / chmod / cleanup-on-error dance so each CMS's
// handler only concerns itself with rendering its own PHP template.

package commands

import (
	"context"
	"fmt"
	"os/user"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/internal/filesafe"
)

// createSSOFileResp is the wire response shape every per-CMS handler
// returns. Shared because the panel-api side uses the same decoder
// regardless of app_type — only the mint endpoint's dispatch decides
// which agent command it calls.
type createSSOFileResp struct {
	FileName      string `json:"file_name"`
	ExpiresAtUnix int64  `json:"expires_at_unix"`
}

// writeSSOFile writes body to <installPath>/jabali-sso-<nonce>.php
// atomically (tmp + rename), chowns to <osUser>:www-data, chmods to
// 0440, and returns the file name.
//
// Symlink safety (Gitea #470): the SSO file is executable PHP written by
// the root agent into a tenant-controlled webroot. The install path is
// bound to the tenant's own home directory and resolved through a filesafe
// scope (openat2 RESOLVE_BENEATH), and every create/chown/chmod/rename
// goes through escape-proof scope primitives, so a symlinked install dir,
// parent component, or target leaf cannot redirect the root write outside
// the tenant home. On any failure the partial file is removed.
func writeSSOFile(ctx context.Context, installPath, osUser, nonce, body string) (createSSOFileResp, error) {
	zero := createSSOFileResp{}
	if osUser == "" {
		return zero, fmt.Errorf("os_user required")
	}

	u, err := user.Lookup(osUser)
	if err != nil {
		return zero, fmt.Errorf("lookup os_user %q: %w", osUser, err)
	}
	scope, err := filesafe.NewScope(osUser, osUser, []string{u.HomeDir})
	if err != nil {
		return zero, fmt.Errorf("scope init: %w", err)
	}
	// Bind + symlink-safe resolve the webroot to the tenant's own home.
	resolvedInstall, err := scope.Resolve(installPath)
	if err != nil {
		return zero, fmt.Errorf("install_path %q must resolve under %s: %w", installPath, u.HomeDir, err)
	}

	fileName := "jabali-sso-" + nonce + ".php"
	dest := resolvedInstall + "/" + fileName
	tmp := resolvedInstall + "/." + fileName + ".tmp"

	// Clear any stale temp so the exclusive create can't EEXIST.
	_ = scope.RemoveInScope(tmp, false)

	f, err := scope.CreateExclInScope(tmp, 0o640)
	if err != nil {
		return zero, fmt.Errorf("open tmp %s: %w", tmp, err)
	}
	cleanup := func() { _ = f.Close(); _ = scope.RemoveInScope(tmp, false) }
	if _, err := f.WriteString(body); err != nil {
		cleanup()
		return zero, fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		cleanup()
		return zero, fmt.Errorf("fsync tmp %s: %w", tmp, err)
	}

	uid, _ := parseUID(u)
	wwwData, gerr := lookupGroup("www-data")
	if gerr != nil {
		cleanup()
		return zero, fmt.Errorf("lookup www-data group: %w", gerr)
	}
	// fd-based chown/chmod (escape-proof) before rename; mode/owner persist.
	if err := f.Chown(uid, wwwData); err != nil {
		cleanup()
		return zero, fmt.Errorf("chown %s:www-data: %w", osUser, err)
	}
	if err := f.Chmod(0o440); err != nil {
		cleanup()
		return zero, fmt.Errorf("chmod 0440: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = scope.RemoveInScope(tmp, false)
		return zero, fmt.Errorf("close tmp %s: %w", tmp, err)
	}
	if err := scope.RenameInScope(tmp, dest); err != nil {
		_ = scope.RemoveInScope(tmp, false)
		return zero, fmt.Errorf("rename %s -> %s: %w", tmp, dest, err)
	}

	return createSSOFileResp{
		FileName:      fileName,
		ExpiresAtUnix: time.Now().Add(time.Duration(ssoTTLSeconds) * time.Second).Unix(),
	}, nil
}
