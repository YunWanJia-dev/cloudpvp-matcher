package asynclock

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testLobbyMutex struct {
	name   string
	extend func(context.Context) (bool, error)
}

func (m *testLobbyMutex) Name() string {
	return m.name
}

func (m *testLobbyMutex) ExtendContext(ctx context.Context) (bool, error) {
	return m.extend(ctx)
}

func (m *testLobbyMutex) UnlockContext(context.Context) (bool, error) {
	return true, nil
}

func TestRunWithLobbyMutexRenewalCancelsCallbackOnRenewalFailure(t *testing.T) {
	renewErr := errors.New("redis unavailable")
	mutexes := []lobbyMutex{
		&testLobbyMutex{
			name: "matcher:lock:lobby:1",
			extend: func(context.Context) (bool, error) {
				return false, renewErr
			},
		},
	}

	callbackCanceled := make(chan struct{})
	err := runWithLobbyMutexRenewal(context.Background(), mutexes, time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		close(callbackCanceled)
		return ctx.Err()
	})

	if !errors.Is(err, renewErr) {
		t.Fatalf("error = %v, want renewal error", err)
	}
	select {
	case <-callbackCanceled:
	default:
		t.Fatal("callback context was not canceled after renewal failure")
	}
}

func TestRunWithLobbyMutexRenewalDoesNotReportShutdownAsFailure(t *testing.T) {
	mutexes := []lobbyMutex{
		&testLobbyMutex{
			name: "matcher:lock:lobby:1",
			extend: func(ctx context.Context) (bool, error) {
				<-ctx.Done()
				return false, ctx.Err()
			},
		},
	}

	err := runWithLobbyMutexRenewal(context.Background(), mutexes, time.Millisecond, func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}
