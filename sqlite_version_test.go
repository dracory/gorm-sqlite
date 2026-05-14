package sqlite

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteVersion(t *testing.T) {
	var version string

	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	row := db.QueryRow("select sqlite_version()")
	if err := row.Scan(&version); err != nil {
		t.Fatalf("Failed to scan version: %v", err)
	}

	t.Log(version)
}
