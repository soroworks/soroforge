package store

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	// Registers the postgres driver that NewWithSourceInstance resolves from
	// the postgres:// scheme in DATABASE_URL.
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
)

// migrationsFS embeds the schema so a built binary carries its own migrations.
// `soroforge migrate up` then works from a single file with nothing to copy
// alongside it — which matters when the binary runs in a CI container.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrator applies schema migrations.
type Migrator struct {
	m *migrate.Migrate
}

// NewMigrator prepares migrations against the given database URL.
//
// It takes a URL rather than the pgx pool because golang-migrate needs its own
// database/sql handle; sharing the pool is not supported.
func NewMigrator(databaseURL string) (*Migrator, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("migrate: database URL is required")
	}

	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("migrate: open database: %w", err)
	}
	return &Migrator{m: m}, nil
}

// Up applies all pending migrations. Applying an already-current schema is not
// an error, so this is safe to run on every start.
func (m *Migrator) Up() error {
	if err := m.m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Down reverts the most recent migration.
//
// It steps back one migration rather than tearing everything down, because
// `migrate down` on a database holding real deployment history should not be a
// single command away from dropping all of it.
func (m *Migrator) Down() error {
	if err := m.m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// Version reports the current schema version and whether the last migration
// left the database dirty (failed partway and needs manual attention).
func (m *Migrator) Version() (version uint, dirty bool, err error) {
	version, dirty, err = m.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("migrate version: %w", err)
	}
	return version, dirty, nil
}

// Close releases the migrator's database handles.
func (m *Migrator) Close() error {
	sourceErr, dbErr := m.m.Close()
	return errors.Join(sourceErr, dbErr)
}

// MigrationsFS exposes the embedded migrations for integration tests that build
// a schema directly.
func MigrationsFS() embed.FS { return migrationsFS }
