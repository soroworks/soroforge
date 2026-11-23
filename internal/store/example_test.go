package store_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soroworks/soroforge/internal/store"
)

// exampleSQLiteURL returns a sqlite:// DATABASE_URL pointing at a fresh file
// in a temp directory, plus a cleanup func. Examples cannot use t.TempDir, so
// they manage the directory themselves.
func exampleSQLiteURL() (string, func()) {
	dir, err := os.MkdirTemp("", "soroforge-example-")
	if err != nil {
		panic(err)
	}
	url := store.SQLiteURLPrefix + filepath.ToSlash(filepath.Join(dir, "history.db"))
	return url, func() { _ = os.RemoveAll(dir) }
}

// ExampleNewMigrator applies the embedded schema to a database. The URL's
// scheme picks the migration set: sqlite:// selects the SQLite migrations,
// anything else the Postgres ones. Up is idempotent, so it is safe to call on
// every start.
func ExampleNewMigrator() {
	databaseURL, cleanup := exampleSQLiteURL()
	defer cleanup()

	m, err := store.NewMigrator(databaseURL)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = m.Close() }()

	if err := m.Up(); err != nil {
		fmt.Println("error:", err)
		return
	}
	// Running Up again on a current schema is not an error.
	if err := m.Up(); err != nil {
		fmt.Println("error:", err)
		return
	}

	version, dirty, err := m.Version()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("migrated:", version > 0)
	fmt.Println("dirty:", dirty)

	// An empty URL is rejected before anything is opened.
	_, err = store.NewMigrator("")
	fmt.Println(err)
	// Output:
	// migrated: true
	// dirty: false
	// migrate: database URL is required
}

// ExampleNewPostgres shows how NewPostgres reports a bad DATABASE_URL. A real
// call passes a postgres:// URL and gets back a pooled, pinged connection;
// the database must already be migrated with NewMigrator.
//
// Parse errors deliberately omit the connection string, which normally
// carries a password, so they are safe to log.
func ExampleNewPostgres() {
	ctx := context.Background()

	_, err := store.NewPostgres(ctx, "")
	fmt.Println(err)

	_, err = store.NewPostgres(ctx, "postgres://user:s3cret@localhost:notaport/db")
	fmt.Println(err)
	// Output:
	// store: database URL is required
	// store: could not parse DATABASE_URL (check its format)
}

// ExampleNewSQLite opens a file-backed store, migrates it, and records a
// contract. SQLite needs no server, which makes it the simplest backend for
// local use and CI.
func ExampleNewSQLite() {
	ctx := context.Background()
	databaseURL, cleanup := exampleSQLiteURL()
	defer cleanup()

	// The schema is applied separately, exactly as `soroforge migrate up` does.
	m, err := store.NewMigrator(databaseURL)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	if err := m.Up(); err != nil {
		fmt.Println("error:", err)
		return
	}
	_ = m.Close()

	s, err := store.NewSQLite(ctx, databaseURL)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = s.Close() }()

	if _, err := s.UpsertContract(ctx, store.Contract{
		Alias:           "counter",
		Network:         "testnet",
		ContractID:      "CCOUNTER",
		CurrentWasmHash: "a1b2c3",
	}); err != nil {
		fmt.Println("error:", err)
		return
	}

	c, err := s.GetContract(ctx, "testnet", "counter")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(c.Network, c.Alias, c.ContractID, c.CurrentWasmHash)
	// Output:
	// testnet counter CCOUNTER a1b2c3
}

// ExampleParseSnapshot decodes and validates a history file as written by
// `soroforge history export`. Validation happens up front, so a hand-edited
// or newer-format file fails with a readable error before anything is
// imported.
func ExampleParseSnapshot() {
	data := []byte(`{
  "version": 1,
  "generated_at": "2026-02-01T09:00:00Z",
  "contracts": [
    {"id": 1, "alias": "counter", "network": "testnet",
     "contract_id": "CCOUNTER", "current_wasm_hash": "a1b2c3",
     "created_at": "2026-01-02T10:00:00Z", "updated_at": "2026-01-02T10:00:00Z"}
  ],
  "deployments": [
    {"id": 1, "alias": "counter", "network": "testnet",
     "contract_id": "CCOUNTER", "wasm_hash": "a1b2c3", "action": "deploy",
     "deployer_pubkey": "GDEPLOYER", "actor": "cli", "ledger": 4500,
     "created_at": "2026-01-02T10:00:00Z"}
  ]
}`)

	snap, err := store.ParseSnapshot(data)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("version:", snap.Version)
	fmt.Println("contracts:", len(snap.Contracts))
	d := snap.Deployments[0]
	fmt.Println("first deployment:", d.ID, d.Action, d.Alias, "ledger", d.Ledger)

	// A file from a newer format version is rejected, not guessed at.
	_, err = store.ParseSnapshot([]byte(`{"version": 2}`))
	fmt.Println(err)
	// Output:
	// version: 1
	// contracts: 1
	// first deployment: 1 deploy counter ledger 4500
	// history: unsupported format version 2 (this build reads version 1)
}

// ExampleSanitizeKeyMaterial shows a secret seed being redacted from error
// text before it is recorded, while public identifiers are left untouched.
func ExampleSanitizeKeyMaterial() {
	seed := "SAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	msg := "signing with " + seed + " failed for GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"

	fmt.Println(store.SanitizeKeyMaterial(msg))
	// Output:
	// signing with [REDACTED-SECRET-KEY] failed for GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H
}
