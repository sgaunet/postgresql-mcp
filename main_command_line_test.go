package main

import (
	"flag"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
)

// Test handleCommandLineFlags function behavior without calling os.Exit
func TestHandleCommandLineFlags(t *testing.T) {
	// Save original os.Args and flag state
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	// Test only the no-flags case since help/version flags call os.Exit()
	t.Run("no flags", func(t *testing.T) {
		// Reset state
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		os.Args = []string{"postgresql-mcp"}

		// Call the function being tested - should not panic or exit
		assert.NotPanics(t, func() {
			handleCommandLineFlags()
		})
	})
}

// Test main function execution paths - we can't really test main() directly
// but we can test the logic flow that would happen in main
func TestMainFunctionLogic(t *testing.T) {
	// Save original state
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	// Test that printHelp works independently
	t.Run("printHelp_function", func(t *testing.T) {
		assert.NotPanics(t, func() {
			printHelp()
		})
	})

	// Test version constant
	t.Run("version_constant", func(t *testing.T) {
		assert.Equal(t, "dev", version)
	})

	// Test normal execution path (no flags)
	t.Run("main_normal_path", func(t *testing.T) {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		os.Args = []string{"postgresql-mcp"}

		assert.NotPanics(t, func() {
			handleCommandLineFlags()
		})

		// Test initialization that would happen in main
		app, logger := initializeApp()
		assert.NotNil(t, app)
		assert.NotNil(t, logger)

		// We can't test the actual MCP server.Run() call, but we can test
		// that our setup functions work
		assert.NotPanics(t, func() {
			// This is similar to what main() would do
			server := server.NewMCPServer("postgresql-mcp", version)
			registerAllTools(server, app, logger)
		})
	})
}
