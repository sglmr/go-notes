package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewTestDB sets up a new test database to use with integration tests
func NewTestDB(t *testing.T, ctx context.Context, dbURL string) *Queries {
	var queries *Queries

	// Connect to the database
	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}

	// Ping the database, timeout after 2 seconds
	if err = dbPool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	// Create a new database queries object
	queries = New(dbPool)

	// Use t.Cleanup() to register a function that will automatically be called when the caller of this function is finishd.
	t.Cleanup(func() {
		// Step 1: Perform down migrations
		// In-memory databse, so not doing that
		err := MigrateDown(dbPool)
		if err != nil {
			t.Fatal(err)
		}

		// Step 2: Close the database connection
		dbPool.Close()
	})

	// Perform migrations on the database
	if err := MigrateUp(dbPool); err != nil {
		t.Fatal(err)
	}

	// read contents of test_setup.sql file
	setupSQL, err := os.ReadFile("../../db/test_setup.sql")
	if err != nil {
		t.Fatal(err)
	}

	// Execute SQL statements from test_setup.sql
	_, err = dbPool.Exec(ctx, string(setupSQL))
	if err != nil {
		t.Fatal(err)
	}

	return queries
}
