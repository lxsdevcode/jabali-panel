package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// PHPPerformanceModeRepository is the store for the 4 admin-editable Performance
// Mode presets (GH #339 phase 2, Step 2). The set is fixed at 4 rows — callers
// only ever GetAll + Update; EnsureDefaults seeds them app-side on first boot
// (migrations stay schema-only per the dirty-DB scar).
type PHPPerformanceModeRepository interface {
	GetAll(ctx context.Context) ([]models.PHPPerformanceMode, error)
	Get(ctx context.Context, mode string) (*models.PHPPerformanceMode, error)
	Update(ctx context.Context, m *models.PHPPerformanceMode) error
	EnsureDefaults(ctx context.Context) (int, error)
}

type phpPerformanceModeRepo struct{ db *gorm.DB }

func NewPHPPerformanceModeRepository(db *gorm.DB) PHPPerformanceModeRepository {
	return &phpPerformanceModeRepo{db: db}
}

func (r *phpPerformanceModeRepo) GetAll(ctx context.Context) ([]models.PHPPerformanceMode, error) {
	var rows []models.PHPPerformanceMode
	if err := r.db.WithContext(ctx).Order("mode ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *phpPerformanceModeRepo) Get(ctx context.Context, mode string) (*models.PHPPerformanceMode, error) {
	var row models.PHPPerformanceMode
	if err := r.db.WithContext(ctx).Where("mode = ?", mode).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// Update replaces the pm.* of one preset. Mode is the PK and is not mutable.
func (r *phpPerformanceModeRepo) Update(ctx context.Context, m *models.PHPPerformanceMode) error {
	return r.db.WithContext(ctx).Model(&models.PHPPerformanceMode{}).
		Where("mode = ?", m.Mode).
		Updates(map[string]any{
			"pm_mode":                           m.PmMode,
			"pm_max_children":                   m.PmMaxChildren,
			"pm_start_servers":                  m.PmStartServers,
			"pm_min_spare_servers":              m.PmMinSpareServers,
			"pm_max_spare_servers":              m.PmMaxSpareServers,
			"pm_max_requests":                   m.PmMaxRequests,
			"request_terminate_timeout_seconds": m.RequestTerminateTimeoutSeconds,
			"process_idle_timeout_seconds":      m.ProcessIdleTimeoutSeconds,
			"updated_at":                        gorm.Expr("CURRENT_TIMESTAMP(6)"),
		}).Error
}

// phpPerformanceModeDefaults — the 4 shipped presets. Balanced/Low-memory/
// High-traffic/WordPress-optimized. Admin can edit these; these are the seed.
var phpPerformanceModeDefaults = []models.PHPPerformanceMode{
	{Mode: "balanced", Label: "Balanced — good default for most sites",
		PmMode: "dynamic", PmMaxChildren: 10, PmStartServers: 2, PmMinSpareServers: 1, PmMaxSpareServers: 3, ProcessIdleTimeoutSeconds: 60},
	{Mode: "low_memory", Label: "Low memory — small VPS / many sites",
		PmMode: "ondemand", PmMaxChildren: 5, PmStartServers: 1, PmMinSpareServers: 1, PmMaxSpareServers: 2, ProcessIdleTimeoutSeconds: 30},
	{Mode: "high_traffic", Label: "High traffic — more workers, more RAM",
		PmMode: "dynamic", PmMaxChildren: 25, PmStartServers: 6, PmMinSpareServers: 3, PmMaxSpareServers: 8, ProcessIdleTimeoutSeconds: 60},
	{Mode: "wordpress", Label: "WordPress optimized — good defaults for WP/WooCommerce",
		PmMode: "dynamic", PmMaxChildren: 15, PmStartServers: 4, PmMinSpareServers: 2, PmMaxSpareServers: 6, PmMaxRequests: 500, ProcessIdleTimeoutSeconds: 60},
}

// EnsureDefaults inserts any missing preset rows (idempotent — never overwrites
// admin edits). Returns how many were seeded.
func (r *phpPerformanceModeRepo) EnsureDefaults(ctx context.Context) (int, error) {
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&phpPerformanceModeDefaults)
	if res.Error != nil {
		return 0, res.Error
	}
	return int(res.RowsAffected), nil
}
