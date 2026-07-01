package models

import "fmt"

// PoolSlug returns the on-disk / systemd-instance identity for a PHP-FPM pool
// (GH #329). It MUST match the agent's derivation in php_pool_apply.go exactly.
//
//   - The user's default pool (isDefault) keeps slug == username — the legacy
//     /run/php/jabali-<user> socket and jabali-fpm@<user> master, byte-identical
//     to pre-#329 so existing vhosts never break.
//   - A per-version pool gets slug "<username>-php<version>" (e.g. alice-php8.2),
//     which the agent provisions as an additional master running as the same OS
//     user.
func PoolSlug(username, phpVersion string, isDefault bool) string {
	if isDefault {
		return username
	}
	return fmt.Sprintf("%s-php%s", username, phpVersion)
}

// PoolSocketPath returns the FPM Unix socket path for a pool — the value nginx
// fastcgi_pass targets and the reconciler polls for readiness. Mirrors the
// agent: /run/php/jabali-<slug>/fpm.sock.
func PoolSocketPath(username, phpVersion string, isDefault bool) string {
	return fmt.Sprintf("/run/php/jabali-%s/fpm.sock", PoolSlug(username, phpVersion, isDefault))
}
