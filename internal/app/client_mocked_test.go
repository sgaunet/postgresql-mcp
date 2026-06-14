package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
		_, err := client.ExplainQuery(context.Background(), "SELECT 1", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no database connection")
	})
}

// TestApp_SchemaDefaulting verifies the schema argument the App resolves before
// delegating to the PostgreSQLClient: an empty schema becomes the default
// ("public") while an explicit schema is preserved. Driving the table-scoped
// operations through the mock client lets us assert the *resolved* schema
// actually reaches the dependency for each case — the behavior the previous
// version claimed to cover but never reached, because it stopped at the
// "no database connection" guard before any defaulting ran.
func TestApp_SchemaDefaulting(t *testing.T) {
	const table = "users"

	cases := []struct {
		name           string
		inputSchema    string
		resolvedSchema string
	}{
		{"empty defaults to public", "", DefaultSchema},
		{"explicit schema preserved", "custom", "custom"},
		{"public schema preserved", "public", "public"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("DescribeTable", func(t *testing.T) {
				mc := &MockPostgreSQLClient{}
				mc.On("Ping", mock.Anything).Return(nil)
				mc.On("DescribeTable", mock.Anything, tc.resolvedSchema, table).
					Return([]*ColumnInfo{{Name: "id", DataType: "integer"}}, nil)

				_, err := New(mc).DescribeTable(context.Background(), tc.inputSchema, table)
				require.NoError(t, err)
				mc.AssertExpectations(t)
			})

			t.Run("GetTableStats", func(t *testing.T) {
				mc := &MockPostgreSQLClient{}
				mc.On("Ping", mock.Anything).Return(nil)
				mc.On("GetTableStats", mock.Anything, tc.resolvedSchema, table).
					Return(&TableInfo{Schema: tc.resolvedSchema, Name: table}, nil)

				_, err := New(mc).GetTableStats(context.Background(), tc.inputSchema, table)
				require.NoError(t, err)
				mc.AssertExpectations(t)
			})

			t.Run("ListIndexes", func(t *testing.T) {
				mc := &MockPostgreSQLClient{}
				mc.On("Ping", mock.Anything).Return(nil)
				mc.On("ListIndexes", mock.Anything, tc.resolvedSchema, table).
					Return([]*IndexInfo{{Name: "users_pkey", Table: table}}, nil)

				_, err := New(mc).ListIndexes(context.Background(), tc.inputSchema, table)
				require.NoError(t, err)
				mc.AssertExpectations(t)
			})
		})
	}
}
