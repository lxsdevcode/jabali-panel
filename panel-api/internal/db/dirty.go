package db

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	mmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// MigrationState reports where the schema is and whether golang-migrate
// considers it mid-flight.
type MigrationState struct {
	Version uint
	Dirty   bool
}

// State reads the current schema version and dirty flag.
//
// Dirty means golang-migrate started a migration and never recorded that it
// finished — the process died in between. Every later `migrate up` then refuses
// to run, which on this codebase means panel-api exits at startup and systemd
// restart-loops it:
//
//	Error: migrate up: Dirty database version 210. Fix and force version.
//
// The panel is down at that point, and until now nothing in the CLI could
// report or clear it — `jabali repair --diagnose` returned all-green while the
// panel was in a crash loop, because it had no detector for this.
func State(dsn string) (MigrationState, error) {
	m, closeFn, err := newMigrator(dsn)
	if err != nil {
		return MigrationState{}, err
	}
	defer closeFn()

	v, dirty, err := m.Version()
	if err == migrate.ErrNilVersion {
		// Fresh database, nothing applied yet. Not an error and not dirty.
		return MigrationState{Version: 0, Dirty: false}, nil
	}
	if err != nil {
		return MigrationState{}, fmt.Errorf("read migration version: %w", err)
	}
	return MigrationState{Version: v, Dirty: dirty}, nil
}

// ForceVersion clears the dirty flag by asserting the schema really is at
// `version`.
//
// This does NOT run or roll back any SQL — it only rewrites golang-migrate's
// bookkeeping. That makes it the right tool when a migration completed and the
// process died before the flag was cleared, and the WRONG tool when the
// migration only half-applied: forcing forward there would skip the remainder
// permanently, and the next `up` would carry on from a schema that is missing
// columns nobody will notice until a query fails.
//
// Callers must therefore confirm the target migration's effects are actually
// present before calling this, which is why the CLI gates it behind an explicit
// version argument rather than inferring one.
func ForceVersion(dsn string, version int) error {
	m, closeFn, err := newMigrator(dsn)
	if err != nil {
		return err
	}
	defer closeFn()

	if err := m.Force(version); err != nil {
		return fmt.Errorf("force version %d: %w", version, err)
	}
	return nil
}

// newMigrator builds a migrate instance over the embedded migrations, mirroring
// Migrate's setup. Returns a close function rather than deferring internally so
// the caller controls the lifetime.
func newMigrator(dsn string) (*migrate.Migrate, func(), error) {
	driverDSN, err := ToDriverDSN(dsn)
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := openRawSQL(driverDSN)
	if err != nil {
		return nil, nil, err
	}
	closeFn := func() { _ = sqlDB.Close() }

	drv, err := mmysql.WithInstance(sqlDB, &mmysql.Config{})
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("migrate driver: %w", err)
	}
	srcFS, err := migrationsFSRoot()
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("migrations fs: %w", err)
	}
	src, err := iofs.New(srcFS, ".")
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("migrations source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", drv)
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("migrate new: %w", err)
	}
	return m, closeFn, nil
}
