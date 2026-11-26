package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// lazyPool builds a pgxpool without contacting a server: pgx dials on the
// first acquire, so Pool can be exercised as a unit test. The port is closed
// on purpose — a test that accidentally connects should fail loudly rather
// than depend on a developer's local Postgres.
func lazyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	cfg, err := pgxpool.ParseConfig("postgres://soroforge:soroforge@127.0.0.1:1/soroforge?sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	return pool
}

// TestPostgresPool pins the accessor migrations and integration tests rely on:
// it must hand back the store's own pool while the store is open, and the same
// pool must be unusable once the store is closed.
func TestPostgresPool(t *testing.T) {
	t.Run("returns the wrapped pool", func(t *testing.T) {
		pool := lazyPool(t)
		p := &Postgres{pool: pool}
		t.Cleanup(pool.Close)

		got := p.Pool()
		require.NotNil(t, got, "migrations and integration tests need the pool")
		require.Same(t, pool, got, "Pool must hand back the pool the store uses, not a copy")
		require.Equal(t, int32(0), got.Stat().TotalConns(), "reading the pool must not dial a connection")
	})

	t.Run("returns a pool that closes with the store", func(t *testing.T) {
		p := &Postgres{pool: lazyPool(t)}
		require.NoError(t, p.Close())

		got := p.Pool()
		require.NotNil(t, got, "an already-created pool is still observable after Close")

		err := got.Ping(context.Background())
		require.Error(t, err, "using the pool after Close must fail instead of connecting")
		require.ErrorContains(t, err, "closed pool")
	})
}
