package dbengine

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// Engine provides DuckDB operations for a single case database.
type Engine struct {
	db     *sql.DB
	dbPath string
	tables map[string]*TableMeta
	mu     sync.RWMutex
}

// TableMeta holds metadata about an imported table.
type TableMeta struct {
	Name       string           `json:"name"`
	Columns    []ColumnMeta     `json:"columns"`
	RowCount   int64            `json:"row_count"`
	SampleData []map[string]any `json:"sample_data,omitempty"`
	ImportedAt time.Time        `json:"imported_at"`
	SourceFile string           `json:"source_file"`
}

// ColumnMeta describes a table column.
type ColumnMeta struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// QueryResult holds the result of a SQL query.
type QueryResult struct {
	SQL      string           `json:"sql"`
	Columns  []string         `json:"columns"`
	Rows     []map[string]any `json:"rows"`
	RowCount int              `json:"row_count"`
	Duration time.Duration    `json:"duration"`
	Error    string           `json:"error,omitempty"`
}

// Open creates or opens a DuckDB database at the given path.
func Open(dbPath string) (*Engine, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}

	e := &Engine{
		db:     db,
		dbPath: dbPath,
		tables: make(map[string]*TableMeta),
	}

	if err := e.rebuildTableMeta(); err != nil {
		db.Close()
		return nil, fmt.Errorf("rebuild metadata: %w", err)
	}

	return e, nil
}

// Close closes the database connection.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

// Import loads a file into the database as a new table.
func (e *Engine) Import(path string, tableName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	tableName = sanitizeIdentifier(tableName)
	ext := strings.ToLower(filepath.Ext(path))

	var importSQL string
	switch ext {
	case ".json":
		importSQL = fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM read_json_auto('%s')",
			tableName, escapeSQLString(path))
	case ".jsonl":
		importSQL = fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM read_json_auto('%s', format='newline_delimited')",
			tableName, escapeSQLString(path))
	case ".csv":
		importSQL = fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM read_csv_auto('%s')",
			tableName, escapeSQLString(path))
	case ".tsv":
		importSQL = fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM read_csv_auto('%s', delim='\\t')",
			tableName, escapeSQLString(path))
	default:
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	if _, err := e.db.Exec(importSQL); err != nil {
		return fmt.Errorf("import %s: %w", path, err)
	}

	// Rebuild metadata for the new table
	meta, err := e.loadTableMeta(tableName)
	if err != nil {
		return fmt.Errorf("load metadata for %s: %w", tableName, err)
	}
	meta.SourceFile = path
	meta.ImportedAt = time.Now()
	e.tables[tableName] = meta

	return nil
}

// RemoveTable drops a table from the database.
func (e *Engine) RemoveTable(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	name = sanitizeIdentifier(name)
	if _, err := e.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", name)); err != nil {
		return fmt.Errorf("drop table %s: %w", name, err)
	}
	delete(e.tables, name)
	return nil
}

// Execute runs a read-only SQL query and returns the results.
func (e *Engine) Execute(query string) (*QueryResult, error) {
	if !IsReadOnlySQL(query) {
		return nil, fmt.Errorf("only SELECT queries are allowed")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	start := time.Now()
	rows, err := e.db.Query(query)
	if err != nil {
		return &QueryResult{SQL: query, Error: err.Error(), Duration: time.Since(start)}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return &QueryResult{
		SQL:      query,
		Columns:  columns,
		Rows:     results,
		RowCount: len(results),
		Duration: time.Since(start),
	}, nil
}

// Tables returns metadata for all tables.
func (e *Engine) Tables() []*TableMeta {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*TableMeta, 0, len(e.tables))
	for _, t := range e.tables {
		result = append(result, t)
	}
	return result
}

// SchemaContext returns a text representation of all table schemas for LLM context.
func (e *Engine) SchemaContext() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var sb strings.Builder
	for _, t := range e.tables {
		fmt.Fprintf(&sb, "Table: %s (%d rows)\n", t.Name, t.RowCount)
		fmt.Fprintf(&sb, "Columns:\n")
		for _, c := range t.Columns {
			nullable := ""
			if c.Nullable {
				nullable = " (nullable)"
			}
			fmt.Fprintf(&sb, "  - %s: %s%s\n", c.Name, c.Type, nullable)
		}
		if len(t.SampleData) > 0 {
			fmt.Fprintf(&sb, "Sample (first %d rows):\n", len(t.SampleData))
			for _, row := range t.SampleData {
				fmt.Fprintf(&sb, "  %v\n", row)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (e *Engine) rebuildTableMeta() error {
	rows, err := e.db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'main'")
	if err != nil {
		return err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tableNames = append(tableNames, name)
	}

	for _, name := range tableNames {
		meta, err := e.loadTableMeta(name)
		if err != nil {
			continue
		}
		e.tables[name] = meta
	}
	return nil
}

func (e *Engine) loadTableMeta(tableName string) (*TableMeta, error) {
	meta := &TableMeta{Name: tableName}

	// Get columns
	colRows, err := e.db.Query(fmt.Sprintf(
		"SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = '%s' AND table_schema = 'main'",
		escapeSQLString(tableName)))
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	for colRows.Next() {
		var c ColumnMeta
		var nullable string
		if err := colRows.Scan(&c.Name, &c.Type, &nullable); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		meta.Columns = append(meta.Columns, c)
	}

	// Get row count
	var count int64
	e.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", sanitizeIdentifier(tableName))).Scan(&count)
	meta.RowCount = count

	// Get sample data (first 5 rows)
	sampleRows, err := e.db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 5", sanitizeIdentifier(tableName)))
	if err == nil {
		defer sampleRows.Close()
		cols, _ := sampleRows.Columns()
		for sampleRows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if sampleRows.Scan(ptrs...) == nil {
				row := make(map[string]any, len(cols))
				for i, col := range cols {
					row[col] = values[i]
				}
				meta.SampleData = append(meta.SampleData, row)
			}
		}
	}

	return meta, nil
}

// sanitizeIdentifier removes non-alphanumeric characters except underscore.
func sanitizeIdentifier(name string) string {
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	result := sb.String()
	if result == "" {
		return "unnamed"
	}
	return result
}

// escapeSQLString escapes single quotes in SQL strings.
func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
