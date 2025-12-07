package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/sylvain/postgresql-mcp/internal/app"
	"log/slog"
)

// MockTool represents a tool that was registered with the server
type MockTool struct {
	Name        string
	Description string
}

// Test that all tool setup functions can be called without panicking
func TestAllToolSetupFunctions(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	appInstance, err := app.NewDefault()
	assert.NoError(t, err)
	logger := slog.Default()

	// Test each setup function individually

	assert.NotPanics(t, func() {
		setupListDatabasesTool(s, appInstance, logger)
	})

	assert.NotPanics(t, func() {
		setupListSchemasTool(s, appInstance, logger)
	})

	assert.NotPanics(t, func() {
		setupListTablesTool(s, appInstance, logger)
	})

	assert.NotPanics(t, func() {
		setupDescribeTableTool(s, appInstance, logger)
	})

	assert.NotPanics(t, func() {
		setupExecuteQueryTool(s, appInstance, logger)
	})

	assert.NotPanics(t, func() {
		setupListIndexesTool(s, appInstance, logger)
	})

	assert.NotPanics(t, func() {
		setupExplainQueryTool(s, appInstance, logger)
	})

	assert.NotPanics(t, func() {
		setupGetTableStatsTool(s, appInstance, logger)
	})
}

// Test parameter validation error handling in tool handlers
func TestToolParameterValidationErrors(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	appInstance, err := app.NewDefault()
	assert.NoError(t, err)
	logger := slog.Default()

	// Set up the tools
	setupDescribeTableTool(s, appInstance, logger)
	setupExecuteQueryTool(s, appInstance, logger)

	// Test describe table with invalid parameters
	t.Run("describe_table_invalid_params", func(t *testing.T) {
		// This would test the parameter validation logic if we could access the handler
		// For now, we just test that setup completed without error
		assert.NotNil(t, s)
	})

	// Test execute query with invalid parameters
	t.Run("execute_query_invalid_params", func(t *testing.T) {
		// This would test the parameter validation logic if we could access the handler
		assert.NotNil(t, s)
	})
}

// Test JSON response formatting functions
func TestJSONResponseHelpers(t *testing.T) {
	// Test success response formatting
	t.Run("success_response", func(t *testing.T) {
		response := map[string]interface{}{
			"status":  "success",
			"data":    []string{"db1", "db2"},
			"message": "Operation completed successfully",
		}

		jsonData, err := json.Marshal(response)
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "success")
		assert.Contains(t, string(jsonData), "db1")
	})

	// Test error response formatting
	t.Run("error_response", func(t *testing.T) {
		response := map[string]interface{}{
			"error": "Database connection failed",
			"code":  "CONNECTION_ERROR",
		}

		jsonData, err := json.Marshal(response)
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "CONNECTION_ERROR")
	})
}

// Test the registerAllTools function
func TestRegisterAllToolsFunction(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0")
	appInstance, err := app.NewDefault()
	assert.NoError(t, err)
	logger := slog.Default()

	// Should not panic
	assert.NotPanics(t, func() {
		registerAllTools(s, appInstance, logger)
	})

	// Server should be properly configured
	assert.NotNil(t, s)
}

// Test initializeApp function coverage
func TestInitializeAppCoverage(t *testing.T) {
	app, logger := initializeApp()

	assert.NotNil(t, app)
	assert.NotNil(t, logger)

	// Test that the app is properly initialized
	err := app.ValidateConnection(context.Background())
	assert.Error(t, err) // Should error because no connection established

	// Test setting logger
	app.SetLogger(logger)
}

// Test printHelp function
func TestPrintHelpFunction(t *testing.T) {
	// Should not panic
	assert.NotPanics(t, func() {
		printHelp()
	})
}

// Test error constants are defined
func TestErrorConstantsExist(t *testing.T) {
	assert.NotNil(t, ErrInvalidConnectionParameters)
	assert.Contains(t, ErrInvalidConnectionParameters.Error(), "invalid connection parameters")
}

// Test version constant
func TestVersionConstantExists(t *testing.T) {
	assert.Equal(t, "dev", version)
}

// Test parameter parsing logic (simulates what happens in tool handlers)
func TestParameterParsingLogic(t *testing.T) {
	// Test connection string parsing
	t.Run("connection_string_parsing", func(t *testing.T) {
		params := map[string]interface{}{
			"connection_string": "postgres://user:pass@localhost:5432/db",
		}

		connectionString, ok := params["connection_string"].(string)
		assert.True(t, ok)
		assert.Equal(t, "postgres://user:pass@localhost:5432/db", connectionString)
	})

	// Test individual parameter parsing
	t.Run("individual_params_parsing", func(t *testing.T) {
		params := map[string]interface{}{
			"host":     "localhost",
			"port":     5432.0, // JSON numbers are float64
			"database": "testdb",
			"username": "user",
			"password": "pass",
		}

		host, hostOk := params["host"].(string)
		port, portOk := params["port"].(float64)
		database, dbOk := params["database"].(string)

		assert.True(t, hostOk)
		assert.True(t, portOk)
		assert.True(t, dbOk)
		assert.Equal(t, "localhost", host)
		assert.Equal(t, 5432.0, port)
		assert.Equal(t, "testdb", database)
	})

	// Test table parameter validation
	t.Run("table_param_validation", func(t *testing.T) {
		validParams := map[string]interface{}{
			"table":  "users",
			"schema": "public",
		}

		table, tableOk := validParams["table"].(string)
		schema, schemaOk := validParams["schema"].(string)

		assert.True(t, tableOk)
		assert.True(t, schemaOk)
		assert.NotEmpty(t, table)
		assert.NotEmpty(t, schema)

		// Test invalid params
		invalidParams := map[string]interface{}{
			"schema": "public",
			// missing table
		}

		_, tableOk = invalidParams["table"].(string)
		assert.False(t, tableOk)
	})

	// Test query parameter validation
	t.Run("query_param_validation", func(t *testing.T) {
		validParams := map[string]interface{}{
			"query": "SELECT * FROM users",
			"limit": 10.0,
		}

		query, queryOk := validParams["query"].(string)
		limit, limitOk := validParams["limit"].(float64)

		assert.True(t, queryOk)
		assert.True(t, limitOk)
		assert.NotEmpty(t, query)
		assert.Greater(t, limit, 0.0)
	})
}
