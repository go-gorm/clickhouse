# GORM ClickHouse Driver

Clickhouse support for GORM

[![test status](https://github.com/go-gorm/clickhouse/workflows/tests/badge.svg?branch=master "test status")](https://github.com/go-gorm/clickhouse/actions)

## Quick Start

You can simply test your connection to your database with the following:

```go
package main

import (
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

type User struct {
	Name string
	Age  int
}

func main() {
	dsn := "tcp://localhost:9000?database=gorm&username=gorm&password=gorm&read_timeout=10&write_timeout=20"
	db, err := gorm.Open(clickhouse.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	// Auto Migrate
	db.AutoMigrate(&User{})
	// Set table options
	db.Set("gorm:table_options", "ENGINE=Distributed(cluster, default, hits)").AutoMigrate(&User{})

	// Insert
	db.Create(&User{Name: "Angeliz", Age: 18})

	// Select
	db.Find(&User{}, "name = ?", "Angeliz")

	// Batch Insert
	user1 := User{Age: 12, Name: "Bruce Lee"}
	user2 := User{Age: 13, Name: "Feynman"}
	user3 := User{Age: 14, Name: "Angeliz"}
	var users = []User{user1, user2, user3}
	db.Create(&users)
	// ...
}

```

## Advanced Configuration

```go
package main

import (
  "gorm.io/driver/clickhouse"
  "gorm.io/gorm"
)

// refer to https://github.com/ClickHouse/clickhouse-go
var dsn = "tcp://localhost:9000?database=gorm&username=gorm&password=gorm&read_timeout=10&write_timeout=20"

func main() {
  db, err := gorm.Open(clickhouse.New(click.Config{
    DSN: dsn,
    Conn: conn,                       // initialize with existing database conn
    DisableDatetimePrecision: true,   // disable datetime64 precision, not supported before clickhouse 20.4
    DontSupportRenameColumn: true,    // rename column not supported before clickhouse 20.4
    SkipInitializeWithVersion: false, // smart configure based on used version
    DefaultGranularity: 3,            // 1 granule = 8192 rows
    DefaultCompression: "LZ4",        // default compression algorithm. LZ4 is lossless
    DefaultIndexType: "minmax",       // index stores extremes of the expression
    DefaultTableEngineOpts: "ENGINE=MergeTree() ORDER BY tuple()",
  }), &gorm.Config{})
}
```

Checkout [https://gorm.io](https://gorm.io) for details.
