// Public Automation API (M44).
//
// Mounted at /api/v1/automation/* behind the HMAC middleware. Each
// route declares its required scope; AutomationScopes.Has matches
// either an exact "read:domains" or a wildcard "read:*".
//
// Response shapes are intentionally THINNER than the regular
// /api/v1 routes: external callers shouldn't accidentally cache
// listen-IP topology, doc-roots, or per-user infra fields. If a
// downstream automation needs a richer view, mint a Kratos session
// instead.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/ssokey"
)

// AutomationConfig wires read repos for the public automation API.
// Required: AutomationTokens + Key (for the HMAC middleware).
// Optional: per-resource repos — when nil the matching route returns
// 503 instead of 404 so the caller can distinguish "feature off" from
// "no rows".
type AutomationConfig struct {
	AutomationTokens repository.AutomationTokenRepository
	Key              *ssokey.Key
	// Redis is the M44 replay-defense store. Required in production
	// (the M14 dispatcher already needs Redis on every install, so
	// this is a no-cost share). Nil disables the SETNX gate — only
	// for tests / non-prod.
	Redis        *redis.Client
	Domains      repository.DomainRepository
	Users        repository.UserRepository
	Applications repository.ApplicationInstallRepository
	// Agent powers the read:status metrics endpoint (JAB-75) — reuses the
	// M31 system.* collectors so a fleet monitor gets the same data the admin
	// Server Status page shows. Nil → /status stays the bare healthy stub.
	Agent agent.AgentInterface
	// Mail-stack read repos power the read:mail endpoints (JAB-77). Nil → those
	// routes aren't mounted.
	Mailboxes  repository.MailboxRepository
	MailGroups repository.MailGroupRepository
	Forwarders repository.EmailForwarderRepository
	// JAB-140 write layer. Operations powers async backups + the idempotency
	// ledger; Audits records every write (M49). Backups drives POST /backups.
	// All optional — a nil repo/agent drops the matching write route (mirrors
	// the read routes' feature-off behaviour).
	Operations repository.AutomationOperationRepository
	Audits     repository.AuditEventRepository
	// Backup wiring (JAB-140): reuse the M30 system-backup path so automation
	// goes through the same job model + destination the GUI uses. When either is
	// nil the async backup still enqueues but resolves to an error op (no dest).
	BackupJobs  repository.BackupJobRepository
	BackupDests repository.BackupDestinationRepository
}

func RegisterAutomation(rg *gin.RouterGroup, cfg AutomationConfig) {
	if cfg.AutomationTokens == nil || cfg.Key == nil {
		return
	}
	g := rg.Group("/automation",
		middleware.RequireAutomationHMAC(cfg.AutomationTokens, cfg.Key, cfg.Redis),
	)

	if cfg.Domains != nil {
		g.GET("/domains", middleware.RequireScope("read:domains"), func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()
			rows, _, err := cfg.Domains.List(ctx, repository.ListOptions{
				Limit: 200,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
				return
			}
			out := make([]map[string]any, 0, len(rows))
			for _, d := range rows {
				out = append(out, map[string]any{
					"id":         d.ID,
					"name":       d.Name,
					"user_id":    d.UserID,
					"is_enabled": d.IsEnabled,
				})
			}
			c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
		})
	}

	if cfg.Users != nil {
		g.GET("/users", middleware.RequireScope("read:users"), func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()
			rows, _, err := cfg.Users.List(ctx, repository.ListOptions{
				Limit: 200,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
				return
			}
			out := make([]map[string]any, 0, len(rows))
			for _, u := range rows {
				username := ""
				if u.Username != nil {
					username = *u.Username
				}
				pkg := ""
				if u.PackageID != nil {
					pkg = *u.PackageID
				}
				out = append(out, map[string]any{
					"id":         u.ID,
					"email":      u.Email,
					"username":   username,
					"package_id": pkg,
					"is_admin":   u.IsAdmin,
				})
			}
			c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
		})
	}

	if cfg.Applications != nil {
		g.GET("/applications", middleware.RequireScope("read:applications"), func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()
			rows, _, err := cfg.Applications.List(ctx, repository.ListOptions{
				Limit: 200,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
				return
			}
			out := make([]map[string]any, 0, len(rows))
			for _, a := range rows {
				out = append(out, map[string]any{
					"id":        a.ID,
					"app_type":  a.AppType,
					"domain_id": a.DomainID,
					"status":    a.Status,
				})
			}
			c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
		})
	}

	// read:status — fleet-monitor metrics (JAB-75). Reuses the M31 agent
	// collectors (system.info = host/load/mem/swap/disk, service_details =
	// unit health). Cached a few seconds so frequent polling doesn't hammer
	// the agent. Falls back to the bare healthy stub when no Agent is wired.
	metrics := newAutomationMetrics(cfg.Agent)
	g.GET("/status", middleware.RequireScope("read:status"), metrics.handle)

	// read:mail — mail-stack inventory for a fleet manager (JAB-77). Thin,
	// secrets-free: never password hashes / SSO tokens. Reuses the same
	// server-wide repos the admin pages use.
	if cfg.Mailboxes != nil {
		mg := g.Group("/mail", middleware.RequireScope("read:mail"))

		mg.GET("/mailboxes", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()
			rows, err := cfg.Mailboxes.ListAllWithDomain(ctx)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
				return
			}
			out := make([]map[string]any, 0, len(rows))
			for _, m := range rows {
				out = append(out, map[string]any{
					"email":            m.EmailCached,
					"domain":           m.DomainName,
					"owner":            m.UserUsername,
					"quota_bytes":      m.QuotaBytes,
					"last_usage_bytes": m.LastUsageBytes,
					"disabled":         m.IsDisabled,
				})
			}
			c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
		})

		mg.GET("/domains", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()
			rows, _, err := cfg.Domains.List(ctx, repository.ListOptions{Limit: 500})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
				return
			}
			out := make([]map[string]any, 0)
			for _, d := range rows {
				if !d.EmailEnabled {
					continue
				}
				out = append(out, map[string]any{"id": d.ID, "name": d.Name, "user_id": d.UserID})
			}
			c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
		})

		mg.GET("/summary", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()
			mailboxes, _ := cfg.Mailboxes.ListAllWithDomain(ctx)
			mailDomains := 0
			if cfg.Domains != nil {
				if doms, _, err := cfg.Domains.List(ctx, repository.ListOptions{Limit: 500}); err == nil {
					for _, d := range doms {
						if d.EmailEnabled {
							mailDomains++
						}
					}
				}
			}
			groups := 0
			if cfg.MailGroups != nil {
				if gr, err := cfg.MailGroups.ListAllWithDomain(ctx); err == nil {
					groups = len(gr)
				}
			}
			forwarders := 0
			if cfg.Forwarders != nil {
				if _, n, err := cfg.Forwarders.ListAll(ctx, repository.ListOptions{Limit: 1}); err == nil {
					forwarders = int(n)
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"mail_domains": mailDomains,
				"mailboxes":    len(mailboxes),
				"forwarders":   forwarders,
				"mail_groups":  groups,
			})
		})
	}

	// JAB-140 write layer + capabilities (additive; each route self-guards on
	// its repo/agent being present, mirroring the read routes).
	registerAutomationWrites(g, cfg)
}

// automationMetrics caches the aggregated server metrics for a short TTL so a
// fleet monitor polling every few seconds doesn't issue a fresh agent fan-out
// per request. Concurrency-safe.
type automationMetrics struct {
	agent    agent.AgentInterface
	mu       sync.Mutex
	cached   gin.H
	cachedAt time.Time
	ttl      time.Duration
}

func newAutomationMetrics(a agent.AgentInterface) *automationMetrics {
	return &automationMetrics{agent: a, ttl: 5 * time.Second}
}

func (m *automationMetrics) handle(c *gin.Context) {
	// No agent wired → keep the original healthy stub (never fail closed).
	if m.agent == nil {
		c.JSON(http.StatusOK, gin.H{"healthy": true, "time": time.Now().UTC()})
		return
	}

	m.mu.Lock()
	if m.cached != nil && time.Since(m.cachedAt) < m.ttl {
		out := m.cached
		m.mu.Unlock()
		c.JSON(http.StatusOK, out)
		return
	}
	m.mu.Unlock()

	ctx := c.Request.Context()
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	var (
		mu       sync.Mutex
		info     json.RawMessage
		services json.RawMessage
		cpu      json.RawMessage
		errs     = map[string]string{}
	)
	fetch := func(name, cmd string, dst *json.RawMessage) {
		g.Go(func() error {
			sub, cancel := context.WithTimeout(gctx, 8*time.Second)
			defer cancel()
			raw, err := m.agent.Call(sub, cmd, nil)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[name] = err.Error()
				return nil
			}
			*dst = raw
			return nil
		})
	}
	fetch("info", "system.info", &info)
	fetch("services", "system.service_details", &services)
	fetch("cpu", "system.cpu_usage", &cpu)
	_ = g.Wait()

	out := gin.H{
		"healthy": len(errs) == 0,
		"time":    time.Now().UTC(),
		"version": Version,
	}
	if info != nil {
		out["system"] = info
	}
	if services != nil {
		out["services"] = services
	}
	if cpu != nil {
		out["cpu"] = cpu
	}
	if len(errs) > 0 {
		out["errors"] = errs
	}

	m.mu.Lock()
	m.cached, m.cachedAt = out, time.Now()
	m.mu.Unlock()
	c.JSON(http.StatusOK, out)
}
