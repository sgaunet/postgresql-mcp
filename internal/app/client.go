package app

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

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
func (c *PostgreSQLClientImpl) Connect(connectionString string) error {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	c.db = db
	c.connectionString = connectionString
	return nil
}

// Close closes the database connection.
func (c *PostgreSQLClientImpl) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Ping checks if the database connection is alive.
func (c *PostgreSQLClientImpl) Ping() error {
	if c.db == nil {
		return fmt.Errorf("no database connection")
	}
	return c.db.Ping()
}

// GetDB returns the underlying sql.DB connection.
func (c *PostgreSQLClientImpl) GetDB() *sql.DB {
	return c.db
}

// ListDatabases returns a list of all databases on the server.
func (c *PostgreSQLClientImpl) ListDatabases() ([]*DatabaseInfo, error) {
	if c.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	query := `
		SELECT datname, pg_catalog.pg_get_userbyid(datdba) as owner, pg_encoding_to_char(encoding) as encoding
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY datname`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var databases []*DatabaseInfo
	for rows.Next() {
		var db DatabaseInfo
		if err := rows.Scan(&db.Name, &db.Owner, &db.Encoding); err != nil {
			return nil, fmt.Errorf("failed to scan database row: %w", err)
		}
		databases = append(databases, &db)
	}

	return databases, rows.Err()
}

// GetCurrentDatabase returns the name of the current database.
func (c *PostgreSQLClientImpl) GetCurrentDatabase() (string, error) {
	if c.db == nil {
		return "", fmt.Errorf("no database connection")
	}

	var dbName string
	err := c.db.QueryRow("SELECT current_database()").Scan(&dbName)
	if err != nil {
		return "", fmt.Errorf("failed to get current database: %w", err)
	}

	return dbName, nil
}

// ListSchemas returns a list of schemas in the current database.
func (c *PostgreSQLClientImpl) ListSchemas() ([]*SchemaInfo, error) {
	if c.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	query := `
		SELECT schema_name, schema_owner
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
		ORDER BY schema_name`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []*SchemaInfo
	for rows.Next() {
		var schema SchemaInfo
		if err := rows.Scan(&schema.Name, &schema.Owner); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}
		schemas = append(schemas, &schema)
	}

	return schemas, rows.Err()
}

// ListTables returns a list of tables in the specified schema.
func (c *PostgreSQLClientImpl) ListTables(schema string) ([]*TableInfo, error) {
	if c.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	if schema == "" {
		schema = "public"
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

	rows, err := c.db.Query(query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []*TableInfo
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type, &table.Owner); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tables = append(tables, &table)
	}

	return tables, rows.Err()
}

// DescribeTable returns detailed column information for a table.
func (c *PostgreSQLClientImpl) DescribeTable(schema, table string) ([]*ColumnInfo, error) {
	if c.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	if schema == "" {
		schema = "public"
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

	rows, err := c.db.Query(query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}
	defer rows.Close()

	var columns []*ColumnInfo
	for rows.Next() {
		var column ColumnInfo
		if err := rows.Scan(&column.Name, &column.DataType, &column.IsNullable, &column.DefaultValue); err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}
		columns = append(columns, &column)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Check if table exists (if no columns found, table doesn't exist)
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s.%s does not exist", schema, table)
	}

	return columns, nil
}

// GetTableStats returns statistics for a specific table.
func (c *PostgreSQLClientImpl) GetTableStats(schema, table string) (*TableInfo, error) {
	if c.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	if schema == "" {
		schema = "public"
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
	err := c.db.QueryRow(countQuery, schema, table).Scan(&rowCount)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}

	// If statistics are not available or show 0 rows, fall back to actual count
	// This is useful for newly created tables where pg_stat hasn't been updated
	if !rowCount.Valid || rowCount.Int64 == 0 {
		actualCountQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"."%s"`, schema, table)
		var actualCount int64
		err := c.db.QueryRow(actualCountQuery).Scan(&actualCount)
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
func (c *PostgreSQLClientImpl) ListIndexes(schema, table string) ([]*IndexInfo, error) {
	if c.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	if schema == "" {
		schema = "public"
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

	rows, err := c.db.Query(query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer rows.Close()

	var indexes []*IndexInfo
	for rows.Next() {
		var index IndexInfo
		var columnsStr string
		if err := rows.Scan(&index.Name, &index.Table, &columnsStr, &index.IsUnique, &index.IsPrimary, &index.IndexType); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}

		// Parse column array from PostgreSQL format
		columnsStr = strings.Trim(columnsStr, "{}")
		if columnsStr != "" {
			index.Columns = strings.Split(columnsStr, ",")
		}

		indexes = append(indexes, &index)
	}

	return indexes, rows.Err()
}

// ExecuteQuery executes a SELECT query and returns the results.
func (c *PostgreSQLClientImpl) ExecuteQuery(query string, args ...interface{}) (*QueryResult, error) {
	if c.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	// Ensure only SELECT queries are allowed for safety
	trimmedQuery := strings.TrimSpace(strings.ToUpper(query))
	if !strings.HasPrefix(trimmedQuery, "SELECT") && !strings.HasPrefix(trimmedQuery, "WITH") {
		return nil, fmt.Errorf("only SELECT and WITH queries are allowed")
	}

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var result [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
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

	return &QueryResult{
		Columns:  columns,
		Rows:     result,
		RowCount: len(result),
	}, rows.Err()
}

// ExplainQuery returns the execution plan for a query.
func (c *PostgreSQLClientImpl) ExplainQuery(query string, args ...interface{}) (*QueryResult, error) {
	if c.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	// Validate that the input query is safe (SELECT or WITH only)
	trimmedQuery := strings.TrimSpace(strings.ToUpper(query))
	if !strings.HasPrefix(trimmedQuery, "SELECT") && !strings.HasPrefix(trimmedQuery, "WITH") {
		return nil, fmt.Errorf("only SELECT and WITH queries are allowed")
	}

	// Construct the EXPLAIN query
	explainQuery := "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + query

	// Execute the EXPLAIN query directly (bypassing ExecuteQuery validation)
	rows, err := c.db.Query(explainQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute explain query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var result [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
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

	return &QueryResult{
		Columns:  columns,
		Rows:     result,
		RowCount: len(result),
	}, rows.Err()
}