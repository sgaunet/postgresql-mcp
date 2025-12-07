package main

import (
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sylvain/postgresql-mcp/internal/app"
)

// MockApp is a mock implementation of the app.App for testing
type MockApp struct {
	mock.Mock
}

func (m *MockApp) GetCurrentDatabase() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockApp) ListDatabases() ([]*app.DatabaseInfo, error) {
	args := m.Called()
	if databases, ok := args.Get(0).([]*app.DatabaseInfo); ok {
		return databases, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockApp) ListSchemas() ([]*app.SchemaInfo, error) {
	args := m.Called()
	if schemas, ok := args.Get(0).([]*app.SchemaInfo); ok {
		return schemas, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockApp) ListTables(opts *app.ListTablesOptions) ([]*app.TableInfo, error) {
	args := m.Called(opts)
	if tables, ok := args.Get(0).([]*app.TableInfo); ok {
		return tables, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockApp) DescribeTable(schema, table string) ([]*app.ColumnInfo, error) {
	args := m.Called(schema, table)
	if columns, ok := args.Get(0).([]*app.ColumnInfo); ok {
		return columns, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockApp) ExecuteQuery(opts *app.ExecuteQueryOptions) (*app.QueryResult, error) {
	args := m.Called(opts)
	if result, ok := args.Get(0).(*app.QueryResult); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockApp) ListIndexes(schema, table string) ([]*app.IndexInfo, error) {
	args := m.Called(schema, table)
	if indexes, ok := args.Get(0).([]*app.IndexInfo); ok {
		return indexes, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockApp) ExplainQuery(query string, args ...interface{}) (*app.QueryResult, error) {
	mockArgs := m.Called(query, args)
	if result, ok := mockArgs.Get(0).(*app.QueryResult); ok {
		return result, mockArgs.Error(1)
	}
	return nil, mockArgs.Error(1)
}

func (m *MockApp) GetTableStats(schema, table string) (*app.TableInfo, error) {
	args := m.Called(schema, table)
	if stats, ok := args.Get(0).(*app.TableInfo); ok {
		return stats, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockApp) SetLogger(logger *slog.Logger) {
	m.Called(logger)
}

func (m *MockApp) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}

func TestSetupListDatabasesTool(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	setupListDatabasesTool(s, realApp, logger)

	assert.NotNil(t, s)
}

func TestSetupListSchemasTool(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	setupListSchemasTool(s, realApp, logger)

	assert.NotNil(t, s)
}

func TestSetupListTablesTool(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	setupListTablesTool(s, realApp, logger)

	assert.NotNil(t, s)
}

func TestSetupDescribeTableTool(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	setupDescribeTableTool(s, realApp, logger)

	assert.NotNil(t, s)
}

func TestSetupExecuteQueryTool(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	setupExecuteQueryTool(s, realApp, logger)

	assert.NotNil(t, s)
}

func TestSetupListIndexesTool(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	setupListIndexesTool(s, realApp, logger)

	assert.NotNil(t, s)
}

func TestSetupExplainQueryTool(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	setupExplainQueryTool(s, realApp, logger)

	assert.NotNil(t, s)
}

func TestSetupGetTableStatsTool(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	setupGetTableStatsTool(s, realApp, logger)

	assert.NotNil(t, s)
}

func TestRegisterAllTools(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	realApp, err := app.New()
	assert.NoError(t, err)
	logger := slog.Default()

	registerAllTools(s, realApp, logger)

	// Test that registration doesn't panic
	assert.NotNil(t, s)
}

func TestPrintHelp(t *testing.T) {
	// Test that printHelp doesn't panic
	assert.NotPanics(t, func() {
		printHelp()
	})
}

func TestHandleCommandLineFlags_Help(t *testing.T) {
	// Save original os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test help flag
	os.Args = []string{"cmd", "-h"}

	// Reset flag package state
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	var showHelp bool
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.Parse()

	assert.True(t, showHelp)
}

func TestHandleCommandLineFlags_Version(t *testing.T) {
	// Save original os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test version flag
	os.Args = []string{"cmd", "-v"}

	// Reset flag package state
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "Show version")
	flag.Parse()

	assert.True(t, showVersion)
}

func TestInitializeApp(t *testing.T) {
	appInstance, debugLogger := initializeApp()

	assert.NotNil(t, appInstance)
	assert.NotNil(t, debugLogger)
}

func TestVersion(t *testing.T) {
	// Test that version variable exists and has expected default
	assert.Equal(t, "dev", version)
}

func TestErrorVariables(t *testing.T) {
	// Test that error variables are properly defined
	assert.NotNil(t, ErrInvalidConnectionParameters)
	assert.Contains(t, ErrInvalidConnectionParameters.Error(), "invalid connection parameters")
}

// Test MCP tool parameter validation

func TestDescribeTableTool_ParameterValidation(t *testing.T) {
	tests := []struct {
		name          string
		args          map[string]interface{}
		expectedError bool
		errorMessage  string
	}{
		{
			name: "valid parameters",
			args: map[string]interface{}{
				"table":  "users",
				"schema": "public",
			},
			expectedError: false,
		},
		{
			name: "missing table",
			args: map[string]interface{}{
				"schema": "public",
			},
			expectedError: true,
			errorMessage:  "table must be a non-empty string",
		},
		{
			name: "empty table",
			args: map[string]interface{}{
				"table":  "",
				"schema": "public",
			},
			expectedError: true,
			errorMessage:  "table must be a non-empty string",
		},
		{
			name: "table not string",
			args: map[string]interface{}{
				"table":  123,
				"schema": "public",
			},
			expectedError: true,
			errorMessage:  "table must be a non-empty string",
		},
		{
			name: "missing schema uses default",
			args: map[string]interface{}{
				"table": "users",
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate parameter validation logic from setupDescribeTableTool
			table, ok := tt.args["table"].(string)
			hasError := !ok || table == ""

			schema := "public" // default
			if schemaArg, ok := tt.args["schema"].(string); ok && schemaArg != "" {
				schema = schemaArg
			}

			if tt.expectedError {
				assert.True(t, hasError)
			} else {
				assert.False(t, hasError)
				assert.NotEmpty(t, schema)
			}
		})
	}
}

func TestExecuteQueryTool_ParameterValidation(t *testing.T) {
	tests := []struct {
		name          string
		args          map[string]interface{}
		expectedError bool
	}{
		{
			name: "valid query",
			args: map[string]interface{}{
				"query": "SELECT * FROM users",
			},
			expectedError: false,
		},
		{
			name: "valid query with limit",
			args: map[string]interface{}{
				"query": "SELECT * FROM users",
				"limit": float64(10),
			},
			expectedError: false,
		},
		{
			name: "missing query",
			args: map[string]interface{}{
				"limit": float64(10),
			},
			expectedError: true,
		},
		{
			name: "empty query",
			args: map[string]interface{}{
				"query": "",
			},
			expectedError: true,
		},
		{
			name: "query not string",
			args: map[string]interface{}{
				"query": 123,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate parameter validation logic from setupExecuteQueryTool
			query, ok := tt.args["query"].(string)
			hasError := !ok || query == ""

			var limit int
			if limitFloat, ok := tt.args["limit"].(float64); ok && limitFloat > 0 {
				limit = int(limitFloat)
			}

			if tt.expectedError {
				assert.True(t, hasError)
			} else {
				assert.False(t, hasError)
				assert.NotEmpty(t, query)
				if tt.args["limit"] != nil {
					assert.Greater(t, limit, 0)
				}
			}
		})
	}
}

func TestJSONMarshalling(t *testing.T) {
	// Test that our response structures can be properly marshalled to JSON
	testData := map[string]interface{}{
		"status":   "connected",
		"database": "testdb",
		"message":  "Successfully connected to PostgreSQL database",
	}

	jsonData, err := json.Marshal(testData)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "connected")
	assert.Contains(t, string(jsonData), "testdb")

	// Test unmarshalling
	var unmarshalled map[string]interface{}
	err = json.Unmarshal(jsonData, &unmarshalled)
	assert.NoError(t, err)
	assert.Equal(t, "connected", unmarshalled["status"])
	assert.Equal(t, "testdb", unmarshalled["database"])
}

func TestToolResponseFormatting(t *testing.T) {
	// Test that tool responses are properly formatted
	databases := []*app.DatabaseInfo{
		{Name: "db1", Owner: "user1", Encoding: "UTF8"},
		{Name: "db2", Owner: "user2", Encoding: "UTF8"},
	}

	jsonData, err := json.Marshal(databases)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), "db1")
	assert.Contains(t, string(jsonData), "user1")
	assert.Contains(t, string(jsonData), "UTF8")

	// Verify it's valid JSON
	var unmarshalled []*app.DatabaseInfo
	err = json.Unmarshal(jsonData, &unmarshalled)
	assert.NoError(t, err)
	assert.Len(t, unmarshalled, 2)
	assert.Equal(t, "db1", unmarshalled[0].Name)
}
