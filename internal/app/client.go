package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lib/pq"
)

const (
	readOnlyOption = "-c default_transaction_read_only=on"

	// commentTokenLen is the length of SQL comment tokens (/* */ --).
	commentTokenLen = 2

	// MaxQueryLength is the maximum allowed query size in bytes (1MB).
	// Queries exceeding this limit are rejected before any processing
	// to prevent memory exhaustion and DoS attacks.
	MaxQueryLength = 1 << 20
)

// injectReadOnlyOption appends default_transaction_read_only=on to the connection string
// so that every connection in the pool is read-only at the PostgreSQL level.
// Handles both URL-style (postgres://...) and keyword-value style connection strings.
func injectReadOnlyOption(connStr string) string {
	connStr = strings.TrimSpace(connStr)
	if connStr == "" {
		return connStr
	}

	// URL-style connection string
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		u, err := url.Parse(connStr)
		if err != nil {
			return connStr
		}
		q := u.Query()
		existing := q.Get("options")
		if existing != "" {
			q.Set("options", existing+" "+readOnlyOption)
		} else {
			q.Set("options", readOnlyOption)
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	// Keyword-value style connection string
	if strings.Contains(connStr, "options=") {
		// Append to existing options value
		// Handle both options='...' and options=...
		optionsPrefix := "options='"
		if idx := strings.Index(connStr, optionsPrefix); idx != -1 {
			// Find closing quote
			afterPrefix := idx + len(optionsPrefix)
			closeIdx := strings.Index(connStr[afterPrefix:], "'")
			if closeIdx != -1 {
				insertPos := afterPrefix + closeIdx
				return connStr[:insertPos] + " " + readOnlyOption + connStr[insertPos:]
			}
		}
		return connStr
	}
	return connStr + " options='" + readOnlyOption + "'"
}

// PostgreSQLClientImpl implements the PostgreSQLClient interface.
type PostgreSQLClientImpl struct {
	db               *sql.DB
	connectionString string
}

// NewPostgreSQLClient creates a new PostgreSQL client.
func NewPostgreSQLClient() *PostgreSQLClientImpl {
	return &PostgreSQLClientImpl{}
}

// Connect establishes a connection to the PostgreSQL database.
// The connection is configured as read-only at the PostgreSQL session level
// to provide defense-in-depth against SQL injection attacks.
func (c *PostgreSQLClientImpl) Connect(ctx context.Context, connectionString string) error {
	readOnlyConnStr := injectReadOnlyOption(connectionString)
	db, err := sql.Open("postgres", readOnlyConnStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	c.db = db
	c.connectionString = connectionString
	return nil
}

// Close closes the database connection.
func (c *PostgreSQLClientImpl) Close() error {
	if c.db == nil {
		return nil
	}
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}

// Ping checks if the database connection is alive.
func (c *PostgreSQLClientImpl) Ping(ctx context.Context) error {
	if c.db == nil {
		return ErrNoDatabaseConnection
	}
	if err := c.db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

// GetDB returns the underlying sql.DB connection.
func (c *PostgreSQLClientImpl) GetDB() *sql.DB {
	return c.db
}

// ListDatabases returns a list of all databases on the server.
func (c *PostgreSQLClientImpl) ListDatabases(ctx context.Context) ([]*DatabaseInfo, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	query := `
		SELECT datname, pg_catalog.pg_get_userbyid(datdba) as owner, pg_encoding_to_char(encoding) as encoding
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY datname`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var databases []*DatabaseInfo
	for rows.Next() {
		var db DatabaseInfo
		if err := rows.Scan(&db.Name, &db.Owner, &db.Encoding); err != nil {
			return nil, fmt.Errorf("failed to scan database row: %w", err)
		}
		databases = append(databases, &db)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate database rows: %w", err)
	}
	return databases, nil
}

// GetCurrentDatabase returns the name of the current database.
func (c *PostgreSQLClientImpl) GetCurrentDatabase(ctx context.Context) (string, error) {
	if c.db == nil {
		return "", ErrNoDatabaseConnection
	}

	var dbName string
	err := c.db.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName)
	if err != nil {
		return "", fmt.Errorf("failed to get current database: %w", err)
	}

	return dbName, nil
}

// ListSchemas returns a list of schemas in the current database.
func (c *PostgreSQLClientImpl) ListSchemas(ctx context.Context) ([]*SchemaInfo, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	query := `
		SELECT schema_name, schema_owner
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
		ORDER BY schema_name`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schemas []*SchemaInfo
	for rows.Next() {
		var schema SchemaInfo
		if err := rows.Scan(&schema.Name, &schema.Owner); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}
		schemas = append(schemas, &schema)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate schema rows: %w", err)
	}
	return schemas, nil
}

// ListTables returns a list of tables in the specified schema.
func (c *PostgreSQLClientImpl) ListTables(ctx context.Context, schema string) ([]*TableInfo, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	query := `
		SELECT
			schemaname,
			tablename,
			'table' as type,
			tableowner as owner
		FROM pg_tables
		WHERE schemaname = $1
		UNION ALL
		SELECT
			schemaname,
			viewname as tablename,
			'view' as type,
			viewowner as owner
		FROM pg_views
		WHERE schemaname = $1
		ORDER BY tablename`

	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []*TableInfo
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type, &table.Owner); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tables = append(tables, &table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate table rows: %w", err)
	}
	return tables, nil
}

// ListTablesWithStats returns a list of tables with size and row count statistics in a single optimized query.
// This eliminates the N+1 query pattern by joining table metadata with pg_stat_user_tables.
// For tables where statistics show 0 rows, it falls back to COUNT(*) to get actual row counts.
func (c *PostgreSQLClientImpl) ListTablesWithStats(ctx context.Context, schema string) ([]*TableInfo, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	// Single optimized query that joins tables with statistics
	// We use n_tup_ins - n_tup_del which is more accurate than n_live_tup for recently modified tables
	query := `
		WITH table_list AS (
			SELECT
				schemaname,
				tablename,
				'table' as type,
				tableowner as owner
			FROM pg_tables
			WHERE schemaname = $1
			UNION ALL
			SELECT
				schemaname,
				viewname as tablename,
				'view' as type,
				viewowner as owner
			FROM pg_views
			WHERE schemaname = $1
		)
		SELECT
			t.schemaname,
			t.tablename,
			t.type,
			t.owner,
			COALESCE(s.n_tup_ins - s.n_tup_del, 0) as row_count,
			pg_size_pretty(COALESCE(pg_total_relation_size(quote_ident(t.schemaname) || '.' || quote_ident(t.tablename)), 0)) as size
		FROM table_list t
		LEFT JOIN pg_stat_user_tables s
			ON t.schemaname = s.schemaname AND t.tablename = s.relname
		ORDER BY t.tablename`

	rows, err := c.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables with stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []*TableInfo
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type, &table.Owner, &table.RowCount, &table.Size); err != nil {
			return nil, fmt.Errorf("failed to scan table row with stats: %w", err)
		}
		tables = append(tables, &table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate table rows with stats: %w", err)
	}

	// For tables where statistics show 0 rows, fall back to actual COUNT(*)
	// This handles newly created tables where pg_stat hasn't been updated yet
	for _, table := range tables {
		if table.RowCount == 0 && table.Type == "table" {
			// Use pq.QuoteIdentifier for SQL-safe identifier escaping to prevent
			// SQL injection via malicious schema or table names.
			countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s",
				pq.QuoteIdentifier(table.Schema),
				pq.QuoteIdentifier(table.Name))
			var actualCount int64
			if err := c.db.QueryRowContext(ctx, countQuery).Scan(&actualCount); err != nil {
				// Log warning but don't fail the entire operation
				continue
			}
			table.RowCount = actualCount
		}
	}

	return tables, nil
}

// DescribeTable returns detailed column information for a table.
func (c *PostgreSQLClientImpl) DescribeTable(ctx context.Context, schema, table string) ([]*ColumnInfo, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	query := `
		SELECT
			column_name,
			data_type,
			is_nullable = 'YES' as is_nullable,
			COALESCE(column_default, '') as default_value
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`

	rows, err := c.db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var columns []*ColumnInfo
	for rows.Next() {
		var column ColumnInfo
		if err := rows.Scan(&column.Name, &column.DataType, &column.IsNullable, &column.DefaultValue); err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}
		columns = append(columns, &column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate column rows: %w", err)
	}

	// Check if table exists (if no columns found, table doesn't exist)
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s.%s: %w", schema, table, ErrTableNotFound)
	}

	return columns, nil
}

// GetTableStats returns statistics for a specific table.
func (c *PostgreSQLClientImpl) GetTableStats(ctx context.Context, schema, table string) (*TableInfo, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	// Get basic table info
	tableInfo := &TableInfo{
		Schema: schema,
		Name:   table,
	}

	// Get row count (approximate for large tables, exact for small tables)
	countQuery := `
		SELECT COALESCE(n_tup_ins - n_tup_del, 0) as estimated_rows
		FROM pg_stat_user_tables
		WHERE schemaname = $1 AND relname = $2`

	var rowCount sql.NullInt64
	err := c.db.QueryRowContext(ctx, countQuery, schema, table).Scan(&rowCount)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}

	// If statistics are not available or show 0 rows, fall back to actual count
	// This is useful for newly created tables where pg_stat hasn't been updated
	if !rowCount.Valid || rowCount.Int64 == 0 {
		// Use pq.QuoteIdentifier for SQL-safe identifier escaping to prevent
		// SQL injection via malicious schema or table names.
		actualCountQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s",
			pq.QuoteIdentifier(schema),
			pq.QuoteIdentifier(table))
		var actualCount int64
		err := c.db.QueryRowContext(ctx, actualCountQuery).Scan(&actualCount)
		if err != nil {
			return nil, fmt.Errorf("failed to get actual row count: %w", err)
		}
		tableInfo.RowCount = actualCount
	} else {
		tableInfo.RowCount = rowCount.Int64
	}

	return tableInfo, nil
}

// ListIndexes returns a list of indexes for the specified table.
func (c *PostgreSQLClientImpl) ListIndexes(ctx context.Context, schema, table string) ([]*IndexInfo, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	query := `
		SELECT
			i.relname as index_name,
			t.relname as table_name,
			array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) as columns,
			ix.indisunique as is_unique,
			ix.indisprimary as is_primary,
			am.amname as index_type
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_am am ON i.relam = am.oid
		JOIN pg_namespace n ON t.relnamespace = n.oid
		JOIN pg_attribute a ON a.attrelid = t.oid
		WHERE n.nspname = $1 AND t.relname = $2 AND a.attnum = ANY(ix.indkey)
		GROUP BY i.relname, t.relname, ix.indisunique, ix.indisprimary, am.amname
		ORDER BY i.relname`

	rows, err := c.db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var indexes []*IndexInfo
	for rows.Next() {
		var index IndexInfo
		var columns pq.StringArray
		if err := rows.Scan(
			&index.Name, &index.Table, &columns,
			&index.IsUnique, &index.IsPrimary, &index.IndexType,
		); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}

		// Convert pq.StringArray to []string
		index.Columns = []string(columns)

		indexes = append(indexes, &index)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate index rows: %w", err)
	}
	return indexes, nil
}

// copySingleQuotedLiteral copies a single-quoted string literal from runes[start]
// (which must be a single quote) into result, returning the new index after the closing quote.
// Handles escaped quotes ('').
func copySingleQuotedLiteral(runes []rune, start int, result *strings.Builder) int {
	result.WriteRune(runes[start])
	i := start + 1
	for i < len(runes) {
		result.WriteRune(runes[i])
		if runes[i] == '\'' {
			if i+1 < len(runes) && runes[i+1] == '\'' {
				result.WriteRune(runes[i+1])
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return i
}

// copyDoubleQuotedIdentifier copies a double-quoted identifier from runes[start]
// (which must be a double quote) into result, returning the new index after the closing quote.
func copyDoubleQuotedIdentifier(runes []rune, start int, result *strings.Builder) int {
	result.WriteRune(runes[start])
	i := start + 1
	for i < len(runes) {
		result.WriteRune(runes[i])
		if runes[i] == '"' {
			return i + 1
		}
		i++
	}
	return i
}

// skipBlockComment skips a block comment starting at runes[start] (which must be '/')
// with nesting support. Returns the new index after the closing */.
func skipBlockComment(runes []rune, start int) int {
	depth := 1
	i := start + commentTokenLen
	for i < len(runes) && depth > 0 {
		switch {
		case i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '*':
			depth++
			i += commentTokenLen
		case i+1 < len(runes) && runes[i] == '*' && runes[i+1] == '/':
			depth--
			i += commentTokenLen
		default:
			i++
		}
	}
	return i
}

// skipLineComment skips a line comment starting at runes[start] (which must be '-').
// Returns the new index at the newline character (or end of runes).
func skipLineComment(runes []rune, start int) int {
	i := start + commentTokenLen
	for i < len(runes) && runes[i] != '\n' {
		i++
	}
	return i
}

// stripComments removes SQL comments from a query while preserving
// content inside single-quoted string literals and double-quoted identifiers.
// Block comments (/* */, including nested) and line comments (--) are replaced with spaces.
func stripComments(query string) string {
	var result strings.Builder
	result.Grow(len(query))
	runes := []rune(query)
	i := 0

	for i < len(runes) {
		switch {
		case runes[i] == '\'':
			i = copySingleQuotedLiteral(runes, i, &result)
		case runes[i] == '"':
			i = copyDoubleQuotedIdentifier(runes, i, &result)
		case i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '*':
			i = skipBlockComment(runes, i)
			result.WriteRune(' ')
		case i+1 < len(runes) && runes[i] == '-' && runes[i+1] == '-':
			i = skipLineComment(runes, i)
			result.WriteRune(' ')
		default:
			result.WriteRune(runes[i])
			i++
		}
	}
	return result.String()
}

// containsSemicolonOutsideLiterals checks if the query contains a semicolon
// that is not inside a single-quoted string literal or double-quoted identifier.
func containsSemicolonOutsideLiterals(query string) bool {
	runes := []rune(query)
	i := 0

	for i < len(runes) {
		// Skip single-quoted string literals
		if runes[i] == '\'' {
			i++
			for i < len(runes) {
				if runes[i] == '\'' {
					if i+1 < len(runes) && runes[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}

		// Skip double-quoted identifiers
		if runes[i] == '"' {
			i++
			for i < len(runes) {
				if runes[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}

		if runes[i] == ';' {
			return true
		}
		i++
	}
	return false
}

// validateQuery checks if the query is allowed (SELECT or WITH only)
// and rejects multi-statement queries.
// Comments are stripped before validation to prevent comment-based injection.
func validateQuery(query string) error {
	if len(query) > MaxQueryLength {
		return ErrQueryTooLong
	}
	stripped := stripComments(query)
	trimmed := strings.TrimSpace(strings.ToUpper(stripped))
	if !strings.HasPrefix(trimmed, "SELECT") && !strings.HasPrefix(trimmed, "WITH") {
		return ErrInvalidQuery
	}
	if containsSemicolonOutsideLiterals(stripped) {
		return ErrMultiStatementQuery
	}
	return nil
}

// processRows processes query result rows and handles type conversion.
func processRows(rows *sql.Rows) ([][]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var result [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert []byte to string for easier JSON serialization
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				values[i] = string(b)
			}
		}

		result = append(result, values)
	}
	return result, nil
}

// ExecuteQuery executes a SELECT query and returns the results.
func (c *PostgreSQLClientImpl) ExecuteQuery(ctx context.Context, query string, args ...any) (*QueryResult, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	if err := validateQuery(query); err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	result, err := processRows(rows)
	if err != nil {
		return nil, err
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate query rows: %w", err)
	}
	return &QueryResult{
		Columns:  columns,
		Rows:     result,
		RowCount: len(result),
	}, nil
}

// ExplainQuery returns the execution plan for a query.
func (c *PostgreSQLClientImpl) ExplainQuery(ctx context.Context, query string, args ...any) (*QueryResult, error) {
	if c.db == nil {
		return nil, ErrNoDatabaseConnection
	}

	if err := validateQuery(query); err != nil {
		return nil, err
	}

	// Construct the EXPLAIN query
	explainQuery := "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + query //nolint:gosec // query is validated by validateQuery above (SELECT/WITH only)

	rows, err := c.db.QueryContext(ctx, explainQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute explain query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	result, err := processRows(rows)
	if err != nil {
		return nil, err
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate query rows: %w", err)
	}
	return &QueryResult{
		Columns:  columns,
		Rows:     result,
		RowCount: len(result),
	}, nil
}
