package store

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// openRaw opens a raw SQLite handle without running Open's schema or
// migrations — for hand-building old-version databases in tests.
func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw %s: %v", path, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// execScript runs a multi-statement SQL script, failing the test on the
// first error.
func execScript(t *testing.T, db *sql.DB, script string) {
	t.Helper()
	if _, err := db.Exec(script); err != nil {
		t.Fatalf("exec script: %v\nscript: %s", err, script)
	}
}
