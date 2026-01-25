package query

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/sofired/grizzle/dialect"
)

// BatchInsertBuilder constructs multi-row INSERT statements.
// It is immutable: every method returns a new copy.
type BatchInsertBuilder struct {
	table       TableSource
	rows        []any
	columns     []string
	onConflict  string
	doUpdate    []string
	doNothing   bool
	returning   []SelectableColumn
	ignoreConfs bool
}

// BatchInsert creates a new builder for multi-row inserts.
func BatchInsert(table TableSource) BatchInsertBuilder {
	return BatchInsertBuilder{table: table}
}

// Values appends one or more rows to insert.
// Each value should be a struct with `db:"column_name"` tags.
func (b BatchInsertBuilder) Values(rows ...any) BatchInsertBuilder {
	nb := b
	nb.rows = make([]any, len(b.rows)+len(rows))
	copy(nb.rows, b.rows)
	copy(nb.rows[len(b.rows):], rows)
	return nb
}

// OnConflict specifies the conflict target column for upsert.
func (b BatchInsertBuilder) OnConflict(col string) BatchInsertBuilder {
	nb := b
	nb.onConflict = col
	return nb
}

// DoUpdateSetExcluded sets the columns to update on conflict using EXCLUDED values.
func (b BatchInsertBuilder) DoUpdateSetExcluded(cols ...string) BatchInsertBuilder {
	nb := b
	nb.doUpdate = make([]string, len(cols))
	copy(nb.doUpdate, cols)
	return nb
}

// DoNothing indicates that conflicts should be silently ignored.
func (b BatchInsertBuilder) DoNothing() BatchInsertBuilder {
	nb := b
	nb.doNothing = true
	return nb
}

// IgnoreConflicts uses INSERT IGNORE (MySQL) or INSERT OR IGNORE (SQLite).
func (b BatchInsertBuilder) IgnoreConflicts() BatchInsertBuilder {
	nb := b
	nb.ignoreConfs = true
	return nb
}

// Returning adds a RETURNING clause (PostgreSQL, SQLite 3.35+).
func (b BatchInsertBuilder) Returning(cols ...SelectableColumn) BatchInsertBuilder {
	nb := b
	nb.returning = make([]SelectableColumn, len(cols))
	copy(nb.returning, cols)
	return nb
}

// extractColumns uses reflection to read `db` struct tags from the first row.
func extractColumns(row any) ([]string, error) {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("batch insert: expected struct, got %s", v.Kind())
	}
	t := v.Type()
	var cols []string
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		// Strip tag options (e.g. "id,omitempty" → "id").
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		if tag == "" {
			continue
		}
		cols = append(cols, tag)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("batch insert: no db-tagged fields found in %s", t.Name())
	}
	return cols, nil
}

// extractValues extracts field values corresponding to the given columns.
func extractValues(row any, columns []string) ([]any, error) {
	v := reflect.ValueOf(row)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("batch insert: expected struct, got %s", v.Kind())
	}
	t := v.Type()

	tagIndex := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		// Strip tag options (e.g. "id,omitempty" → "id").
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		if tag != "" {
			tagIndex[tag] = i
		}
	}

	vals := make([]any, 0, len(columns))
	for _, col := range columns {
		idx, ok := tagIndex[col]
		if !ok {
			return nil, fmt.Errorf("batch insert: column %q not found in struct", col)
		}
		vals = append(vals, v.Field(idx).Interface())
	}
	return vals, nil
}

// Build generates the SQL and argument slice for the batch insert.
func (b BatchInsertBuilder) Build(d dialect.Dialect) (string, []any) {
	if len(b.rows) == 0 {
		return "", nil
	}

	// Extract columns from the first row.
	columns, err := extractColumns(b.rows[0])
	if err != nil {
		panic(err)
	}

	tableName := b.table.GRizTableName()
	var sb strings.Builder
	var args []any

	// INSERT [IGNORE] INTO table (cols)
	if b.ignoreConfs {
		switch d.Name() {
		case "mysql":
			sb.WriteString("INSERT IGNORE INTO ")
		case "sqlite":
			sb.WriteString("INSERT OR IGNORE INTO ")
		default:
			sb.WriteString("INSERT INTO ")
		}
	} else {
		sb.WriteString("INSERT INTO ")
	}
	sb.WriteString(tableName)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(columns, ", "))
	sb.WriteString(") VALUES ")

	// Build value tuples for each row.
	paramIdx := 1
	for i, row := range b.rows {
		if i > 0 {
			sb.WriteString(", ")
		}
		vals, err := extractValues(row, columns)
		if err != nil {
			panic(err)
		}
		sb.WriteString("(")
		for j := range vals {
			if j > 0 {
				sb.WriteString(", ")
			}
			switch d.Name() {
			case "postgres":
				sb.WriteString(fmt.Sprintf("$%d", paramIdx))
			default:
				sb.WriteString("?")
			}
			paramIdx++
		}
		sb.WriteString(")")
		args = append(args, vals...)
	}

	// ON CONFLICT clause.
	if b.onConflict != "" {
		if b.doNothing {
			switch d.Name() {
			case "mysql":
				sb.WriteString(" ON DUPLICATE KEY UPDATE ")
				sb.WriteString(columns[0])
				sb.WriteString(" = ")
				sb.WriteString(columns[0])
			default:
				sb.WriteString(" ON CONFLICT (")
				sb.WriteString(b.onConflict)
				sb.WriteString(") DO NOTHING")
			}
		} else if len(b.doUpdate) > 0 {
			switch d.Name() {
			case "mysql":
				sb.WriteString(" ON DUPLICATE KEY UPDATE ")
				for i, col := range b.doUpdate {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(col)
					sb.WriteString(" = VALUES(")
					sb.WriteString(col)
					sb.WriteString(")")
				}
			default:
				sb.WriteString(" ON CONFLICT (")
				sb.WriteString(b.onConflict)
				sb.WriteString(") DO UPDATE SET ")
				for i, col := range b.doUpdate {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(col)
					sb.WriteString(" = EXCLUDED.")
					sb.WriteString(col)
				}
			}
		}
	}

	// RETURNING clause (only for dialects that support it).
	if len(b.returning) > 0 && d.SupportsReturning() {
		sb.WriteString(" RETURNING ")
		for i, col := range b.returning {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(col.ColumnName())
		}
	}

	return sb.String(), args
}
