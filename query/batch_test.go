package query

import (
	"strings"
	"testing"

	"github.com/sofired/grizzle/dialect"
)

// testBatchTable implements TableSource for testing.
type testBatchTable struct{}

func (testBatchTable) TableName() string { return "users" }

// testReturnCol implements SelectableColumn for RETURNING tests.
type testReturnCol struct{ name string }

func (c testReturnCol) ColumnName() string { return c.name }
func (c testReturnCol) TableAlias() string { return "users" }

// Row types for testing.
type batchUserRow struct {
	ID    int    `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

func TestBatchInsert_Postgres(t *testing.T) {
	b := BatchInsert(testBatchTable{}).Values(
		batchUserRow{ID: 1, Name: "Alice", Email: "alice@example.com"},
		batchUserRow{ID: 2, Name: "Bob", Email: "bob@example.com"},
	)
	sql, args := b.Build(dialect.Postgres)

	expected := `INSERT INTO users (id, name, email) VALUES ($1, $2, $3), ($4, $5, $6)`
	if sql != expected {
		t.Errorf("SQL mismatch.\n  got:  %s\n  want: %s", sql, expected)
	}
	if len(args) != 6 {
		t.Errorf("expected 6 args, got %d", len(args))
	}
	if args[0] != 1 || args[1] != "Alice" || args[3] != 2 || args[4] != "Bob" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestBatchInsert_MySQL(t *testing.T) {
	b := BatchInsert(testBatchTable{}).Values(
		batchUserRow{ID: 1, Name: "Alice", Email: "a@b.com"},
		batchUserRow{ID: 2, Name: "Bob", Email: "b@c.com"},
	)
	sql, args := b.Build(dialect.MySQL)

	expected := `INSERT INTO users (id, name, email) VALUES (?, ?, ?), (?, ?, ?)`
	if sql != expected {
		t.Errorf("SQL mismatch.\n  got:  %s\n  want: %s", sql, expected)
	}
	if len(args) != 6 {
		t.Errorf("expected 6 args, got %d", len(args))
	}
}

func TestBatchInsert_SingleRow(t *testing.T) {
	b := BatchInsert(testBatchTable{}).Values(
		batchUserRow{ID: 1, Name: "Solo", Email: "solo@test.com"},
	)
	sql, args := b.Build(dialect.Postgres)

	expected := `INSERT INTO users (id, name, email) VALUES ($1, $2, $3)`
	if sql != expected {
		t.Errorf("SQL mismatch.\n  got:  %s\n  want: %s", sql, expected)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestBatchInsert_Empty(t *testing.T) {
	b := BatchInsert(testBatchTable{})
	sql, args := b.Build(dialect.Postgres)
	if sql != "" {
		t.Errorf("expected empty SQL for no rows, got: %s", sql)
	}
	if args != nil {
		t.Errorf("expected nil args for no rows, got: %v", args)
	}
}

func TestBatchInsert_OnConflictDoNothing_Postgres(t *testing.T) {
	b := BatchInsert(testBatchTable{}).
		Values(batchUserRow{ID: 1, Name: "Alice", Email: "a@b.com"}).
		OnConflict("id").
		DoNothing()

	sql, _ := b.Build(dialect.Postgres)
	expected := `INSERT INTO users (id, name, email) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`
	if sql != expected {
		t.Errorf("SQL mismatch.\n  got:  %s\n  want: %s", sql, expected)
	}
}

func TestBatchInsert_OnConflictDoUpdate_Postgres(t *testing.T) {
	b := BatchInsert(testBatchTable{}).
		Values(
			batchUserRow{ID: 1, Name: "Alice", Email: "alice@new.com"},
			batchUserRow{ID: 2, Name: "Bob", Email: "bob@new.com"},
		).
		OnConflict("id").
		DoUpdateSetExcluded("name", "email")

	sql, args := b.Build(dialect.Postgres)
	expected := `INSERT INTO users (id, name, email) VALUES ($1, $2, $3), ($4, $5, $6) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, email = EXCLUDED.email`
	if sql != expected {
		t.Errorf("SQL mismatch.\n  got:  %s\n  want: %s", sql, expected)
	}
	if len(args) != 6 {
		t.Errorf("expected 6 args, got %d", len(args))
	}
}

func TestBatchInsert_OnConflictDoUpdate_MySQL(t *testing.T) {
	b := BatchInsert(testBatchTable{}).
		Values(batchUserRow{ID: 1, Name: "Alice", Email: "a@b.com"}).
		OnConflict("id").
		DoUpdateSetExcluded("name", "email")

	sql, _ := b.Build(dialect.MySQL)
	expected := `INSERT INTO users (id, name, email) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE name = VALUES(name), email = VALUES(email)`
	if sql != expected {
		t.Errorf("SQL mismatch.\n  got:  %s\n  want: %s", sql, expected)
	}
}

func TestBatchInsert_IgnoreConflicts_MySQL(t *testing.T) {
	b := BatchInsert(testBatchTable{}).
		Values(batchUserRow{ID: 1, Name: "Alice", Email: "a@b.com"}).
		IgnoreConflicts()

	sql, _ := b.Build(dialect.MySQL)
	if !strings.HasPrefix(sql, "INSERT IGNORE INTO") {
		t.Errorf("expected INSERT IGNORE, got: %s", sql)
	}
}

func TestBatchInsert_IgnoreConflicts_SQLite(t *testing.T) {
	b := BatchInsert(testBatchTable{}).
		Values(batchUserRow{ID: 1, Name: "Alice", Email: "a@b.com"}).
		IgnoreConflicts()

	sql, _ := b.Build(dialect.SQLite)
	if !strings.HasPrefix(sql, "INSERT OR IGNORE INTO") {
		t.Errorf("expected INSERT OR IGNORE, got: %s", sql)
	}
}

func TestBatchInsert_ParameterNumbering(t *testing.T) {
	b := BatchInsert(testBatchTable{}).Values(
		batchUserRow{ID: 1, Name: "A", Email: "a"},
		batchUserRow{ID: 2, Name: "B", Email: "b"},
		batchUserRow{ID: 3, Name: "C", Email: "c"},
	)
	sql, args := b.Build(dialect.Postgres)

	if len(args) != 9 {
		t.Errorf("expected 9 args for 3 rows x 3 cols, got %d", len(args))
	}
	if !strings.Contains(sql, "$9") {
		t.Errorf("expected $9 in SQL, got: %s", sql)
	}
}

func TestBatchInsert_Immutable(t *testing.T) {
	b1 := BatchInsert(testBatchTable{}).Values(batchUserRow{ID: 1, Name: "A", Email: "a"})
	b2 := b1.Values(batchUserRow{ID: 2, Name: "B", Email: "b"})

	_, args1 := b1.Build(dialect.Postgres)
	_, args2 := b2.Build(dialect.Postgres)

	if len(args1) != 3 {
		t.Errorf("b1 should have 3 args, got %d", len(args1))
	}
	if len(args2) != 6 {
		t.Errorf("b2 should have 6 args, got %d", len(args2))
	}
}
