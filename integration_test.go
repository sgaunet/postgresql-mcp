package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sylvain/postgresql-mcp/internal/app"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/lib/pq"
)

// Integration tests use testcontainers to spin up PostgreSQL instances
// These tests can be skipped if SKIP_INTEGRATION_TESTS environment variable is set
// Docker is required to run these tests

const (
	testTimeout = 30 * time.Second
)

func skipIfNoIntegration(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests (SKIP_INTEGRATION_TESTS=true)")
	}
}

func setupTestContainer(t *testing.T) (*postgres.PostgresContainer, string, func()) {
	skipIfNoIntegration(t)

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:17",
		postgres.WithDatabase("test_db"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Test that we can actually connect
	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err)
	defer db.Close()

	// Wait for database to be ready with retries
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		if i == maxRetries-1 {
			require.NoError(t, err, "Failed to connect to test database after %d retries", maxRetries)
		}
		time.Sleep(time.Second)
	}

	// Cleanup function
	cleanup := func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return postgresContainer, connStr, cleanup
}

func setupTestDatabase(t *testing.T) (*sql.DB, string, func()) {
	_, connectionString, containerCleanup := setupTestContainer(t)

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", connectionString)
	require.NoError(t, err)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	err = db.PingContext(ctx)
	require.NoError(t, err)

	// Create test schema and tables
	testSchema := "test_mcp_schema"
	testTable := "test_users"

	_, err = db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", testSchema))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", testSchema))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s.%s (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE,
			age INTEGER,
			active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, testSchema, testTable))
	require.NoError(t, err)

	// Create an index
	_, err = db.ExecContext(ctx, fmt.Sprintf(`
		CREATE INDEX idx_%s_email ON %s.%s (email)
	`, testTable, testSchema, testTable))
	require.NoError(t, err)

	// Insert test data
	_, err = db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s.%s (name, email, age, active) VALUES
		('John Doe', 'john@example.com', 30, true),
		('Jane Smith', 'jane@example.com', 25, true),
		('Bob Johnson', 'bob@example.com', 35, false),
		('Alice Brown', 'alice@example.com', 28, true)
	`, testSchema, testTable))
	require.NoError(t, err)

	// Cleanup function
	cleanup := func() {
		_, _ = db.ExecContext(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", testSchema))
		db.Close()
		containerCleanup() // Clean up container
	}

	return db, connectionString, cleanup
}

func TestIntegration_App_Connect(t *testing.T) {
	_, connectionString, cleanup := setupTestContainer(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	// Test explicit connection with connection string
	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	// Test that we can get current database
	dbName, err := appInstance.GetCurrentDatabase(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, dbName)
}

func TestIntegration_App_ConnectWithEnvironmentVariable(t *testing.T) {
	_, connectionString, cleanup := setupTestContainer(t)
	defer cleanup()

	// Test with POSTGRES_URL environment variable (backward compatibility via tryConnect)
	os.Setenv("POSTGRES_URL", connectionString)
	defer os.Unsetenv("POSTGRES_URL")

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	// Explicitly call ensureConnection which will trigger tryConnect() fallback
	err = appInstance.ValidateConnection(ctx)
	assert.NoError(t, err)

	// Verify connection works
	dbName, err := appInstance.GetCurrentDatabase(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, dbName)
}

func TestIntegration_App_ListDatabases(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	databases, err := appInstance.ListDatabases(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, databases)

	// Should at least have the test database
	found := false
	for _, db := range databases {
		if db.Name == "test_db" {
			found = true
			assert.NotEmpty(t, db.Owner)
			assert.NotEmpty(t, db.Encoding)
		}
	}
	assert.True(t, found, "Should find test_db database")
}

func TestIntegration_App_ListSchemas(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	schemas, err := appInstance.ListSchemas(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, schemas)

	// Should have at least public and our test schema
	schemaNames := make([]string, len(schemas))
	for i, schema := range schemas {
		schemaNames[i] = schema.Name
	}

	assert.Contains(t, schemaNames, "public")
	assert.Contains(t, schemaNames, "test_mcp_schema")
}

func TestIntegration_App_ListTables(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	// List tables in test schema
	listOpts := &app.ListTablesOptions{
		Schema: "test_mcp_schema",
	}

	tables, err := appInstance.ListTables(ctx, listOpts)
	assert.NoError(t, err)
	assert.NotEmpty(t, tables)

	// Should find our test table
	found := false
	for _, table := range tables {
		if table.Name == "test_users" {
			found = true
			assert.Equal(t, "test_mcp_schema", table.Schema)
			assert.Equal(t, "table", table.Type)
			assert.NotEmpty(t, table.Owner)
		}
	}
	assert.True(t, found, "Should find test_users table")
}

func TestIntegration_App_ListTablesWithSize(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	// List tables with size information
	listOpts := &app.ListTablesOptions{
		Schema:      "test_mcp_schema",
		IncludeSize: true,
	}

	tables, err := appInstance.ListTables(ctx, listOpts)
	assert.NoError(t, err)
	assert.NotEmpty(t, tables)

	// Check that size information is included
	for _, table := range tables {
		if table.Name == "test_users" {
			// Row count should be 4 (from our test data)
			assert.Equal(t, int64(4), table.RowCount)
		}
	}
}

func TestIntegration_App_DescribeTable(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	columns, err := appInstance.DescribeTable(ctx, "test_mcp_schema", "test_users")
	assert.NoError(t, err)
	assert.NotEmpty(t, columns)

	// Verify expected columns
	columnNames := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = col.Name
	}

	expectedColumns := []string{"id", "name", "email", "age", "active", "created_at"}
	for _, expected := range expectedColumns {
		assert.Contains(t, columnNames, expected)
	}

	// Check specific column properties
	for _, col := range columns {
		switch col.Name {
		case "id":
			assert.Equal(t, "integer", col.DataType)
			assert.False(t, col.IsNullable)
		case "name":
			assert.Contains(t, col.DataType, "character varying")
			assert.False(t, col.IsNullable)
		case "email":
			assert.Contains(t, col.DataType, "character varying")
			assert.True(t, col.IsNullable)
		case "active":
			assert.Equal(t, "boolean", col.DataType)
			assert.True(t, col.IsNullable)
		}
	}
}

func TestIntegration_App_ExecuteQuery(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	// Test simple SELECT query
	queryOpts := &app.ExecuteQueryOptions{
		Query: "SELECT id, name, email FROM test_mcp_schema.test_users WHERE active = true ORDER BY id",
	}

	result, err := appInstance.ExecuteQuery(ctx, queryOpts)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check result structure
	assert.Equal(t, []string{"id", "name", "email"}, result.Columns)
	assert.Equal(t, 3, result.RowCount) // 3 active users in test data
	assert.Len(t, result.Rows, 3)

	// Check first row data
	firstRow := result.Rows[0]
	assert.Len(t, firstRow, 3)
	assert.Equal(t, "1", fmt.Sprintf("%v", firstRow[0])) // ID can be int64 or other numeric type
	assert.Equal(t, "John Doe", firstRow[1])
	assert.Equal(t, "john@example.com", firstRow[2])
}

func TestIntegration_App_ExecuteQueryWithLimit(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	// Test query with limit
	queryOpts := &app.ExecuteQueryOptions{
		Query: "SELECT * FROM test_mcp_schema.test_users ORDER BY id",
		Limit: 2,
	}

	result, err := appInstance.ExecuteQuery(ctx, queryOpts)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should only return 2 rows due to limit
	assert.Equal(t, 2, result.RowCount)
	assert.Len(t, result.Rows, 2)
}

func TestIntegration_App_ListIndexes(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	indexes, err := appInstance.ListIndexes(ctx, "test_mcp_schema", "test_users")
	assert.NoError(t, err)
	assert.NotEmpty(t, indexes)

	// Should have at least primary key and email index
	indexNames := make([]string, len(indexes))
	for i, idx := range indexes {
		indexNames[i] = idx.Name
	}

	// Check for primary key
	foundPK := false
	foundEmailIdx := false
	for _, idx := range indexes {
		if idx.IsPrimary {
			foundPK = true
			assert.Contains(t, idx.Columns, "id")
		}
		if idx.Name == "idx_test_users_email" {
			foundEmailIdx = true
			assert.Contains(t, idx.Columns, "email")
			assert.False(t, idx.IsPrimary)
		}
	}

	assert.True(t, foundPK, "Should find primary key index")
	assert.True(t, foundEmailIdx, "Should find email index")
}

func TestIntegration_App_ListIndexes_SpecialCharacters(t *testing.T) {
	_, connectionString, cleanup := setupTestContainer(t)
	defer cleanup()

	db, err := sql.Open("postgres", connectionString)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Create test schema
	testSchema := "test_special_chars"
	_, err = db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", testSchema))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", testSchema))
	require.NoError(t, err)

	// Create table with columns containing special characters
	_, err = db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s.test_table (
			id SERIAL PRIMARY KEY,
			"Column Name" VARCHAR(255),
			"col,with,commas" VARCHAR(255),
			"col{value}" VARCHAR(255),
			"col}data" VARCHAR(255),
			normal_col VARCHAR(255)
		)
	`, testSchema))
	require.NoError(t, err)

	// Create indexes with special character column names
	_, err = db.ExecContext(ctx, fmt.Sprintf(`
		CREATE INDEX idx_quoted_name ON %s.test_table ("Column Name")
	`, testSchema))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, fmt.Sprintf(`
		CREATE INDEX idx_with_commas ON %s.test_table ("col,with,commas", normal_col)
	`, testSchema))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, fmt.Sprintf(`
		CREATE INDEX idx_braces ON %s.test_table ("col{value}", "col}data")
	`, testSchema))
	require.NoError(t, err)

	// Test with app
	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	indexes, err := appInstance.ListIndexes(ctx, testSchema, "test_table")
	assert.NoError(t, err)
	assert.NotEmpty(t, indexes)

	// Verify each index is parsed correctly
	indexMap := make(map[string]*app.IndexInfo)
	for _, idx := range indexes {
		indexMap[idx.Name] = idx
	}

	// Check idx_quoted_name
	if idx, ok := indexMap["idx_quoted_name"]; ok {
		assert.Len(t, idx.Columns, 1, "idx_quoted_name should have 1 column")
		assert.Equal(t, "Column Name", idx.Columns[0], "Column name should be 'Column Name'")
	} else {
		t.Error("idx_quoted_name not found")
	}

	// Check idx_with_commas - should have 2 columns, not split by commas inside quotes
	if idx, ok := indexMap["idx_with_commas"]; ok {
		assert.Len(t, idx.Columns, 2, "idx_with_commas should have exactly 2 columns")
		assert.Contains(t, idx.Columns, "col,with,commas", "Should contain column with commas")
		assert.Contains(t, idx.Columns, "normal_col", "Should contain normal_col")
	} else {
		t.Error("idx_with_commas not found")
	}

	// Check idx_braces
	if idx, ok := indexMap["idx_braces"]; ok {
		assert.Len(t, idx.Columns, 2, "idx_braces should have 2 columns")
		assert.Contains(t, idx.Columns, "col{value}", "Should contain 'col{value}'")
		assert.Contains(t, idx.Columns, "col}data", "Should contain 'col}data'")
	} else {
		t.Error("idx_braces not found")
	}

	// Cleanup
	_, _ = db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", testSchema))
}

func TestIntegration_App_ExplainQuery(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	// Test EXPLAIN query
	result, err := appInstance.ExplainQuery(ctx, "SELECT * FROM test_mcp_schema.test_users WHERE active = true")
	require.NoError(t, err)
	require.NotNil(t, result)

	// EXPLAIN should return execution plan
	assert.NotEmpty(t, result.Columns)
	assert.NotEmpty(t, result.Rows)
}

func TestIntegration_App_GetTableStats(t *testing.T) {
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(t, err)

	stats, err := appInstance.GetTableStats(ctx, "test_mcp_schema", "test_users")
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	assert.Equal(t, "test_mcp_schema", stats.Schema)
	assert.Equal(t, "test_users", stats.Name)
	// Row count might be 0 initially due to how PostgreSQL tracks stats
	assert.GreaterOrEqual(t, stats.RowCount, int64(0))
}

func TestIntegration_App_ErrorHandling(t *testing.T) {
	_, connectionString, cleanup := setupTestContainer(t)
	defer cleanup()

	// Set environment variable for connection
	os.Setenv("POSTGRES_URL", connectionString)
	defer os.Unsetenv("POSTGRES_URL")

	appInstance, err := app.New()
	require.NoError(t, err)
	defer appInstance.Disconnect()

	ctx := context.Background()

	// Test query to non-existent table
	_, err = appInstance.DescribeTable(ctx, "public", "nonexistent_table")
	assert.Error(t, err)

	// Test invalid query
	queryOpts := &app.ExecuteQueryOptions{
		Query: "INVALID SQL QUERY",
	}
	_, err = appInstance.ExecuteQuery(ctx, queryOpts)
	assert.Error(t, err)

	// Test non-existent schema
	listOpts := &app.ListTablesOptions{
		Schema: "nonexistent_schema",
	}
	tables, err := appInstance.ListTables(ctx, listOpts)
	assert.NoError(t, err) // This might succeed but return empty results
	assert.Empty(t, tables)
}

func TestIntegration_App_ConnectionValidation(t *testing.T) {
	_, connectionString, cleanup := setupTestContainer(t)
	defer cleanup()

	// Test validation without environment variable
	appInstance, err := app.New()
	require.NoError(t, err)

	ctx := context.Background()

	err = appInstance.ValidateConnection(ctx)
	assert.Error(t, err)

	// Set environment variable and test validation
	os.Setenv("POSTGRES_URL", connectionString)
	defer os.Unsetenv("POSTGRES_URL")

	// Create new instance with environment variable set
	appInstance2, err := app.New()
	require.NoError(t, err)
	defer appInstance2.Disconnect()

	err = appInstance2.ValidateConnection(ctx)
	assert.NoError(t, err)
}

// Benchmark tests for performance

func BenchmarkIntegration_ListTables(b *testing.B) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		b.Skip("Skipping integration benchmarks")
	}

	// Use a testing.T wrapper for setup functions
	t := &testing.T{}
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()

	appInstance, err := app.New()
	require.NoError(b, err)
	defer appInstance.Disconnect()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(b, err)

	listOpts := &app.ListTablesOptions{
		Schema: "test_mcp_schema",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := appInstance.ListTables(ctx, listOpts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIntegration_ExecuteQuery(b *testing.B) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		b.Skip("Skipping integration benchmarks")
	}

	// Use a testing.T wrapper for setup functions
	t := &testing.T{}
	_, connectionString, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()

	appInstance, err := app.New()
	require.NoError(b, err)
	defer appInstance.Disconnect()

	err = appInstance.Connect(ctx, connectionString)
	require.NoError(b, err)

	queryOpts := &app.ExecuteQueryOptions{
		Query: "SELECT COUNT(*) FROM test_mcp_schema.test_users",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := appInstance.ExecuteQuery(ctx, queryOpts)
		if err != nil {
			b.Fatal(err)
		}
	}
}
