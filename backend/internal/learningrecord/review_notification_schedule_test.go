package learningrecord

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"
)

func TestRunReviewNotificationSchedule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		start := time.Now()
		calls := make(chan time.Duration, 2)
		logs := make(chan string, 2)
		done := make(chan struct{})
		go func() {
			defer close(done)
			RunReviewNotificationSchedule(ctx, ReviewNotificationSchedule{
				Interval: ReviewNotificationInterval,
				Notify: func(_ context.Context, now time.Time) error {
					calls <- now.Sub(start)
					if now.Equal(start) {
						return errors.New("db down")
					}
					return nil
				},
				Logf: func(format string, args ...any) {
					logs <- fmt.Sprintf(format, args...)
				},
			})
		}()

		synctest.Wait()
		if len(calls) != 1 {
			t.Fatalf("起動直後の呼び出し回数 = %d, want 1", len(calls))
		}
		if got := <-calls; got != 0 {
			t.Fatalf("初回呼び出しまでの時間 = %v, want 0s", got)
		}
		if len(logs) != 1 {
			t.Fatalf("失敗ログ件数 = %d, want 1", len(logs))
		}
		if got := <-logs; got != "review notification failed: db down" {
			t.Fatalf("失敗ログ = %q", got)
		}
		time.Sleep(ReviewNotificationInterval - time.Nanosecond)
		synctest.Wait()
		if len(calls) != 0 {
			t.Fatal("次の間隔を待たずに再実行した")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if len(calls) != 1 {
			t.Fatalf("次の間隔での呼び出し回数 = %d, want 1", len(calls))
		}
		if got := <-calls; got != ReviewNotificationInterval {
			t.Fatalf("次の呼び出しまでの時間 = %v, want %v", got, ReviewNotificationInterval)
		}
		cancel()
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("キャンセル後にスケジュールが停止しなかった")
		}
	})
}
