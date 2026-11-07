// Package dbtest creates a throwaway, fully migrated database per test.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/platform/database"
)

const defaultServerURL = "postgres://testforge:testforge@localhost:5433/postgres?sslmode=disable"

// New returns a pool connected to a fresh database that is dropped when the
// test ends. It fails the test when Postgres is unreachable (run `make db-up`).
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	serverURL := os.Getenv("TEST_DATABASE_URL")
	if serverURL == "" {
		serverURL = defaultServerURL
	}

	admin, err := pgx.Connect(ctx, serverURL)
	if err != nil {
		t.Fatalf("connect to test Postgres (run `make db-up`): %v", err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "tf_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name

	pool, err := database.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})

	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return pool
}
