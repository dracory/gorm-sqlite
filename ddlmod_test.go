package sqlite

import (
	"database/sql"
	"testing"

	"gorm.io/gorm/migrator"
	"gorm.io/gorm/utils/tests"
)

func TestParseDDL(t *testing.T) {
	params := []struct {
		name    string
		sql     []string
		nFields int
		columns []migrator.ColumnType
	}{
		{
			"with_fk", []string{
				"CREATE TABLE `notes` (" +
					"`id` integer NOT NULL,`text` varchar(500) DEFAULT \"hello\",`age` integer DEFAULT 18,`user_id` integer,PRIMARY KEY (`id`),CONSTRAINT `fk_users_notes` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`))",
				"CREATE UNIQUE INDEX `idx_profiles_refer` ON `profiles`(`text`)",
			}, 6, []migrator.ColumnType{
				{NameValue: sql.NullString{String: "id", Valid: true}, DataTypeValue: sql.NullString{String: "integer", Valid: true}, ColumnTypeValue: sql.NullString{String: "integer", Valid: true}, PrimaryKeyValue: sql.NullBool{Bool: true, Valid: true}, NullableValue: sql.NullBool{Valid: true}, UniqueValue: sql.NullBool{Valid: true}, DefaultValueValue: sql.NullString{Valid: false}},
				{NameValue: sql.NullString{String: "text", Valid: true}, DataTypeValue: sql.NullString{String: "varchar", Valid: true}, LengthValue: sql.NullInt64{Int64: 500, Valid: true}, ColumnTypeValue: sql.NullString{String: "varchar(500)", Valid: true}, DefaultValueValue: sql.NullString{String: "hello", Valid: true}, NullableValue: sql.NullBool{Bool: true, Valid: true}, UniqueValue: sql.NullBool{Bool: false, Valid: true}, PrimaryKeyValue: sql.NullBool{Valid: true}},
				{NameValue: sql.NullString{String: "age", Valid: true}, DataTypeValue: sql.NullString{String: "integer", Valid: true}, ColumnTypeValue: sql.NullString{String: "integer", Valid: true}, DefaultValueValue: sql.NullString{String: "18", Valid: true}, NullableValue: sql.NullBool{Bool: true, Valid: true}, UniqueValue: sql.NullBool{Valid: true}, PrimaryKeyValue: sql.NullBool{Valid: true}},
				{NameValue: sql.NullString{String: "user_id", Valid: true}, DataTypeValue: sql.NullString{String: "integer", Valid: true}, ColumnTypeValue: sql.NullString{String: "integer", Valid: true}, DefaultValueValue: sql.NullString{Valid: false}, NullableValue: sql.NullBool{Bool: true, Valid: true}, UniqueValue: sql.NullBool{Valid: true}, PrimaryKeyValue: sql.NullBool{Valid: true}},
			},
		},
		{
			"with_check", []string{"CREATE TABLE Persons (ID int NOT NULL,LastName varchar(255) NOT NULL,FirstName varchar(255),Age int,CHECK (Age>=18),CHECK (FirstName<>'John'))"}, 6, []migrator.ColumnType{
				{NameValue: sql.NullString{String: "ID", Valid: true}, DataTypeValue: sql.NullString{String: "int", Valid: true}, ColumnTypeValue: sql.NullString{String: "int", Valid: true}, NullableValue: sql.NullBool{Valid: true}, DefaultValueValue: sql.NullString{Valid: false}, UniqueValue: sql.NullBool{Valid: true}, PrimaryKeyValue: sql.NullBool{Valid: true}},
				{NameValue: sql.NullString{String: "LastName", Valid: true}, DataTypeValue: sql.NullString{String: "varchar", Valid: true}, LengthValue: sql.NullInt64{Int64: 255, Valid: true}, ColumnTypeValue: sql.NullString{String: "varchar(255)", Valid: true}, NullableValue: sql.NullBool{Bool: false, Valid: true}, DefaultValueValue: sql.NullString{Valid: false}, UniqueValue: sql.NullBool{Valid: true}, PrimaryKeyValue: sql.NullBool{Valid: true}},
				{NameValue: sql.NullString{String: "FirstName", Valid: true}, DataTypeValue: sql.NullString{String: "varchar", Valid: true}, LengthValue: sql.NullInt64{Int64: 255, Valid: true}, ColumnTypeValue: sql.NullString{String: "varchar(255)", Valid: true}, DefaultValueValue: sql.NullString{Valid: false}, NullableValue: sql.NullBool{Bool: true, Valid: true}, UniqueValue: sql.NullBool{Valid: true}, PrimaryKeyValue: sql.NullBool{Valid: true}},
				{NameValue: sql.NullString{String: "Age", Valid: true}, DataTypeValue: sql.NullString{String: "int", Valid: true}, ColumnTypeValue: sql.NullString{String: "int", Valid: true}, DefaultValueValue: sql.NullString{Valid: false}, NullableValue: sql.NullBool{Bool: true, Valid: true}, UniqueValue: sql.NullBool{Valid: true}, PrimaryKeyValue: sql.NullBool{Valid: true}},
			},
		},
		{
			"lowercase", []string{"create table test (ID int NOT NULL)"}, 1, []migrator.ColumnType{
				{NameValue: sql.NullString{String: "ID", Valid: true}, DataTypeValue: sql.NullString{String: "int", Valid: true}, ColumnTypeValue: sql.NullString{String: "int", Valid: true}, NullableValue: sql.NullBool{Bool: false, Valid: true}, DefaultValueValue: sql.NullString{Valid: false}, UniqueValue: sql.NullBool{Valid: true}, PrimaryKeyValue: sql.NullBool{Valid: true}},
			},
		},
		{
			"no brackets", []string{"create table test"}, 0, nil,
		},
	}

	for _, p := range params {
		t.Run(p.name, func(t *testing.T) {
			ddl, err := parseDDL(p.sql...)

			if err != nil {
				t.Fatalf("parseDDL failed: %v", err)
			}

			tests.AssertEqual(t, p.sql[0], ddl.compile())
			if len(ddl.fields) != p.nFields {
				t.Fatalf("fields length doesn't match: expect: %v, got %v", p.nFields, len(ddl.fields))
			}
			tests.AssertEqual(t, ddl.columns, p.columns)
		})
	}
}
