// Package repository — DockerApp repository (M48 Phase 1).
//
// The handlers (Phase 4) call into this; the reconciler (Phase 3) calls
// into this. The agent never touches the DB.
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

// DockerAppRepository is the data-access surface for the docker_apps,
// docker_app_published_ports, and docker_app_backups tables.
//
// Ports and backups are owned by the app (CASCADE on delete in SQL);
// the repo helpers below denormalise that relationship for callers
// that want a single round-trip.
type DockerAppRepository interface {
	// --- docker_apps -----------------------------------------------------
	Create(ctx context.Context, app *models.DockerApp) error
	FindByID(ctx context.Context, id string) (*models.DockerApp, error)
	FindBySlugName(ctx context.Context, slug, name string) (*models.DockerApp, error)
	ListAll(ctx context.Context) ([]*models.DockerApp, error)
	ListByStatus(ctx context.Context, status string) ([]*models.DockerApp, error)
	UpdateStatus(ctx context.Context, id, status string, lastError *string) error
	UpdateImageSHA(ctx context.Context, id, imageSHA string) error
	Update(ctx context.Context, app *models.DockerApp) error
	Delete(ctx context.Context, id string) error

	// --- docker_app_published_ports --------------------------------------
	CreatePort(ctx context.Context, p *models.DockerAppPublishedPort) error
	ListPortsForApp(ctx context.Context, appID string) ([]*models.DockerAppPublishedPort, error)
	DeletePort(ctx context.Context, id string) error

	// FindFreeHostPort scans the 10000..19999 pool and returns the lowest
	// host_port not currently bound to (bindInterface, protocol). Returns
	// ErrNotFound when the pool is exhausted (which would mean ~10k
	// concurrent published ports on a single interface+protocol — never
	// going to happen in practice, but the caller should still surface
	// a clean 503 rather than a 500).
	FindFreeHostPort(ctx context.Context, bindInterface, protocol string) (int, error)

	// --- docker_app_backups ----------------------------------------------
	CreateBackup(ctx context.Context, b *models.DockerAppBackup) error
	ListBackupsForApp(ctx context.Context, appID string) ([]*models.DockerAppBackup, error)
	DeleteBackup(ctx context.Context, id string) error
}

type dockerAppRepo struct{ db *gorm.DB }

// NewDockerAppRepository returns a DockerAppRepository backed by GORM.
func NewDockerAppRepository(db *gorm.DB) DockerAppRepository {
	return &dockerAppRepo{db: db}
}

// --- docker_apps ------------------------------------------------------------

func (r *dockerAppRepo) Create(ctx context.Context, app *models.DockerApp) error {
	return translate(r.db.WithContext(ctx).Create(app).Error)
}

func (r *dockerAppRepo) FindByID(ctx context.Context, id string) (*models.DockerApp, error) {
	var a models.DockerApp
	if err := r.db.WithContext(ctx).First(&a, "id = ?", id).Error; err != nil {
		return nil, translate(err)
	}
	return &a, nil
}

func (r *dockerAppRepo) FindBySlugName(ctx context.Context, slug, name string) (*models.DockerApp, error) {
	var a models.DockerApp
	if err := r.db.WithContext(ctx).
		Where("slug = ? AND name = ?", slug, name).
		First(&a).Error; err != nil {
		return nil, translate(err)
	}
	return &a, nil
}

func (r *dockerAppRepo) ListAll(ctx context.Context) ([]*models.DockerApp, error) {
	var apps []*models.DockerApp
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

func (r *dockerAppRepo) ListByStatus(ctx context.Context, status string) ([]*models.DockerApp, error) {
	var apps []*models.DockerApp
	if err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at ASC").
		Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

func (r *dockerAppRepo) UpdateStatus(ctx context.Context, id, status string, lastError *string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	// nil means "leave the previous error intact"; the empty string
	// pointer means "clear it" (transition to a success state).
	if lastError != nil {
		updates["last_error"] = *lastError
	}
	return r.db.WithContext(ctx).
		Model(&models.DockerApp{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *dockerAppRepo) UpdateImageSHA(ctx context.Context, id, imageSHA string) error {
	return r.db.WithContext(ctx).
		Model(&models.DockerApp{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"image_sha":  imageSHA,
			"updated_at": time.Now(),
		}).Error
}

func (r *dockerAppRepo) Update(ctx context.Context, app *models.DockerApp) error {
	return r.db.WithContext(ctx).Save(app).Error
}

func (r *dockerAppRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&models.DockerApp{}).Error
}

// --- docker_app_published_ports ---------------------------------------------

func (r *dockerAppRepo) CreatePort(ctx context.Context, p *models.DockerAppPublishedPort) error {
	return translate(r.db.WithContext(ctx).Create(p).Error)
}

func (r *dockerAppRepo) ListPortsForApp(ctx context.Context, appID string) ([]*models.DockerAppPublishedPort, error) {
	var ports []*models.DockerAppPublishedPort
	if err := r.db.WithContext(ctx).
		Where("app_id = ?", appID).
		Order("port_name ASC").
		Find(&ports).Error; err != nil {
		return nil, err
	}
	return ports, nil
}

func (r *dockerAppRepo) DeletePort(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&models.DockerAppPublishedPort{}).Error
}

// hostPortPoolMin / hostPortPoolMax bound the auto-allocator. The pool
// was deliberately chosen well above conventional service ports (8080,
// 8443, 9000-9100 metrics range) so an operator pinning a custom port
// elsewhere doesn't collide with the auto-allocated apps.
const (
	hostPortPoolMin = 10000
	hostPortPoolMax = 19999
)

func (r *dockerAppRepo) FindFreeHostPort(ctx context.Context, bindInterface, protocol string) (int, error) {
	// Pull the set of (host_port) values currently bound to this
	// (bindInterface, protocol). Sort and walk to find the lowest gap.
	var used []int
	if err := r.db.WithContext(ctx).
		Model(&models.DockerAppPublishedPort{}).
		Where("bind_interface = ? AND protocol = ?", bindInterface, protocol).
		Order("host_port ASC").
		Pluck("host_port", &used).Error; err != nil {
		return 0, err
	}
	usedSet := make(map[int]struct{}, len(used))
	for _, p := range used {
		usedSet[p] = struct{}{}
	}
	for p := hostPortPoolMin; p <= hostPortPoolMax; p++ {
		if _, taken := usedSet[p]; !taken {
			return p, nil
		}
	}
	return 0, ErrNotFound
}

// --- docker_app_backups -----------------------------------------------------

func (r *dockerAppRepo) CreateBackup(ctx context.Context, b *models.DockerAppBackup) error {
	return translate(r.db.WithContext(ctx).Create(b).Error)
}

func (r *dockerAppRepo) ListBackupsForApp(ctx context.Context, appID string) ([]*models.DockerAppBackup, error) {
	var rows []*models.DockerAppBackup
	if err := r.db.WithContext(ctx).
		Where("app_id = ?", appID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *dockerAppRepo) DeleteBackup(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&models.DockerAppBackup{}).Error
}

// translate is the package-local error mapper from repository.go; we
// re-import the local errors here to keep the typed error contract
// uniform across the package.
var _ = errors.New
