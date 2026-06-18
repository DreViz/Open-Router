package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ──────────────────────────────────────────────────────
// Store is how the rest of our code talks to PostgreSQL.
//
// ANALOGY:
// Store is like a librarian. You tell the librarian
// "get me the user with this email" or "create a new API key"
// and they go to the database (the library) and do it.
//
// The rest of the code never writes SQL — it calls Store methods.
// This keeps database logic in one place.
//
// pgxpool.Pool is a CONNECTION POOL:
//   - Instead of opening a new connection for every request (slow),
//     it keeps a pool of connections open and reuses them.
//   - Like a taxi stand: instead of calling a new taxi each time,
//     there are always a few waiting.
// ──────────────────────────────────────────────────────
type Store struct {
	db *pgxpool.Pool
}

// New connects to PostgreSQL, runs migrations, returns a Store.
//
// Parameters:
//   - databaseURL: connection string like "postgres://user:pass@localhost:5432/dbname"
//   - migrationsPath: path to SQL migration files like "file://./migrations"
//   - logger: for logging migration status
func New(ctx context.Context, databaseURL, migrationsPath string, logger *slog.Logger) (*Store, error) {

	// ── Run migrations first ──
	//
	// Migrations are SQL files that create/modify tables.
	// We run them every time the server starts.
	// If the tables already exist, migrate knows and skips them.
	// If there are new migrations, it runs only the new ones.
	//
	// This way the database schema is always up to date.
	// ── Run migrations first ──
	//
	// Migrations are SQL files that create/modify tables.
	// We run them every time the server starts.
	// If the tables already exist, migrate knows and skips them.
	// If there are new migrations, it runs only the new ones.
	//
	// If migrations fail, we return an error — the server MUST NOT
	// start with an inconsistent database schema.
	if err := runMigrations(migrationsPath, databaseURL, logger); err != nil {
		return nil, err
	}

	// ── Connect to PostgreSQL ──
	//
	// pgxpool.New creates a connection pool.
	// It opens several connections upfront and keeps them ready.
	// When a query comes in, it hands out an idle connection.
	// When the query is done, the connection goes back to the pool.
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify the connection actually works
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("connected to database")
	return &Store{db: pool}, nil
}

// runMigrations runs all pending SQL migration files.
// Returns an error if migrations fail — the server should NOT start
// with an inconsistent database schema.
func runMigrations(migrationsPath, databaseURL string, logger *slog.Logger) error {
	m, err := migrate.New(migrationsPath, "pgx5://"+databaseURL[len("postgres://"):])
	if err != nil {
		return fmt.Errorf("migration setup failed: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	logger.Info("database migrations applied")
	return nil
}

// Close shuts down the connection pool.
// Call this when the server is shutting down.
func (s *Store) Close() {
	s.db.Close()
}

// DB returns the underlying connection pool.
// Used by store/user.go, store/apikey.go, etc. to run queries.
func (s *Store) DB() *pgxpool.Pool {
	return s.db
}
