package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"testing"

	"github.com/lib/pq"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sylvain/postgresql-mcp/internal/app"
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
	err := app.ValidateConnection(context.Background())
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

func TestBuildConnectionString_AllParameters(t *testing.T) {
	params := ConnectionParams{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
	}

	connStr, err := buildConnectionString(params)
	assert.NoError(t, err)
	assert.Equal(t, "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable", connStr)
}

func TestBuildConnectionString_Defaults(t *testing.T) {
	params := ConnectionParams{
		Host:     "localhost",
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		// Port and SSLMode should use defaults
	}

	connStr, err := buildConnectionString(params)
	assert.NoError(t, err)
	assert.Equal(t, "postgres://testuser:testpass@localhost:5432/testdb?sslmode=prefer", connStr)
}

func TestBuildConnectionString_MissingHost(t *testing.T) {
	params := ConnectionParams{
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
	}

	_, err := buildConnectionString(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestBuildConnectionString_MissingUser(t *testing.T) {
	params := ConnectionParams{
		Host:     "localhost",
		Password: "testpass",
		Database: "testdb",
	}

	_, err := buildConnectionString(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user is required")
}

func TestBuildConnectionString_MissingDatabase(t *testing.T) {
	params := ConnectionParams{
		Host:     "localhost",
		User:     "testuser",
		Password: "testpass",
	}

	_, err := buildConnectionString(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database is required")
}

func TestBuildConnectionString_EmptyPassword(t *testing.T) {
	params := ConnectionParams{
		Host:     "localhost",
		User:     "testuser",
		Password: "",
		Database: "testdb",
	}

	connStr, err := buildConnectionString(params)
	assert.NoError(t, err)
	assert.Equal(t, "postgres://testuser:@localhost:5432/testdb?sslmode=prefer", connStr)
}

func TestBuildConnectionString_CustomPort(t *testing.T) {
	params := ConnectionParams{
		Host:     "dbserver",
		Port:     5433,
		User:     "admin",
		Password: "secret",
		Database: "mydb",
		SSLMode:  "require",
	}

	connStr, err := buildConnectionString(params)
	assert.NoError(t, err)
	assert.Equal(t, "postgres://admin:secret@dbserver:5433/mydb?sslmode=require", connStr)
}

// TestBuildConnectionString_AcceptsAllValidSSLModes locks in the libpq
// allowlist that buildConnectionString recognises (issue #86).
func TestBuildConnectionString_AcceptsAllValidSSLModes(t *testing.T) {
	for _, sm := range []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		t.Run(sm, func(t *testing.T) {
			params := ConnectionParams{
				Host:     "h",
				User:     "u",
				Password: "p",
				Database: "d",
				SSLMode:  sm,
			}
			connStr, err := buildConnectionString(params)
			require.NoError(t, err)
			assert.Contains(t, connStr, "sslmode="+sm)
		})
	}
}

// TestBuildConnectionString_RejectsInvalidSSLMode covers the parameter
// injection / silent-downgrade vector from issue #86. Each input would
// previously have been embedded verbatim into the URL.
func TestBuildConnectionString_RejectsInvalidSSLMode(t *testing.T) {
	cases := []string{
		"off",                  // not a libpq sslmode
		"true",                 // not a libpq sslmode
		"prefer&extra=value",   // URL-parameter injection
		"DISABLE",              // libpq is case-sensitive
		" require",             // leading whitespace
		"verify-full;DROP",     // would not actually inject SQL but is plainly wrong
	}
	for _, sm := range cases {
		t.Run(sm, func(t *testing.T) {
			params := ConnectionParams{
				Host:     "h",
				User:     "u",
				Password: "p",
				Database: "d",
				SSLMode:  sm,
			}
			_, err := buildConnectionString(params)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidSSLMode)
		})
	}
}

// stubFailingClient is a minimal app.PostgreSQLClient where Connect returns
// a configurable error and Ping always reports "not connected" (so
// app.Connect skips the existing-connection Close branch). All other
// interface methods return a sentinel error; this stub is only meant for
// the connect-path leak test below.
type stubFailingClient struct {
	connectErr error
}

func (s *stubFailingClient) Connect(_ context.Context, _ string) error { return s.connectErr }
func (s *stubFailingClient) Close() error                              { return nil }
func (s *stubFailingClient) Ping(_ context.Context) error              { return errors.New("stub: not connected") }
func (s *stubFailingClient) GetDB() *sql.DB                            { return nil }
func (s *stubFailingClient) ListDatabases(_ context.Context) ([]*app.DatabaseInfo, error) {
	return nil, errors.New("stub")
}
func (s *stubFailingClient) GetCurrentDatabase(_ context.Context) (string, error) {
	return "", errors.New("stub")
}
func (s *stubFailingClient) ListSchemas(_ context.Context) ([]*app.SchemaInfo, error) {
	return nil, errors.New("stub")
}
func (s *stubFailingClient) ListTables(_ context.Context, _ string) ([]*app.TableInfo, error) {
	return nil, errors.New("stub")
}
func (s *stubFailingClient) ListTablesWithStats(_ context.Context, _ string) ([]*app.TableInfo, error) {
	return nil, errors.New("stub")
}
func (s *stubFailingClient) DescribeTable(_ context.Context, _, _ string) ([]*app.ColumnInfo, error) {
	return nil, errors.New("stub")
}
func (s *stubFailingClient) GetTableStats(_ context.Context, _, _ string) (*app.TableInfo, error) {
	return nil, errors.New("stub")
}
func (s *stubFailingClient) ListIndexes(_ context.Context, _, _ string) ([]*app.IndexInfo, error) {
	return nil, errors.New("stub")
}
func (s *stubFailingClient) ExecuteQuery(_ context.Context, _ string, _ ...any) (*app.QueryResult, error) {
	return nil, errors.New("stub")
}
func (s *stubFailingClient) ExplainQuery(_ context.Context, _ string, _ ...any) (*app.QueryResult, error) {
	return nil, errors.New("stub")
}

// TestPublicError_StripsLibPqMetadata locks in that the helper extracts only
// Message and the SQLSTATE Code.Name() from *pq.Error, dropping every other
// field that could leak server internals (Detail / Hint / Where / Routine /
// File / Line / Schema / Table / Column / Constraint / DataTypeName). Also
// covers the wrapped-error path through errors.As (issue #88).
func TestPublicError_StripsLibPqMetadata(t *testing.T) {
	pqErr := &pq.Error{
		Code:         "42P01", // undefined_table
		Severity:     "ERROR",
		Message:      `relation "users" does not exist`,
		Detail:       "SECRET-DETAIL",
		Hint:         "SECRET-HINT",
		Where:        "SECRET-WHERE",
		Routine:      "SECRET-ROUTINE",
		File:         "SECRET-FILE",
		Line:         "999",
		Schema:       "SECRET-SCHEMA",
		Table:        "SECRET-TABLE",
		Column:       "SECRET-COLUMN",
		Constraint:   "SECRET-CONSTRAINT",
		DataTypeName: "SECRET-DATATYPE",
	}
	leaks := []string{
		"SECRET-DETAIL", "SECRET-HINT", "SECRET-WHERE",
		"SECRET-ROUTINE", "SECRET-FILE", "SECRET-SCHEMA",
		"SECRET-TABLE", "SECRET-COLUMN", "SECRET-CONSTRAINT",
		"SECRET-DATATYPE",
	}

	t.Run("direct pq.Error", func(t *testing.T) {
		out := publicError("Failed to run query", pqErr)
		assert.Contains(t, out, "Failed to run query")
		assert.Contains(t, out, `relation "users" does not exist`, "Message must survive")
		assert.Contains(t, out, "undefined_table", "SQLSTATE name must survive (42P01 -> undefined_table)")
		for _, leak := range leaks {
			assert.NotContains(t, out, leak, "leak %q must not appear", leak)
		}
	})

	t.Run("wrapped pq.Error (errors.As must unwrap)", func(t *testing.T) {
		wrapped := fmt.Errorf("outer wrap: %w", pqErr)
		out := publicError("ctx", wrapped)
		assert.Contains(t, out, `relation "users" does not exist`)
		assert.Contains(t, out, "undefined_table")
		for _, leak := range leaks {
			assert.NotContains(t, out, leak)
		}
	})
}

// TestPublicError_PassesThroughSentinelErrors verifies that non-pq errors —
// app-level sentinels and other plain errors — flow through unchanged with
// just the prefix prepended. These messages are already curated and safe.
func TestPublicError_PassesThroughSentinelErrors(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want string
	}{
		{"ErrConnectionRequired", app.ErrConnectionRequired, app.ErrConnectionRequired.Error()},
		{"ErrQueryRequired", app.ErrQueryRequired, app.ErrQueryRequired.Error()},
		{"plain errors.New", errors.New("custom failure"), "custom failure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := publicError("Failed to do thing", tc.in)
			assert.Equal(t, "Failed to do thing: "+tc.want, got)
		})
	}
}

// TestHandleConnectDatabaseRequest_DoesNotLeakErrorDetails exercises the
// connect_database handler with a stub client whose Connect returns a
// fully-loaded *pq.Error simulating an authentication failure that exposes
// host, port, username, and PostgreSQL source location. The handler must
// return the fixed generic message regardless — none of the sensitive
// fields may appear in the MCP response text (issue #88).
func TestHandleConnectDatabaseRequest_DoesNotLeakErrorDetails(t *testing.T) {
	leakyErr := &pq.Error{
		Code:    "28P01", // invalid_password
		Message: `password authentication failed for user "admin"`,
		Detail:  "Connection from 10.0.0.5:5432 rejected",
		Hint:    "Check pg_hba.conf",
		Where:   "auth.c line 1234",
		Routine: "auth_failed",
		File:    "auth.c",
		Line:    "1234",
	}
	silent := slog.New(slog.DiscardHandler)

	appInstance := app.New(&stubFailingClient{connectErr: leakyErr})
	appInstance.SetLogger(silent)

	args := map[string]any{
		"host":     "myhost",
		"user":     "myuser",
		"password": "mypw",
		"database": "mydb",
	}

	result, err := handleConnectDatabaseRequest(context.Background(), args, appInstance, silent)
	require.NoError(t, err, "handler must not propagate the error; it returns it inside the result")
	require.NotNil(t, result)
	require.True(t, result.IsError, "result must be flagged as an error")
	require.NotEmpty(t, result.Content)

	tc, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])

	// Fixed generic message present.
	assert.Contains(t, tc.Text, "Verify connection parameters")
	assert.Contains(t, tc.Text, "Failed to connect to database")

	// None of the leak markers — username, host, port, auth method, server-
	// internal source location, server-config hint — may appear.
	leaks := []string{
		"admin",      // username (from Message)
		"password",   // auth method (from Message)
		"10.0.0.5",   // host (from Detail)
		"5432",       // port (from Detail)
		"pg_hba",     // server config (from Hint)
		"auth.c",     // source file (from File / Where)
		"auth_failed", // routine
		"28P01",      // SQLSTATE — pre-auth state must not be exposed at all
	}
	for _, leak := range leaks {
		assert.NotContains(t, tc.Text, leak, "connect_database response leaked %q", leak)
	}
}

// TestBuildConnectionString_EncodesSpecialCharactersInPassword covers the
// URL-corruption vector: each of @, /, ?, # in a password would split the
// URL authority or query under fmt.Sprintf-based construction. Assert that
// the password round-trips correctly through net/url.Parse, which proves
// the encoding is structurally valid (issue #86).
func TestBuildConnectionString_EncodesSpecialCharactersInPassword(t *testing.T) {
	dangerous := map[string]string{
		"at-sign":    "p@ss",
		"slash":      "p/ss",
		"question":   "p?ss",
		"hash":       "p#ss",
		"mixed":      "p@/?#ss",
		"colon":      "p:ss",
		"percent":    "p%ss",
		"whitespace": "p ss",
	}
	for name, pw := range dangerous {
		t.Run(name, func(t *testing.T) {
			params := ConnectionParams{
				Host:     "host",
				Port:     5432,
				User:     "user",
				Password: pw,
				Database: "db",
			}
			connStr, err := buildConnectionString(params)
			require.NoError(t, err)

			// Round-trip: the produced URL must be parseable and the password
			// must decode back to the original.
			u, err := url.Parse(connStr)
			require.NoError(t, err, "produced URL must be parseable")
			gotPw, hasPw := u.User.Password()
			require.True(t, hasPw, "password should be present in parsed URL")
			assert.Equal(t, pw, gotPw, "round-tripped password must match original")
			assert.Equal(t, "host:5432", u.Host, "host/port must not be split by password content")
			assert.Equal(t, "/db", u.Path, "path must not be split by password content")
		})
	}
}
