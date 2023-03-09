package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/hashicorp/go-version"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)

type Config struct {
	DriverName                   string
	DSN                          string
	Conn                         gorm.ConnPool
	DisableDatetimePrecision     bool
	DontSupportRenameColumn      bool
	DontSupportColumnPrecision   bool
	DontSupportEmptyDefaultValue bool
	SkipInitializeWithVersion    bool
	DefaultGranularity           int    // 1 granule = 8192 rows
	DefaultCompression           string // default compression algorithm. LZ4 is lossless
	DefaultIndexType             string // index stores extremes of the expression
	DefaultTableEngineOpts       string
}

type Dialector struct {
	*Config
	Version string
}

func Open(dsn string) gorm.Dialector {
	return &Dialector{Config: &Config{DSN: dsn}}
}

func New(config Config) gorm.Dialector {
	return &Dialector{Config: &config}
}

func (dialector Dialector) Name() string {
	return "clickhouse"
}

func (dialector Dialector) Initialize(db *gorm.DB) (err error) {
	// register callbacks
	ctx := context.Background()
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
		DeleteClauses: []string{"DELETE", "WHERE"},
	})
	db.Callback().Create().Replace("gorm:create", Create)

	// assign option fields to default values
	if dialector.DriverName == "" {
		dialector.DriverName = "clickhouse"
	}

	// default settings
	if dialector.Config.DefaultGranularity == 0 {
		dialector.Config.DefaultGranularity = 3
	}

	if dialector.Config.DefaultCompression == "" {
		dialector.Config.DefaultCompression = "LZ4"
	}

	if dialector.DefaultIndexType == "" {
		dialector.DefaultIndexType = "minmax"
	}

	if dialector.DefaultTableEngineOpts == "" {
		dialector.DefaultTableEngineOpts = "ENGINE=MergeTree() ORDER BY tuple()"
	}

	if dialector.Conn != nil {
		db.ConnPool = dialector.Conn
	} else {
		db.ConnPool, err = sql.Open(dialector.DriverName, dialector.DSN)
		if err != nil {
			return err
		}
	}

	if !dialector.SkipInitializeWithVersion {
		err = db.ConnPool.QueryRowContext(ctx, "SELECT version()").Scan(&dialector.Version)
		if err != nil {
			return err
		}
		if dbversion, err := version.NewVersion(dialector.Version); err == nil {
			versionNoRenameColumn, _ := version.NewConstraint("< 20.4")

			if versionNoRenameColumn.Check(dbversion) {
				dialector.Config.DontSupportRenameColumn = true
			}

			versionNoPrecisionColumn, _ := version.NewConstraint("< 21.11")
			if versionNoPrecisionColumn.Check(dbversion) {
				dialector.DontSupportColumnPrecision = true
			}
		}
	}

	for k, v := range dialector.ClauseBuilders() {
		db.ClauseBuilders[k] = v
	}
	return
}

func modifyStatementWhereConds(stmt *gorm.Statement) {
	if c, ok := stmt.Clauses["WHERE"]; ok {
		if where, ok := c.Expression.(clause.Where); ok {
			modifyExprs(where.Exprs)
		}
	}
}

func modifyExprs(exprs []clause.Expression) {
	for idx, expr := range exprs {
		switch v := expr.(type) {
		case clause.AndConditions:
			modifyExprs(v.Exprs)
		case clause.NotConditions:
			modifyExprs(v.Exprs)
		case clause.OrConditions:
			modifyExprs(v.Exprs)
		default:
			reflectValue := reflect.ValueOf(expr)
			if reflectValue.Kind() == reflect.Struct {
				if field := reflectValue.FieldByName("Column"); field.IsValid() && !field.IsZero() {
					if column, ok := field.Interface().(clause.Column); ok {
						column.Table = ""
						result := reflect.New(reflectValue.Type()).Elem()
						result.Set(reflectValue)
						result.FieldByName("Column").Set(reflect.ValueOf(column))
						exprs[idx] = result.Interface().(clause.Expression)
					}
				}
			}
		}
	}
}

func (dialector Dialector) ClauseBuilders() map[string]clause.ClauseBuilder {
	clauseBuilders := map[string]clause.ClauseBuilder{
		"DELETE": func(c clause.Clause, builder clause.Builder) {
			builder.WriteString("ALTER TABLE ")

			var addedTable bool
			if stmt, ok := builder.(*gorm.Statement); ok {
				if c, ok := stmt.Clauses["FROM"]; ok {
					addedTable = true
					c.Name = ""
					c.Build(builder)
				}
				modifyStatementWhereConds(stmt)
			}

			if !addedTable {
				builder.WriteQuoted(clause.Table{Name: clause.CurrentTable})
			}
			builder.WriteString(" DELETE")
		},
		"UPDATE": func(c clause.Clause, builder clause.Builder) {
			builder.WriteString("ALTER TABLE ")

			var addedTable bool
			if stmt, ok := builder.(*gorm.Statement); ok {
				if c, ok := stmt.Clauses["FROM"]; ok {
					addedTable = true
					c.Name = ""
					c.Build(builder)
				}
				modifyStatementWhereConds(stmt)
			}

			if !addedTable {
				builder.WriteQuoted(clause.Table{Name: clause.CurrentTable})
			}
			builder.WriteString(" UPDATE")
		},
		"SET": func(c clause.Clause, builder clause.Builder) {
			c.Name = ""
			c.Build(builder)
		},
	}

	return clauseBuilders
}

func (dialector Dialector) Migrator(db *gorm.DB) gorm.Migrator {
	return Migrator{
		Migrator: migrator.Migrator{
			Config: migrator.Config{
				DB:        db,
				Dialector: dialector,
			},
		},
		Dialector: dialector,
	}
}

func (dialector Dialector) DataTypeOf(field *schema.Field) string {
	switch field.DataType {
	case schema.Bool:
		return "UInt8"
	case schema.Int, schema.Uint:
		sqlType := "Int64"
		switch {
		case field.Size <= 8:
			sqlType = "Int8"
		case field.Size <= 16:
			sqlType = "Int16"
		case field.Size <= 32:
			sqlType = "Int32"
		}
		if field.DataType == schema.Uint {
			sqlType = "U" + sqlType
		}
		return sqlType
	case schema.Float:
		if field.Precision > 0 {
			return fmt.Sprintf("decimal(%d, %d)", field.Precision, field.Scale)
		}
		if field.Size <= 32 {
			return "Float32"
		}
		return "Float64"
	case schema.String:
		if field.Size == 0 {
			return "String"
		}
		return fmt.Sprintf("FixedString(%d)", field.Size)
	case schema.Bytes:
		return "String"
	case schema.Time:
		// TODO: support TimeZone
		precision := ""
		if !dialector.DisableDatetimePrecision {
			if field.Precision == 0 {
				field.Precision = 3
			}
			if field.Precision > 0 {
				precision = fmt.Sprintf("(%d)", field.Precision)
			}
		}
		return "DateTime64" + precision
	}

	return string(field.DataType)
}

func (dialector Dialector) DefaultValueOf(field *schema.Field) clause.Expression {
	return clause.Expr{SQL: "DEFAULT"}
}

func (dialector Dialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
	writer.WriteByte('?')
}

func (dialector Dialector) QuoteTo(writer clause.Writer, str string) {
	writer.WriteByte('`')
	if strings.Contains(str, ".") {
		for idx, str := range strings.Split(str, ".") {
			if idx > 0 {
				writer.WriteString(".`")
			}
			writer.WriteString(str)
			writer.WriteByte('`')
		}
	} else {
		writer.WriteString(str)
		writer.WriteByte('`')
	}
}

func (dialector Dialector) Explain(sql string, vars ...interface{}) string {
	return logger.ExplainSQL(sql, nil, `'`, vars...)
}

func (dialectopr Dialector) SavePoint(tx *gorm.DB, name string) error {
	return gorm.ErrUnsupportedDriver
}

func (dialectopr Dialector) RollbackTo(tx *gorm.DB, name string) error {
	return gorm.ErrUnsupportedDriver
}
