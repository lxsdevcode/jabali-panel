// wordpress_cache.go — GH #406. Enable/disable the jabali-wp-cache plugin on a
// WordPress install, as the tenant, via wp-cli. The Redis object cache is gated
// by the per-tenant ACL user (ADR-0148): panel-api creates wp_<osuser> scoped to
// ~jc:<prefix>:* and passes the token here; the plugin connects with it.
//
// The plugin source is bundled read-only at bundledWPCachePluginDir (install.sh
// syncs it) so tenants never supply plugin code. Idempotent.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

const bundledWPCachePluginDir = "/usr/local/share/jabali/wp-plugins/jabali-wp-cache"

// redisClientsGroup gates /run/redis/redis.sock. Distinct from jabali-sockets
// (which also fronts the root agent socket) so tenants get Redis but nothing else.
const redisClientsGroup = "jabali-redis-clients"

type wordpressCacheSetParams struct {
	InstallPath   string `json:"install_path"`
	OSUser        string `json:"os_user"`
	Enable        bool   `json:"enable"`
	RedisSocket   string `json:"redis_socket"`
	RedisDB       int    `json:"redis_db"`
	Prefix        string `json:"prefix"`         // the jc:<...>: namespace (matches the ACL ~jc:<...>:*)
	RedisPassword string `json:"redis_password"` // the per-tenant ACL token
}

type wordpressCacheSetResult struct {
	Ok      bool   `json:"ok"`
	Enabled bool   `json:"enabled"`
	Detail  string `json:"detail,omitempty"`
}

func wordpressCacheSetHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var p wordpressCacheSetParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse params: %v", err)}
	}
	if !strings.HasPrefix(p.InstallPath, "/home/") || strings.Contains(p.InstallPath, "..") {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "install_path must be under /home/ with no .."}
	}
	if p.OSUser == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "os_user required"}
	}

	if !p.Enable {
		// Disable: drop-ins out, plugin deactivated. Best-effort (a missing
		// plugin shouldn't fail the disable).
		_ = runWPAsTenant(ctx, p.OSUser, p.InstallPath, "jabali-wp-cache", "disable")
		_ = runWPAsTenant(ctx, p.OSUser, p.InstallPath, "plugin", "deactivate", "jabali-wp-cache")
		_ = setWPConfigCacheConstants(p.InstallPath, "", 0, "", "", false) // strip the managed block
		return wordpressCacheSetResult{Ok: true, Enabled: false}, nil
	}

	// 1. Stage the plugin into the install (root copy, then chown to the tenant).
	dest := filepath.Join(p.InstallPath, "wp-content", "plugins", "jabali-wp-cache")
	if _, err := os.Stat(bundledWPCachePluginDir); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeFailedPrecondition,
			Message: fmt.Sprintf("bundled plugin missing at %s — re-run install.sh", bundledWPCachePluginDir)}
	}
	if err := exec.CommandContext(ctx, "rm", "-rf", dest).Run(); err != nil {
		return nil, bkInternal("clear old plugin", err)
	}
	if err := exec.CommandContext(ctx, "cp", "-a", bundledWPCachePluginDir, dest).Run(); err != nil {
		return nil, bkInternal("stage plugin", err)
	}
	if err := exec.CommandContext(ctx, "chown", "-R", p.OSUser+":www-data", dest).Run(); err != nil {
		return nil, bkInternal("chown plugin", err)
	}

	// 2. Write the config file the drop-ins read (wp-content/jabali-wp-cache-config.php).
	socket := p.RedisSocket
	if socket == "" {
		socket = "/run/redis/redis.sock"
	}
	db := p.RedisDB
	if db == 0 {
		db = 1
	}
	cfgPath := filepath.Join(p.InstallPath, "wp-content", "jabali-wp-cache-config.php")
	cfg := wpCacheConfigPHP(socket, db, p.Prefix, p.RedisPassword)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o640); err != nil {
		return nil, bkInternal("write cache config", err)
	}
	if err := exec.CommandContext(ctx, "chown", p.OSUser+":www-data", cfgPath).Run(); err != nil {
		return nil, bkInternal("chown cache config", err)
	}

	// 2b. Pin the jabali settings as CONSTANTS in wp-config.php. The plugin's
	//     activation regenerates wp-content/jabali-wp-cache-config.php with a
	//     SELF-DERIVED key prefix (md5 of the site URL), which would NOT match the
	//     per-tenant Redis ACL (~jc:<osuser>:*) — so the object cache would NOPERM
	//     even with socket access. apply_constants() in the plugin makes these
	//     JABALI_CACHE_* defines authoritative over that regenerated file.
	if err := setWPConfigCacheConstants(p.InstallPath, socket, db, p.Prefix, p.RedisPassword, true); err != nil {
		return nil, bkInternal("write wp-config constants", err)
	}

	// 3. Grant the tenant access to the Redis client socket group so its
	// php-fpm workers can open /run/redis/redis.sock (the socket is group
	// jabali-redis-clients, NOT jabali-sockets — tenants must never reach the
	// root agent socket; ADR-0148). Group membership is fixed at the fpm master
	// start, so the per-user master is RESTARTED (not reloaded) to pick it up.
	_ = exec.CommandContext(ctx, "groupadd", "-f", redisClientsGroup).Run()
	if err := exec.CommandContext(ctx, "usermod", "-aG", redisClientsGroup, p.OSUser).Run(); err != nil {
		return nil, bkInternal("add tenant to "+redisClientsGroup, err)
	}
	// Best-effort: a missing/!active fpm master (e.g. CLI-only install) isn't fatal.
	_ = exec.CommandContext(ctx, "systemctl", "restart", "jabali-fpm@"+p.OSUser+".service").Run()

	// 4. Activate + enable (drop-ins + WP_CACHE), as the tenant.
	if out, err := runWPAsTenantOut(ctx, p.OSUser, p.InstallPath, "plugin", "activate", "jabali-wp-cache"); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal,
			Message: fmt.Sprintf("wp plugin activate: %v: %s", err, out)}
	}
	if out, err := runWPAsTenantOut(ctx, p.OSUser, p.InstallPath, "jabali-wp-cache", "enable"); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal,
			Message: fmt.Sprintf("wp jabali-wp-cache enable: %v: %s", err, out)}
	}
	return wordpressCacheSetResult{Ok: true, Enabled: true}, nil
}

// wpCacheConfigPHP renders the PHP array config the plugin's drop-ins read. Page
// cache stays off — nginx owns page caching (the /domains/:id/cache microcache);
// this enables the Redis OBJECT cache only.
func wpCacheConfigPHP(socket string, db int, prefix, password string) string {
	// PHP single-quoted strings treat only \ and ' as special. Escape the
	// backslash FIRST (else we'd double-escape the ones we add for the quote),
	// then the quote — a value ending in \ would otherwise escape the closing '.
	esc := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "'", "\\'")
		return s
	}
	return "<?php\n// Managed by jabali (GH #406). Do NOT hand-edit.\nreturn array(\n" +
		"  'enabled'    => true,\n" +
		"  'page_cache' => false,\n" +
		"  'scheme'     => 'unix',\n" +
		"  'socket'     => '" + esc(socket) + "',\n" +
		"  'database'   => " + strconv.Itoa(db) + ",\n" +
		"  'password'   => '" + esc(password) + "',\n" +
		"  'prefix'     => '" + esc(prefix) + "',\n" +
		");\n"
}

// wpConfigBeginMarker / wpConfigEndMarker fence the jabali-managed define block
// in wp-config.php so it can be replaced/removed idempotently.
const (
	wpConfigBeginMarker = "// BEGIN Jabali WP Cache (managed by jabali #406) — do not edit"
	wpConfigEndMarker   = "// END Jabali WP Cache"
)

// setWPConfigCacheConstants writes (enable) or strips (disable) the jabali-managed
// JABALI_CACHE_* define block in <installPath>/wp-config.php. The block is inserted
// just before the "stop editing" marker (or wp-settings.php require), so the defines
// land before WordPress — and the plugin's drop-ins — load.
func setWPConfigCacheConstants(installPath, socket string, db int, prefix, password string, enable bool) error {
	cfgPath := filepath.Join(installPath, "wp-config.php")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	content := stripWPCacheBlock(string(raw))

	if enable {
		esc := func(s string) string { // PHP single-quote: escape \ then '
			s = strings.ReplaceAll(s, "\\", "\\\\")
			return strings.ReplaceAll(s, "'", "\\'")
		}
		block := wpConfigBeginMarker + "\n" +
			"if ( ! defined( 'JABALI_CACHE_SOCKET' ) )   define( 'JABALI_CACHE_SOCKET', '" + esc(socket) + "' );\n" +
			"if ( ! defined( 'JABALI_CACHE_DB' ) )       define( 'JABALI_CACHE_DB', " + strconv.Itoa(db) + " );\n" +
			"if ( ! defined( 'JABALI_CACHE_PASSWORD' ) ) define( 'JABALI_CACHE_PASSWORD', '" + esc(password) + "' );\n" +
			"if ( ! defined( 'JABALI_CACHE_PREFIX' ) )   define( 'JABALI_CACHE_PREFIX', '" + esc(prefix) + "' );\n" +
			wpConfigEndMarker + "\n"
		content = insertBeforeWPSettings(content, block)
	}

	if err := os.WriteFile(cfgPath, []byte(content), 0o640); err != nil {
		return err
	}
	// wp-config.php is owned <user>:www-data on a jabali install; preserve that.
	_ = exec.Command("chown", "--reference="+filepath.Dir(cfgPath), cfgPath).Run()
	return nil
}

// stripWPCacheBlock removes a previously-inserted BEGIN..END jabali block
// (idempotent re-apply / clean disable). No block → returned unchanged.
func stripWPCacheBlock(content string) string {
	start := strings.Index(content, wpConfigBeginMarker)
	if start < 0 {
		return content
	}
	end := strings.Index(content[start:], wpConfigEndMarker)
	if end < 0 {
		return content // malformed; leave alone rather than truncate
	}
	end = start + end + len(wpConfigEndMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start] + content[end:]
}

// insertBeforeWPSettings places block before the "stop editing" marker or the
// wp-settings.php require (whichever appears first); falls back to appending.
func insertBeforeWPSettings(content, block string) string {
	for _, anchor := range []string{"/* That's all, stop editing!", "require_once ABSPATH . 'wp-settings.php'", "require_once(ABSPATH . 'wp-settings.php')"} {
		if i := strings.Index(content, anchor); i >= 0 {
			return content[:i] + block + content[i:]
		}
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + block
}

func runWPAsTenant(ctx context.Context, osUser, installPath string, args ...string) error {
	_, err := runWPAsTenantOut(ctx, osUser, installPath, args...)
	return err
}

func runWPAsTenantOut(ctx context.Context, osUser, installPath string, args ...string) (string, error) {
	full := append([]string{"wp"}, args...)
	full = append(full, "--path="+installPath)
	cmd := buildSystemdRunCmd(ctx, osUser, full...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func init() {
	Default.Register("wordpress.cache_set", wordpressCacheSetHandler)
}
