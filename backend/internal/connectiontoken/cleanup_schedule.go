package connectiontoken

import (
	"context"
	"log"
	"time"
)

// CleanupInterval is how long the server waits between cleanup runs.
// The first run is one interval after process start. Spec 07 does not ask
// for a run at startup.
const CleanupInterval = 24 * time.Hour

// CleanupSchedule is the daily loop. Interval and Now are injected in tests.
// There is no lock against a second server.
type CleanupSchedule struct {
	Interval time.Duration
	Now      func() time.Time
	Cleanup  func(context.Context, time.Time) (CleanupResult, error)
	Logf     func(string, ...any)
}

// RunCleanupSchedule calls Cleanup once per Interval until ctx is cancelled.
// A failed call is logged and left for the next interval. It does not retry sooner.
func RunCleanupSchedule(ctx context.Context, s CleanupSchedule) {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Logf == nil {
		s.Logf = log.Printf
	}
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if ctx.Err() != nil {
			return
		}
		result, err := s.Cleanup(ctx, s.Now())
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			s.Logf("connection token cleanup failed: %v", err)
			continue
		}
		s.Logf("connection token cleanup deleted %d reconnections and %d tokens", result.Reconnections, result.Tokens)
	}
}
