package wordpressssh

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/migrate"
)

// Live integration test — SKIPPED unless JABALI_WPSSH_LIVE_HOST is set. Validates
// Connect + DiscoverWordPress + ExportDatabase + PullFilesTarball against a real
// WordPress-over-SSH source. Never runs in CI. GH #647.
func TestLive_PullHalf(t *testing.T) {
	host := os.Getenv("JABALI_WPSSH_LIVE_HOST")
	if host == "" {
		t.Skip("set JABALI_WPSSH_LIVE_HOST=<ip> JABALI_WPSSH_LIVE_SECRET=<file> JABALI_WPSSH_LIVE_PATH=<wp-root> to run")
	}
	secret := migrate.SecretRef{Path: os.Getenv("JABALI_WPSSH_LIVE_SECRET")}
	user := os.Getenv("JABALI_WPSSH_LIVE_USER")
	if user == "" {
		user = "root"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	sess, err := Connect(ctx, host, 22, user, secret, true /*allowPrivate — test host is RFC1918*/)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer sess.Close()

	facts, err := DiscoverWordPress(ctx, sess, os.Getenv("JABALI_WPSSH_LIVE_PATH"))
	if err != nil {
		t.Fatalf("DiscoverWordPress: %v", err)
	}
	t.Logf("facts: root=%s home=%s siteurl=%s prefix=%s wp=%s php=%s bytes=%d multisite=%v wpcli=%v",
		facts.Root, facts.Home, facts.SiteURL, facts.TablePrefix, facts.WPVersion, facts.PHPVersion, facts.BytesTotal, facts.Multisite, facts.WPCLI)
	if facts.Root == "" || facts.TablePrefix == "" {
		t.Fatalf("discovery incomplete: %+v", facts)
	}

	dir := t.TempDir()
	if err := sess.ExportDatabase(ctx, facts.Root, "livejob01", filepath.Join(dir, "dump.sql")); err != nil {
		t.Fatalf("ExportDatabase: %v", err)
	}
	st, _ := os.Stat(filepath.Join(dir, "dump.sql"))
	t.Logf("dump.sql = %d bytes", st.Size())
	if st.Size() < 100 {
		t.Fatalf("dump.sql suspiciously small: %d", st.Size())
	}
	if err := sess.PullFilesTarball(ctx, facts.Root, filepath.Join(dir, "files.tar.gz")); err != nil {
		t.Fatalf("PullFilesTarball: %v", err)
	}
	ft, _ := os.Stat(filepath.Join(dir, "files.tar.gz"))
	t.Logf("files.tar.gz = %d bytes", ft.Size())
	if ft.Size() < 100 {
		t.Fatalf("files.tar.gz suspiciously small: %d", ft.Size())
	}
}
