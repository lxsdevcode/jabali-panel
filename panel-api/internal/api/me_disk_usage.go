// me_disk_usage.go — per-user disk-usage breakdown (cPanel-style), for the
// tenant panel. Aggregates the three storage areas a hosting user occupies:
//
//   - Files     — the home directory, via the POSIX quota report
//                 (`user.limits.report`). Web content, logs, etc.
//   - Email     — sum of the user's mailboxes' last sampled usage
//                 (`mailboxes.last_usage_bytes`, kept fresh by the reconciler;
//                 no live agent call needed). Mail lives in Stalwart's store,
//                 NOT under /home, so it doesn't double-count files.
//   - Databases — per-database size via `db.size` (MariaDB). PostgreSQL has no
//                 size verb yet, so those report 0 for now.
//
//	GET /api/v1/me/disk-usage -> { total_bytes, quota_bytes, files, email, databases }
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/agent"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// DiskUsageConfig carries the repos + agent the disk-usage aggregator needs.
type DiskUsageConfig struct {
	Users      repository.UserRepository
	Domains    repository.DomainRepository
	Mailboxes  repository.MailboxRepository
	Databases  repository.DatabaseRepository
	Agent      agent.AgentInterface
	QuotaMount string
}

// RegisterMeDiskUsageRoutes mounts GET /me/disk-usage on a group that already
// carries auth.
func RegisterMeDiskUsageRoutes(g *gin.RouterGroup, cfg DiskUsageConfig) {
	if cfg.Users == nil || cfg.Domains == nil || cfg.Mailboxes == nil || cfg.Databases == nil {
		return
	}
	h := &meDiskUsageHandler{cfg: cfg}
	g.GET("/me/disk-usage", h.get)
}

type meDiskUsageHandler struct{ cfg DiskUsageConfig }

type diskUsageItem struct {
	Name   string `json:"name"`
	Bytes  uint64 `json:"bytes"`
	Engine string `json:"engine,omitempty"`
}

type diskUsageCategory struct {
	Bytes uint64          `json:"bytes"`
	Items []diskUsageItem `json:"items"`
}

type diskUsageResponse struct {
	TotalBytes uint64            `json:"total_bytes"`
	QuotaBytes uint64            `json:"quota_bytes"` // home quota limit; 0 = unlimited/unknown
	Files      diskUsageCategory `json:"files"`
	Email      diskUsageCategory `json:"email"`
	Databases  diskUsageCategory `json:"databases"`
}

// diskUsageListLimit bounds the per-category enumeration. A single hosting
// user with thousands of domains/dbs is pathological; this keeps the page
// bounded rather than unbounded-querying.
const diskUsageListLimit = 2000

func (h *meDiskUsageHandler) get(c *gin.Context) {
	ctx := c.Request.Context()
	claims := ginctx.Claims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.cfg.Users.FindByID(ctx, claims.UserID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user_not_found"})
		return
	}
	opts := repository.ListOptions{Limit: diskUsageListLimit}

	resp := diskUsageResponse{
		Files:     diskUsageCategory{Items: []diskUsageItem{}},
		Email:     diskUsageCategory{Items: []diskUsageItem{}},
		Databases: diskUsageCategory{Items: []diskUsageItem{}},
	}

	// --- Files: home-directory quota report (admins have no Linux account) ---
	if user.Username != nil && *user.Username != "" && h.cfg.Agent != nil && h.cfg.QuotaMount != "" {
		used, limit := h.homeUsage(ctx, *user.Username)
		resp.Files.Bytes = used
		resp.QuotaBytes = limit
		resp.Files.Items = append(resp.Files.Items, diskUsageItem{Name: "Home directory", Bytes: used})
	}

	// --- Email: sum cached mailbox usage across the user's domains ---
	if domains, _, derr := h.cfg.Domains.ListByUserID(ctx, claims.UserID, opts); derr == nil {
		for i := range domains {
			mbs, _, merr := h.cfg.Mailboxes.ListByDomainID(ctx, domains[i].ID, opts)
			if merr != nil {
				continue
			}
			for j := range mbs {
				resp.Email.Bytes += mbs[j].LastUsageBytes
				resp.Email.Items = append(resp.Email.Items, diskUsageItem{
					Name:  mbs[j].EmailCached,
					Bytes: mbs[j].LastUsageBytes,
				})
			}
		}
	}

	// --- Databases: per-db size (MariaDB; PostgreSQL has no size verb yet) ---
	if dbs, _, derr := h.cfg.Databases.ListByUserID(ctx, claims.UserID, opts); derr == nil {
		for i := range dbs {
			var sz uint64
			if dbs[i].Engine == "mariadb" && h.cfg.Agent != nil {
				sz = h.dbSize(ctx, dbs[i].Name)
			}
			resp.Databases.Bytes += sz
			resp.Databases.Items = append(resp.Databases.Items, diskUsageItem{
				Name:   dbs[i].Name,
				Bytes:  sz,
				Engine: dbs[i].Engine,
			})
		}
	}

	resp.TotalBytes = resp.Files.Bytes + resp.Email.Bytes + resp.Databases.Bytes
	c.JSON(http.StatusOK, resp)
}

// homeUsage returns (usedBytes, limitBytes) from the agent's quota report.
// Best-effort: any failure yields (0, 0).
func (h *meDiskUsageHandler) homeUsage(ctx context.Context, username string) (uint64, uint64) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	raw, err := h.cfg.Agent.Call(cctx, "user.limits.report", map[string]any{
		"username":    username,
		"quota_mount": h.cfg.QuotaMount,
	})
	if err != nil {
		return 0, 0
	}
	var v struct {
		Disk struct {
			UsedKB  uint64 `json:"used_kb"`
			LimitKB uint64 `json:"limit_kb"`
		} `json:"disk"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return 0, 0
	}
	return v.Disk.UsedKB * 1024, v.Disk.LimitKB * 1024
}

// dbSize returns a MariaDB database's size in bytes (0 on any error).
func (h *meDiskUsageHandler) dbSize(ctx context.Context, name string) uint64 {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	raw, err := h.cfg.Agent.Call(cctx, "db.size", map[string]any{"db_name": name})
	if err != nil {
		return 0
	}
	var v struct {
		SizeBytes int64 `json:"size_bytes"`
	}
	if json.Unmarshal(raw, &v) != nil || v.SizeBytes < 0 {
		return 0
	}
	return uint64(v.SizeBytes)
}
