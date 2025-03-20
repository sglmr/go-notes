package db

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
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

	_ = migrator.Force(3)

	// TODO: Remove this one day
	err = migrator.Down()
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		// do nothing
	case err != nil:
		return err
	}

	// Apply all the available up migrations
	err = migrator.Up()
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		// do nothing
	case err != nil:
		return err
	}

	return nil
}

// GenerateID makes up a text unique ID for a database record.
func GenerateID(prefix string) (string, error) {
	// Validate prefix is
	if prefix == "" {
		return "", errors.New("prefix cant be blank")
	}
	// Validate prefix is lowercase
	if strings.ToLower(prefix) != prefix {
		return "", errors.New("prefix must be lowercase")
	}
	// Validate prefix has only letters
	for _, r := range prefix {
		if !unicode.IsLetter(r) {
			return "", errors.New("non letter character in prefix")
		}
	}

	// Generate a random UUID
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("new uuid error: %w", err)
	}

	// Return the concatenated ID
	return fmt.Sprintf("%s_%s", prefix, Base58Encode(id[:])), nil
}

// Base58Encode encodes a byte slice to a base58 string
func Base58Encode(input []byte) string {
	// Base58 alphabet (Bitcoin)
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// Convert bytes to big integer
	x := new(big.Int).SetBytes(input)
	// Base58 is like base 58
	base := big.NewInt(58)
	zero := big.NewInt(0)

	// Perform base conversion
	var result bytes.Buffer
	mod := new(big.Int)

	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result.WriteByte(alphabet[mod.Int64()])
	}

	// Leading zeros in input become leading '1's
	for _, b := range input {
		if b != 0 {
			break
		}
		result.WriteByte('1')
	}

	// Reverse the result
	resultStr := result.String()
	resultBytes := []byte(resultStr)
	for i, j := 0, len(resultBytes)-1; i < j; i, j = i+1, j-1 {
		resultBytes[i], resultBytes[j] = resultBytes[j], resultBytes[i]
	}

	return string(resultBytes)
}
