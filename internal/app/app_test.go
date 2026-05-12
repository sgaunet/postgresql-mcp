package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockPostgreSQLClient is a mock implementation of PostgreSQLClient for testing
type MockPostgreSQLClient struct {
	mock.Mock
}

func (m *MockPostgreSQLClient) Connect(ctx context.Context, connectionString string) error {
	args := m.Called(ctx, connectionString)
	return args.Error(0)
}

func (m *MockPostgreSQLClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockPostgreSQLClient) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockPostgreSQLClient) ListDatabases(ctx context.Context) ([]*DatabaseInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*DatabaseInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) GetCurrentDatabase(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockPostgreSQLClient) ListSchemas(ctx context.Context) ([]*SchemaInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*SchemaInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) ListTables(ctx context.Context, schema string) ([]*TableInfo, error) {
	args := m.Called(ctx, schema)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*TableInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) ListTablesWithStats(ctx context.Context, schema string) ([]*TableInfo, error) {
	args := m.Called(ctx, schema)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*TableInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) DescribeTable(ctx context.Context, schema, table string) ([]*ColumnInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ColumnInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) GetTableStats(ctx context.Context, schema, table string) (*TableInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TableInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) ListIndexes(ctx context.Context, schema, table string) ([]*IndexInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*IndexInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) ExecuteQuery(ctx context.Context, query string, queryArgs ...any) (*QueryResult, error) {
	mockArgs := m.Called(ctx, query, queryArgs)
	if mockArgs.Get(0) == nil {
		return nil, mockArgs.Error(1)
	}
	return mockArgs.Get(0).(*QueryResult), mockArgs.Error(1)
}

func (m *MockPostgreSQLClient) ExplainQuery(ctx context.Context, query string, queryArgs ...any) (*QueryResult, error) {
	mockArgs := m.Called(ctx, query, queryArgs)
	if mockArgs.Get(0) == nil {
		return nil, mockArgs.Error(1)
	}
	return mockArgs.Get(0).(*QueryResult), mockArgs.Error(1)
}

func (m *MockPostgreSQLClient) GetDB() *sql.DB {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*sql.DB)
}

func TestNew(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)
	assert.NotNil(t, app)
	assert.NotNil(t, app.client)
	assert.NotNil(t, app.logger)
	assert.Equal(t, mockClient, app.client)
}

func TestNewDefault(t *testing.T) {
	app, err := NewDefault()
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.client)
	assert.NotNil(t, app.logger)
}

func TestApp_SetLogger(t *testing.T) {
	app, _ := NewDefault()
	originalLogger := app.logger

	// Create a new logger
	newLogger := slog.Default()
	app.SetLogger(newLogger)

	assert.NotEqual(t, originalLogger, app.logger)
	assert.Equal(t, newLogger, app.logger)
}

func TestApp_Disconnect(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	mockClient.On("Close").Return(nil)

	err := app.Disconnect()
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_DisconnectWithNilClient(t *testing.T) {
	app := New(nil)

	err := app.Disconnect()
	assert.NoError(t, err)
}

func TestApp_ValidateConnection(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	mockClient.On("Ping", mock.Anything).Return(nil)

	err := app.ValidateConnection(context.Background())
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ValidateConnectionNilClient(t *testing.T) {
	app := New(nil)

	err := app.ValidateConnection(context.Background())
	assert.Error(t, err)
	assert.Equal(t, ErrConnectionRequired, err)
}

func TestApp_ValidateConnectionPingError(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	// Mock ping failure and reconnection failure (no env vars set)
	pingError := errors.New("ping failed")
	mockClient.On("Ping", mock.Anything).Return(pingError)

	err := app.ValidateConnection(context.Background())
	assert.Error(t, err)
	assert.Equal(t, ErrConnectionRequired, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ListDatabases(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedDatabases := []*DatabaseInfo{
		{Name: "db1", Owner: "user1", Encoding: "UTF8"},
		{Name: "db2", Owner: "user2", Encoding: "UTF8"},
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ListDatabases", mock.Anything).Return(expectedDatabases, nil)

	databases, err := app.ListDatabases(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectedDatabases, databases)
	mockClient.AssertExpectations(t)
}

func TestApp_ListDatabasesConnectionError(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedError := errors.New("connection error")
	mockClient.On("Ping", mock.Anything).Return(expectedError)

	databases, err := app.ListDatabases(context.Background())
	assert.Error(t, err)
	assert.Nil(t, databases)
	// ensureConnection returns ErrConnectionRequired, now wrapped with operation context
	assert.ErrorIs(t, err, ErrConnectionRequired)
	mockClient.AssertExpectations(t)
}

func TestApp_GetCurrentDatabase(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedDB := "testdb"

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("GetCurrentDatabase", mock.Anything).Return(expectedDB, nil)

	dbName, err := app.GetCurrentDatabase(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectedDB, dbName)
	mockClient.AssertExpectations(t)
}

func TestApp_ListSchemas(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedSchemas := []*SchemaInfo{
		{Name: "public", Owner: "postgres"},
		{Name: "private", Owner: "user"},
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ListSchemas", mock.Anything).Return(expectedSchemas, nil)

	schemas, err := app.ListSchemas(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectedSchemas, schemas)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTables(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedTables := []*TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
		{Schema: "public", Name: "posts", Type: "table", Owner: "user"},
	}

	opts := &ListTablesOptions{
		Schema: "public",
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ListTables", mock.Anything, "public").Return(expectedTables, nil)

	tables, err := app.ListTables(context.Background(), opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithDefaultSchema(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedTables := []*TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
	}

	opts := &ListTablesOptions{}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ListTables", mock.Anything, DefaultSchema).Return(expectedTables, nil)

	tables, err := app.ListTables(context.Background(), opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithNilOptions(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedTables := []*TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ListTables", mock.Anything, DefaultSchema).Return(expectedTables, nil)

	tables, err := app.ListTables(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithSize(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	tablesWithStats := []*TableInfo{
		{
			Schema:   "public",
			Name:     "users",
			Type:     "table",
			Owner:    "postgres",
			RowCount: 1000,
			Size:     "5MB",
		},
	}

	opts := &ListTablesOptions{
		Schema:      "public",
		IncludeSize: true,
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ListTablesWithStats", mock.Anything, "public").Return(tablesWithStats, nil)

	tables, err := app.ListTables(context.Background(), opts)
	assert.NoError(t, err)
	assert.Len(t, tables, 1)
	assert.Equal(t, int64(1000), tables[0].RowCount)
	assert.Equal(t, "5MB", tables[0].Size)
	mockClient.AssertExpectations(t)
}

func TestApp_DescribeTable(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedColumns := []*ColumnInfo{
		{Name: "id", DataType: "integer", IsNullable: false},
		{Name: "name", DataType: "varchar(255)", IsNullable: true},
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("DescribeTable", mock.Anything, "public", "users").Return(expectedColumns, nil)

	columns, err := app.DescribeTable(context.Background(), "public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedColumns, columns)
	mockClient.AssertExpectations(t)
}

func TestApp_DescribeTableEmptyTableName(t *testing.T) {
	app, _ := NewDefault()

	columns, err := app.DescribeTable(context.Background(), "public", "")
	assert.Error(t, err)
	assert.Nil(t, columns)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_DescribeTableDefaultSchema(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedColumns := []*ColumnInfo{
		{Name: "id", DataType: "integer", IsNullable: false},
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("DescribeTable", mock.Anything, DefaultSchema, "users").Return(expectedColumns, nil)

	columns, err := app.DescribeTable(context.Background(), "", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedColumns, columns)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedResult := &QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]interface{}{{1, "John"}, {2, "Jane"}},
		RowCount: 2,
	}

	opts := &ExecuteQueryOptions{
		Query: "SELECT id, name FROM users",
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExecuteQuery", mock.Anything, "SELECT id, name FROM users", []interface{}(nil)).Return(expectedResult, nil)

	result, err := app.ExecuteQuery(context.Background(), opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryWithLimit(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	// Server returns only the limited rows (PostgreSQL applies the LIMIT,
	// no over-fetch). Issue #91.
	limitedResult := &QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]any{{1, "John"}, {2, "Jane"}},
		RowCount: 2,
	}

	opts := &ExecuteQueryOptions{
		Query: "SELECT id, name FROM users",
		Limit: 2,
	}

	expectedWrappedQuery := "SELECT * FROM (SELECT id, name FROM users) AS _postgres_mcp_limit_sub LIMIT 2"

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExecuteQuery", mock.Anything, expectedWrappedQuery, []any(nil)).Return(limitedResult, nil)

	result, err := app.ExecuteQuery(context.Background(), opts)
	assert.NoError(t, err)
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, 2, result.RowCount)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryLimitTrimsTrailingSemicolon(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	opts := &ExecuteQueryOptions{
		Query: "SELECT 1  \n",
		Limit: 5,
	}

	expectedWrappedQuery := "SELECT * FROM (SELECT 1) AS _postgres_mcp_limit_sub LIMIT 5"

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExecuteQuery", mock.Anything, expectedWrappedQuery, []any(nil)).
		Return(&QueryResult{Columns: []string{"?column?"}, Rows: [][]any{{1}}, RowCount: 1}, nil)

	_, err := app.ExecuteQuery(context.Background(), opts)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryWithoutLimitPassesQueryThrough(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	opts := &ExecuteQueryOptions{
		Query: "SELECT id, name FROM users",
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	// No wrap: original query is forwarded verbatim when Limit is unset.
	mockClient.On("ExecuteQuery", mock.Anything, "SELECT id, name FROM users", []any(nil)).
		Return(&QueryResult{Columns: []string{"id", "name"}, Rows: [][]any{}, RowCount: 0}, nil)

	_, err := app.ExecuteQuery(context.Background(), opts)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryNilOptions(t *testing.T) {
	app, _ := NewDefault()

	result, err := app.ExecuteQuery(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_ExecuteQueryEmptyQuery(t *testing.T) {
	app, _ := NewDefault()

	opts := &ExecuteQueryOptions{}

	result, err := app.ExecuteQuery(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_ExplainQuery(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedResult := &QueryResult{
		Columns:  []string{"QUERY PLAN"},
		Rows:     [][]interface{}{{"Seq Scan on users"}},
		RowCount: 1,
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExplainQuery", mock.Anything, "SELECT * FROM users", []interface{}(nil)).Return(expectedResult, nil)

	result, err := app.ExplainQuery(context.Background(), "SELECT * FROM users")
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockClient.AssertExpectations(t)
}

func TestApp_ExplainQueryEmptyQuery(t *testing.T) {
	app, _ := NewDefault()

	result, err := app.ExplainQuery(context.Background(), "")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_GetTableStats(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedStats := &TableInfo{
		Schema:   "public",
		Name:     "users",
		RowCount: 1000,
		Size:     "5MB",
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("GetTableStats", mock.Anything, "public", "users").Return(expectedStats, nil)

	stats, err := app.GetTableStats(context.Background(), "public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats)
	mockClient.AssertExpectations(t)
}

func TestApp_ListIndexes(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	expectedIndexes := []*IndexInfo{
		{Name: "users_pkey", Table: "users", Columns: []string{"id"}, IsUnique: true, IsPrimary: true},
		{Name: "idx_users_email", Table: "users", Columns: []string{"email"}, IsUnique: true, IsPrimary: false},
	}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ListIndexes", mock.Anything, "public", "users").Return(expectedIndexes, nil)

	indexes, err := app.ListIndexes(context.Background(), "public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedIndexes, indexes)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_Success(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	connectionString := "postgres://user:pass@localhost/db"

	// Mock expectations
	mockClient.On("Ping", mock.Anything).Return(errors.New("not connected")) // No existing connection
	mockClient.On("Connect", mock.Anything, connectionString).Return(nil)

	err := app.Connect(context.Background(), connectionString)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_EmptyString(t *testing.T) {
	app, _ := NewDefault()

	err := app.Connect(context.Background(), "")
	assert.Error(t, err)
	assert.Equal(t, ErrNoConnectionString, err)
}

func TestApp_Connect_ReconnectClosesExisting(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	connectionString := "postgres://user:pass@localhost/db"

	// Mock expectations for reconnection scenario
	mockClient.On("Ping", mock.Anything).Return(nil).Once()      // Existing connection is alive
	mockClient.On("Close").Return(nil).Once()     // Close existing
	mockClient.On("Connect", mock.Anything, connectionString).Return(nil)

	err := app.Connect(context.Background(), connectionString)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_ConnectError(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	connectionString := "postgres://user:pass@localhost/db"
	expectedError := errors.New("connection failed")

	// Mock expectations
	mockClient.On("Ping", mock.Anything).Return(errors.New("not connected")) // No existing connection
	mockClient.On("Connect", mock.Anything, connectionString).Return(expectedError)

	err := app.Connect(context.Background(), connectionString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery_SecurityAudit_InvalidQuery(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	opts := &ExecuteQueryOptions{Query: "INSERT INTO users VALUES (1)"}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExecuteQuery", mock.Anything, opts.Query, []interface{}(nil)).Return((*QueryResult)(nil), ErrInvalidQuery)

	result, err := app.ExecuteQuery(context.Background(), opts)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrInvalidQuery)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery_SecurityAudit_MultiStatement(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	opts := &ExecuteQueryOptions{Query: "SELECT 1; DROP TABLE users"}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExecuteQuery", mock.Anything, opts.Query, []interface{}(nil)).Return((*QueryResult)(nil), ErrMultiStatementQuery)

	result, err := app.ExecuteQuery(context.Background(), opts)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrMultiStatementQuery)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery_SecurityAudit_ResultTooLarge(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	opts := &ExecuteQueryOptions{Query: "SELECT * FROM huge_table"}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExecuteQuery", mock.Anything, opts.Query, []any(nil)).Return((*QueryResult)(nil), ErrResultTooLarge)

	result, err := app.ExecuteQuery(context.Background(), opts)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrResultTooLarge)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery_SecurityAudit_QueryTooLong(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	opts := &ExecuteQueryOptions{Query: "SELECT very long query"}

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExecuteQuery", mock.Anything, opts.Query, []interface{}(nil)).Return((*QueryResult)(nil), ErrQueryTooLong)

	result, err := app.ExecuteQuery(context.Background(), opts)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrQueryTooLong)
	mockClient.AssertExpectations(t)
}

func TestApp_ExplainQuery_SecurityAudit_InvalidQuery(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	query := "DELETE FROM users"

	mockClient.On("Ping", mock.Anything).Return(nil)
	mockClient.On("ExplainQuery", mock.Anything, query, []interface{}(nil)).Return((*QueryResult)(nil), ErrInvalidQuery)

	result, err := app.ExplainQuery(context.Background(), query)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrInvalidQuery)
	mockClient.AssertExpectations(t)
}

func TestTruncateQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		maxLen   int
		expected string
	}{
		{name: "short query unchanged", query: "SELECT 1", maxLen: 100, expected: "SELECT 1"},
		{name: "exact length unchanged", query: "SELECT 1", maxLen: 8, expected: "SELECT 1"},
		{name: "long query truncated", query: "SELECT * FROM very_long_table_name WHERE id = 1", maxLen: 20, expected: "SELECT * FROM very_l..."},
		{name: "empty query", query: "", maxLen: 100, expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateQuery(tt.query, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestApp_ReconnectUsesStickyConnectionString_Issue87 verifies that after an
// explicit Connect (e.g. via the connect_database tool), an auto-reconnect
// triggered by a ping failure re-uses the originally-supplied connection
// string instead of silently falling back to POSTGRES_URL / DATABASE_URL.
// Without this fix, a network blip could switch the session to a different
// database (issue #87).
func TestApp_ReconnectUsesStickyConnectionString_Issue87(t *testing.T) {
	const explicitConn = "postgres://userA:pw@host-a:5432/dbA"
	const envFallback = "postgres://userB:pw@host-b:5432/dbB"

	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	// Initial explicit Connect: App.Connect first calls Ping (to decide
	// whether to Close an existing pool); we return an error so Close is
	// skipped. Then it calls Connect with our explicit string.
	mockClient.On("Ping", mock.Anything).Return(errors.New("no connection")).Once()
	mockClient.On("Connect", mock.Anything, explicitConn).Return(nil)

	require.NoError(t, app.Connect(context.Background(), explicitConn))

	// Set an env var pointing at a DIFFERENT database — the dangerous
	// scenario from the issue. The fix must not let this override the
	// caller's explicit choice.
	t.Setenv("POSTGRES_URL", envFallback)

	// Sever the connection: every subsequent Ping fails.
	mockClient.On("Ping", mock.Anything).Return(errors.New("connection lost"))

	require.NoError(t, app.ensureConnection(context.Background()))

	// Every Connect call must have targeted the explicit string. If the bug
	// were present, a reconnect with `envFallback` would be issued and the
	// mock — which has no expectation for that argument — would surface it
	// as an unexpected Call entry.
	var connectArgs []string
	for _, call := range mockClient.Calls {
		if call.Method == "Connect" {
			connectArgs = append(connectArgs, call.Arguments[1].(string))
		}
	}
	require.Len(t, connectArgs, 2, "expected initial Connect + one reconnect")
	for _, arg := range connectArgs {
		assert.Equal(t, explicitConn, arg,
			"reconnect must reuse the explicit connection string, not env fallback (issue #87)")
	}
}

// TestApp_ReconnectFallsBackToEnvOnInitialBootstrap verifies that the env-var
// fallback still applies when no prior Connect has been made — i.e. for the
// very first connection of a fresh process where the user has set POSTGRES_URL
// instead of calling connect_database. This is the legitimate complement to
// the sticky-session behavior.
func TestApp_ReconnectFallsBackToEnvOnInitialBootstrap(t *testing.T) {
	const envConn = "postgres://user:pw@host:5432/db"

	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)

	t.Setenv("POSTGRES_URL", envConn)

	// No prior Connect → connStr is empty → tryConnect must fall back to env.
	// App.Connect's internal Ping (existing-conn check) returns err → skip Close.
	mockClient.On("Ping", mock.Anything).Return(errors.New("no connection"))
	mockClient.On("Connect", mock.Anything, envConn).Return(nil)

	require.NoError(t, app.ensureConnection(context.Background()))

	var connectArgs []string
	for _, call := range mockClient.Calls {
		if call.Method == "Connect" {
			connectArgs = append(connectArgs, call.Arguments[1].(string))
		}
	}
	require.Len(t, connectArgs, 1)
	assert.Equal(t, envConn, connectArgs[0],
		"initial bootstrap (no prior session) must still honor POSTGRES_URL")
}
