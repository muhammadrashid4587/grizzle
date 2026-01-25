package query

import (
	"testing"

	"github.com/sofired/grizzle/dialect"
)

// Benchmark row type.
type benchRow struct {
	ID    int    `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int    `db:"age"`
}

// ---------------------------------------------------------------------------
// Batch insert benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBatchInsert_1Row(b *testing.B) {
	b.ReportAllocs()
	tbl := testBatchTable{}
	row := batchUserRow{ID: 1, Name: "Alice", Email: "alice@example.com"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BatchInsert(tbl).Values(row).Build(dialect.Postgres)
	}
}

func BenchmarkBatchInsert_10Rows(b *testing.B) {
	b.ReportAllocs()
	tbl := testBatchTable{}
	rows := make([]any, 10)
	for i := range rows {
		rows[i] = batchUserRow{ID: i, Name: "User", Email: "u@test.com"}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BatchInsert(tbl).Values(rows...).Build(dialect.Postgres)
	}
}

func BenchmarkBatchInsert_100Rows(b *testing.B) {
	b.ReportAllocs()
	tbl := testBatchTable{}
	rows := make([]any, 100)
	for i := range rows {
		rows[i] = batchUserRow{ID: i, Name: "User", Email: "u@test.com"}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BatchInsert(tbl).Values(rows...).Build(dialect.Postgres)
	}
}

func BenchmarkBatchInsert_WithUpsert(b *testing.B) {
	b.ReportAllocs()
	tbl := testBatchTable{}
	rows := make([]any, 10)
	for i := range rows {
		rows[i] = batchUserRow{ID: i, Name: "User", Email: "u@test.com"}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BatchInsert(tbl).
			Values(rows...).
			OnConflict("id").
			DoUpdateSetExcluded("name", "email").
			Build(dialect.Postgres)
	}
}

func BenchmarkBatchInsert_MySQL(b *testing.B) {
	b.ReportAllocs()
	tbl := testBatchTable{}
	rows := make([]any, 10)
	for i := range rows {
		rows[i] = batchUserRow{ID: i, Name: "User", Email: "u@test.com"}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BatchInsert(tbl).Values(rows...).Build(dialect.MySQL)
	}
}

// ---------------------------------------------------------------------------
// Reflection benchmarks
// ---------------------------------------------------------------------------

func BenchmarkExtractColumns(b *testing.B) {
	b.ReportAllocs()
	row := benchRow{ID: 1, Name: "Test", Email: "t@t.com", Age: 25}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractColumns(row)
	}
}

func BenchmarkExtractValues(b *testing.B) {
	b.ReportAllocs()
	row := benchRow{ID: 1, Name: "Test", Email: "t@t.com", Age: 25}
	cols := []string{"id", "name", "email", "age"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractValues(row, cols)
	}
}
