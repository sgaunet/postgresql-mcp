package app

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
)

// TestApp_EnsureConnection_DedupesConcurrentReconnects_Issue83 verifies that
// when N handler goroutines observe a connection failure simultaneously, only
// one underlying Connect call is issued. Without the singleflight gate, N
// concurrent reconnects could each open a fresh *sql.DB pool and the close-
// then-reopen dance would create a thundering herd on the database.
func TestApp_EnsureConnection_DedupesConcurrentReconnects_Issue83(t *testing.T) {
	mockClient := &MockPostgreSQLClient{}
	app := New(mockClient)
	app.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Ping always reports the connection as lost.
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
			_ = app.ensureConnection(context.Background())
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
	app := New(mockClient)
	app.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	mockClient.On("Ping", mock.Anything).Return(errors.New("connection lost"))
	// Leader's Connect blocks long enough that the follower's ctx times out first.
	mockClient.On("Connect", mock.Anything, mock.Anything).
		Run(func(_ mock.Arguments) { time.Sleep(500 * time.Millisecond) }).
		Return(nil)
	t.Setenv("POSTGRES_URL", "postgres://test:test@localhost:5432/test")

	leaderStarted := make(chan struct{})
	go func() {
		close(leaderStarted)
		_ = app.ensureConnection(context.Background())
	}()
	<-leaderStarted
	// Give the leader a moment to enter its Connect call.
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := app.ensureConnection(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err, "follower should observe its own ctx cancellation")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 200*time.Millisecond,
		"follower should return shortly after ctx expires, not wait for the leader's full Connect")
}
