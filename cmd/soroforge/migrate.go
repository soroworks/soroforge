package main

import (
	"fmt"
	"log/slog"

	"github.com/soroworks/soroforge/internal/store"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage the deployment history schema",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "up",
			Short: "Apply pending migrations",
			Long: `Apply all pending migrations.

Running this against an already-current schema is a no-op, so it is safe to run
on every start.`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return withMigrator(func(m *store.Migrator) error {
					if err := m.Up(); err != nil {
						return err
					}
					version, dirty, err := m.Version()
					if err != nil {
						return err
					}
					fmt.Fprintf(out(), "Schema is at version %d (dirty: %t).\n", version, dirty)
					return nil
				})
			},
		},
		&cobra.Command{
			Use:   "down",
			Short: "Revert the most recent migration",
			Long: `Revert one migration.

This steps back a single migration rather than tearing the schema down, but it
can still destroy deployment history. Take a backup first.`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return withMigrator(func(m *store.Migrator) error {
					if err := m.Down(); err != nil {
						return err
					}
					version, dirty, err := m.Version()
					if err != nil {
						return err
					}
					fmt.Fprintf(out(), "Schema is at version %d (dirty: %t).\n", version, dirty)
					return nil
				})
			},
		},
		&cobra.Command{
			Use:   "version",
			Short: "Show the current schema version",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return withMigrator(func(m *store.Migrator) error {
					version, dirty, err := m.Version()
					if err != nil {
						return err
					}
					if dirty {
						fmt.Fprintf(out(),
							"Schema is at version %d and DIRTY — a migration failed partway "+
								"and needs manual repair.\n", version)
						return nil
					}
					fmt.Fprintf(out(), "Schema is at version %d.\n", version)
					return nil
				})
			},
		},
	)

	return cmd
}

// withMigrator runs fn against a migrator built from DATABASE_URL.
//
// Migrations only need the database, not the full application, so this avoids
// loading soroforge.yaml — `migrate up` should work before a project config
// exists.
func withMigrator(fn func(*store.Migrator) error) error {
	databaseURL, err := loadEnvOnly().RequireDatabaseURL()
	if err != nil {
		return err
	}

	migrator, err := store.NewMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := migrator.Close(); err != nil {
			slog.Warn("closing migrator", "error", err)
		}
	}()

	return fn(migrator)
}
