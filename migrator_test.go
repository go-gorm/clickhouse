package clickhouse_test

import (
	"testing"
	"time"
)

type User struct {
	ID        uint `gorm:"primaryKey"`
	Name      string
	FirstName string
	LastName  string
	Age       int64
	Active    bool
	Salary    float32
	CreatedAt time.Time
	UpdatedAt time.Time
}

func TestAutoMigrate(t *testing.T) {
	type UserMigrateColumn struct {
		ID           uint
		Name         string
		IsAdmin      bool
		Birthday     time.Time `gorm:"precision:4"`
		Debit        float64   `gorm:"precision:4"`
		Note         string    `gorm:"size:10;comment:my note"`
		DefaultValue string    `gorm:"default:hello world"`
	}

	if DB.Migrator().HasColumn("users", "is_admin") {
		t.Fatalf("users's is_admin column should not exists")
	}

	if err := DB.Table("users").AutoMigrate(&UserMigrateColumn{}); err != nil {
		t.Fatalf("no error should happen when auto migrate")
	}

	if !DB.Migrator().HasTable("users") {
		t.Fatalf("users should exists")
	}

	if !DB.Migrator().HasColumn("users", "is_admin") {
		t.Fatalf("users's is_admin column should exists after auto migrate")
	}

	columnTypes, err := DB.Migrator().ColumnTypes("users")
	if err != nil {
		t.Fatalf("failed to get column types, got error %v", err)
	}

	for _, columnType := range columnTypes {
		switch columnType.Name() {
		case "id":
			if columnType.DatabaseTypeName() != "UInt64" {
				t.Fatalf("column id primary key should be correct, name: %v, column: %#v", columnType.Name(), columnType)
			}
		case "note":
			if length, ok := columnType.Length(); !ok || length != 10 {
				t.Fatalf("column name length should be correct, name: %v, column: %#v", columnType.Name(), columnType)
			}

			if comment, ok := columnType.Comment(); !ok || comment != "my note" {
				t.Fatalf("column name length should be correct, name: %v, column: %#v", columnType.Name(), columnType)
			}
		case "default_value":
			if defaultValue, ok := columnType.DefaultValue(); !ok || defaultValue != "hello world" {
				t.Fatalf("column name default_value should be correct, name: %v, column: %#v", columnType.Name(), columnType)
			}
		case "debit":
			if decimal, scale, ok := columnType.DecimalSize(); !ok || scale != 0 || decimal != 4 {
				t.Fatalf("column name debit should be correct, name: %v, column: %#v", columnType.Name(), columnType)
			}
		case "birthday":
			if decimal, scale, ok := columnType.DecimalSize(); !ok || scale != 0 || decimal != 4 {
				t.Fatalf("column name birthday should be correct, name: %v, column: %#v", columnType.Name(), columnType)
			}
		}
	}
}

func TestMigrator_HasIndex(t *testing.T) {
	type UserWithIndex struct {
		FirstName string    `gorm:"index:full_name"`
		LastName  string    `gorm:"index:full_name"`
		CreatedAt time.Time `gorm:"index"`
	}
	if DB.Migrator().HasIndex("users", "full_name") {
		t.Fatalf("users's full_name index should not exists")
	}

	if err := DB.Table("users").AutoMigrate(&UserWithIndex{}); err != nil {
		t.Fatalf("no error should happen when auto migrate")
	}

	if !DB.Migrator().HasIndex("users", "full_name") {
		t.Fatalf("users's full_name index should exists after auto migrate")
	}

	if err := DB.Table("users").AutoMigrate(&UserWithIndex{}); err != nil {
		t.Fatalf("no error should happen when auto migrate again")
	}
}
