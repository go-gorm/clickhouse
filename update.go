package clickhouse

import (
	"regexp"

	"github.com/ClickHouse/clickhouse-go/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
)

const updateLocalTableName = "gorm:clickhouse:update_local_table_name"

var tableRegexp = regexp.MustCompile("(?i)(alter\\s+table\\s+(?:`?[\\d\\w_]+`?\\.)?`?)([\\d\\w_]+)(`?)")

type UpdateLocalTable struct {
	Table  string
	Prefix string
	Suffix string
}

// ModifyStatement modify operation mode
func (t UpdateLocalTable) ModifyStatement(stmt *gorm.Statement) {
	stmt.Settings.Store(updateLocalTableName, t)
}

// Build implements clause.Expression interface
func (t UpdateLocalTable) Build(clause.Builder) {
}

func (t UpdateLocalTable) ModifySQL(sql string) string {
	switch {
	case t.Suffix != "":
		return tableRegexp.ReplaceAllString(sql, "${1}${2}"+t.Suffix+"${3}")
	case t.Prefix != "":
		return tableRegexp.ReplaceAllString(sql, "${1}"+t.Prefix+"${2}${3}")
	case t.Table != "":
		return tableRegexp.ReplaceAllString(sql, "${1}"+t.Table+"${3}")
	}
	return sql
}

func (dialector *Dialector) Update(db *gorm.DB) {
	if db.Error != nil {
		return
	}

	if db.Statement.Schema != nil {
		for _, c := range db.Statement.Schema.UpdateClauses {
			db.Statement.AddClause(c)
		}
	}

	if db.Statement.SQL.Len() == 0 {
		db.Statement.SQL.Grow(180)
		db.Statement.AddClauseIfNotExists(clause.Update{})
		if _, ok := db.Statement.Clauses["SET"]; !ok {
			if set := callbacks.ConvertToAssignments(db.Statement); len(set) != 0 {
				defer delete(db.Statement.Clauses, "SET")
				db.Statement.AddClause(set)
			} else {
				return
			}
		}

		db.Statement.Build(db.Statement.BuildClauses...)
	}

	if db.Error != nil {
		return
	}

	updateSQL := db.Statement.SQL.String()
	if updateLocalTableClause, ok := db.Statement.Settings.Load(updateLocalTableName); ok && len(dialector.options.Addr) >= 1 {
		if updateLocalTable, ok := updateLocalTableClause.(UpdateLocalTable); ok {
			var (
				err       error
				opts      = dialector.options
				addrs     = opts.Addr
				updateSQL = updateLocalTable.ModifySQL(updateSQL)
			)

			db.Statement.SQL.Reset()
			db.Statement.SQL.WriteString(updateSQL)

			if !db.DryRun {
				for i := 0; i < len(addrs); i++ {
					opts := opts
					opts.Addr = []string{addrs[i]}

					for j := 0; j < 3; j++ {
						if conn, e := clickhouse.Open(&opts); e == nil {
							if e = conn.Exec(db.Statement.Context, updateSQL, db.Statement.Vars...); e == nil {
								break
							}
							err = e
						}
					}

					if err != nil {
						break
					}
				}
			}
			db.AddError(err)
			return
		}
	}

	if !db.DryRun {
		result, err := db.Statement.ConnPool.ExecContext(db.Statement.Context, updateSQL, db.Statement.Vars...)

		if db.AddError(err) == nil {
			db.RowsAffected, _ = result.RowsAffected()
		}
	}
}
