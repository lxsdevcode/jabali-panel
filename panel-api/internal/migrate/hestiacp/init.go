package hestiacp

import (
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/migrate"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func init() {
	migrate.Register(models.MigrationSourceHestia, func() migrate.Discoverer {
		return New()
	})
}
