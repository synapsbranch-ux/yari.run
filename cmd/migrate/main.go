// Command migrate applies the goose migrations, then the River migrations, as the owner role.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/synapsbranch-ux/yari.run/db"
	"github.com/synapsbranch-ux/yari.run/internal/config"
)

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := config.FromEnv()
	if cfg.MigrationsDatabaseURL == "" {
		return fmt.Errorf("MIGRATIONS_DATABASE_URL is required (owner role, not the application role)")
	}

	if err := migrateGoose(ctx, cfg.MigrationsDatabaseURL); err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	if err := migrateRiver(ctx, cfg.MigrationsDatabaseURL); err != nil {
		return fmt.Errorf("river: %w", err)
	}
	slog.Info("migrations complete")
	return nil
}

func migrateGoose(ctx context.Context, url string) error {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, sqlDB, "migrations")
}

func migrateRiver(ctx context.Context, url string) error {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}

	// River tables are not tenant tables: the application role needs plain access to enqueue and
	// run jobs. Granted here because River creates them after the goose migrations.
	_, err = pool.Exec(ctx, `
DO $$
DECLARE t text;
BEGIN
    FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename LIKE 'river\_%' LOOP
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON %I TO yari_app', t);
    END LOOP;
    FOR t IN SELECT sequencename FROM pg_sequences WHERE schemaname = 'public' AND sequencename LIKE 'river\_%' LOOP
        EXECUTE format('GRANT USAGE, SELECT ON SEQUENCE %I TO yari_app', t);
    END LOOP;
END
$$`)
	return err
}

var _ = stdlib.GetDefaultDriver // registers the "pgx" database/sql driver
