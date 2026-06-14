package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sylvain/postgresql-mcp/internal/app"
)

// fakeClient is an in-memory app.PostgreSQLClient used to drive the MCP tool
// handlers end-to-end. GetDB returns a non-nil pool so App.ensureConnection
// takes its fast path (no proactive ping — issue #93); the data methods return
// the configured canned values (or opErr
// when set, to exercise error paths) and record the schema/table/query/analyze
// arguments they receive. That lets each test assert both the observable
// CallToolResult and that the handler/App resolved its defaults (e.g. schema
// "public") before reaching the client.
type fakeClient struct {
	// Canned return values for the happy paths.
	databases []*app.DatabaseInfo
	schemas   []*app.SchemaInfo
	tables    []*app.TableInfo
	columns   []*app.ColumnInfo
	indexes   []*app.IndexInfo
	stats     *app.TableInfo
	queryRes  *app.QueryResult
	currentDB string

	// opErr, when non-nil, is returned by every data method to drive error paths.
	opErr error

	// Captured arguments (spy).
	gotSchema  string
	gotTable   string
	gotQuery   string
	gotAnalyze bool
}

// fakeDBHandle is a non-nil, never-connected *sql.DB so App.ensureConnection's
// GetDB() gate takes its fast path without a real connection (issue #93).
var fakeDBHandle, _ = sql.Open("postgres", "")

func (f *fakeClient) Connect(_ context.Context, _ string) error { return nil }
func (f *fakeClient) Close() error                              { return nil }
func (f *fakeClient) Ping(_ context.Context) error              { return nil }
func (f *fakeClient) GetDB() *sql.DB                            { return fakeDBHandle }

func (f *fakeClient) ListDatabases(_ context.Context) ([]*app.DatabaseInfo, error) {
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.databases, nil
}

func (f *fakeClient) GetCurrentDatabase(_ context.Context) (string, error) {
	if f.opErr != nil {
		return "", f.opErr
	}
	return f.currentDB, nil
}

func (f *fakeClient) ListSchemas(_ context.Context) ([]*app.SchemaInfo, error) {
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.schemas, nil
}

func (f *fakeClient) ListTables(_ context.Context, schema string) ([]*app.TableInfo, error) {
	f.gotSchema = schema
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.tables, nil
}

func (f *fakeClient) ListTablesWithStats(_ context.Context, schema string) ([]*app.TableInfo, error) {
	f.gotSchema = schema
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.tables, nil
}

func (f *fakeClient) DescribeTable(_ context.Context, schema, table string) ([]*app.ColumnInfo, error) {
	f.gotSchema, f.gotTable = schema, table
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.columns, nil
}

func (f *fakeClient) GetTableStats(_ context.Context, schema, table string) (*app.TableInfo, error) {
	f.gotSchema, f.gotTable = schema, table
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.stats, nil
}

func (f *fakeClient) ListIndexes(_ context.Context, schema, table string) ([]*app.IndexInfo, error) {
	f.gotSchema, f.gotTable = schema, table
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.indexes, nil
}

func (f *fakeClient) ExecuteQuery(_ context.Context, query string, _ ...any) (*app.QueryResult, error) {
	f.gotQuery = query
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.queryRes, nil
}

func (f *fakeClient) ExplainQuery(_ context.Context, query string, analyze bool, _ ...any) (*app.QueryResult, error) {
	f.gotQuery, f.gotAnalyze = query, analyze
	if f.opErr != nil {
		return nil, f.opErr
	}
	return f.queryRes, nil
}

// newToolServer wires the fake client through a real App and registers every
// MCP tool on a fresh server, so tests invoke the production handlers.
func newToolServer(client app.PostgreSQLClient) *server.MCPServer {
	silent := slog.New(slog.DiscardHandler)
	s := server.NewMCPServer("test", "1.0.0")
	appInstance := app.New(client)
	appInstance.SetLogger(silent)
	registerAllTools(s, appInstance, silent)
	return s
}

// callTool invokes a registered tool's handler with the given arguments and
// returns its CallToolResult. A non-nil err here is a transport-level failure;
// tool-level errors are reported inside result.IsError instead.
func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	st := s.GetTool(name)
	require.NotNil(t, st, "tool %q is not registered", name)

	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	result, err := st.Handler(context.Background(), req)
	require.NoError(t, err, "handler returned a transport error; tool errors belong in the result")
	require.NotNil(t, result)
	return result
}

// resultText returns the first text content block of a tool result.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content)
	tc, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])
	return tc.Text
}

func TestRegisterAllTools_RegistersExpectedTools(t *testing.T) {
	s := newToolServer(&fakeClient{})

	want := []string{
		"connect_database", "list_databases", "list_schemas", "list_tables",
		"describe_table", "execute_query", "list_indexes", "explain_query", "get_table_stats",
	}

	tools := s.ListTools()
	assert.Len(t, tools, len(want), "exactly the expected tools must be registered")
	for _, name := range want {
		assert.Contains(t, tools, name, "tool %q must be registered", name)
	}
}

func TestConnectDatabaseTool(t *testing.T) {
	// Happy path. The error path (no sensitive-detail leakage) is covered by
	// TestHandleConnectDatabaseRequest_DoesNotLeakErrorDetails.
	s := newToolServer(&fakeClient{currentDB: "appdb"})

	res := callTool(t, s, "connect_database", map[string]any{
		"host":     "localhost",
		"user":     "u",
		"password": "p",
		"database": "appdb",
	})

	require.False(t, res.IsError)
	txt := resultText(t, res)
	assert.Contains(t, txt, "connected")
	assert.Contains(t, txt, "appdb")
}

func TestListDatabasesTool(t *testing.T) {
	t.Run("happy path lists databases", func(t *testing.T) {
		s := newToolServer(&fakeClient{
			databases: []*app.DatabaseInfo{{Name: "appdb", Owner: "u", Encoding: "UTF8"}},
		})
		res := callTool(t, s, "list_databases", map[string]any{})
		require.False(t, res.IsError)
		assert.Contains(t, resultText(t, res), "appdb")
	})

	t.Run("error path surfaces operation failure", func(t *testing.T) {
		s := newToolServer(&fakeClient{opErr: errors.New("boom")})
		res := callTool(t, s, "list_databases", map[string]any{})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), "boom")
	})
}

func TestListSchemasTool(t *testing.T) {
	t.Run("happy path lists schemas", func(t *testing.T) {
		s := newToolServer(&fakeClient{
			schemas: []*app.SchemaInfo{{Name: "public", Owner: "u"}},
		})
		res := callTool(t, s, "list_schemas", map[string]any{})
		require.False(t, res.IsError)
		assert.Contains(t, resultText(t, res), "public")
	})

	t.Run("error path surfaces operation failure", func(t *testing.T) {
		s := newToolServer(&fakeClient{opErr: errors.New("boom")})
		res := callTool(t, s, "list_schemas", map[string]any{})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), "boom")
	})
}

func TestListTablesTool(t *testing.T) {
	t.Run("defaults schema to public when omitted", func(t *testing.T) {
		f := &fakeClient{tables: []*app.TableInfo{{Schema: "public", Name: "users", Type: "table", Owner: "u"}}}
		s := newToolServer(f)
		res := callTool(t, s, "list_tables", map[string]any{})
		require.False(t, res.IsError)
		assert.Contains(t, resultText(t, res), "users")
		assert.Equal(t, "public", f.gotSchema, "schema must default to public when omitted")
	})

	t.Run("preserves explicit schema", func(t *testing.T) {
		f := &fakeClient{tables: []*app.TableInfo{{Schema: "custom", Name: "t", Type: "table", Owner: "u"}}}
		s := newToolServer(f)
		res := callTool(t, s, "list_tables", map[string]any{"schema": "custom"})
		require.False(t, res.IsError)
		assert.Equal(t, "custom", f.gotSchema)
	})

	t.Run("error path surfaces operation failure", func(t *testing.T) {
		s := newToolServer(&fakeClient{opErr: errors.New("boom")})
		res := callTool(t, s, "list_tables", map[string]any{})
		require.True(t, res.IsError)
	})
}

func TestDescribeTableTool(t *testing.T) {
	t.Run("happy path describes columns and defaults schema", func(t *testing.T) {
		f := &fakeClient{columns: []*app.ColumnInfo{{Name: "id", DataType: "integer"}}}
		s := newToolServer(f)
		res := callTool(t, s, "describe_table", map[string]any{"table": "users"})
		require.False(t, res.IsError)
		txt := resultText(t, res)
		assert.Contains(t, txt, "id")
		assert.Contains(t, txt, "integer")
		assert.Equal(t, "public", f.gotSchema)
		assert.Equal(t, "users", f.gotTable)
	})

	t.Run("preserves explicit schema", func(t *testing.T) {
		f := &fakeClient{columns: []*app.ColumnInfo{{Name: "id", DataType: "integer"}}}
		s := newToolServer(f)
		res := callTool(t, s, "describe_table", map[string]any{"table": "users", "schema": "custom"})
		require.False(t, res.IsError)
		assert.Equal(t, "custom", f.gotSchema)
	})

	t.Run("missing table returns error result", func(t *testing.T) {
		s := newToolServer(&fakeClient{})
		res := callTool(t, s, "describe_table", map[string]any{})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), app.ErrTableRequired.Error())
	})
}

func TestExecuteQueryTool(t *testing.T) {
	t.Run("happy path returns rows", func(t *testing.T) {
		f := &fakeClient{queryRes: &app.QueryResult{Columns: []string{"n"}, Rows: [][]any{{1}}, RowCount: 1}}
		s := newToolServer(f)
		res := callTool(t, s, "execute_query", map[string]any{"query": "SELECT 1"})
		require.False(t, res.IsError)
		txt := resultText(t, res)
		assert.Contains(t, txt, "row_count")
		assert.Contains(t, txt, `"n"`)
	})

	t.Run("limit is accepted", func(t *testing.T) {
		f := &fakeClient{queryRes: &app.QueryResult{Columns: []string{"n"}, Rows: [][]any{{1}}, RowCount: 1}}
		s := newToolServer(f)
		res := callTool(t, s, "execute_query", map[string]any{"query": "SELECT 1", "limit": float64(10)})
		require.False(t, res.IsError)
	})

	t.Run("empty query returns error result", func(t *testing.T) {
		s := newToolServer(&fakeClient{})
		res := callTool(t, s, "execute_query", map[string]any{"query": ""})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), "query must be a non-empty string")
	})

	t.Run("validation rejection surfaces as error result", func(t *testing.T) {
		// Read-only validation lives in the client layer; simulate it rejecting a
		// non-SELECT statement and assert the handler surfaces that as an error.
		s := newToolServer(&fakeClient{opErr: app.ErrInvalidQuery})
		res := callTool(t, s, "execute_query", map[string]any{"query": "DELETE FROM users"})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), app.ErrInvalidQuery.Error())
	})
}

func TestExplainQueryTool(t *testing.T) {
	t.Run("happy path returns plan without analyze", func(t *testing.T) {
		f := &fakeClient{queryRes: &app.QueryResult{Columns: []string{"QUERY PLAN"}, Rows: [][]any{{"Seq Scan"}}, RowCount: 1}}
		s := newToolServer(f)
		res := callTool(t, s, "explain_query", map[string]any{"query": "SELECT 1"})
		require.False(t, res.IsError)
		assert.Contains(t, resultText(t, res), "QUERY PLAN")
		assert.False(t, f.gotAnalyze, "analyze must default to false")
	})

	t.Run("analyze flag is forwarded", func(t *testing.T) {
		f := &fakeClient{queryRes: &app.QueryResult{}}
		s := newToolServer(f)
		res := callTool(t, s, "explain_query", map[string]any{"query": "SELECT 1", "analyze": true})
		require.False(t, res.IsError)
		assert.True(t, f.gotAnalyze)
	})

	t.Run("empty query returns error result", func(t *testing.T) {
		s := newToolServer(&fakeClient{})
		res := callTool(t, s, "explain_query", map[string]any{"query": ""})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), "query must be a non-empty string")
	})
}

func TestListIndexesTool(t *testing.T) {
	t.Run("happy path lists indexes and defaults schema", func(t *testing.T) {
		f := &fakeClient{indexes: []*app.IndexInfo{
			{Name: "users_pkey", Table: "users", Columns: []string{"id"}, IsPrimary: true, IndexType: "btree"},
		}}
		s := newToolServer(f)
		res := callTool(t, s, "list_indexes", map[string]any{"table": "users"})
		require.False(t, res.IsError)
		assert.Contains(t, resultText(t, res), "users_pkey")
		assert.Equal(t, "public", f.gotSchema)
		assert.Equal(t, "users", f.gotTable)
	})

	t.Run("missing table returns error result", func(t *testing.T) {
		s := newToolServer(&fakeClient{})
		res := callTool(t, s, "list_indexes", map[string]any{})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), app.ErrTableRequired.Error())
	})
}

func TestGetTableStatsTool(t *testing.T) {
	t.Run("happy path returns stats and defaults schema", func(t *testing.T) {
		f := &fakeClient{stats: &app.TableInfo{Schema: "public", Name: "users", Type: "table", Owner: "u", RowCount: 42}}
		s := newToolServer(f)
		res := callTool(t, s, "get_table_stats", map[string]any{"table": "users"})
		require.False(t, res.IsError)
		txt := resultText(t, res)
		assert.Contains(t, txt, "users")
		assert.Contains(t, txt, "42")
		assert.Equal(t, "public", f.gotSchema)
		assert.Equal(t, "users", f.gotTable)
	})

	t.Run("missing table returns error result", func(t *testing.T) {
		s := newToolServer(&fakeClient{})
		res := callTool(t, s, "get_table_stats", map[string]any{})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), app.ErrTableRequired.Error())
	})
}
