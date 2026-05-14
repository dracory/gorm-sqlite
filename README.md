# gorm-sqlite

A CGO-free SQLite dialector for [GORM](https://gorm.io), built on top of [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite). No C compiler required.

## Requirements

- Go 1.26+
- GORM v2 (`gorm.io/gorm`)

## Installation

```bash
go get github.com/dracory/gorm-sqlite
```

## Usage

### Open a file-based database

```go
import (
    "gorm.io/gorm"
    gormsqlite "github.com/dracory/gorm-sqlite"
    _ "modernc.org/sqlite" // register the SQLite driver
)

db, err := gorm.Open(gormsqlite.Open("app.db"), &gorm.Config{})
```

### Open an in-memory database

```go
db, err := gorm.Open(gormsqlite.Open(":memory:"), &gorm.Config{})
```

### Auto-migrate a model

```go
type User struct {
    gorm.Model
    Name  string
    Email string `gorm:"uniqueIndex"`
}

db.AutoMigrate(&User{})
```

### Use a custom driver name

If you want to plug in a different registered SQLite driver (e.g. SQLCipher):

```go
db, err := gorm.Open(&gormsqlite.Dialector{
    DriverName: "sqlcipher",
    DSN:        "encrypted.db",
}, &gorm.Config{})
```

## Features

- **CGO-free** — uses `modernc.org/sqlite`, so no C toolchain is needed
- **Full GORM migration support** — `AutoMigrate`, `CreateTable`, `DropTable`, `AddColumn`, `DropColumn`, `AlterColumn`, `CreateIndex`, `DropIndex`, `RenameIndex`, `CreateConstraint`, `DropConstraint`
- **SQLite ≥ 3.35 RETURNING clause** — detected at runtime; `INSERT`/`UPDATE`/`DELETE ... RETURNING` enabled automatically
- **Row locking silenced** — `FOR UPDATE` / `LOCK IN SHARE MODE` clauses are ignored rather than returning an error, since SQLite uses file-level locking
- **Connection pool pre-configured** — `MaxOpenConns=1`, `MaxIdleConns=1`, `ConnMaxLifetime=1h` (SQLite serialises writes, so a single connection is the correct default)

## Security Notes

- DSN validation rejects path traversal sequences (`..`) and absolute paths by default. Pass a `gorm.ConnPool` directly if you need an absolute path.
- `Explain` redacts string parameters whose values contain common sensitive keywords (`password`, `secret`, `token`, etc.) before they appear in log output.
- DDL parsing is bounded to 1 MB per statement and wrapped in a 5-second timeout to prevent ReDoS.

## License

See [LICENSE](LICENSE).