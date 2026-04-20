package dbengine

import (
	"regexp"
	"strings"
)

// dangerousKeywords are SQL keywords that modify data or schema.
var dangerousKeywords = regexp.MustCompile(
	`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|REPLACE|MERGE|GRANT|REVOKE|EXEC|EXECUTE|CALL|ATTACH|DETACH|COPY|LOAD|INSTALL)\b`)

// IsReadOnlySQL checks if a SQL query is read-only.
// Returns false for any query that could modify data or schema.
func IsReadOnlySQL(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}

	// Remove SQL comments
	cleaned := stripComments(query)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return false
	}

	// Block multi-statement queries (semicolons not at the end)
	withoutTrailing := strings.TrimRight(cleaned, "; \t\n\r")
	if strings.Contains(withoutTrailing, ";") {
		return false
	}

	// Must start with a read-only prefix
	upper := strings.ToUpper(cleaned)
	validPrefixes := []string{"SELECT", "EXPLAIN", "DESCRIBE", "SHOW", "WITH", "PRAGMA"}
	hasValidPrefix := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(upper, prefix) {
			hasValidPrefix = true
			break
		}
	}
	if !hasValidPrefix {
		return false
	}

	// Strip string literals before checking for dangerous keywords
	noStrings := stripStringLiterals(cleaned)
	if dangerousKeywords.MatchString(noStrings) {
		return false
	}

	return true
}

// stripComments removes SQL line comments (--) and block comments (/* */).
func stripComments(sql string) string {
	var result strings.Builder
	i := 0
	inString := false
	stringChar := byte(0)

	for i < len(sql) {
		if inString {
			result.WriteByte(sql[i])
			if sql[i] == stringChar {
				inString = false
			}
			i++
			continue
		}

		// Check for string start
		if sql[i] == '\'' || sql[i] == '"' {
			inString = true
			stringChar = sql[i]
			result.WriteByte(sql[i])
			i++
			continue
		}

		// Check for line comment
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			result.WriteByte(' ')
			continue
		}

		// Check for block comment
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			if i+1 < len(sql) {
				i += 2
			}
			result.WriteByte(' ')
			continue
		}

		result.WriteByte(sql[i])
		i++
	}

	return result.String()
}

// stripStringLiterals replaces string literals with empty strings.
func stripStringLiterals(sql string) string {
	var result strings.Builder
	i := 0
	for i < len(sql) {
		if sql[i] == '\'' || sql[i] == '"' {
			quote := sql[i]
			i++ // skip opening quote
			for i < len(sql) && sql[i] != quote {
				i++
			}
			if i < len(sql) {
				i++ // skip closing quote
			}
			result.WriteString("''") // placeholder
			continue
		}
		result.WriteByte(sql[i])
		i++
	}
	return result.String()
}
