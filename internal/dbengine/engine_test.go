package dbengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAndClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestImportCSV(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte("name,age\nAlice,30\nBob,25\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.Import(csvPath, "people"); err != nil {
		t.Fatal(err)
	}

	tables := engine.Tables()
	if len(tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(tables))
	}
	if tables[0].Name != "people" {
		t.Errorf("table name = %q, want %q", tables[0].Name, "people")
	}
	if tables[0].RowCount != 2 {
		t.Errorf("row_count = %d, want 2", tables[0].RowCount)
	}
	if len(tables[0].Columns) != 2 {
		t.Errorf("columns = %d, want 2", len(tables[0].Columns))
	}
}

func TestImportJSON(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	jsonPath := filepath.Join(dir, "data.json")
	if err := os.WriteFile(jsonPath, []byte(`[{"id":1,"val":"a"},{"id":2,"val":"b"}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.Import(jsonPath, "items"); err != nil {
		t.Fatal(err)
	}

	result, err := engine.Execute("SELECT COUNT(*) AS cnt FROM items")
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Fatalf("result rows = %d, want 1", result.RowCount)
	}
}

func TestImportJSONL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	jsonlPath := filepath.Join(dir, "data.jsonl")
	if err := os.WriteFile(jsonlPath, []byte("{\"x\":1}\n{\"x\":2}\n{\"x\":3}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.Import(jsonlPath, "logs"); err != nil {
		t.Fatal(err)
	}

	tables := engine.Tables()
	if len(tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(tables))
	}
	if tables[0].RowCount != 3 {
		t.Errorf("row_count = %d, want 3", tables[0].RowCount)
	}
}

func TestExecuteReadOnly(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	result, err := engine.Execute("SELECT 1 AS num, 'hello' AS msg")
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Errorf("rows = %d, want 1", result.RowCount)
	}
	if len(result.Columns) != 2 {
		t.Errorf("columns = %d, want 2", len(result.Columns))
	}
}

func TestExecuteBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE t (id INT)")
	if err == nil {
		t.Error("expected error for CREATE query")
	}
}

func TestRemoveTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	csvPath := filepath.Join(dir, "data.csv")
	os.WriteFile(csvPath, []byte("a\n1\n"), 0o600)

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Import(csvPath, "temp")
	if len(engine.Tables()) != 1 {
		t.Fatal("expected 1 table after import")
	}

	if err := engine.RemoveTable("temp"); err != nil {
		t.Fatal(err)
	}
	if len(engine.Tables()) != 0 {
		t.Error("expected 0 tables after remove")
	}
}

func TestSchemaContext(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	csvPath := filepath.Join(dir, "data.csv")
	os.WriteFile(csvPath, []byte("name,age\nAlice,30\n"), 0o600)

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	engine.Import(csvPath, "people")

	ctx := engine.SchemaContext()
	if !strings.Contains(ctx, "people") {
		t.Error("schema context should contain table name")
	}
	if !strings.Contains(ctx, "name") {
		t.Error("schema context should contain column names")
	}
}

func TestReopenPreservesTables(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	csvPath := filepath.Join(dir, "data.csv")
	os.WriteFile(csvPath, []byte("val\n42\n"), 0o600)

	engine, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	engine.Import(csvPath, "nums")
	engine.Close()

	// Reopen
	engine2, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer engine2.Close()

	tables := engine2.Tables()
	if len(tables) != 1 {
		t.Fatalf("tables after reopen = %d, want 1", len(tables))
	}
	if tables[0].Name != "nums" {
		t.Errorf("table name = %q, want %q", tables[0].Name, "nums")
	}
}
