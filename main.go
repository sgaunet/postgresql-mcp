package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sylvain/postgresql-mcp/internal/app"
	"github.com/sylvain/postgresql-mcp/internal/logger"
)

// Version information injected at build time.
var version = "dev"

// Error variables for static errors.
var (
	ErrInvalidConnectionParameters = errors.New("invalid connection parameters")
)


// setupListDatabasesTool creates and registers the list_databases tool.
func setupListDatabasesTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	listDBTool := mcp.NewTool("list_databases",
		mcp.WithDescription("List all databases on the PostgreSQL server"),
	)

	s.AddTool(listDBTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		debugLogger.Debug("Received list_databases tool request")

		// List databases
		databases, err := appInstance.ListDatabases()
		if err != nil {
			debugLogger.Error("Failed to list databases", "error", err)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list databases: %v", err)), nil
		}

		// Convert to JSON
		jsonData, err := json.Marshal(databases)
		if err != nil {
			debugLogger.Error("Failed to marshal databases to JSON", "error", err)
			return mcp.NewToolResultError("Failed to format databases response"), nil
		}

		debugLogger.Info("Successfully listed databases", "count", len(databases))
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

// setupListSchemasTool creates and registers the list_schemas tool.
func setupListSchemasTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	listSchemasTool := mcp.NewTool("list_schemas",
		mcp.WithDescription("List all schemas in the current database"),
	)

	s.AddTool(listSchemasTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		debugLogger.Debug("Received list_schemas tool request")

		// List schemas
		schemas, err := appInstance.ListSchemas()
		if err != nil {
			debugLogger.Error("Failed to list schemas", "error", err)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list schemas: %v", err)), nil
		}

		// Convert to JSON
		jsonData, err := json.Marshal(schemas)
		if err != nil {
			debugLogger.Error("Failed to marshal schemas to JSON", "error", err)
			return mcp.NewToolResultError("Failed to format schemas response"), nil
		}

		debugLogger.Info("Successfully listed schemas", "count", len(schemas))
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

// setupListTablesTool creates and registers the list_tables tool.
func setupListTablesTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	listTablesTool := mcp.NewTool("list_tables",
		mcp.WithDescription("List tables in a specific schema"),
		mcp.WithString("schema",
			mcp.Description("Schema name to list tables from (default: public)"),
		),
		mcp.WithBoolean("include_size",
			mcp.Description("Include table size and row count information (default: false)"),
		),
	)

	s.AddTool(listTablesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		debugLogger.Debug("Received list_tables tool request", "args", args)

		// Extract options
		opts := &app.ListTablesOptions{}

		if schema, ok := args["schema"].(string); ok && schema != "" {
			opts.Schema = schema
		}

		if includeSize, ok := args["include_size"].(bool); ok {
			opts.IncludeSize = includeSize
		}

		debugLogger.Debug("Processing list_tables request", "schema", opts.Schema, "include_size", opts.IncludeSize)

		// List tables
		tables, err := appInstance.ListTables(opts)
		if err != nil {
			debugLogger.Error("Failed to list tables", "error", err)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list tables: %v", err)), nil
		}

		// Convert to JSON
		jsonData, err := json.Marshal(tables)
		if err != nil {
			debugLogger.Error("Failed to marshal tables to JSON", "error", err)
			return mcp.NewToolResultError("Failed to format tables response"), nil
		}

		debugLogger.Info("Successfully listed tables", "count", len(tables), "schema", opts.Schema)
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

// setupDescribeTableTool creates and registers the describe_table tool.
func setupDescribeTableTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	describeTableTool := mcp.NewTool("describe_table",
		mcp.WithDescription("Get detailed information about a table's structure (columns, types, constraints)"),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Table name to describe"),
		),
		mcp.WithString("schema",
			mcp.Description("Schema name (default: public)"),
		),
	)

	s.AddTool(describeTableTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		debugLogger.Debug("Received describe_table tool request", "args", args)

		// Extract table name (required)
		table, ok := args["table"].(string)
		if !ok || table == "" {
			debugLogger.Error("table name is missing or not a string", "value", args["table"])
			return mcp.NewToolResultError("table must be a non-empty string"), nil
		}

		// Extract schema (optional)
		schema := "public"
		if schemaArg, ok := args["schema"].(string); ok && schemaArg != "" {
			schema = schemaArg
		}

		debugLogger.Debug("Processing describe_table request", "schema", schema, "table", table)

		// Describe table
		columns, err := appInstance.DescribeTable(schema, table)
		if err != nil {
			debugLogger.Error("Failed to describe table", "error", err, "schema", schema, "table", table)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to describe table: %v", err)), nil
		}

		// Convert to JSON
		jsonData, err := json.Marshal(columns)
		if err != nil {
			debugLogger.Error("Failed to marshal columns to JSON", "error", err)
			return mcp.NewToolResultError("Failed to format table description response"), nil
		}

		debugLogger.Info("Successfully described table", "column_count", len(columns), "schema", schema, "table", table)
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

// setupExecuteQueryTool creates and registers the execute_query tool.
func setupExecuteQueryTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	executeQueryTool := mcp.NewTool("execute_query",
		mcp.WithDescription("Execute a read-only SQL query (SELECT or WITH statements only)"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("SQL query to execute (SELECT or WITH statements only)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of rows to return (default: no limit)"),
		),
	)

	s.AddTool(executeQueryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		debugLogger.Debug("Received execute_query tool request", "args", args)

		// Extract query (required)
		query, ok := args["query"].(string)
		if !ok || query == "" {
			debugLogger.Error("query is missing or not a string", "value", args["query"])
			return mcp.NewToolResultError("query must be a non-empty string"), nil
		}

		// Extract options
		opts := &app.ExecuteQueryOptions{
			Query: query,
		}

		if limitFloat, ok := args["limit"].(float64); ok && limitFloat > 0 {
			opts.Limit = int(limitFloat)
		}

		debugLogger.Debug("Processing execute_query request", "query", query, "limit", opts.Limit)

		// Execute query
		result, err := appInstance.ExecuteQuery(opts)
		if err != nil {
			debugLogger.Error("Failed to execute query", "error", err, "query", query)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to execute query: %v", err)), nil
		}

		// Convert to JSON
		jsonData, err := json.Marshal(result)
		if err != nil {
			debugLogger.Error("Failed to marshal query result to JSON", "error", err)
			return mcp.NewToolResultError("Failed to format query result"), nil
		}

		debugLogger.Info("Successfully executed query", "row_count", result.RowCount)
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

// setupListIndexesTool creates and registers the list_indexes tool.
func setupListIndexesTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	listIndexesTool := mcp.NewTool("list_indexes",
		mcp.WithDescription("List indexes for a specific table"),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Table name to list indexes for"),
		),
		mcp.WithString("schema",
			mcp.Description("Schema name (default: public)"),
		),
	)

	s.AddTool(listIndexesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		debugLogger.Debug("Received list_indexes tool request", "args", args)

		// Extract table name (required)
		table, ok := args["table"].(string)
		if !ok || table == "" {
			debugLogger.Error("table name is missing or not a string", "value", args["table"])
			return mcp.NewToolResultError("table must be a non-empty string"), nil
		}

		// Extract schema (optional)
		schema := "public"
		if schemaArg, ok := args["schema"].(string); ok && schemaArg != "" {
			schema = schemaArg
		}

		debugLogger.Debug("Processing list_indexes request", "schema", schema, "table", table)

		// List indexes
		indexes, err := appInstance.ListIndexes(schema, table)
		if err != nil {
			debugLogger.Error("Failed to list indexes", "error", err, "schema", schema, "table", table)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list indexes: %v", err)), nil
		}

		// Convert to JSON
		jsonData, err := json.Marshal(indexes)
		if err != nil {
			debugLogger.Error("Failed to marshal indexes to JSON", "error", err)
			return mcp.NewToolResultError("Failed to format indexes response"), nil
		}

		debugLogger.Info("Successfully listed indexes", "count", len(indexes), "schema", schema, "table", table)
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

// setupExplainQueryTool creates and registers the explain_query tool.
func setupExplainQueryTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	explainQueryTool := mcp.NewTool("explain_query",
		mcp.WithDescription("Get the execution plan for a SQL query"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("SQL query to explain (SELECT or WITH statements only)"),
		),
	)

	s.AddTool(explainQueryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		debugLogger.Debug("Received explain_query tool request", "args", args)

		// Extract query (required)
		query, ok := args["query"].(string)
		if !ok || query == "" {
			debugLogger.Error("query is missing or not a string", "value", args["query"])
			return mcp.NewToolResultError("query must be a non-empty string"), nil
		}

		debugLogger.Debug("Processing explain_query request", "query", query)

		// Explain query
		result, err := appInstance.ExplainQuery(query)
		if err != nil {
			debugLogger.Error("Failed to explain query", "error", err, "query", query)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to explain query: %v", err)), nil
		}

		// Convert to JSON
		jsonData, err := json.Marshal(result)
		if err != nil {
			debugLogger.Error("Failed to marshal explain result to JSON", "error", err)
			return mcp.NewToolResultError("Failed to format explain result"), nil
		}

		debugLogger.Info("Successfully explained query")
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

// setupGetTableStatsTool creates and registers the get_table_stats tool.
func setupGetTableStatsTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	getTableStatsTool := mcp.NewTool("get_table_stats",
		mcp.WithDescription("Get detailed statistics for a specific table"),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Table name to get statistics for"),
		),
		mcp.WithString("schema",
			mcp.Description("Schema name (default: public)"),
		),
	)

	s.AddTool(getTableStatsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		debugLogger.Debug("Received get_table_stats tool request", "args", args)

		// Extract table name (required)
		table, ok := args["table"].(string)
		if !ok || table == "" {
			debugLogger.Error("table name is missing or not a string", "value", args["table"])
			return mcp.NewToolResultError("table must be a non-empty string"), nil
		}

		// Extract schema (optional)
		schema := "public"
		if schemaArg, ok := args["schema"].(string); ok && schemaArg != "" {
			schema = schemaArg
		}

		debugLogger.Debug("Processing get_table_stats request", "schema", schema, "table", table)

		// Get table stats
		stats, err := appInstance.GetTableStats(schema, table)
		if err != nil {
			debugLogger.Error("Failed to get table stats", "error", err, "schema", schema, "table", table)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get table stats: %v", err)), nil
		}

		// Convert to JSON
		jsonData, err := json.Marshal(stats)
		if err != nil {
			debugLogger.Error("Failed to marshal table stats to JSON", "error", err)
			return mcp.NewToolResultError("Failed to format table stats response"), nil
		}

		debugLogger.Info("Successfully retrieved table stats", "schema", schema, "table", table)
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

func printHelp() {
	fmt.Printf(`PostgreSQL MCP Server %s

A Model Context Protocol (MCP) server that provides PostgreSQL integration tools for Claude Code.

USAGE:
    postgresql-mcp [OPTIONS]

OPTIONS:
    -h, --help     Show this help message
    -v, --version  Show version information

ENVIRONMENT VARIABLES:
    POSTGRES_URL   PostgreSQL connection URL (format: postgres://user:password@host:port/dbname?sslmode=prefer)
    DATABASE_URL   Alternative to POSTGRES_URL

DESCRIPTION:
    This MCP server provides the following tools for PostgreSQL integration:

    • list_databases      - List all databases on the server
    • list_schemas        - List schemas in the current database
    • list_tables         - List tables in a schema with optional metadata
    • describe_table      - Get detailed table structure information
    • execute_query       - Execute read-only SQL queries (SELECT, WITH)
    • list_indexes        - List indexes for a specific table
    • explain_query       - Get execution plan for SQL queries
    • get_table_stats     - Get detailed statistics for a table

    The server communicates via JSON-RPC 2.0 over stdin/stdout and is designed
    to be used with Claude Code's MCP architecture.

EXAMPLES:
    # Start the MCP server (typically called by Claude Code)
    postgresql-mcp

    # Show help
    postgresql-mcp -h

    # Show version
    postgresql-mcp -v

For more information, visit: https://github.com/sylvain/postgresql-mcp
`, version)
}

// handleCommandLineFlags processes command line arguments and exits if necessary.
func handleCommandLineFlags() {
	var (
		showHelp        = flag.Bool("h", false, "Show help message")
		showHelpLong    = flag.Bool("help", false, "Show help message")
		showVersion     = flag.Bool("v", false, "Show version information")
		showVersionLong = flag.Bool("version", false, "Show version information")
	)

	flag.Parse()

	// Handle help flags
	if *showHelp || *showHelpLong {
		printHelp()
		os.Exit(0)
	}

	// Handle version flags
	if *showVersion || *showVersionLong {
		fmt.Printf("%s\n", version)
		os.Exit(0)
	}
}

// initializeApp creates and configures the application instance.
func initializeApp() (*app.App, *slog.Logger) {
	// Initialize the app
	appInstance, err := app.New()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	// Set debug logger
	debugLogger := logger.NewLogger("debug")
	appInstance.SetLogger(debugLogger)

	debugLogger.Info("Starting PostgreSQL MCP Server", "version", version)

	return appInstance, debugLogger
}

// registerAllTools registers all available tools with the MCP server.
func registerAllTools(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
	setupListDatabasesTool(s, appInstance, debugLogger)
	setupListSchemasTool(s, appInstance, debugLogger)
	setupListTablesTool(s, appInstance, debugLogger)
	setupDescribeTableTool(s, appInstance, debugLogger)
	setupExecuteQueryTool(s, appInstance, debugLogger)
	setupListIndexesTool(s, appInstance, debugLogger)
	setupExplainQueryTool(s, appInstance, debugLogger)
	setupGetTableStatsTool(s, appInstance, debugLogger)
}

func main() {
	handleCommandLineFlags()
	appInstance, debugLogger := initializeApp()

	// Create MCP server
	s := server.NewMCPServer(
		"PostgreSQL MCP Server",
		version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, false), // No resources for now
	)

	registerAllTools(s, appInstance, debugLogger)

	// Cleanup on exit
	defer func() {
		if err := appInstance.Disconnect(); err != nil {
			debugLogger.Error("Failed to disconnect from database", "error", err)
		}
	}()

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}