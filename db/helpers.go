package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/sglmr/gowebstart/assets"
)

// MigrateUp performs all the available Up migrations on the database with golang-migrate.
func MigrateUp(dsn string) error {
	// Open connection to database
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed migration db connection: %w", err)
	}
	defer db.Close()

	// Create a new postgres driver for migrations
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed creating pg driver: %w", err)
	}

	// Create an in-memory file system driver that can read the embedded migration files
	iofsDriver, err := iofs.New(assets.EmbeddedFiles, "/migrations")
	if err != nil {
		return fmt.Errorf("failed creating io/fs driver: %w", err)
	}

	// Create a new migrate instance
	migrator, err := migrate.NewWithInstance("iofs", iofsDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed creating migrate instance: %w", err)
	}

	// Apply all the available up migrations
	err = migrator.Up()
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		return nil
	case err != nil:
		return fmt.Errorf("error migrating up: %w", err)
	}

	return nil
}
