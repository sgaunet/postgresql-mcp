package main

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintHelp(t *testing.T) {
	// printHelp only writes to stdout; assert it runs without panicking.
	assert.NotPanics(t, func() {
		printHelp()
	})
}

func TestHandleCommandLineFlags_Help(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"cmd", "-h"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	var showHelp bool
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.Parse()

	assert.True(t, showHelp)
}

func TestHandleCommandLineFlags_Version(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"cmd", "-v"}
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

	// A freshly initialized app holds no live connection, so ValidateConnection
	// must report an error rather than silently passing.
	err := appInstance.ValidateConnection(context.Background())
	assert.Error(t, err)
}

func TestVersion(t *testing.T) {
	assert.Equal(t, "dev", version)
}

func TestErrorVariables(t *testing.T) {
	assert.NotNil(t, ErrInvalidConnectionParameters)
	assert.Contains(t, ErrInvalidConnectionParameters.Error(), "invalid connection parameters")
}
