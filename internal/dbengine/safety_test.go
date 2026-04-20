package dbengine

import "testing"

func TestIsReadOnlySQL(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"simple select", "SELECT * FROM t", true},
		{"select with where", "SELECT name FROM t WHERE id = 1", true},
		{"explain", "EXPLAIN SELECT * FROM t", true},
		{"describe", "DESCRIBE t", true},
		{"show tables", "SHOW TABLES", true},
		{"with cte", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"pragma", "PRAGMA version", true},
		{"case insensitive", "select * from t", true},

		{"empty", "", false},
		{"insert", "INSERT INTO t VALUES (1)", false},
		{"update", "UPDATE t SET x = 1", false},
		{"delete", "DELETE FROM t", false},
		{"drop", "DROP TABLE t", false},
		{"create", "CREATE TABLE t (id INT)", false},
		{"alter", "ALTER TABLE t ADD col INT", false},
		{"truncate", "TRUNCATE TABLE t", false},

		{"comment hiding insert", "SELECT 1; INSERT INTO t VALUES (1)", false},
		{"multi-statement", "SELECT 1; SELECT 2", false},
		{"line comment hiding", "SELECT 1 -- safe\n; DROP TABLE t", false},
		{"block comment obfuscation", "SELECT * FROM t /* safe */ WHERE 1=1", true},

		// Subquery with dangerous keyword in context
		{"select with insert keyword in string", "SELECT 'INSERT' FROM t", true},
		{"trailing semicolon", "SELECT * FROM t;", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsReadOnlySQL(tt.query)
			if got != tt.want {
				t.Errorf("IsReadOnlySQL(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestStripComments(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SELECT 1 -- comment", "SELECT 1  "},
		{"SELECT /* block */ 1", "SELECT   1"},
		{"SELECT 'not--comment'", "SELECT 'not--comment'"},
	}

	for _, tt := range tests {
		got := stripComments(tt.input)
		if got != tt.want {
			t.Errorf("stripComments(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"valid_name", "valid_name"},
		{"has spaces", "hasspaces"},
		{"has-dashes", "hasdashes"},
		{"DROP TABLE t;", "DROPTABLEt"},
		{"", "unnamed"},
		{"日本語", "unnamed"},
	}

	for _, tt := range tests {
		got := sanitizeIdentifier(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
