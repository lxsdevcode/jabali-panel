package directadmin

import (
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/migrate"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func init() {
	migrate.Register(models.MigrationSourceDirectAdmin, func() migrate.Discoverer {
		return New()
	})
}
