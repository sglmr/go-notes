package db

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/sglmr/gowebstart/assets"
)

// MigrateUp performs all the available Up migrations on the PostgreSQL database with golang-migrate.
func MigrateUp(conn *pgxpool.Pool) error {
	// Convert pgx connection to sql.DB
	db := stdlib.OpenDBFromPool(conn)

	// Create a driver for golang-migrate
	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	// Create an in-memory file system driver that can read the embedded migration files
	iofsDriver, err := iofs.New(assets.EmbeddedFiles, "migrations")
	if err != nil {
		return err
	}

	// Create a new migrate instance
	migrator, err := migrate.NewWithInstance("iofs", iofsDriver, "postgres", dbDriver)
	if err != nil {
		return err
	}

	// Apply all the available up migrations
	err = migrator.Up()
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		break // do nothing
	case err != nil:
		return err
	}

	return nil
}
