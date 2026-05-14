package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"reflect"
	"strconv"
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

// SQLiteTime is a custom time type for SQLite that handles time string values
type SQLiteTime struct {
	time.Time
}

// Scan implements sql.Scanner
func (t *SQLiteTime) Scan(value interface{}) error {
	if value == nil {
		t.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		var err error
		t.Time, err = time.Parse("2006-01-02 15:04:05.999999999-07:00", string(v))
		if err != nil {
			// Try parsing without timezone
			t.Time, err = time.Parse("2006-01-02 15:04:05", string(v))
			if err != nil {
				// Try parsing RFC3339
				t.Time, err = time.Parse(time.RFC3339, string(v))
				if err != nil {
					return err
				}
			}
		}
		return nil
	case string:
		var err error
		t.Time, err = time.Parse("2006-01-02 15:04:05.999999999-07:00", v)
		if err != nil {
			t.Time, err = time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				t.Time, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return nil
	}
}

// Value implements driver.Valuer
func (t SQLiteTime) Value() (driver.Value, error) {
	return t.Time, nil
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
		conn, err := sql.Open(dialector.DriverName, dialector.DSN)
		if err != nil {
			return err
		}
		db.ConnPool = conn
	}

	// Register time scanner for SQLite
	db.Dialector = dialector

	var version string
	if err := db.ConnPool.QueryRowContext(context.Background(), "select sqlite_version()").Scan(&version); err != nil {
		return err
	}
	// https://www.sqlite.org/releaselog/3_35_0.html
	if compareVersion(version, "3.35.0") >= 0 {
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

	// Register time scanner callback
	db.Callback().Query().Before("gorm:query").Register("sqlite:time_scanner", func(db *gorm.DB) {
		if db.Statement.Dest != nil {
			destValue := reflect.ValueOf(db.Statement.Dest)
			if destValue.Kind() == reflect.Ptr {
				destValue = destValue.Elem()
			}
			if destValue.Kind() == reflect.Struct {
				for i := 0; i < destValue.NumField(); i++ {
					field := destValue.Field(i)
					if field.Type() == reflect.TypeOf(time.Time{}) && field.CanSet() {
						if strVal, ok := field.Interface().(string); ok {
							if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", strVal); err == nil {
								field.Set(reflect.ValueOf(t))
							} else if t, err := time.Parse("2006-01-02 15:04:05", strVal); err == nil {
								field.Set(reflect.ValueOf(t))
							}
						}
					}
				}
			}
		}
	})

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
	var (
		underQuoted, selfQuoted bool
		continuousBacktick      int8
		shiftDelimiter          int8
	)

	for _, v := range []byte(str) {
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
			writer.WriteByte(v)
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

			writer.WriteByte(v)
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

func (dialector Dialector) Explain(sql string, vars ...interface{}) string {
	return logger.ExplainSQL(sql, nil, `"`, vars...)
}

func compareVersion(version1, version2 string) int {
	n, m := len(version1), len(version2)
	i, j := 0, 0
	for i < n || j < m {
		var x, y int
		for i < n && version1[i] != '.' {
			x = x*10 + int(version1[i]-'0')
			i++
		}
		i++ // skip dot
		for j < m && version2[j] != '.' {
			y = y*10 + int(version2[j]-'0')
			j++
		}
		j++ // skip dot
		if x > y {
			return 1
		}
		if x < y {
			return -1
		}
	}
	return 0
}
