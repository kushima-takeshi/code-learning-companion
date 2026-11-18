package connectiontoken

import (
	"context"
	"log"
	"time"
)

// CleanupInterval is how long the server waits between cleanup runs.
// The first run is at process start, then once per interval. A restart
// inside the interval still cleans once. A failed run waits for the next interval.
const CleanupInterval = 24 * time.Hour

// CleanupSchedule is the daily loop. Interval and Now are injected in tests.
// There is no lock against a second server.
type CleanupSchedule struct {
	Interval time.Duration
	Now      func() time.Time
	Cleanup  func(context.Context, time.Time) (CleanupResult, error)
	Logf     func(string, ...any)
}

// RunCleanupSchedule calls Cleanup once at start, then once per Interval, until ctx is cancelled.
// A failed call, including the one at start, is logged and left for the next interval.
func RunCleanupSchedule(ctx context.Context, s CleanupSchedule) {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Logf == nil {
		s.Logf = log.Printf
	}
	if !runCleanupOnce(ctx, s) {
		return
	}
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if !runCleanupOnce(ctx, s) {
			return
		}
	}
}

func runCleanupOnce(ctx context.Context, s CleanupSchedule) bool {
	if ctx.Err() != nil {
		return false
	}
	result, err := s.Cleanup(ctx, s.Now())
	if ctx.Err() != nil {
		return false
	}
	if err != nil {
		s.Logf("connection token cleanup failed: %v", err)
		return true
	}
	s.Logf("connection token cleanup deleted %d reconnections and %d tokens", result.Reconnections, result.Tokens)
	return true
}
