package learningrecord

import (
	"context"
	"log"
	"time"
)

const ReviewNotificationInterval = time.Hour

type ReviewNotificationSchedule struct {
	Interval time.Duration
	Now      func() time.Time
	Notify   func(context.Context, time.Time) error
	Logf     func(string, ...any)
}

// RunReviewNotificationSchedule runs once at startup and then waits an interval between ticks.
func RunReviewNotificationSchedule(ctx context.Context, s ReviewNotificationSchedule) {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Logf == nil {
		s.Logf = log.Printf
	}
	if !runReviewNotificationOnce(ctx, s) {
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
		if !runReviewNotificationOnce(ctx, s) {
			return
		}
	}
}

func runReviewNotificationOnce(ctx context.Context, s ReviewNotificationSchedule) bool {
	if ctx.Err() != nil {
		return false
	}
	err := s.Notify(ctx, s.Now())
	if ctx.Err() != nil {
		return false
	}
	if err != nil {
		s.Logf("review notification failed: %v", err)
	}
	return true
}
