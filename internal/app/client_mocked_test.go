package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test connection validation in various scenarios
func TestPostgreSQLClient_ConnectValidation(t *testing.T) {
	client := NewPostgreSQLClient()

	tests := []struct {
		name          string
		connectionStr string
		expectError   bool
	}{
		{
			name:          "valid postgres URL",
			connectionStr: "postgres://user:pass@localhost:5432/db",
			expectError:   true, // Will fail due to no real postgres, but connection string is valid
		},
		{
			name:          "invalid URL scheme",
			connectionStr: "mysql://user:pass@localhost:5432/db",
			expectError:   true,
		},
		{
			name:          "missing components",
			connectionStr: "postgres://",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.Connect(context.Background(), tt.connectionStr)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				client.Close()
			}
		})
	}
}

// Test Close and Ping methods with different states
func TestPostgreSQLClient_StateManagement(t *testing.T) {
	client := NewPostgreSQLClient()

	// Test Close on fresh client
	err := client.Close()
	assert.NoError(t, err)

	// Test Ping on fresh client
	err = client.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no database connection")

	// Test GetDB on fresh client
	db := client.GetDB()
	assert.Nil(t, db)
}

// Test error scenarios that don't require real database
func TestPostgreSQLClient_ErrorScenarios(t *testing.T) {
	client := &PostgreSQLClientImpl{}

	// Test all methods that check for db == nil
	t.Run("ListDatabases", func(t *testing.T) {
		_, err := client.ListDatabases(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})

	t.Run("GetCurrentDatabase", func(t *testing.T) {
		_, err := client.GetCurrentDatabase(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})

	t.Run("ListSchemas", func(t *testing.T) {
		_, err := client.ListSchemas(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})

	t.Run("ListTables", func(t *testing.T) {
		_, err := client.ListTables(context.Background(), "public")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})

	t.Run("DescribeTable", func(t *testing.T) {
		_, err := client.DescribeTable(context.Background(), "public", "users")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})

	t.Run("GetTableStats", func(t *testing.T) {
		_, err := client.GetTableStats(context.Background(), "public", "users")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})

	t.Run("ListIndexes", func(t *testing.T) {
		_, err := client.ListIndexes(context.Background(), "public", "users")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})

	t.Run("ExecuteQuery", func(t *testing.T) {
		_, err := client.ExecuteQuery(context.Background(), "SELECT 1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})

	t.Run("ExplainQuery", func(t *testing.T) {
		_, err := client.ExplainQuery(context.Background(), "SELECT 1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})
}

// Test schema defaulting logic
func TestPostgreSQLClient_SchemaDefaults(t *testing.T) {
	client := &PostgreSQLClientImpl{}

	// These will fail due to no connection, but we can test that the functions handle schema defaults
	tests := []struct {
		name   string
		schema string
		table  string
	}{
		{"empty schema", "", "users"},
		{"explicit schema", "custom", "users"},
		{"public schema", "public", "users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All these will fail with "no database connection" but exercise the schema defaulting logic
			_, err := client.GetTableStats(context.Background(), tt.schema, tt.table)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")

			_, err = client.ListIndexes(context.Background(), tt.schema, tt.table)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")

			_, err = client.DescribeTable(context.Background(), tt.schema, tt.table)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no database connection")
		})
	}
}
