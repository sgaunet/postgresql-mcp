package app_test

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/sylvain/postgresql-mcp/internal/app"
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

func (m *MockPostgreSQLClient) ListDatabases(ctx context.Context) ([]*app.DatabaseInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*app.DatabaseInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) GetCurrentDatabase(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockPostgreSQLClient) ListSchemas(ctx context.Context) ([]*app.SchemaInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*app.SchemaInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) ListTables(ctx context.Context, schema string) ([]*app.TableInfo, error) {
	args := m.Called(ctx, schema)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*app.TableInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) ListTablesWithStats(ctx context.Context, schema string) ([]*app.TableInfo, error) {
	args := m.Called(ctx, schema)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*app.TableInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) DescribeTable(ctx context.Context, schema, table string) ([]*app.ColumnInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*app.ColumnInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) GetTableStats(ctx context.Context, schema, table string) (*app.TableInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*app.TableInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) ListIndexes(ctx context.Context, schema, table string) ([]*app.IndexInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*app.IndexInfo), args.Error(1)
}

func (m *MockPostgreSQLClient) ExecuteQuery(ctx context.Context, query string, queryArgs ...any) (*app.QueryResult, error) {
	mockArgs := m.Called(ctx, query, queryArgs)
	if mockArgs.Get(0) == nil {
		return nil, mockArgs.Error(1)
	}
	return mockArgs.Get(0).(*app.QueryResult), mockArgs.Error(1)
}

func (m *MockPostgreSQLClient) ExplainQuery(ctx context.Context, query string, analyze bool, queryArgs ...any) (*app.QueryResult, error) {
	mockArgs := m.Called(ctx, query, analyze, queryArgs)
	if mockArgs.Get(0) == nil {
		return nil, mockArgs.Error(1)
	}
	return mockArgs.Get(0).(*app.QueryResult), mockArgs.Error(1)
}

func (m *MockPostgreSQLClient) GetDB() *sql.DB {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*sql.DB)
}

// stubDB returns a non-nil *sql.DB — a lazily-initialized pool that never
// actually opens a connection — so mock-based tests can simulate "a connection
// pool exists" for ensureConnection's GetDB() gate (issue #93).
func stubDB() *sql.DB {
	db, _ := sql.Open("postgres", "")
	return db
}

func TestNew(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)
	assert.NotNil(t, a)
	assert.NotNil(t, a.Client())
	assert.NotNil(t, a.Logger())
	assert.Equal(t, mockClient, a.Client())
}

// TestApp_NoProactivePingOnEstablishedConnection asserts that, with a live pool,
// a read operation issues no proactive Ping — only the query itself reaches the
// client (one round-trip per call, issue #93).
func TestApp_NoProactivePingOnEstablishedConnection(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExecuteQuery", mock.Anything, "SELECT 1", []any(nil)).
		Return(&app.QueryResult{Columns: []string{"?column?"}, Rows: [][]any{{1}}, RowCount: 1}, nil)

	_, err := a.ExecuteQuery(context.Background(), &app.ExecuteQueryOptions{Query: "SELECT 1"})
	require.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockClient.AssertNotCalled(t, "Ping", mock.Anything)
}

func TestNewDefault(t *testing.T) {
	a, err := app.NewDefault()
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.NotNil(t, a.Client())
	assert.NotNil(t, a.Logger())
}

func TestApp_SetLogger(t *testing.T) {
	a, _ := app.NewDefault()
	originalLogger := a.Logger()

	// Create a new logger
	newLogger := slog.Default()
	a.SetLogger(newLogger)

	assert.NotEqual(t, originalLogger, a.Logger())
	assert.Equal(t, newLogger, a.Logger())
}

func TestApp_Disconnect(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	mockClient.On("Close").Return(nil)

	err := a.Disconnect()
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_DisconnectWithNilClient(t *testing.T) {
	a := app.New(nil)

	err := a.Disconnect()
	assert.NoError(t, err)
}

func TestApp_ValidateConnection(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	// ValidateConnection ensures a pool exists (GetDB) and then actively pings.
	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("Ping", mock.Anything).Return(nil)

	err := a.ValidateConnection(context.Background())
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ValidateConnectionNilClient(t *testing.T) {
	a := app.New(nil)

	err := a.ValidateConnection(context.Background())
	assert.Error(t, err)
	assert.Equal(t, app.ErrConnectionRequired, err)
}

func TestApp_ValidateConnectionPingError(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	// A live pool exists, but the explicit liveness ping fails. ValidateConnection
	// must surface that ping error (issue #93: only ValidateConnection still pings).
	pingError := errors.New("ping failed")
	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("Ping", mock.Anything).Return(pingError)

	err := a.ValidateConnection(context.Background())
	assert.Error(t, err)
	assert.ErrorIs(t, err, pingError)
	mockClient.AssertExpectations(t)
}

func TestApp_ListDatabases(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedDatabases := []*app.DatabaseInfo{
		{Name: "db1", Owner: "user1", Encoding: "UTF8"},
		{Name: "db2", Owner: "user2", Encoding: "UTF8"},
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ListDatabases", mock.Anything).Return(expectedDatabases, nil)

	databases, err := a.ListDatabases(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectedDatabases, databases)
	mockClient.AssertExpectations(t)
}

func TestApp_ListDatabasesConnectionError(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	// No pool yet and no connection string available (no env, no prior Connect):
	// ensureConnection's bootstrap fails, surfacing ErrConnectionRequired wrapped
	// with operation context.
	mockClient.On("GetDB").Return(nil)

	databases, err := a.ListDatabases(context.Background())
	assert.Error(t, err)
	assert.Nil(t, databases)
	assert.ErrorIs(t, err, app.ErrConnectionRequired)
	mockClient.AssertExpectations(t)
}

func TestApp_GetCurrentDatabase(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedDB := "testdb"

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("GetCurrentDatabase", mock.Anything).Return(expectedDB, nil)

	dbName, err := a.GetCurrentDatabase(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectedDB, dbName)
	mockClient.AssertExpectations(t)
}

func TestApp_ListSchemas(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedSchemas := []*app.SchemaInfo{
		{Name: "public", Owner: "postgres"},
		{Name: "private", Owner: "user"},
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ListSchemas", mock.Anything).Return(expectedSchemas, nil)

	schemas, err := a.ListSchemas(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectedSchemas, schemas)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTables(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedTables := []*app.TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
		{Schema: "public", Name: "posts", Type: "table", Owner: "user"},
	}

	opts := &app.ListTablesOptions{
		Schema: "public",
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ListTables", mock.Anything, "public").Return(expectedTables, nil)

	tables, err := a.ListTables(context.Background(), opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithDefaultSchema(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedTables := []*app.TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
	}

	opts := &app.ListTablesOptions{}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ListTables", mock.Anything, app.DefaultSchema).Return(expectedTables, nil)

	tables, err := a.ListTables(context.Background(), opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithNilOptions(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedTables := []*app.TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ListTables", mock.Anything, app.DefaultSchema).Return(expectedTables, nil)

	tables, err := a.ListTables(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithSize(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	tablesWithStats := []*app.TableInfo{
		{
			Schema:   "public",
			Name:     "users",
			Type:     "table",
			Owner:    "postgres",
			RowCount: 1000,
			Size:     "5MB",
		},
	}

	opts := &app.ListTablesOptions{
		Schema:      "public",
		IncludeSize: true,
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ListTablesWithStats", mock.Anything, "public").Return(tablesWithStats, nil)

	tables, err := a.ListTables(context.Background(), opts)
	assert.NoError(t, err)
	assert.Len(t, tables, 1)
	assert.Equal(t, int64(1000), tables[0].RowCount)
	assert.Equal(t, "5MB", tables[0].Size)
	mockClient.AssertExpectations(t)
}

func TestApp_DescribeTable(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedColumns := []*app.ColumnInfo{
		{Name: "id", DataType: "integer", IsNullable: false},
		{Name: "name", DataType: "varchar(255)", IsNullable: true},
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("DescribeTable", mock.Anything, "public", "users").Return(expectedColumns, nil)

	columns, err := a.DescribeTable(context.Background(), "public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedColumns, columns)
	mockClient.AssertExpectations(t)
}

func TestApp_DescribeTableEmptyTableName(t *testing.T) {
	a, _ := app.NewDefault()

	columns, err := a.DescribeTable(context.Background(), "public", "")
	assert.Error(t, err)
	assert.Nil(t, columns)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_DescribeTableDefaultSchema(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedColumns := []*app.ColumnInfo{
		{Name: "id", DataType: "integer", IsNullable: false},
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("DescribeTable", mock.Anything, app.DefaultSchema, "users").Return(expectedColumns, nil)

	columns, err := a.DescribeTable(context.Background(), "", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedColumns, columns)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedResult := &app.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]interface{}{{1, "John"}, {2, "Jane"}},
		RowCount: 2,
	}

	opts := &app.ExecuteQueryOptions{
		Query: "SELECT id, name FROM users",
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExecuteQuery", mock.Anything, "SELECT id, name FROM users", []interface{}(nil)).Return(expectedResult, nil)

	result, err := a.ExecuteQuery(context.Background(), opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryWithLimit(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	// Server returns only the limited rows (PostgreSQL applies the LIMIT,
	// no over-fetch). Issue #91.
	limitedResult := &app.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]any{{1, "John"}, {2, "Jane"}},
		RowCount: 2,
	}

	opts := &app.ExecuteQueryOptions{
		Query: "SELECT id, name FROM users",
		Limit: 2,
	}

	expectedWrappedQuery := "SELECT * FROM (SELECT id, name FROM users) AS _postgres_mcp_limit_sub LIMIT 2"

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExecuteQuery", mock.Anything, expectedWrappedQuery, []any(nil)).Return(limitedResult, nil)

	result, err := a.ExecuteQuery(context.Background(), opts)
	assert.NoError(t, err)
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, 2, result.RowCount)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryLimitTrimsTrailingSemicolon(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	opts := &app.ExecuteQueryOptions{
		Query: "SELECT 1  \n",
		Limit: 5,
	}

	expectedWrappedQuery := "SELECT * FROM (SELECT 1) AS _postgres_mcp_limit_sub LIMIT 5"

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExecuteQuery", mock.Anything, expectedWrappedQuery, []any(nil)).
		Return(&app.QueryResult{Columns: []string{"?column?"}, Rows: [][]any{{1}}, RowCount: 1}, nil)

	_, err := a.ExecuteQuery(context.Background(), opts)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryWithoutLimitPassesQueryThrough(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	opts := &app.ExecuteQueryOptions{
		Query: "SELECT id, name FROM users",
	}

	mockClient.On("GetDB").Return(stubDB())
	// No wrap: original query is forwarded verbatim when Limit is unset.
	mockClient.On("ExecuteQuery", mock.Anything, "SELECT id, name FROM users", []any(nil)).
		Return(&app.QueryResult{Columns: []string{"id", "name"}, Rows: [][]any{}, RowCount: 0}, nil)

	_, err := a.ExecuteQuery(context.Background(), opts)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryNilOptions(t *testing.T) {
	a, _ := app.NewDefault()

	result, err := a.ExecuteQuery(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_ExecuteQueryEmptyQuery(t *testing.T) {
	a, _ := app.NewDefault()

	opts := &app.ExecuteQueryOptions{}

	result, err := a.ExecuteQuery(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_ExplainQuery(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedResult := &app.QueryResult{
		Columns:  []string{"QUERY PLAN"},
		Rows:     [][]interface{}{{"Seq Scan on users"}},
		RowCount: 1,
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExplainQuery", mock.Anything, "SELECT * FROM users", false, []interface{}(nil)).Return(expectedResult, nil)

	result, err := a.ExplainQuery(context.Background(), "SELECT * FROM users", false)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockClient.AssertExpectations(t)
}

func TestApp_ExplainQueryEmptyQuery(t *testing.T) {
	a, _ := app.NewDefault()

	result, err := a.ExplainQuery(context.Background(), "", false)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_GetTableStats(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedStats := &app.TableInfo{
		Schema:   "public",
		Name:     "users",
		RowCount: 1000,
		Size:     "5MB",
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("GetTableStats", mock.Anything, "public", "users").Return(expectedStats, nil)

	stats, err := a.GetTableStats(context.Background(), "public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats)
	mockClient.AssertExpectations(t)
}

func TestApp_ListIndexes(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	expectedIndexes := []*app.IndexInfo{
		{Name: "users_pkey", Table: "users", Columns: []string{"id"}, IsUnique: true, IsPrimary: true},
		{Name: "idx_users_email", Table: "users", Columns: []string{"email"}, IsUnique: true, IsPrimary: false},
	}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ListIndexes", mock.Anything, "public", "users").Return(expectedIndexes, nil)

	indexes, err := a.ListIndexes(context.Background(), "public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedIndexes, indexes)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_Success(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	connectionString := "postgres://user:pass@localhost/db"

	// Mock expectations
	mockClient.On("Ping", mock.Anything).Return(errors.New("not connected")) // No existing connection
	mockClient.On("Connect", mock.Anything, connectionString).Return(nil)

	err := a.Connect(context.Background(), connectionString)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_EmptyString(t *testing.T) {
	a, _ := app.NewDefault()

	err := a.Connect(context.Background(), "")
	assert.Error(t, err)
	assert.Equal(t, app.ErrNoConnectionString, err)
}

func TestApp_Connect_ReconnectClosesExisting(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	connectionString := "postgres://user:pass@localhost/db"

	// Mock expectations for reconnection scenario
	mockClient.On("Ping", mock.Anything).Return(nil).Once() // Existing connection is alive
	mockClient.On("Close").Return(nil).Once()               // Close existing
	mockClient.On("Connect", mock.Anything, connectionString).Return(nil)

	err := a.Connect(context.Background(), connectionString)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_ConnectError(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	connectionString := "postgres://user:pass@localhost/db"
	expectedError := errors.New("connection failed")

	// Mock expectations
	mockClient.On("Ping", mock.Anything).Return(errors.New("not connected")) // No existing connection
	mockClient.On("Connect", mock.Anything, connectionString).Return(expectedError)

	err := a.Connect(context.Background(), connectionString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery_SecurityAudit_InvalidQuery(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	opts := &app.ExecuteQueryOptions{Query: "INSERT INTO users VALUES (1)"}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExecuteQuery", mock.Anything, opts.Query, []interface{}(nil)).Return((*app.QueryResult)(nil), app.ErrInvalidQuery)

	result, err := a.ExecuteQuery(context.Background(), opts)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, app.ErrInvalidQuery)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery_SecurityAudit_MultiStatement(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	opts := &app.ExecuteQueryOptions{Query: "SELECT 1; DROP TABLE users"}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExecuteQuery", mock.Anything, opts.Query, []interface{}(nil)).Return((*app.QueryResult)(nil), app.ErrMultiStatementQuery)

	result, err := a.ExecuteQuery(context.Background(), opts)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, app.ErrMultiStatementQuery)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery_SecurityAudit_ResultTooLarge(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	opts := &app.ExecuteQueryOptions{Query: "SELECT * FROM huge_table"}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExecuteQuery", mock.Anything, opts.Query, []any(nil)).Return((*app.QueryResult)(nil), app.ErrResultTooLarge)

	result, err := a.ExecuteQuery(context.Background(), opts)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, app.ErrResultTooLarge)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery_SecurityAudit_QueryTooLong(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	opts := &app.ExecuteQueryOptions{Query: "SELECT very long query"}

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExecuteQuery", mock.Anything, opts.Query, []interface{}(nil)).Return((*app.QueryResult)(nil), app.ErrQueryTooLong)

	result, err := a.ExecuteQuery(context.Background(), opts)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, app.ErrQueryTooLong)
	mockClient.AssertExpectations(t)
}

func TestApp_ExplainQuery_SecurityAudit_InvalidQuery(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)

	query := "DELETE FROM users"

	mockClient.On("GetDB").Return(stubDB())
	mockClient.On("ExplainQuery", mock.Anything, query, false, []interface{}(nil)).Return((*app.QueryResult)(nil), app.ErrInvalidQuery)

	result, err := a.ExplainQuery(context.Background(), query, false)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, app.ErrInvalidQuery)
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
			result := app.TruncateQuery(tt.query, tt.maxLen)
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
	a := app.New(mockClient)

	// App.Connect calls Ping (to decide whether to Close an existing pool);
	// we always return an error so Close is skipped. The explicit Connect
	// stores the connection string as the sticky session string.
	mockClient.On("Ping", mock.Anything).Return(errors.New("no connection"))
	mockClient.On("Connect", mock.Anything, explicitConn).Return(nil)

	require.NoError(t, a.Connect(context.Background(), explicitConn))

	// Set an env var pointing at a DIFFERENT database — the dangerous
	// scenario from the issue. The sticky string must win over this.
	t.Setenv("POSTGRES_URL", envFallback)

	// The pool is gone (GetDB() == nil), so the next operation bootstraps —
	// and bootstrap must reuse the sticky connection string, not the env var.
	mockClient.On("GetDB").Return(nil)

	require.NoError(t, a.EnsureConnection(context.Background()))

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
	a := app.New(mockClient)

	t.Setenv("POSTGRES_URL", envConn)

	// No pool yet (GetDB() == nil) and no prior Connect → connStr is empty →
	// tryConnect must fall back to env. App.Connect's internal Ping
	// (existing-conn check) returns err → skip Close.
	mockClient.On("GetDB").Return(nil)
	mockClient.On("Ping", mock.Anything).Return(errors.New("no connection"))
	mockClient.On("Connect", mock.Anything, envConn).Return(nil)

	require.NoError(t, a.EnsureConnection(context.Background()))

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
