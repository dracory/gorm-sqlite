package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)

// DriverName is the default driver name for SQLite.
const DriverName = "sqlite"

func validateDSN(dsn string) error {
	// Allow in-memory databases
	if dsn == ":memory:" {
		return nil
	}

	decoded, err := url.PathUnescape(dsn)
	if err != nil {
		return fmt.Errorf("invalid DSN encoding: %w", err)
	}

	// Check for path traversal attempts
	if strings.Contains(decoded, "..") {
		return fmt.Errorf("DSN contains path traversal sequence")
	}

	if filepath.IsAbs(decoded) {
		return fmt.Errorf("absolute paths are not allowed in DSN")
	}

	return nil
}

type Dialector struct {
	DriverName string
	DSN        string
	Conn       gorm.ConnPool
}

func Open(dsn string) gorm.Dialector {
	return &Dialector{DSN: dsn}
}

func (dialector Dialector) Name() string {
	return "sqlite"
}

func (dialector Dialector) Initialize(db *gorm.DB) (err error) {
	if dialector.DriverName == "" {
		dialector.DriverName = DriverName
	}

	if dialector.Conn != nil {
		db.ConnPool = dialector.Conn
	} else {
		// Validate DSN before opening connection
		if err := validateDSN(dialector.DSN); err != nil {
			return fmt.Errorf("invalid DSN: %w", err)
		}
		conn, err := sql.Open(dialector.DriverName, dialector.DSN)
		if err != nil {
			return err
		}
		conn.SetMaxOpenConns(1)
		conn.SetMaxIdleConns(1)
		conn.SetConnMaxLifetime(time.Hour)
		db.ConnPool = conn
	}

	db.Dialector = dialector

	var version string
	if err := db.ConnPool.QueryRowContext(context.Background(), "select sqlite_version()").Scan(&version); err != nil {
		return err
	}
	// https://www.sqlite.org/releaselog/3_35_0.html
	cmp, err := compareVersion(version, "3.35.0")
	if err != nil {
		return err
	}
	if cmp >= 0 {
		callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
			CreateClauses:        []string{"INSERT", "VALUES", "ON CONFLICT", "RETURNING"},
			UpdateClauses:        []string{"UPDATE", "SET", "FROM", "WHERE", "RETURNING"},
			DeleteClauses:        []string{"DELETE", "FROM", "WHERE", "RETURNING"},
			LastInsertIDReversed: true,
		})
	} else {
		callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
			LastInsertIDReversed: true,
		})
	}

	for k, v := range dialector.ClauseBuilders() {
		db.ClauseBuilders[k] = v
	}

	return
}

func (dialector Dialector) ClauseBuilders() map[string]clause.ClauseBuilder {
	return map[string]clause.ClauseBuilder{
		"INSERT": func(c clause.Clause, builder clause.Builder) {
			if insert, ok := c.Expression.(clause.Insert); ok {
				if stmt, ok := builder.(*gorm.Statement); ok {
					stmt.WriteString("INSERT ")
					if insert.Modifier != "" {
						stmt.WriteString(insert.Modifier)
						stmt.WriteByte(' ')
					}

					stmt.WriteString("INTO ")
					if insert.Table.Name == "" {
						stmt.WriteQuoted(stmt.Table)
					} else {
						stmt.WriteQuoted(insert.Table)
					}
					return
				}
			}

			c.Build(builder)
		},
		"LIMIT": func(c clause.Clause, builder clause.Builder) {
			if limit, ok := c.Expression.(clause.Limit); ok {
				var lmt = -1
				if limit.Limit != nil && *limit.Limit >= 0 {
					lmt = *limit.Limit
				}
				if lmt >= 0 || limit.Offset > 0 {
					builder.WriteString("LIMIT ")
					builder.WriteString(strconv.Itoa(lmt))
				}
				if limit.Offset > 0 {
					builder.WriteString(" OFFSET ")
					builder.WriteString(strconv.Itoa(limit.Offset))
				}
			}
		},
		"FOR": func(c clause.Clause, builder clause.Builder) {
			if _, ok := c.Expression.(clause.Locking); ok {
				// SQLite3 does not support row-level locking.
				return
			}
			c.Build(builder)
		},
	}
}

func (dialector Dialector) DefaultValueOf(field *schema.Field) clause.Expression {
	if field.AutoIncrement {
		return clause.Expr{SQL: "NULL"}
	}

	// doesn't work, will raise error
	return clause.Expr{SQL: "DEFAULT"}
}

func (dialector Dialector) Migrator(db *gorm.DB) gorm.Migrator {
	return Migrator{migrator.Migrator{Config: migrator.Config{
		DB:                          db,
		Dialector:                   dialector,
		CreateIndexAfterCreateTable: true,
	}}}
}

func (dialector Dialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
	writer.WriteByte('?')
}

func (dialector Dialector) QuoteTo(writer clause.Writer, str string) {
	// Validate length to prevent memory issues
	const maxIdentifierLength = 1000 // SQLite default limit
	if len(str) > maxIdentifierLength {
		writer.WriteString("[INVALID_IDENTIFIER]")
		return
	}

	var (
		underQuoted, selfQuoted bool
		continuousBacktick      int8
		shiftDelimiter          int8
	)

	for _, v := range str {
		switch v {
		case '`':
			continuousBacktick++
			if continuousBacktick == 2 {
				writer.WriteString("``")
				continuousBacktick = 0
			}
		case '.':
			if continuousBacktick > 0 || !selfQuoted {
				shiftDelimiter = 0
				underQuoted = false
				continuousBacktick = 0
				writer.WriteString("`")
			}
			writer.WriteByte(byte(v))
			continue
		default:
			if shiftDelimiter-continuousBacktick <= 0 && !underQuoted {
				writer.WriteString("`")
				underQuoted = true
				if selfQuoted = continuousBacktick > 0; selfQuoted {
					continuousBacktick -= 1
				}
			}

			for ; continuousBacktick > 0; continuousBacktick -= 1 {
				writer.WriteString("``")
			}

			writer.WriteString(string(v))
		}

		if continuousBacktick > 0 {
			writer.WriteString("`")
			continuousBacktick -= 1
		}

		shiftDelimiter++
	}

	if continuousBacktick > 0 {
		writer.WriteString("`")
	}
	if underQuoted || selfQuoted {
		writer.WriteString("`")
	}
}

func (dialector Dialector) DataTypeOf(field *schema.Field) string {
	return string(field.DataType)
}

func isSensitive(v interface{}) bool {
	if v == nil {
		return false
	}
	s := fmt.Sprintf("%v", v)
	sensitivePatterns := []string{
		"password", "passwd", "secret", "token", "api_key",
		"credit_card", "ssn", "social_security",
	}
	lower := strings.ToLower(s)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func (dialector Dialector) Explain(sql string, vars ...interface{}) string {
	// Redact sensitive parameters
	redactedVars := make([]interface{}, len(vars))
	for i, v := range vars {
		if isSensitive(v) {
			redactedVars[i] = "[REDACTED]"
		} else {
			redactedVars[i] = v
		}
	}
	return logger.ExplainSQL(sql, nil, `"`, redactedVars...)
}

func compareVersion(version1, version2 string) (int, error) {
	n, m := len(version1), len(version2)
	i, j := 0, 0
	for i < n || j < m {
		var x, y int
		for i < n && version1[i] != '.' {
			if version1[i] < '0' || version1[i] > '9' {
				return 0, fmt.Errorf("invalid version string: %q", version1)
			}
			x = x*10 + int(version1[i]-'0')
			i++
		}
		i++ // skip dot
		for j < m && version2[j] != '.' {
			if version2[j] < '0' || version2[j] > '9' {
				return 0, fmt.Errorf("invalid version string: %q", version2)
			}
			y = y*10 + int(version2[j]-'0')
			j++
		}
		j++ // skip dot
		if x > y {
			return 1, nil
		}
		if x < y {
			return -1, nil
		}
	}
	return 0, nil
}
