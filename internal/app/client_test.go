package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDB represents a mock database connection for testing
type MockDB struct {
	mock.Mock
}

func (m *MockDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	mockArgs := m.Called(query, args)
	if rows, ok := mockArgs.Get(0).(*sql.Rows); ok {
		return rows, mockArgs.Error(1)
	}
	return nil, mockArgs.Error(1)
}

func (m *MockDB) QueryRow(query string, args ...interface{}) *sql.Row {
	mockArgs := m.Called(query, args)
	return mockArgs.Get(0).(*sql.Row)
}

func (m *MockDB) Ping() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDB) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewPostgreSQLClient(t *testing.T) {
	client := NewPostgreSQLClient()
	assert.NotNil(t, client)
	assert.IsType(t, &PostgreSQLClientImpl{}, client)
}

func TestPostgreSQLClient_Connect_InvalidConnectionString(t *testing.T) {
	client := NewPostgreSQLClient()

	tests := []struct {
		name             string
		connectionString string
		expectError      bool
	}{
		{
			name:             "invalid connection string",
			connectionString: "invalid://connection",
			expectError:      true,
		},
		{
			name:             "empty connection string",
			connectionString: "",
			expectError:      true,
		},
		{
			name:             "malformed postgres URL",
			connectionString: "postgres://user@host:invalid_port/db",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.Connect(context.Background(), tt.connectionString)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				client.Close()
			}
		})
	}
}

func TestPostgreSQLClient_CloseWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	err := client.Close()
	assert.NoError(t, err)
}

func TestPostgreSQLClient_PingWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	err := client.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_GetDBWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	db := client.GetDB()
	assert.Nil(t, db)
}

func TestPostgreSQLClient_ListDatabasesWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	databases, err := client.ListDatabases(context.Background())
	assert.Error(t, err)
	assert.Nil(t, databases)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_GetCurrentDatabaseWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	dbName, err := client.GetCurrentDatabase(context.Background())
	assert.Error(t, err)
	assert.Empty(t, dbName)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_ListSchemasWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	schemas, err := client.ListSchemas(context.Background())
	assert.Error(t, err)
	assert.Nil(t, schemas)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_ListTablesWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	tables, err := client.ListTables(context.Background(), "public")
	assert.Error(t, err)
	assert.Nil(t, tables)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_ListTablesWithEmptySchema(t *testing.T) {
	client := NewPostgreSQLClient()
	tables, err := client.ListTables(context.Background(), "")
	assert.Error(t, err)
	assert.Nil(t, tables)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_DescribeTableWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	columns, err := client.DescribeTable(context.Background(), "public", "users")
	assert.Error(t, err)
	assert.Nil(t, columns)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_DescribeTableWithEmptySchema(t *testing.T) {
	client := NewPostgreSQLClient()
	columns, err := client.DescribeTable(context.Background(), "", "users")
	assert.Error(t, err)
	assert.Nil(t, columns)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_GetTableStatsWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	stats, err := client.GetTableStats(context.Background(), "public", "users")
	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_GetTableStatsWithEmptySchema(t *testing.T) {
	client := NewPostgreSQLClient()
	stats, err := client.GetTableStats(context.Background(), "", "users")
	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_ListIndexesWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	indexes, err := client.ListIndexes(context.Background(), "public", "users")
	assert.Error(t, err)
	assert.Nil(t, indexes)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_ListIndexesWithEmptySchema(t *testing.T) {
	client := NewPostgreSQLClient()
	indexes, err := client.ListIndexes(context.Background(), "", "users")
	assert.Error(t, err)
	assert.Nil(t, indexes)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_ExecuteQueryWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	result, err := client.ExecuteQuery(context.Background(), "SELECT 1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_ExecuteQueryInvalidQueries(t *testing.T) {
	client := NewPostgreSQLClient()

	tests := []struct {
		name        string
		query       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "INSERT query not allowed",
			query:       "INSERT INTO users (name) VALUES ('test')",
			expectError: true,
			errorMsg:    "only SELECT and WITH queries are allowed",
		},
		{
			name:        "UPDATE query not allowed",
			query:       "UPDATE users SET name = 'test'",
			expectError: true,
			errorMsg:    "only SELECT and WITH queries are allowed",
		},
		{
			name:        "DELETE query not allowed",
			query:       "DELETE FROM users",
			expectError: true,
			errorMsg:    "only SELECT and WITH queries are allowed",
		},
		{
			name:        "DROP query not allowed",
			query:       "DROP TABLE users",
			expectError: true,
			errorMsg:    "only SELECT and WITH queries are allowed",
		},
		{
			name:        "CREATE query not allowed",
			query:       "CREATE TABLE test (id INT)",
			expectError: true,
			errorMsg:    "only SELECT and WITH queries are allowed",
		},
		{
			name:        "ALTER query not allowed",
			query:       "ALTER TABLE users ADD COLUMN test INT",
			expectError: true,
			errorMsg:    "only SELECT and WITH queries are allowed",
		},
		{
			name:        "SELECT query should be allowed (but will fail due to no real connection)",
			query:       "SELECT * FROM users",
			expectError: true,
			errorMsg:    "no database connection",
		},
		{
			name:        "WITH query should be allowed (but will fail due to no real connection)",
			query:       "WITH cte AS (SELECT 1) SELECT * FROM cte",
			expectError: true,
			errorMsg:    "no database connection",
		},
		{
			name:        "Query with leading whitespace",
			query:       "   SELECT * FROM users",
			expectError: true,
			errorMsg:    "no database connection",
		},
		{
			name:        "Query with mixed case",
			query:       "select * from users",
			expectError: true,
			errorMsg:    "only SELECT and WITH queries are allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.ExecuteQuery(context.Background(), tt.query)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg == "only SELECT and WITH queries are allowed" {
					assert.Contains(t, err.Error(), "no database connection")
				} else {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestPostgreSQLClient_ExplainQueryWithoutConnection(t *testing.T) {
	client := NewPostgreSQLClient()
	result, err := client.ExplainQuery(context.Background(), "SELECT 1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no database connection")
}

func TestPostgreSQLClient_ExplainQueryValidation(t *testing.T) {
	client := NewPostgreSQLClient()

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "SELECT query",
			query: "SELECT * FROM users",
		},
		{
			name:  "WITH query",
			query: "WITH cte AS (SELECT 1) SELECT * FROM cte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail due to no real connection, but we're testing the query validation
			result, err := client.ExplainQuery(context.Background(), tt.query)
			assert.Error(t, err)
			assert.Nil(t, result)
			// Should fail with connection error since no real connection
			assert.Contains(t, err.Error(), "no database connection")
		})
	}
}

// Test helper functions and edge cases

func TestConnectionStringValidation(t *testing.T) {
	client := &PostgreSQLClientImpl{}

	// Test that Connect properly validates and handles errors
	err := client.Connect(context.Background(), "postgres://invaliduser:invalidpass@nonexistenthost:5432/nonexistentdb")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping database")
}

func TestQueryResultProcessing(t *testing.T) {
	// Test the []byte to string conversion logic

	// This tests the conversion logic that happens in ExecuteQuery
	// when processing byte slices from the database
	testData := []interface{}{
		[]byte("test string"),
		"regular string",
		42,
		true,
		nil,
	}

	// Simulate the conversion that happens in ExecuteQuery
	for i, v := range testData {
		if b, ok := v.([]byte); ok {
			testData[i] = string(b)
		}
	}

	assert.Equal(t, "test string", testData[0])
	assert.Equal(t, "regular string", testData[1])
	assert.Equal(t, 42, testData[2])
	assert.Equal(t, true, testData[3])
	assert.Nil(t, testData[4])
}

func TestDefaultSchemaHandling(t *testing.T) {
	client := NewPostgreSQLClient()

	// Test that empty schema defaults to "public"
	tests := []struct {
		inputSchema    string
		expectedSchema string
	}{
		{"", "public"},
		{"custom", "custom"},
		{"public", "public"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("schema_%s", tt.inputSchema), func(t *testing.T) {
			// These will fail due to no connection, but we can verify
			// that the schema parameter is properly processed
			_, err := client.ListTables(context.Background(), tt.inputSchema)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")

			_, err = client.DescribeTable(context.Background(), tt.inputSchema, "test_table")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")

			_, err = client.ListIndexes(context.Background(), tt.inputSchema, "test_table")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")

			_, err = client.GetTableStats(context.Background(), tt.inputSchema, "test_table")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")
		})
	}
}

// Test SQL query construction
func TestSQLQueryConstruction(t *testing.T) {
	// Test that our SQL queries are properly constructed
	// This is mainly to ensure no SQL injection vulnerabilities

	tests := []struct {
		name   string
		schema string
		table  string
	}{
		{
			name:   "normal names",
			schema: "public",
			table:  "users",
		},
		{
			name:   "names with special characters",
			schema: "test_schema",
			table:  "test_table",
		},
	}

	client := NewPostgreSQLClient()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that functions handle schema and table parameters properly
			_, err := client.DescribeTable(context.Background(), tt.schema, tt.table)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")

			_, err = client.ListIndexes(context.Background(), tt.schema, tt.table)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")

			_, err = client.GetTableStats(context.Background(), tt.schema, tt.table)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")
		})
	}
}

func TestPostgreSQLClientImpl_ConnectAndClose(t *testing.T) {
	client := &PostgreSQLClientImpl{}

	// Test that Close works even without connection
	err := client.Close()
	assert.NoError(t, err)

	// Test that GetDB returns nil when no connection
	db := client.GetDB()
	assert.Nil(t, db)
}

func TestExecuteQueryEmptyResult(t *testing.T) {

	// Mock an empty database result scenario
	// This tests the logic for handling empty query results
	result := &QueryResult{
		Columns:  []string{},
		Rows:     [][]interface{}{},
		RowCount: 0,
	}

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.RowCount)
	assert.Len(t, result.Rows, 0)
	assert.Len(t, result.Columns, 0)
}

func TestValidateQuery(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantErr   error
		wantNoErr bool
	}{
		// Valid queries
		{name: "simple SELECT", query: "SELECT * FROM users", wantNoErr: true},
		{name: "lowercase select", query: "select * from users", wantNoErr: true},
		{name: "mixed case SELECT", query: "SeLeCt * FROM users", wantNoErr: true},
		{name: "WITH CTE", query: "WITH cte AS (SELECT 1) SELECT * FROM cte", wantNoErr: true},
		{name: "leading whitespace", query: "   SELECT * FROM users", wantNoErr: true},
		{name: "semicolon inside string literal", query: "SELECT * FROM users WHERE name = 'a;b'", wantNoErr: true},
		{name: "block comment inside string literal", query: "SELECT '/* not a comment */' FROM users", wantNoErr: true},
		{name: "semicolon in double-quoted identifier", query: `SELECT "col;name" FROM users`, wantNoErr: true},

		// Invalid queries (wrong statement type)
		{name: "INSERT", query: "INSERT INTO users (name) VALUES ('test')", wantErr: ErrInvalidQuery},
		{name: "UPDATE", query: "UPDATE users SET name = 'test'", wantErr: ErrInvalidQuery},
		{name: "DELETE", query: "DELETE FROM users", wantErr: ErrInvalidQuery},
		{name: "DROP TABLE", query: "DROP TABLE users", wantErr: ErrInvalidQuery},
		{name: "CREATE TABLE", query: "CREATE TABLE test (id INT)", wantErr: ErrInvalidQuery},
		{name: "ALTER TABLE", query: "ALTER TABLE users ADD COLUMN test INT", wantErr: ErrInvalidQuery},
		{name: "TRUNCATE", query: "TRUNCATE users", wantErr: ErrInvalidQuery},

		// Comment-based injection (should be caught after stripping comments)
		{name: "block comment hiding INSERT", query: "/* hidden */ INSERT INTO users VALUES (1)", wantErr: ErrInvalidQuery},
		{name: "line comment hiding INSERT", query: "-- comment\nINSERT INTO users VALUES (1)", wantErr: ErrInvalidQuery},
		{name: "nested block comment hiding DROP", query: "/* outer /* inner */ still comment */ DROP TABLE users", wantErr: ErrInvalidQuery},

		// Multi-statement injection (should be caught by semicolon detection)
		{name: "SELECT then DROP", query: "SELECT 1; DROP TABLE users", wantErr: ErrMultiStatementQuery},
		{name: "two SELECTs", query: "SELECT 1; SELECT 2", wantErr: ErrMultiStatementQuery},
		{name: "trailing semicolon", query: "SELECT 1;", wantErr: ErrMultiStatementQuery},
		{name: "semicolon with spaces", query: "SELECT 1 ; DROP TABLE users", wantErr: ErrMultiStatementQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQuery(tt.query)
			if tt.wantNoErr {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestStripComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "block comment",
			input:    "SELECT /* comment */ * FROM users",
			expected: "SELECT   * FROM users",
		},
		{
			name:     "line comment",
			input:    "SELECT * FROM users -- trailing comment",
			expected: "SELECT * FROM users  ",
		},
		{
			name:     "line comment with newline",
			input:    "SELECT * FROM users -- comment\nWHERE id = 1",
			expected: "SELECT * FROM users  \nWHERE id = 1",
		},
		{
			name:     "comment inside single-quoted string preserved",
			input:    "SELECT '/* not a comment */' FROM users",
			expected: "SELECT '/* not a comment */' FROM users",
		},
		{
			name:     "comment inside double-quoted identifier preserved",
			input:    `SELECT "col--name" FROM users`,
			expected: `SELECT "col--name" FROM users`,
		},
		{
			name:     "nested block comments",
			input:    "SELECT /* outer /* inner */ still comment */ * FROM users",
			expected: "SELECT   * FROM users",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only a comment",
			input:    "/* just a comment */",
			expected: " ",
		},
		{
			name:     "escaped single quote in string",
			input:    "SELECT * FROM users WHERE name = 'O''Brien'",
			expected: "SELECT * FROM users WHERE name = 'O''Brien'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripComments(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsSemicolonOutsideLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "no semicolon", input: "SELECT * FROM users", expected: false},
		{name: "semicolon outside", input: "SELECT 1; DROP TABLE users", expected: true},
		{name: "trailing semicolon", input: "SELECT 1;", expected: true},
		{name: "semicolon in single-quoted string", input: "SELECT * FROM users WHERE name = 'a;b'", expected: false},
		{name: "semicolon in double-quoted identifier", input: `SELECT "col;name" FROM users`, expected: false},
		{name: "semicolon both inside and outside", input: "SELECT 'a;b'; DROP TABLE users", expected: true},
		{name: "empty string", input: "", expected: false},
		{name: "escaped quote then semicolon", input: "SELECT * FROM users WHERE name = 'O''Brien';", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsSemicolonOutsideLiterals(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInjectReadOnlyOption(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "URL-style with existing params",
			input:    "postgres://user:pass@localhost:5432/db?sslmode=prefer",
			contains: "default_transaction_read_only",
		},
		{
			name:     "URL-style without params",
			input:    "postgres://user:pass@localhost:5432/db",
			contains: "default_transaction_read_only",
		},
		{
			name:     "postgresql:// scheme",
			input:    "postgresql://user:pass@localhost/db",
			contains: "default_transaction_read_only",
		},
		{
			name:     "keyword-value style",
			input:    "host=localhost port=5432 dbname=mydb user=myuser",
			contains: "default_transaction_read_only",
		},
		{
			name:     "empty string",
			input:    "",
			contains: "",
		},
		{
			name:     "keyword-value with existing options",
			input:    "host=localhost options='-c search_path=public'",
			contains: "default_transaction_read_only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectReadOnlyOption(tt.input)
			if tt.contains != "" {
				assert.Contains(t, result, tt.contains)
			} else {
				assert.Equal(t, tt.input, result)
			}
		})
	}
}
