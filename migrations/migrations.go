// Package migrations embeds the SQL migration files so binaries and tests can
// apply them without the golang-migrate CLI. Files are numbered globally and
// named after the owning module: NNNN_<module>_<change>.{up,down}.sql.
package migrations

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the "pgx5" driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var FS embed.FS

// New builds a migrator for a postgres:// URL using the embedded files.
func New(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(FS, ".")
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}
	// golang-migrate's pgx/v5 driver is registered under the pgx5:// scheme.
	url := "pgx5://" + strings.TrimPrefix(strings.TrimPrefix(databaseURL, "postgres://"), "postgresql://")

	// Pin the version table to public. Otherwise it is created in the first schema
	// of search_path ("$user", public); with a DB user named "wallet" that becomes
	// the wallet module's schema as soon as migration 0003 creates it.
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	url += sep + "x-migrations-table=%22public%22.%22schema_migrations%22&x-migrations-table-quoted=1"
	m, err := migrate.NewWithSourceInstance("iofs", src, url)
	if err != nil {
		return nil, fmt.Errorf("init migrate: %w", err)
	}
	return m, nil
}

// Up applies all pending migrations. "Already up to date" is not an error.
func Up(databaseURL string) error {
	m, err := New(databaseURL)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
