package sqlite

import (
	"database/sql"
	"log"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteVersion(t *testing.T) {
	var version string

	db, err := sql.Open(DriverName, ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	row := db.QueryRow("select sqlite_version()")
	if err := row.Scan(&version); err != nil {
		log.Fatal(err)
	}

	t.Log(version)
}
