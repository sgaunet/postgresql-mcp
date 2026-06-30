package app_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/sylvain/postgresql-mcp/internal/app"
)

// TestApp_EnsureConnection_DedupesConcurrentReconnects_Issue83 verifies that
// when N handler goroutines observe a connection failure simultaneously, only
// one underlying Connect call is issued. Without the singleflight gate, N
// concurrent reconnects could each open a fresh *sql.DB pool and the close-
// then-reopen dance would create a thundering herd on the database.
func TestApp_EnsureConnection_DedupesConcurrentReconnects_Issue83(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)
	a.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// No pool exists yet (HasConnection returns false), so each goroutine attempts bootstrap.
	mockClient.On("HasConnection").Return(false)
	// App.Connect (reached via bootstrap) pings to decide whether to close an
	// existing pool; return an error so it skips Close.
	mockClient.On("Ping", mock.Anything).Return(errors.New("connection lost"))

	// Connect "succeeds" after a delay long enough for every follower
	// goroutine to enqueue behind the singleflight leader.
	mockClient.On("Connect", mock.Anything, mock.Anything).
		Run(func(_ mock.Arguments) { time.Sleep(50 * time.Millisecond) }).
		Return(nil)

	// tryConnect needs a connection string from the environment.
	t.Setenv("POSTGRES_URL", "postgres://test:test@localhost:5432/test")

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})

	for range goroutines {
		go func() {
			defer wg.Done()
			<-start
			_ = a.EnsureConnection(context.Background())
		}()
	}
	close(start)
	wg.Wait()

	connectCalls := 0
	for _, call := range mockClient.Calls {
		if call.Method == "Connect" {
			connectCalls++
		}
	}
	assert.Equal(t, 1, connectCalls,
		"singleflight must dedupe concurrent reconnect attempts (issue #83); got %d Connect calls", connectCalls)
}

// TestApp_EnsureConnection_FollowerHonorsCtxCancel verifies that a goroutine
// waiting behind the singleflight leader returns when its own request context
// is cancelled, rather than blocking until the leader finishes.
func TestApp_EnsureConnection_FollowerHonorsCtxCancel(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)
	a.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	mockClient.On("HasConnection").Return(false)
	// App.Connect (reached via bootstrap) pings to decide whether to close an
	// existing pool; return an error so it skips Close.
	mockClient.On("Ping", mock.Anything).Return(errors.New("connection lost"))
	// Leader's Connect blocks long enough that the follower's ctx times out first.
	mockClient.On("Connect", mock.Anything, mock.Anything).
		Run(func(_ mock.Arguments) { time.Sleep(500 * time.Millisecond) }).
		Return(nil)
	t.Setenv("POSTGRES_URL", "postgres://test:test@localhost:5432/test")

	leaderStarted := make(chan struct{})
	go func() {
		close(leaderStarted)
		_ = a.EnsureConnection(context.Background())
	}()
	<-leaderStarted
	// Give the leader a moment to enter its Connect call.
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := a.EnsureConnection(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err, "follower should observe its own ctx cancellation")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 200*time.Millisecond,
		"follower should return shortly after ctx expires, not wait for the leader's full Connect")
}

// TestApp_Connect_SerializesConcurrentCalls drives many concurrent Connect
// calls through the connectMu-guarded Ping→Close→Connect sequence. Its value is
// under `go test -race`: it proves the pool-swap and connStr write are free of
// data races (review finding on connectMu).
func TestApp_Connect_SerializesConcurrentCalls(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)
	a.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Ping returns an error so the close branch is skipped; Connect succeeds.
	mockClient.On("HasConnection").Return(false)
	mockClient.On("Ping", mock.Anything).Return(errors.New("no connection"))
	mockClient.On("Connect", mock.Anything, mock.Anything).Return(nil)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})

	for range goroutines {
		go func() {
			defer wg.Done()
			<-start
			_ = a.Connect(context.Background(), "postgres://test:test@localhost:5432/test")
		}()
	}
	close(start)
	wg.Wait()

	connectCalls := 0
	for _, call := range mockClient.Calls {
		if call.Method == "Connect" {
			connectCalls++
		}
	}
	// Explicit Connect always replaces, so every call connects; connectMu
	// serializes them rather than deduping.
	assert.Equal(t, goroutines, connectCalls,
		"every explicit Connect should run; connectMu serializes, not dedupes")
}

// TestApp_SetLogger_ConcurrentWithReads runs SetLogger writers alongside log
// readers (via the exported Logger() accessor). Its value is under
// `go test -race`: it proves the atomic.Pointer[slog.Logger] read/write pair is
// race-free, matching the SetLogger doc guarantee.
func TestApp_SetLogger_ConcurrentWithReads(t *testing.T) {
	a := app.New(&MockPostgreSQLClient{})

	const writers, readers = 8, 8
	var wg sync.WaitGroup
	wg.Add(writers + readers)
	start := make(chan struct{})

	for range writers {
		go func() {
			defer wg.Done()
			<-start
			a.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
		}()
	}
	for range readers {
		go func() {
			defer wg.Done()
			<-start
			_ = a.Logger()
		}()
	}
	close(start)
	wg.Wait()

	assert.NotNil(t, a.Logger(), "logger must remain set after concurrent writes")
}

// TestApp_ConnectIfNeeded_SkipsWhenConnected guards review finding H3: the
// reconnect path (connect with replaceExisting=false) must no-op when a pool
// already exists rather than tearing it down and rebuilding it.
func TestApp_ConnectIfNeeded_SkipsWhenConnected(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	a := app.New(mockClient)
	a.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// A pool already exists.
	mockClient.On("HasConnection").Return(true)

	err := a.ConnectIfNeeded(context.Background(), "postgres://test:test@localhost:5432/test")

	require.NoError(t, err)
	mockClient.AssertNotCalled(t, "Ping", mock.Anything)
	mockClient.AssertNotCalled(t, "Close")
	mockClient.AssertNotCalled(t, "Connect", mock.Anything, mock.Anything)
}
