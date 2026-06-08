package reconciler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// reconcileErrorPages converges the three shared branded error pages
// (404/403/500) from the editable page_template rows onto disk at
// /var/www/jabali-errors, which every per-domain vhost points its
// error_page directives at. The bodies are global (no per-domain
// placeholders), so one set serves all vhosts. Hash-gated: it only calls
// the agent when the content actually changed (admin edit / reset), so the
// steady-state tick is a pure noop. On a failed sync it does NOT cache the
// hash, so the next tick retries.
func (r *Reconciler) reconcileErrorPages(ctx context.Context) {
	if r.pageTemplates == nil || r.agent == nil {
		return
	}
	body := func(key string) string {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if row, err := r.pageTemplates.Get(c, key); err == nil && row != nil && row.Content != "" {
			return row.Content
		}
		// Row missing/empty — fall back to the compiled default so the
		// files always exist for nginx's error_page to find.
		return repository.DefaultPageTemplateBody(key)
	}
	e404 := body(models.PageTemplateError404)
	e403 := body(models.PageTemplateError403)
	e500 := body(models.PageTemplateError500)

	sum := sha256.Sum256([]byte(e404 + "\x00" + e403 + "\x00" + e500))
	hash := hex.EncodeToString(sum[:])
	if hash == r.errorPagesHash {
		return
	}

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := r.agent.Call(cctx, "page_templates.sync_error_pages", map[string]any{
		"error_404": e404,
		"error_403": e403,
		"error_500": e500,
	}); err != nil {
		r.log.Warn("error-pages sync failed", "error", err)
		return
	}
	r.errorPagesHash = hash
	r.log.Debug("branded error pages synced to /var/www/jabali-errors")
}
