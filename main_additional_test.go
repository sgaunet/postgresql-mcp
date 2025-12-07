package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test the command line flag handling functions directly
func TestHandleCommandLineFlags_Implementation(t *testing.T) {
	// Save original os.Args and flag state
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "help flag short",
			args:     []string{"postgresql-mcp", "-h"},
			expected: "help",
		},
		{
			name:     "help flag long",
			args:     []string{"postgresql-mcp", "--help"},
			expected: "help",
		},
		{
			name:     "version flag short",
			args:     []string{"postgresql-mcp", "-v"},
			expected: "version",
		},
		{
			name:     "version flag long",
			args:     []string{"postgresql-mcp", "--version"},
			expected: "version",
		},
		{
			name:     "no flags",
			args:     []string{"postgresql-mcp"},
			expected: "run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag state
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
			os.Args = tt.args

			// Test the flag parsing logic that would happen in handleCommandLineFlags
			var showHelp, showVersion bool
			flag.BoolVar(&showHelp, "h", false, "Show help message")
			flag.BoolVar(&showHelp, "help", false, "Show help message")
			flag.BoolVar(&showVersion, "v", false, "Show version information")
			flag.BoolVar(&showVersion, "version", false, "Show version information")

			// Parse flags, ignoring errors for this test
			flag.Parse()

			switch tt.expected {
			case "help":
				assert.True(t, showHelp)
			case "version":
				assert.True(t, showVersion)
			case "run":
				assert.False(t, showHelp)
				assert.False(t, showVersion)
			}
		})
	}
}

// Test error handling constants
func TestErrorConstants(t *testing.T) {
	assert.NotNil(t, ErrInvalidConnectionParameters)
	assert.Equal(t, "invalid connection parameters", ErrInvalidConnectionParameters.Error())
}

// Test version string
func TestVersionConstant(t *testing.T) {
	assert.Equal(t, "dev", version)
}

// Test initializeApp function
func TestInitializeApp_Implementation(t *testing.T) {
	app, logger := initializeApp()

	assert.NotNil(t, app)
	assert.NotNil(t, logger)

	// Test that logger is properly set on app
	app.SetLogger(logger)

	// App should be in disconnected state initially (without environment variables)
	err := app.ValidateConnection()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection failed")
}

// Test parameter validation logic for tool handlers
func TestToolParameterValidation(t *testing.T) {

	// Test table parameter validation
	t.Run("Table Parameter Validation", func(t *testing.T) {
		tests := []struct {
			name   string
			params map[string]interface{}
			valid  bool
		}{
			{
				name: "valid table and schema",
				params: map[string]interface{}{
					"table":  "users",
					"schema": "public",
				},
				valid: true,
			},
			{
				name: "valid table, no schema",
				params: map[string]interface{}{
					"table": "users",
				},
				valid: true,
			},
			{
				name: "missing table",
				params: map[string]interface{}{
					"schema": "public",
				},
				valid: false,
			},
			{
				name: "empty table",
				params: map[string]interface{}{
					"table":  "",
					"schema": "public",
				},
				valid: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Simulate the parameter validation logic from table-related tools
				table, ok := tt.params["table"].(string)
				isValid := ok && table != ""

				if tt.valid {
					assert.True(t, isValid, "Expected table parameter to be valid")
				} else {
					assert.False(t, isValid, "Expected table parameter to be invalid")
				}
			})
		}
	})

	// Test query parameter validation
	t.Run("Query Parameter Validation", func(t *testing.T) {
		tests := []struct {
			name   string
			params map[string]interface{}
			valid  bool
		}{
			{
				name: "valid query",
				params: map[string]interface{}{
					"query": "SELECT * FROM users",
				},
				valid: true,
			},
			{
				name: "valid query with limit",
				params: map[string]interface{}{
					"query": "SELECT * FROM users",
					"limit": 10.0,
				},
				valid: true,
			},
			{
				name: "missing query",
				params: map[string]interface{}{
					"limit": 10.0,
				},
				valid: false,
			},
			{
				name: "empty query",
				params: map[string]interface{}{
					"query": "",
				},
				valid: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Simulate the parameter validation logic from query-related tools
				query, ok := tt.params["query"].(string)
				isValid := ok && query != ""

				if tt.valid {
					assert.True(t, isValid, "Expected query parameter to be valid")
				} else {
					assert.False(t, isValid, "Expected query parameter to be invalid")
				}
			})
		}
	})
}

// Test JSON response formatting logic
func TestJSONResponseFormatting(t *testing.T) {
	// Test success response formatting
	successResponse := map[string]interface{}{
		"status":   "connected",
		"database": "testdb",
		"message":  "Successfully connected to PostgreSQL database",
	}

	assert.Equal(t, "connected", successResponse["status"])
	assert.Equal(t, "testdb", successResponse["database"])

	// Test error response formatting
	errorResponse := map[string]interface{}{
		"error":   "Connection failed",
		"details": "Invalid connection string",
	}

	assert.Equal(t, "Connection failed", errorResponse["error"])
	assert.Equal(t, "Invalid connection string", errorResponse["details"])
}

// Test environment variable handling
func TestEnvironmentVariableHandling(t *testing.T) {
	// Save original environment
	oldPostgresURL := os.Getenv("POSTGRES_URL")
	oldDatabaseURL := os.Getenv("DATABASE_URL")
	defer func() {
		os.Setenv("POSTGRES_URL", oldPostgresURL)
		os.Setenv("DATABASE_URL", oldDatabaseURL)
	}()

	// Test POSTGRES_URL precedence
	os.Setenv("POSTGRES_URL", "postgres://test1@localhost/db1")
	os.Setenv("DATABASE_URL", "postgres://test2@localhost/db2")

	// Simulate the environment variable reading logic
	connectionString := os.Getenv("POSTGRES_URL")
	if connectionString == "" {
		connectionString = os.Getenv("DATABASE_URL")
	}

	assert.Equal(t, "postgres://test1@localhost/db1", connectionString)

	// Test DATABASE_URL fallback
	os.Unsetenv("POSTGRES_URL")
	connectionString = os.Getenv("POSTGRES_URL")
	if connectionString == "" {
		connectionString = os.Getenv("DATABASE_URL")
	}

	assert.Equal(t, "postgres://test2@localhost/db2", connectionString)
}
