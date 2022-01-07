module gorm.io/driver/clickhouse

go 1.14

require (
	github.com/ClickHouse/clickhouse-go v1.4.5
	github.com/hashicorp/go-version v1.3.0
	gorm.io/gorm v1.22.4
)

replace github.com/ClickHouse/clickhouse-go => github.com/go-gorm/clickhouse-go v1.4.5
