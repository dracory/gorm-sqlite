package sqlite

import "errors"

var (
	ErrConstraintsNotImplemented = errors.New("constraints not implemented on sqlite, consider using DisableForeignKeyConstraintWhenMigrating. See GORM documentation for migration details.")
)
