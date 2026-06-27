package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/agent"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/ginctx"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/middleware"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

type ServerSettingsHandlerConfig struct {
	Repo  repository.ServerSettingsRepository
	Agent agent.AgentInterface
	Log   *slog.Logger
}

// RegisterServerSettingsRoutes mounts server settings endpoints under the
// given group (typically /api/v1 with auth middleware already applied).
func RegisterServerSettingsRoutes(g *gin.RouterGroup, cfg ServerSettingsHandlerConfig) {
	h := &serverSettingsHandler{cfg: cfg}
	admin := g.Group("/admin/settings")
	admin.Use(middleware.RequireAdmin())
	admin.GET("", h.get)
	admin.PATCH("", h.update)
	admin.GET("/python-runtime", h.pythonRuntimeStatus)
	// GH #243: reveal / rotate the Stalwart admin GUI login (admin:<token>).
	admin.POST("/stalwart-admin-credential", h.stalwartAdminCredential)
}

type serverSettingsHandler struct{ cfg ServerSettingsHandlerConfig }

func (h *serverSettingsHandler) get(c *gin.Context) {
	ctx := c.Request.Context()
	s, err := h.cfg.Repo.Get(ctx)
	if errors.Is(err, repository.ErrNotFound) {
		// First boot before the seed has run, or a brand-new install
		// where config.toml had no [server] identity to seed from.
		// Return an empty shell instead of 500 so the form loads clean.
		s = &models.ServerSettings{ID: 1}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	// Timezone fallback: when the DB column is empty (admin never picked
	// one yet), surface the OS-configured zone so the UI dropdown shows
	// the actual current value instead of blank. Doesn't write back —
	// the row stays empty until the admin saves explicitly.
	if s.Timezone == "" && h.cfg.Agent != nil {
		infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if raw, err := h.cfg.Agent.Call(infoCtx, "system.info", nil); err == nil {
			var info struct {
				Timezone string `json:"timezone"`
			}
			if jsonErr := json.Unmarshal(raw, &info); jsonErr == nil && info.Timezone != "" {
				s.Timezone = info.Timezone
			}
		} else {
			h.cfg.Log.Debug("agent system.info failed during timezone fallback", "err", err)
		}
	}

	c.JSON(http.StatusOK, s)
}

type updateServerSettingsRequest struct {
	Hostname                     *string `json:"hostname,omitempty"`
	PublicIPv4                   *string `json:"public_ipv4,omitempty"`
	PublicIPv6                   *string `json:"public_ipv6,omitempty"`
	NS1Name                      *string `json:"ns1_name,omitempty"`
	NS1IPv4                      *string `json:"ns1_ipv4,omitempty"`
	NS2Name                      *string `json:"ns2_name,omitempty"`
	NS2IPv4                      *string `json:"ns2_ipv4,omitempty"`
	AdminEmail                   *string `json:"admin_email,omitempty"`
	Timezone                     *string `json:"timezone,omitempty"`
	SSHPort                      *uint16 `json:"ssh_port,omitempty"`
	SSHPasswordAuth              *bool   `json:"ssh_password_auth,omitempty"`
	SSHUserPasswordAuth          *bool   `json:"ssh_user_password_auth,omitempty"`
	PanelBrandText               *string `json:"panel_brand_text,omitempty"`
	PanelFontSize                *string `json:"panel_font_size,omitempty"`
	PanelPrimaryColor            *string `json:"panel_primary_color,omitempty"`
	PanelAccentColor             *string `json:"panel_accent_color,omitempty"`
	PanelSuccessColor            *string `json:"panel_success_color,omitempty"`
	PanelWarningColor            *string `json:"panel_warning_color,omitempty"`
	PanelErrorColor              *string `json:"panel_error_color,omitempty"`
	PanelInfoColor               *string `json:"panel_info_color,omitempty"`
	PanelLinkColor               *string `json:"panel_link_color,omitempty"`
	PanelLightTopbarColor        *string `json:"panel_light_topbar_color,omitempty"`
	PanelDarkTopbarColor         *string `json:"panel_dark_topbar_color,omitempty"`
	PanelLightBgColor            *string `json:"panel_light_bg_color,omitempty"`
	PanelDarkBgColor             *string `json:"panel_dark_bg_color,omitempty"`
	PanelLightContainerColor     *string `json:"panel_light_container_color,omitempty"`
	PanelDarkContainerColor      *string `json:"panel_dark_container_color,omitempty"`
	PanelLightTextColor          *string `json:"panel_light_text_color,omitempty"`
	PanelDarkTextColor           *string `json:"panel_dark_text_color,omitempty"`
	ReleaseChannel               *string `json:"release_channel,omitempty"`
	// DNS record-type permissions (GH #466): supply either a named preset
	// (permissive | hosting-safe | locked-down) or the full per-type matrix.
	DNSUserRecordPolicy       *models.DNSUserRecordPolicy `json:"dns_user_record_policy,omitempty"`
	DNSUserRecordPolicyPreset *string                     `json:"dns_user_record_policy_preset,omitempty"`
	DiskQuotaEnabled             *bool   `json:"disk_quota_enabled,omitempty"`
	RootTerminalEnabled          *bool   `json:"root_terminal_enabled,omitempty"`
	BandwidthQuotaEnforceEnabled *bool   `json:"bandwidth_quota_enforce_enabled,omitempty"`
	UploadMaxSizeMB              *uint32 `json:"upload_max_size_mb,omitempty"`

	// M13 SSH shell sandbox.
	SSHSandboxMode            *string `json:"ssh_sandbox_mode,omitempty"`
	DefaultNspawnImageVersion *string `json:"default_nspawn_image_version,omitempty"`

	// M30.1 backup concurrency. Per-schedule retention is the source
	// of truth — server-wide keep_* fields exist on the row but are
	// no longer writeable from the API.
	BackupMaxConcurrentJobs *uint32 `json:"backup_max_concurrent_jobs,omitempty"`

	// M37 PostgreSQL parity (ADR-0091). PostgresEnabled gates the
	// /databases POST handler from accepting engine="postgres" + the
	// reconciler from starting the postgresql service.
	PostgresEnabled *bool `json:"postgres_enabled,omitempty"`
	// M48 docker-app marketplace opt-in. flip true -> agent installs
	// docker via install_docker_engine; flip false -> agent stops +
	// disables docker units (data kept).
	DockerMarketplaceEnabled *bool `json:"docker_marketplace_enabled,omitempty"`
	// Admin opt-in: expose tenant Docker Apps to users (GH docker-opt-in).
	DockerAppsForUsersEnabled *bool `json:"docker_apps_for_users_enabled,omitempty"`
	// Admin-curated CSV of tenant-exposed Docker app slugs (empty = all eligible).
	DockerTenantApps *string `json:"docker_tenant_apps,omitempty"`
	// ADR-0131: Python Application Manager opt-in.
	PythonAppsEnabled *bool `json:"python_apps_enabled,omitempty"`
	// GH #243 (ADR-0142): opt-in Stalwart WebAdmin exposure + IP allowlist.
	StalwartWebadminEnabled    *bool   `json:"stalwart_webadmin_enabled,omitempty"`
	StalwartWebadminAllowCIDRs *string `json:"stalwart_webadmin_allow_cidrs,omitempty"`
	// Transient (not persisted): force a fresh gateway credential.
	PostgresMaxConnectionsPerUser *uint16 `json:"postgres_max_connections_per_user,omitempty"`

	// M35 SSRF override. When true, migrate.ValidateHost accepts
	// RFC1918 / loopback / link-local targets. Operator opts in for
	// local-network test migrations; default-deny remains the safe
	// production stance. ADR-0095 decision 8.
	MigrationAllowPrivateHosts *bool `json:"migration_allow_private_hosts,omitempty"`

	// WorkingFolder is the on-disk base for migrations + backups.
	// Default '/var/lib/jabali'. Must be an absolute path. Operator
	// retargets to a larger disk by setting + symlinking the legacy
	// jabali-migrations / jabali-backups dirs underneath.
	WorkingFolder *string `json:"working_folder,omitempty"`

	// DefaultDNSTTL is the server-wide default for new DNS records
	// (seconds). Operator-settable via Server Settings → DNS. Range
	// 60–86400 (1 min to 1 day); values outside that range are
	// rejected with HTTP 422.
	DefaultDNSTTL *uint32 `json:"default_dns_ttl,omitempty"`

	// CrowdsecSensitivity preset. relaxed | balanced | strict.
	// Applied via agent verb security.crowdsec.sensitivity.apply
	// which writes scenario/profile/anomaly drop-ins.
	CrowdsecSensitivity *string `json:"crowdsec_sensitivity,omitempty"`

	// CrowdsecBouncerMode preset. live | stream. Applied via agent verb
	// security.crowdsec.bouncer.mode.apply which rewrites MODE= in the
	// nginx-bouncer conf + reloads nginx.
	CrowdsecBouncerMode *string `json:"crowdsec_bouncer_mode,omitempty"`

	// M55 Nginx Settings tab. All optional; any present field triggers a
	// re-render + nginx.tunables.apply dispatch (re-sync semantics, like SSH).
	NginxClientMaxBodySize   *string `json:"nginx_client_max_body_size,omitempty"`
	NginxKeepaliveTimeout    *string `json:"nginx_keepalive_timeout,omitempty"`
	NginxServerTokens        *bool   `json:"nginx_server_tokens,omitempty"`
	NginxGzip                *bool   `json:"nginx_gzip,omitempty"`
	NginxClientBodyTimeout   *string `json:"nginx_client_body_timeout,omitempty"`
	NginxClientHeaderTimeout *string `json:"nginx_client_header_timeout,omitempty"`
	NginxSendTimeout         *string `json:"nginx_send_timeout,omitempty"`
	NginxProxyConnectTimeout *string `json:"nginx_proxy_connect_timeout,omitempty"`
	NginxProxyReadTimeout    *string `json:"nginx_proxy_read_timeout,omitempty"`
	NginxProxySendTimeout    *string `json:"nginx_proxy_send_timeout,omitempty"`
	NginxWorkerProcesses     *string `json:"nginx_worker_processes,omitempty"`
	NginxWorkerConnections   *uint32 `json:"nginx_worker_connections,omitempty"`
	NginxCustomHTTP          *string `json:"nginx_custom_http,omitempty"`
}

func (h *serverSettingsHandler) update(c *gin.Context) {
	claims := ginctx.Claims(c)
	if claims == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	var req updateServerSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "validation_failed", "detail": err.Error()})
		return
	}

	ctx := c.Request.Context()
	current, err := h.cfg.Repo.Get(ctx)
	if errors.Is(err, repository.ErrNotFound) {
		// No row yet (e.g. seed hadn't run or config was empty at boot).
		// Treat PATCH as initial save so the first admin edit lands cleanly.
		current = &models.ServerSettings{ID: 1}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	prevHostname := current.Hostname
	prevPostgresEnabled := current.PostgresEnabled
	prevPanelBrandText := current.PanelBrandText
	prevDockerEnabled := current.DockerMarketplaceEnabled
	prevDockerForUsers := current.DockerAppsForUsersEnabled
	prevPythonEnabled := current.PythonAppsEnabled
	prevWebadminEnabled := current.StalwartWebadminEnabled
	prevWebadminCIDRs := current.StalwartWebadminAllowCIDRs
	prevTimezone := current.Timezone
	prevSSHPort := current.SSHPort
	prevSSHPasswordAuth := current.SSHPasswordAuth
	prevSSHUserPasswordAuth := current.SSHUserPasswordAuth
	prevSSHSandboxMode := current.SSHSandboxMode
	prevCrowdsecSensitivity := current.CrowdsecSensitivity
	prevCrowdsecBouncerMode := current.CrowdsecBouncerMode
	prevDefaultNspawnImageVersion := current.DefaultNspawnImageVersion

	if req.Hostname != nil {
		current.Hostname = strings.TrimSpace(*req.Hostname)
	}
	if req.PublicIPv4 != nil {
		current.PublicIPv4 = strings.TrimSpace(*req.PublicIPv4)
	}
	if req.PublicIPv6 != nil {
		current.PublicIPv6 = strings.TrimSpace(*req.PublicIPv6)
	}
	if req.NS1Name != nil {
		current.NS1Name = strings.TrimSpace(*req.NS1Name)
	}
	if req.NS1IPv4 != nil {
		current.NS1IPv4 = strings.TrimSpace(*req.NS1IPv4)
	}
	if req.NS2Name != nil {
		current.NS2Name = strings.TrimSpace(*req.NS2Name)
	}
	if req.NS2IPv4 != nil {
		current.NS2IPv4 = strings.TrimSpace(*req.NS2IPv4)
	}
	if req.AdminEmail != nil {
		current.AdminEmail = strings.TrimSpace(*req.AdminEmail)
	}
	if req.Timezone != nil {
		current.Timezone = strings.TrimSpace(*req.Timezone)
	}
	if req.SSHPort != nil {
		current.SSHPort = *req.SSHPort
	}
	if req.DefaultDNSTTL != nil {
		current.DefaultDNSTTL = *req.DefaultDNSTTL
	}
	if req.CrowdsecSensitivity != nil {
		v := strings.TrimSpace(*req.CrowdsecSensitivity)
		switch v {
		case "relaxed", "balanced", "strict":
			current.CrowdsecSensitivity = v
		default:
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "invalid_crowdsec_sensitivity",
				"message": "must be one of: relaxed, balanced, strict",
			})
			return
		}
	}
	if req.CrowdsecBouncerMode != nil {
		v := strings.TrimSpace(*req.CrowdsecBouncerMode)
		switch v {
		case "live", "stream":
			current.CrowdsecBouncerMode = v
		default:
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "invalid_crowdsec_bouncer_mode",
				"message": "must be one of: live, stream",
			})
			return
		}
	}
	if req.SSHPasswordAuth != nil {
		current.SSHPasswordAuth = *req.SSHPasswordAuth
	}
	if req.SSHUserPasswordAuth != nil {
		current.SSHUserPasswordAuth = *req.SSHUserPasswordAuth
	}
	if req.PanelBrandText != nil {
		current.PanelBrandText = strings.TrimSpace(*req.PanelBrandText)
	}
	if req.PanelFontSize != nil {
		v := strings.TrimSpace(*req.PanelFontSize)
		switch v {
		case "small", "medium", "large":
			current.PanelFontSize = v
		default:
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "invalid_panel_font_size",
				"message": "must be one of: small, medium, large",
			})
			return
		}
	}
	// "Look and feel" colors — each empty-or-hex, validated uniformly.
	validateColor := func(reqVal *string, dst *string, name string) bool {
		if reqVal == nil {
			return true
		}
		v := strings.TrimSpace(*reqVal)
		if !isValidHexColor(v) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "invalid_" + name,
				"message": "must be empty or a hex color (#rgb, #rrggbb, #rrggbbaa)",
			})
			return false
		}
		*dst = v
		return true
	}
	if !validateColor(req.PanelPrimaryColor, &current.PanelPrimaryColor, "panel_primary_color") {
		return
	}
	if !validateColor(req.PanelAccentColor, &current.PanelAccentColor, "panel_accent_color") {
		return
	}
	if !validateColor(req.PanelSuccessColor, &current.PanelSuccessColor, "panel_success_color") {
		return
	}
	if !validateColor(req.PanelWarningColor, &current.PanelWarningColor, "panel_warning_color") {
		return
	}
	if !validateColor(req.PanelErrorColor, &current.PanelErrorColor, "panel_error_color") {
		return
	}
	if !validateColor(req.PanelInfoColor, &current.PanelInfoColor, "panel_info_color") {
		return
	}
	if !validateColor(req.PanelLinkColor, &current.PanelLinkColor, "panel_link_color") {
		return
	}
	if !validateColor(req.PanelLightTopbarColor, &current.PanelLightTopbarColor, "panel_light_topbar_color") {
		return
	}
	if !validateColor(req.PanelDarkTopbarColor, &current.PanelDarkTopbarColor, "panel_dark_topbar_color") {
		return
	}
	if !validateColor(req.PanelLightBgColor, &current.PanelLightBgColor, "panel_light_bg_color") {
		return
	}
	if !validateColor(req.PanelDarkBgColor, &current.PanelDarkBgColor, "panel_dark_bg_color") {
		return
	}
	if !validateColor(req.PanelLightContainerColor, &current.PanelLightContainerColor, "panel_light_container_color") {
		return
	}
	if !validateColor(req.PanelDarkContainerColor, &current.PanelDarkContainerColor, "panel_dark_container_color") {
		return
	}
	if !validateColor(req.PanelLightTextColor, &current.PanelLightTextColor, "panel_light_text_color") {
		return
	}
	if !validateColor(req.PanelDarkTextColor, &current.PanelDarkTextColor, "panel_dark_text_color") {
		return
	}
	if req.ReleaseChannel != nil {
		ch := strings.TrimSpace(*req.ReleaseChannel)
		if ch != "stable" && ch != "development" {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "invalid_release_channel",
				"message": "must be 'stable' or 'development'",
			})
			return
		}
		current.ReleaseChannel = ch
	}
	if req.DNSUserRecordPolicyPreset != nil {
		pol, ok := models.DNSPolicyPreset(*req.DNSUserRecordPolicyPreset)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":   "invalid_dns_policy_preset",
				"message": "must be 'permissive', 'hosting-safe', or 'locked-down'",
			})
			return
		}
		current.DNSUserRecordPolicy = pol
	} else if req.DNSUserRecordPolicy != nil {
		// Sanitize: drop NS/SOA and unknown types so a PUT can't grant a
		// policy for an admin-only record type.
		current.DNSUserRecordPolicy = req.DNSUserRecordPolicy.Sanitize()
	}
	if req.DiskQuotaEnabled != nil {
		current.DiskQuotaEnabled = *req.DiskQuotaEnabled
	}
	if req.RootTerminalEnabled != nil {
		current.RootTerminalEnabled = *req.RootTerminalEnabled
	}
	if req.BandwidthQuotaEnforceEnabled != nil {
		current.BandwidthQuotaEnforceEnabled = *req.BandwidthQuotaEnforceEnabled
	}
	if req.UploadMaxSizeMB != nil {
		current.UploadMaxSizeMB = *req.UploadMaxSizeMB
	}
	if req.SSHSandboxMode != nil {
		current.SSHSandboxMode = strings.TrimSpace(*req.SSHSandboxMode)
	}
	if req.DefaultNspawnImageVersion != nil {
		// Reject empty string — keeps UIs from clobbering a valid pin
		// when the field is hidden / not yet loaded on submit. To clear,
		// the operator must change the column DEFAULT in a migration.
		v := strings.TrimSpace(*req.DefaultNspawnImageVersion)
		if v != "" {
			current.DefaultNspawnImageVersion = v
		}
	}
	if req.BackupMaxConcurrentJobs != nil {
		// 0 acts as "use default" in the dispatcher. Cap at 64 — even a
		// single restic chain at 8 cores tops out the IO ceiling well
		// before then.
		v := *req.BackupMaxConcurrentJobs
		if v > 64 {
			v = 64
		}
		current.BackupMaxConcurrentJobs = v
	}
	if req.PostgresEnabled != nil {
		current.PostgresEnabled = *req.PostgresEnabled
	}
	if req.DockerMarketplaceEnabled != nil {
		current.DockerMarketplaceEnabled = *req.DockerMarketplaceEnabled
	}
	if req.DockerAppsForUsersEnabled != nil {
		current.DockerAppsForUsersEnabled = *req.DockerAppsForUsersEnabled
	}
	if req.DockerTenantApps != nil {
		current.DockerTenantApps = strings.TrimSpace(*req.DockerTenantApps)
	}
	if req.PythonAppsEnabled != nil {
		current.PythonAppsEnabled = *req.PythonAppsEnabled
	}
	if req.StalwartWebadminEnabled != nil {
		current.StalwartWebadminEnabled = *req.StalwartWebadminEnabled
	}
	if req.StalwartWebadminAllowCIDRs != nil {
		current.StalwartWebadminAllowCIDRs = strings.TrimSpace(*req.StalwartWebadminAllowCIDRs)
	}
	if req.MigrationAllowPrivateHosts != nil {
		current.MigrationAllowPrivateHosts = *req.MigrationAllowPrivateHosts
	}
	if req.PostgresMaxConnectionsPerUser != nil {
		v := *req.PostgresMaxConnectionsPerUser
		if v == 0 {
			v = 25
		}
		if v > 1000 {
			v = 1000
		}
		current.PostgresMaxConnectionsPerUser = v
	}
	if req.WorkingFolder != nil {
		v := strings.TrimSpace(*req.WorkingFolder)
		if v == "" {
			v = "/var/lib/jabali"
		}
		current.WorkingFolder = v
	}

	// M55 nginx tunables. nginxTouched drives the re-sync dispatch below.
	nginxTouched := req.NginxClientMaxBodySize != nil || req.NginxKeepaliveTimeout != nil ||
		req.NginxServerTokens != nil || req.NginxGzip != nil ||
		req.NginxClientBodyTimeout != nil || req.NginxClientHeaderTimeout != nil ||
		req.NginxSendTimeout != nil || req.NginxProxyConnectTimeout != nil ||
		req.NginxProxyReadTimeout != nil || req.NginxProxySendTimeout != nil ||
		req.NginxWorkerProcesses != nil || req.NginxWorkerConnections != nil ||
		req.NginxCustomHTTP != nil
	if req.NginxClientMaxBodySize != nil {
		current.NginxClientMaxBodySize = strings.TrimSpace(*req.NginxClientMaxBodySize)
	}
	if req.NginxKeepaliveTimeout != nil {
		current.NginxKeepaliveTimeout = strings.TrimSpace(*req.NginxKeepaliveTimeout)
	}
	if req.NginxServerTokens != nil {
		current.NginxServerTokens = *req.NginxServerTokens
	}
	if req.NginxGzip != nil {
		current.NginxGzip = *req.NginxGzip
	}
	if req.NginxClientBodyTimeout != nil {
		current.NginxClientBodyTimeout = strings.TrimSpace(*req.NginxClientBodyTimeout)
	}
	if req.NginxClientHeaderTimeout != nil {
		current.NginxClientHeaderTimeout = strings.TrimSpace(*req.NginxClientHeaderTimeout)
	}
	if req.NginxSendTimeout != nil {
		current.NginxSendTimeout = strings.TrimSpace(*req.NginxSendTimeout)
	}
	if req.NginxProxyConnectTimeout != nil {
		current.NginxProxyConnectTimeout = strings.TrimSpace(*req.NginxProxyConnectTimeout)
	}
	if req.NginxProxyReadTimeout != nil {
		current.NginxProxyReadTimeout = strings.TrimSpace(*req.NginxProxyReadTimeout)
	}
	if req.NginxProxySendTimeout != nil {
		current.NginxProxySendTimeout = strings.TrimSpace(*req.NginxProxySendTimeout)
	}
	if req.NginxWorkerProcesses != nil {
		current.NginxWorkerProcesses = strings.TrimSpace(*req.NginxWorkerProcesses)
	}
	if req.NginxWorkerConnections != nil {
		current.NginxWorkerConnections = *req.NginxWorkerConnections
	}
	if req.NginxCustomHTTP != nil {
		current.NginxCustomHTTP = strings.TrimSpace(*req.NginxCustomHTTP)
	}

	// Validate — reject obviously bad input so we don't persist garbage.
	if err := validateServerSettings(current); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_settings", "detail": err.Error()})
		return
	}

	// Admin opt-in for tenant Docker apps. Enabling requires the engine; the
	// host tenant setup itself is dispatched AFTER persist (below), detached
	// from this request — see the background goroutine.
	if current.DockerAppsForUsersEnabled != prevDockerForUsers &&
		current.DockerAppsForUsersEnabled && !current.DockerMarketplaceEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "docker_not_installed", "detail": "enable the Docker marketplace first"})
		return
	}

	if err := h.cfg.Repo.Upsert(ctx, current); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	// Apply hostname to the OS via agent, best-effort. If the agent isn't
	// reachable we still report success — the DB has the truth and a later
	// reconciliation can sync.
	if current.Hostname != prevHostname && h.cfg.Agent != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "system.set_hostname", map[string]any{
				"hostname": current.Hostname,
			}); err != nil {
				h.cfg.Log.Error("agent set_hostname failed", "err", err)
			}
		}()
	}

	// M37 Phase 4: Postgres opt-in. flip true → install + start;
	// flip false → stop + disable (data preserved). Dispatch in
	// the background — install can take up to ~60s on apt-get and
	// the PATCH should not block the UI for that long. The DB row
	// already reflects the operator intent; reconcile will eventually
	// catch up on retry.
	if current.PostgresEnabled != prevPostgresEnabled && h.cfg.Agent != nil {
		go func(target bool) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			method := "db.postgres.disable"
			if target {
				method = "db.postgres.install"
			}
			if _, err := h.cfg.Agent.Call(bgCtx, method, map[string]any{}); err != nil {
				h.cfg.Log.Error("agent postgres lifecycle failed",
					"method", method, "err", err)
			}
		}(current.PostgresEnabled)
	}

	// GH #200: brand text drives Bulwark webmail branding (app name +
	// login company name) server-wide. Push it when it changes; the
	// agent restarts jabali-webmail only on a real change.
	if current.PanelBrandText != prevPanelBrandText && h.cfg.Agent != nil {
		go func(brandText string) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "webmail.branding.apply", map[string]any{
				"brand_text": brandText,
			}); err != nil {
				h.cfg.Log.Warn("webmail branding sync failed", "err", err)
			}
		}(current.PanelBrandText)
	}

	// M48 docker marketplace opt-in. Mirrors the postgres pattern
	// above: flip true -> install_docker_engine; flip false -> stop
	// + disable units, data under /var/lib/jabali/docker-apps left
	// intact. Background dispatch so the operator's PATCH does not
	// block on apt-get.
	if current.DockerMarketplaceEnabled != prevDockerEnabled && h.cfg.Agent != nil {
		go func(target bool) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			method := "docker.disable"
			if target {
				method = "docker.install"
			}
			if _, err := h.cfg.Agent.Call(bgCtx, method, map[string]any{}); err != nil {
				h.cfg.Log.Error("agent docker lifecycle failed",
					"method", method, "err", err)
			}
		}(current.DockerMarketplaceEnabled)
	}

	// Tenant Docker opt-in: run the host tenant setup (userns-remap, ~minutes)
	// or teardown DETACHED from this request, so a client disconnect can't kill
	// a half-done docker retrofit. The host flag the agent writes is the source
	// of truth for the docker_apps_user_enabled capability, so a failed enable
	// simply leaves the tab hidden (no DB/host drift to reconcile).
	if current.DockerAppsForUsersEnabled != prevDockerForUsers && h.cfg.Agent != nil {
		go func(target bool) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "docker.tenant_set", map[string]any{"enabled": target}); err != nil {
				h.cfg.Log.Error("agent docker.tenant_set failed", "enabled", target, "err", err)
				// On a failed ENABLE (e.g. unprivileged LXC — GH #272), revert the
				// DB flag so the UI toggle reflects reality (host setup did NOT
				// happen) instead of showing "enabling…" forever. Fresh context:
				// bgCtx may already be exhausted from the agent call/timeout.
				if target {
					rctx, rcancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer rcancel()
					if cur, gerr := h.cfg.Repo.Get(rctx); gerr == nil && cur != nil {
						cur.DockerAppsForUsersEnabled = false
						if uerr := h.cfg.Repo.Upsert(rctx, cur); uerr != nil {
							h.cfg.Log.Error("revert docker_apps_for_users after failed enable", "err", uerr)
						}
					}
				}
			}
		}(current.DockerAppsForUsersEnabled)
	}

	// ADR-0131: install the Python app runtime prerequisites on enable
	// (mirrors the docker/postgres opt-ins). Disable leaves packages in
	// place. Background dispatch so the PATCH does not block on apt.
	if current.PythonAppsEnabled != prevPythonEnabled && current.PythonAppsEnabled && h.cfg.Agent != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "app.python.install_runtime", map[string]any{}); err != nil {
				h.cfg.Log.Error("agent python runtime install failed", "err", err)
			}
		}()
	}

	// Apply timezone to the OS via agent if changed and not empty.
	if current.Timezone != prevTimezone && current.Timezone != "" && h.cfg.Agent != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "system.set_timezone", map[string]any{
				"timezone": current.Timezone,
			}); err != nil {
				h.cfg.Log.Error("agent set_timezone failed", "err", err)
			}
		}()
	}

	// Apply SSH config to the OS via agent if any SSH-affecting field changed.
	// The agent rewrites both /etc/ssh/sshd_config.d/jabali-sshd.conf (global,
	// affects root/admin) and /etc/ssh/sshd_config.d/jabali-sftp.conf (M12
	// Match Group block, affects hosting users), validates with sshd -t,
	// and reloads sshd. See ADR-0028 for the M12 jabali-sftp design.
	// Re-apply when ANY SSH-section field is in the request, even if
	// its value matches the DB row. Without this an operator hitting a
	// drifted file (DB=1, file=no) couldn't fix it by re-submitting —
	// `current==prev` short-circuited the agent.Call. With this,
	// re-saving Server Settings always re-syncs the file. The startup
	// reconcile in serve.go handles the boot-time case; this handles
	// the operator-driven re-sync without forcing a toggle off→on.
	sshFieldTouched := req.SSHPort != nil || req.SSHPasswordAuth != nil || req.SSHUserPasswordAuth != nil
	if (sshFieldTouched ||
		current.SSHPort != prevSSHPort ||
		current.SSHPasswordAuth != prevSSHPasswordAuth ||
		current.SSHUserPasswordAuth != prevSSHUserPasswordAuth) && h.cfg.Agent != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "system.set_ssh_config", map[string]any{
				"port":               current.SSHPort,
				"password_auth":      current.SSHPasswordAuth,
				"user_password_auth": current.SSHUserPasswordAuth,
			}); err != nil {
				h.cfg.Log.Error("agent set_ssh_config failed", "err", err)
			}
		}()
	}

	// Sandbox mode + default-image flips: write the matching files on
	// disk so the wrapper picks them up on the next connect. No sshd
	// reload needed (wrapper reads on every exec).
	if (current.SSHSandboxMode != prevSSHSandboxMode ||
		current.DefaultNspawnImageVersion != prevDefaultNspawnImageVersion) && h.cfg.Agent != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "system.set_ssh_sandbox_mode", map[string]any{
				"mode":          current.SSHSandboxMode,
				"default_image": current.DefaultNspawnImageVersion,
			}); err != nil {
				h.cfg.Log.Error("agent set_ssh_sandbox_mode failed", "err", err)
			}
		}()
	}

	// CrowdSec sensitivity preset: re-apply when either the operator
	// explicitly touched the field OR the persisted level diverged
	// from what was on disk (drift after a manual edit / reset).
	if (req.CrowdsecSensitivity != nil || current.CrowdsecSensitivity != prevCrowdsecSensitivity) && h.cfg.Agent != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "security.crowdsec.sensitivity.apply", map[string]any{
				"level": current.CrowdsecSensitivity,
			}); err != nil {
				h.cfg.Log.Error("agent crowdsec sensitivity apply failed", "err", err)
			}
		}()
	}

	// CrowdSec nginx-bouncer mode: rewrite MODE= + reload nginx when the
	// operator flipped the preset (or the persisted value diverged from
	// what's on disk after a manual edit).
	if (req.CrowdsecBouncerMode != nil || current.CrowdsecBouncerMode != prevCrowdsecBouncerMode) && h.cfg.Agent != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "security.crowdsec.bouncer.mode.apply", map[string]any{
				"mode": current.CrowdsecBouncerMode,
			}); err != nil {
				h.cfg.Log.Error("agent crowdsec bouncer mode apply failed", "err", err)
			}
		}()
	}

	// M55 nginx tunables: re-render the http-scope fragment + patch worker
	// lines whenever the operator touched the Nginx form. Background dispatch
	// (atomic-write + nginx -t gate + reload happen agent-side). Re-sync
	// semantics: re-saving the form always re-applies, healing on-disk drift.
	if nginxTouched && h.cfg.Agent != nil {
		go func(s models.ServerSettings) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if _, err := h.cfg.Agent.Call(bgCtx, "nginx.tunables.apply", map[string]any{
				"client_max_body_size":  s.NginxClientMaxBodySize,
				"keepalive_timeout":     s.NginxKeepaliveTimeout,
				"server_tokens":         s.NginxServerTokens,
				"gzip":                  s.NginxGzip,
				"client_body_timeout":   s.NginxClientBodyTimeout,
				"client_header_timeout": s.NginxClientHeaderTimeout,
				"send_timeout":          s.NginxSendTimeout,
				"proxy_connect_timeout": s.NginxProxyConnectTimeout,
				"proxy_read_timeout":    s.NginxProxyReadTimeout,
				"proxy_send_timeout":    s.NginxProxySendTimeout,
				"worker_processes":      s.NginxWorkerProcesses,
				"worker_connections":    s.NginxWorkerConnections,
				"custom_http":           s.NginxCustomHTTP,
			}); err != nil {
				h.cfg.Log.Error("agent nginx tunables apply failed", "err", err)
			}
		}(*current)
	}

	h.cfg.Log.Info("event=audit kind=server_settings_updated actor_id=" + claims.UserID)
	// GH #243: apply the Stalwart WebAdmin reverse-proxy SYNCHRONOUSLY when the
	// toggle or allowlist changed. Stalwart itself stays loopback-pinned; the
	// gate is TLS + the IP allowlist + Stalwart's own admin login (ADR-0142).
	webadminChanged := current.StalwartWebadminEnabled != prevWebadminEnabled ||
		current.StalwartWebadminAllowCIDRs != prevWebadminCIDRs
	if webadminChanged && h.cfg.Agent != nil {
		if aerr := h.applyStalwartWebadmin(ctx, current); aerr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "webadmin_apply_failed", "detail": firstLineString(aerr.Error())})
			return
		}
	}

	c.JSON(http.StatusOK, current)
}

// validateServerSettings does lenient input validation matching the installer.
func validateServerSettings(s *models.ServerSettings) error {
	if s.Hostname != "" && !isValidHostname(s.Hostname) {
		return fmt.Errorf("invalid hostname")
	}
	// IPv4
	for label, v := range map[string]string{"public_ipv4": s.PublicIPv4, "ns1_ipv4": s.NS1IPv4, "ns2_ipv4": s.NS2IPv4} {
		if v == "" {
			continue
		}
		if net.ParseIP(v) == nil || net.ParseIP(v).To4() == nil {
			return fmt.Errorf("%s: not a valid IPv4", label)
		}
	}
	// IPv6 (optional)
	if s.PublicIPv6 != "" {
		ip := net.ParseIP(s.PublicIPv6)
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("public_ipv6: not a valid IPv6")
		}
	}
	// NS names
	for label, v := range map[string]string{"ns1_name": s.NS1Name, "ns2_name": s.NS2Name} {
		if v == "" {
			continue
		}
		if !isValidHostname(v) {
			return fmt.Errorf("%s: invalid hostname", label)
		}
	}
	// Admin email
	if s.AdminEmail != "" {
		if _, err := mail.ParseAddress(s.AdminEmail); err != nil {
			return fmt.Errorf("admin_email: invalid")
		}
	}
	// Timezone (optional, empty means "use OS default")
	if s.Timezone != "" && !isValidTimezone(s.Timezone) {
		return fmt.Errorf("timezone: invalid format")
	}
	// SSH port
	if s.SSHPort < 1 || s.SSHPort > 65535 {
		return fmt.Errorf("ssh_port: must be between 1 and 65535")
	}
	if s.DefaultDNSTTL != 0 && (s.DefaultDNSTTL < 60 || s.DefaultDNSTTL > 86400) {
		return fmt.Errorf("default_dns_ttl must be 60–86400 seconds (or 0 to use built-in fallback)")
	}
	// Panel brand text: free-form but capped at 60 chars.
	if len(s.PanelBrandText) > 60 {
		return fmt.Errorf("panel_brand_text: must be <= 60 chars")
	}
	// Upload cap. 0 == "use compile-time default (1 GB)"; otherwise
	// 1 MB minimum and 10 GB ceiling (matches the practical browser-
	// upload limit and the nginx vhost client_max_body_size; admins
	// wanting bigger should use SFTP/SCP).
	if s.UploadMaxSizeMB != 0 && (s.UploadMaxSizeMB < 1 || s.UploadMaxSizeMB > 10240) {
		return fmt.Errorf("upload_max_size_mb: must be 0 or between 1 and 10240")
	}
	// M13 SSH sandbox.
	if s.SSHSandboxMode != "" && s.SSHSandboxMode != "bubblewrap" && s.SSHSandboxMode != "nspawn" {
		return fmt.Errorf("ssh_sandbox_mode: must be 'bubblewrap' or 'nspawn'")
	}
	if s.DefaultNspawnImageVersion != "" && !isImageNamePattern(s.DefaultNspawnImageVersion) {
		return fmt.Errorf("default_nspawn_image_version: must match [a-z0-9-]+")
	}
	// WorkingFolder must be absolute. install.sh ensures /var/lib/
	// jabali exists at first boot; operator who points elsewhere is
	// responsible for pre-creating + ACLing that dir.
	if s.WorkingFolder != "" {
		if !strings.HasPrefix(s.WorkingFolder, "/") {
			return fmt.Errorf("working_folder: must be an absolute path")
		}
		if strings.Contains(s.WorkingFolder, "..") {
			return fmt.Errorf("working_folder: must not contain '..'")
		}
	}
	// M55 nginx tunables. Sizes/timeouts go verbatim into a config file,
	// so reject anything not matching nginx's value grammar.
	if s.NginxClientMaxBodySize != "" && !nginxSizeRE.MatchString(s.NginxClientMaxBodySize) {
		return fmt.Errorf("nginx_client_max_body_size: invalid nginx size (e.g. 50m)")
	}
	for label, v := range map[string]string{
		"nginx_keepalive_timeout":     s.NginxKeepaliveTimeout,
		"nginx_client_body_timeout":   s.NginxClientBodyTimeout,
		"nginx_client_header_timeout": s.NginxClientHeaderTimeout,
		"nginx_send_timeout":          s.NginxSendTimeout,
		"nginx_proxy_connect_timeout": s.NginxProxyConnectTimeout,
		"nginx_proxy_read_timeout":    s.NginxProxyReadTimeout,
		"nginx_proxy_send_timeout":    s.NginxProxySendTimeout,
	} {
		if v != "" && !nginxTimeRE.MatchString(v) {
			return fmt.Errorf("%s: invalid nginx time (e.g. 300s)", label)
		}
	}
	if s.NginxWorkerProcesses != "" && !nginxWorkerProcRE.MatchString(s.NginxWorkerProcesses) {
		return fmt.Errorf("nginx_worker_processes: must be 'auto' or 1-99")
	}
	if s.NginxWorkerConnections > 1048576 {
		return fmt.Errorf("nginx_worker_connections: must be <= 1048576")
	}
	if len(s.NginxCustomHTTP) > 4000 {
		return fmt.Errorf("nginx_custom_http: must be <= 4000 chars")
	}
	return nil
}

// isImageNamePattern matches the [a-z0-9-]+ shape used everywhere
// from agent helpers to wrapper scripts. Kept inline here so the
// file doesn't depend on regexp at build time for one validation.
func isImageNamePattern(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

var (
	hostnameRE = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	timezoneRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]*(/[A-Za-z0-9_+-]+)*$`)

	// M55 nginx tunable value grammars.
	nginxSizeRE       = regexp.MustCompile(`^[0-9]+[kKmMgG]?$`)
	nginxTimeRE       = regexp.MustCompile(`^[0-9]+(ms|s|m|h)?$`)
	nginxWorkerProcRE = regexp.MustCompile(`^(auto|[1-9][0-9]?)$`)
)

func isValidHostname(s string) bool {
	if len(s) > 253 {
		return false
	}
	return hostnameRE.MatchString(s)
}

func isValidTimezone(s string) bool {
	if len(s) > 64 {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	if strings.HasPrefix(s, "/") {
		return false
	}
	return timezoneRE.MatchString(s)
}

// pythonRuntimeStatus reports whether the Python app runtime prerequisites
// are installed, so the Apps-tab card can poll after enabling (ADR-0131).
func (h *serverSettingsHandler) pythonRuntimeStatus(c *gin.Context) {
	if h.cfg.Agent == nil {
		c.JSON(http.StatusOK, gin.H{"installed": false})
		return
	}
	raw, err := h.cfg.Agent.Call(c.Request.Context(), "app.python.runtime_status", map[string]any{})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"installed": false})
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

// --- GH #243: Stalwart WebAdmin reverse-proxy ---

// stalwartWebadminPort is the dedicated HTTPS port the WebAdmin proxy listens
// on (panel hostname, panel cert). Distinct from the panel :8443.
const stalwartWebadminPort = 8449

// stalwartAdminCredentialResponse carries the Stalwart admin GUI login. The
// password is the recovery-admin token — the same credential the panel uses to
// manage Stalwart — so a rotate restarts jabali-stalwart + jabali-panel.
type stalwartAdminCredentialResponse struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Rotated  bool   `json:"rotated"`
	URL      string `json:"url"`
}

// stalwartAdminCredential reveals or rotates the Stalwart admin GUI login.
// POST /admin/settings/stalwart-admin-credential {action:"reveal"|"rotate"}.
func (h *serverSettingsHandler) stalwartAdminCredential(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	if req.Action != "reveal" && req.Action != "rotate" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_action", "detail": "action must be reveal or rotate"})
		return
	}
	if h.cfg.Agent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_unavailable"})
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	raw, err := h.cfg.Agent.Call(callCtx, "mail.admin_cred.manage", map[string]any{"action": req.Action})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "admin_cred_failed", "detail": firstLineString(err.Error())})
		return
	}
	var resp struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Rotated  bool   `json:"rotated"`
	}
	if uerr := json.Unmarshal(raw, &resp); uerr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "admin_cred_decode"})
		return
	}
	hostname := ""
	if s, gerr := h.cfg.Repo.Get(ctx); gerr == nil && s != nil {
		hostname = s.Hostname
	}
	c.JSON(http.StatusOK, stalwartAdminCredentialResponse{
		User:     resp.User,
		Password: resp.Password,
		Rotated:  resp.Rotated,
		URL:      fmt.Sprintf("https://%s:%d/", hostname, stalwartWebadminPort),
	})
}

// applyStalwartWebadmin dispatches mail.webadmin.apply with the effective
// state. Serves on a dedicated port of the panel hostname, reusing the panel
// cert; Stalwart stays loopback. No gateway credential — the gate is TLS + the
// IP allowlist + Stalwart's own admin login (ADR-0142).
func (h *serverSettingsHandler) applyStalwartWebadmin(ctx context.Context, s *models.ServerSettings) error {
	cidrs := splitCIDRList(s.StalwartWebadminAllowCIDRs)
	params := map[string]any{
		"enabled":       s.StalwartWebadminEnabled,
		"server_name":   s.Hostname,
		"port":          stalwartWebadminPort,
		"ssl_cert_path": "/etc/jabali/tls/panel.crt",
		"ssl_key_path":  "/etc/jabali/tls/panel.key",
		"allow_cidrs":   cidrs,
	}
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_, err := h.cfg.Agent.Call(callCtx, "mail.webadmin.apply", params)
	return err
}

// splitCIDRList parses a comma/space-separated allowlist into a clean slice.
func splitCIDRList(in string) []string {
	out := []string{}
	for _, f := range strings.FieldsFunc(in, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// hexColorRE matches an empty string (use default) or a #rgb / #rrggbb /
// #rrggbbaa hex color — the accepted shapes for the "Look and feel" panel
// colors.
var hexColorRE = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

func isValidHexColor(v string) bool { return v == "" || hexColorRE.MatchString(v) }
