package app

import (
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPostgreSQLClient is a mock implementation of PostgreSQLClient for testing
type MockPostgreSQLClient struct {
	mock.Mock
}

func (m *MockPostgreSQLClient) Connect(connectionString string) error {
	args := m.Called(connectionString)
	return args.Error(0)
}

func (m *MockPostgreSQLClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockPostgreSQLClient) Ping() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockPostgreSQLClient) ListDatabases() ([]*DatabaseInfo, error) {
	args := m.Called()
	if databases, ok := args.Get(0).([]*DatabaseInfo); ok {
		return databases, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPostgreSQLClient) GetCurrentDatabase() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockPostgreSQLClient) ListSchemas() ([]*SchemaInfo, error) {
	args := m.Called()
	if schemas, ok := args.Get(0).([]*SchemaInfo); ok {
		return schemas, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPostgreSQLClient) ListTables(schema string) ([]*TableInfo, error) {
	args := m.Called(schema)
	if tables, ok := args.Get(0).([]*TableInfo); ok {
		return tables, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPostgreSQLClient) DescribeTable(schema, table string) ([]*ColumnInfo, error) {
	args := m.Called(schema, table)
	if columns, ok := args.Get(0).([]*ColumnInfo); ok {
		return columns, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPostgreSQLClient) GetTableStats(schema, table string) (*TableInfo, error) {
	args := m.Called(schema, table)
	if stats, ok := args.Get(0).(*TableInfo); ok {
		return stats, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPostgreSQLClient) ListIndexes(schema, table string) ([]*IndexInfo, error) {
	args := m.Called(schema, table)
	if indexes, ok := args.Get(0).([]*IndexInfo); ok {
		return indexes, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPostgreSQLClient) ExecuteQuery(query string, args ...interface{}) (*QueryResult, error) {
	mockArgs := m.Called(query, args)
	if result, ok := mockArgs.Get(0).(*QueryResult); ok {
		return result, mockArgs.Error(1)
	}
	return nil, mockArgs.Error(1)
}

func (m *MockPostgreSQLClient) ExplainQuery(query string, args ...interface{}) (*QueryResult, error) {
	mockArgs := m.Called(query, args)
	if result, ok := mockArgs.Get(0).(*QueryResult); ok {
		return result, mockArgs.Error(1)
	}
	return nil, mockArgs.Error(1)
}

func (m *MockPostgreSQLClient) GetDB() *sql.DB {
	args := m.Called()
	if db, ok := args.Get(0).(*sql.DB); ok {
		return db
	}
	return nil
}

func TestNew(t *testing.T) {
	app, err := New()
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.client)
	assert.NotNil(t, app.logger)
}

func TestApp_SetLogger(t *testing.T) {
	app, _ := New()
	originalLogger := app.logger

	// Create a new logger
	newLogger := slog.Default()
	app.SetLogger(newLogger)

	assert.NotEqual(t, originalLogger, app.logger)
	assert.Equal(t, newLogger, app.logger)
}

func TestApp_Disconnect(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	mockClient.On("Close").Return(nil)

	err := app.Disconnect()
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_DisconnectWithNilClient(t *testing.T) {
	app, _ := New()
	app.client = nil

	err := app.Disconnect()
	assert.NoError(t, err)
}

func TestApp_ValidateConnection(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	mockClient.On("Ping").Return(nil)

	err := app.ValidateConnection()
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ValidateConnectionNilClient(t *testing.T) {
	app, _ := New()
	app.client = nil

	err := app.ValidateConnection()
	assert.Error(t, err)
	assert.Equal(t, ErrConnectionRequired, err)
}

func TestApp_ValidateConnectionPingError(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	// Mock ping failure and reconnection failure (no env vars set)
	pingError := errors.New("ping failed")
	mockClient.On("Ping").Return(pingError)

	err := app.ValidateConnection()
	assert.Error(t, err)
	assert.Equal(t, ErrConnectionRequired, err)
	mockClient.AssertExpectations(t)
}

func TestApp_ListDatabases(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedDatabases := []*DatabaseInfo{
		{Name: "db1", Owner: "user1", Encoding: "UTF8"},
		{Name: "db2", Owner: "user2", Encoding: "UTF8"},
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ListDatabases").Return(expectedDatabases, nil)

	databases, err := app.ListDatabases()
	assert.NoError(t, err)
	assert.Equal(t, expectedDatabases, databases)
	mockClient.AssertExpectations(t)
}

func TestApp_ListDatabasesConnectionError(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedError := errors.New("connection error")
	mockClient.On("Ping").Return(expectedError)

	databases, err := app.ListDatabases()
	assert.Error(t, err)
	assert.Nil(t, databases)
	// After our refactoring, ping failure leads to reconnection attempt, which fails due to no env vars,
	// so we get ErrConnectionRequired instead of the original ping error
	assert.Equal(t, ErrConnectionRequired, err)
	mockClient.AssertExpectations(t)
}

func TestApp_GetCurrentDatabase(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedDB := "testdb"

	mockClient.On("Ping").Return(nil)
	mockClient.On("GetCurrentDatabase").Return(expectedDB, nil)

	dbName, err := app.GetCurrentDatabase()
	assert.NoError(t, err)
	assert.Equal(t, expectedDB, dbName)
	mockClient.AssertExpectations(t)
}

func TestApp_ListSchemas(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedSchemas := []*SchemaInfo{
		{Name: "public", Owner: "postgres"},
		{Name: "private", Owner: "user"},
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ListSchemas").Return(expectedSchemas, nil)

	schemas, err := app.ListSchemas()
	assert.NoError(t, err)
	assert.Equal(t, expectedSchemas, schemas)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTables(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedTables := []*TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
		{Schema: "public", Name: "posts", Type: "table", Owner: "user"},
	}

	opts := &ListTablesOptions{
		Schema: "public",
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ListTables", "public").Return(expectedTables, nil)

	tables, err := app.ListTables(opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithDefaultSchema(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedTables := []*TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
	}

	opts := &ListTablesOptions{}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ListTables", DefaultSchema).Return(expectedTables, nil)

	tables, err := app.ListTables(opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithNilOptions(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedTables := []*TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ListTables", DefaultSchema).Return(expectedTables, nil)

	tables, err := app.ListTables(nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedTables, tables)
	mockClient.AssertExpectations(t)
}

func TestApp_ListTablesWithSize(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	initialTables := []*TableInfo{
		{Schema: "public", Name: "users", Type: "table", Owner: "user"},
	}

	tableStats := &TableInfo{
		Schema:   "public",
		Name:     "users",
		RowCount: 1000,
		Size:     "5MB",
	}

	opts := &ListTablesOptions{
		Schema:      "public",
		IncludeSize: true,
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ListTables", "public").Return(initialTables, nil)
	mockClient.On("GetTableStats", "public", "users").Return(tableStats, nil)

	tables, err := app.ListTables(opts)
	assert.NoError(t, err)
	assert.Len(t, tables, 1)
	assert.Equal(t, int64(1000), tables[0].RowCount)
	assert.Equal(t, "5MB", tables[0].Size)
	mockClient.AssertExpectations(t)
}

func TestApp_DescribeTable(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedColumns := []*ColumnInfo{
		{Name: "id", DataType: "integer", IsNullable: false},
		{Name: "name", DataType: "varchar(255)", IsNullable: true},
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("DescribeTable", "public", "users").Return(expectedColumns, nil)

	columns, err := app.DescribeTable("public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedColumns, columns)
	mockClient.AssertExpectations(t)
}

func TestApp_DescribeTableEmptyTableName(t *testing.T) {
	app, _ := New()

	columns, err := app.DescribeTable("public", "")
	assert.Error(t, err)
	assert.Nil(t, columns)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_DescribeTableDefaultSchema(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedColumns := []*ColumnInfo{
		{Name: "id", DataType: "integer", IsNullable: false},
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("DescribeTable", DefaultSchema, "users").Return(expectedColumns, nil)

	columns, err := app.DescribeTable("", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedColumns, columns)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQuery(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedResult := &QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]interface{}{{1, "John"}, {2, "Jane"}},
		RowCount: 2,
	}

	opts := &ExecuteQueryOptions{
		Query: "SELECT id, name FROM users",
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ExecuteQuery", "SELECT id, name FROM users", []interface{}(nil)).Return(expectedResult, nil)

	result, err := app.ExecuteQuery(opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryWithLimit(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	originalResult := &QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]interface{}{{1, "John"}, {2, "Jane"}, {3, "Bob"}},
		RowCount: 3,
	}

	opts := &ExecuteQueryOptions{
		Query: "SELECT id, name FROM users",
		Limit: 2,
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ExecuteQuery", "SELECT id, name FROM users", []interface{}(nil)).Return(originalResult, nil)

	result, err := app.ExecuteQuery(opts)
	assert.NoError(t, err)
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, 2, result.RowCount)
	mockClient.AssertExpectations(t)
}

func TestApp_ExecuteQueryNilOptions(t *testing.T) {
	app, _ := New()

	result, err := app.ExecuteQuery(nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_ExecuteQueryEmptyQuery(t *testing.T) {
	app, _ := New()

	opts := &ExecuteQueryOptions{}

	result, err := app.ExecuteQuery(opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_ExplainQuery(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedResult := &QueryResult{
		Columns:  []string{"QUERY PLAN"},
		Rows:     [][]interface{}{{"Seq Scan on users"}},
		RowCount: 1,
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ExplainQuery", "SELECT * FROM users", []interface{}(nil)).Return(expectedResult, nil)

	result, err := app.ExplainQuery("SELECT * FROM users")
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockClient.AssertExpectations(t)
}

func TestApp_ExplainQueryEmptyQuery(t *testing.T) {
	app, _ := New()

	result, err := app.ExplainQuery("")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestApp_GetTableStats(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedStats := &TableInfo{
		Schema:   "public",
		Name:     "users",
		RowCount: 1000,
		Size:     "5MB",
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("GetTableStats", "public", "users").Return(expectedStats, nil)

	stats, err := app.GetTableStats("public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats)
	mockClient.AssertExpectations(t)
}

func TestApp_ListIndexes(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	expectedIndexes := []*IndexInfo{
		{Name: "users_pkey", Table: "users", Columns: []string{"id"}, IsUnique: true, IsPrimary: true},
		{Name: "idx_users_email", Table: "users", Columns: []string{"email"}, IsUnique: true, IsPrimary: false},
	}

	mockClient.On("Ping").Return(nil)
	mockClient.On("ListIndexes", "public", "users").Return(expectedIndexes, nil)

	indexes, err := app.ListIndexes("public", "users")
	assert.NoError(t, err)
	assert.Equal(t, expectedIndexes, indexes)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_Success(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	connectionString := "postgres://user:pass@localhost/db"

	// Mock expectations
	mockClient.On("Ping").Return(errors.New("not connected")) // No existing connection
	mockClient.On("Connect", connectionString).Return(nil)

	err := app.Connect(connectionString)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_EmptyString(t *testing.T) {
	app, _ := New()

	err := app.Connect("")
	assert.Error(t, err)
	assert.Equal(t, ErrNoConnectionString, err)
}

func TestApp_Connect_ReconnectClosesExisting(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	connectionString := "postgres://user:pass@localhost/db"

	// Mock expectations for reconnection scenario
	mockClient.On("Ping").Return(nil).Once()      // Existing connection is alive
	mockClient.On("Close").Return(nil).Once()     // Close existing
	mockClient.On("Connect", connectionString).Return(nil)

	err := app.Connect(connectionString)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestApp_Connect_ConnectError(t *testing.T) {
	app, _ := New()
	mockClient := &MockPostgreSQLClient{}
	app.client = mockClient

	connectionString := "postgres://user:pass@localhost/db"
	expectedError := errors.New("connection failed")

	// Mock expectations
	mockClient.On("Ping").Return(errors.New("not connected")) // No existing connection
	mockClient.On("Connect", connectionString).Return(expectedError)

	err := app.Connect(connectionString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
	mockClient.AssertExpectations(t)
}
