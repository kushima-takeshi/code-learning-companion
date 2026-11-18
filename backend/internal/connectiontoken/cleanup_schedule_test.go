package connectiontoken

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRunCleanupScheduleStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var calls int
	done := make(chan struct{})
	go func() {
		RunCleanupSchedule(ctx, CleanupSchedule{
			Interval: 20 * time.Millisecond,
			Now:      func() time.Time { return time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC) },
			Cleanup: func(context.Context, time.Time) (CleanupResult, error) {
				mu.Lock()
				calls++
				mu.Unlock()
				return CleanupResult{}, nil
			},
			Logf: func(string, ...any) {},
		})
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		n := calls
		mu.Unlock()
		if n >= 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("contextを消してもループが止まらない")
	}
	mu.Lock()
	stopped := calls
	mu.Unlock()
	time.Sleep(60 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if calls != stopped || stopped < 1 {
		t.Fatalf("停止後に実行が続いた: before=%d after=%d", stopped, calls)
	}
}

func TestRunCleanupScheduleContinuesAfterFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var calls []time.Time
	var logs []string
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	done := make(chan struct{})
	go func() {
		RunCleanupSchedule(ctx, CleanupSchedule{
			Interval: 40 * time.Millisecond,
			Now:      func() time.Time { return now },
			Cleanup: func(context.Context, time.Time) (CleanupResult, error) {
				mu.Lock()
				calls = append(calls, time.Now())
				n := len(calls)
				mu.Unlock()
				if n == 1 {
					return CleanupResult{}, errors.New("db down")
				}
				return CleanupResult{Reconnections: 1, Tokens: 2}, nil
			},
			Logf: func(format string, _ ...any) {
				mu.Lock()
				logs = append(logs, format)
				n := len(logs)
				mu.Unlock()
				if n == 2 {
					close(done)
				}
			},
		})
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("失敗のあと次の間隔まで進まない")
	}
	cancel()
	mu.Lock()
	defer mu.Unlock()
	if len(calls) < 2 {
		t.Fatalf("失敗でループが止まった: %d", len(calls))
	}
	if gap := calls[1].Sub(calls[0]); gap < 30*time.Millisecond {
		t.Fatalf("失敗の直後に再試行した: %s", gap)
	}
	if len(logs) < 2 || logs[0] != "connection token cleanup failed: %v" || logs[1] != "connection token cleanup deleted %d reconnections and %d tokens" {
		t.Fatalf("ログ: %#v", logs)
	}
}
