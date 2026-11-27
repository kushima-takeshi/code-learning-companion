package learningrecord

import (
	"database/sql"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

const reviewNotificationMigration = "000009_review_notification_days.up.sql"
const notificationAppURL = "https://app.example"

func TestLogDueReviewNotifications(t *testing.T) {
	// 08:00 JST on November 27 is still November 26 in UTC.
	morning := time.Date(2026, 11, 26, 23, 0, 0, 0, time.UTC)

	t.Run("same JST day and after restart", func(t *testing.T) {
		db := newIntegrationDB(t, reviewMigration, reviewNotificationMigration)
		userID := listUser(t, db, 1)
		insertReviewRecord(t, db, uuid.NewString(), userID, morning.Add(-firstReviewDelay))

		service := New(db)
		assertNotificationLogs(t, service, morning, notificationLog(userID, 1))
		assertNotificationLogs(t, service, morning.Add(time.Hour))
		assertNotificationLogs(t, New(db), morning)
		assertNotificationRows(t, db, 1)
	})

	t.Run("next JST day at 07:59:59 and 08:00", func(t *testing.T) {
		db := newIntegrationDB(t, reviewMigration, reviewNotificationMigration)
		userID := listUser(t, db, 1)
		insertReviewRecord(t, db, uuid.NewString(), userID, morning.Add(-96*time.Hour))
		service := New(db)

		previousMorning := morning.Add(-24 * time.Hour)
		assertNotificationLogs(t, service, previousMorning, notificationLog(userID, 1))
		assertNotificationLogs(t, service, morning.Add(-time.Second))
		assertNotificationRows(t, db, 1)
		assertNotificationLogs(t, service, morning, notificationLog(userID, 1))
		assertNotificationRows(t, db, 2)

		var loggedAt time.Time
		err := db.QueryRow(`
			SELECT logged_at FROM review_notification_days
			WHERE user_id = $1 AND notified_on = DATE '2026-11-27'
		`, userID).Scan(&loggedAt)
		if err != nil || !loggedAt.Equal(morning) {
			t.Fatalf("JSTの2026-11-27の記録: logged_at=%v, err=%v", loggedAt, err)
		}
	})

	t.Run("no records, future deadline, or all stages saved", func(t *testing.T) {
		db := newIntegrationDB(t, reviewMigration, reviewNotificationMigration)
		listUser(t, db, 1)
		future := listUser(t, db, 2)
		completed := listUser(t, db, 3)
		insertReviewRecord(t, db, uuid.NewString(), future, morning.Add(-firstReviewDelay+time.Second))
		insertNotificationReview(t, db, completed, morning.Add(-720*time.Hour), morning, 3)

		assertNotificationLogs(t, New(db), morning)
		assertNotificationRows(t, db, 0)
	})

	t.Run("counts next due stages per user at a noon startup", func(t *testing.T) {
		db := newIntegrationDB(t, reviewMigration, reviewNotificationMigration)
		owner, other, future := listUser(t, db, 1), listUser(t, db, 2), listUser(t, db, 3)
		noon := morning.Add(4 * time.Hour)
		insertReviewRecord(t, db, uuid.NewString(), owner, noon.Add(-72*time.Hour))
		insertNotificationReview(t, db, owner, noon.Add(-336*time.Hour), noon, 1)
		insertNotificationReview(t, db, owner, noon.Add(-720*time.Hour), noon, 2)
		insertNotificationReview(t, db, other, noon.Add(-336*time.Hour), noon, 1)
		insertNotificationReview(t, db, future, noon.Add(-720*time.Hour+time.Second), noon, 2)

		var logs []string
		err := New(db).LogDueReviewNotifications(t.Context(), noon, notificationAppURL, func(format string, args ...any) {
			// A separate query must see the committed rows when the logger is called.
			assertNotificationRows(t, db, 2)
			logs = append(logs, fmt.Sprintf(format, args...))
		})
		want := []string{notificationLog(owner, 3), notificationLog(other, 1)}
		slices.Sort(logs)
		slices.Sort(want)
		if err != nil || !slices.Equal(logs, want) {
			t.Fatalf("ログ = %q, err = %v, want %q", logs, err, want)
		}
		var dueCount int
		err = db.QueryRow("SELECT due_count FROM review_notification_days WHERE user_id = $1", owner).Scan(&dueCount)
		if err != nil || dueCount != 3 {
			t.Fatalf("DBの期限到来件数 = %d, err = %v, want 3", dueCount, err)
		}
	})

	t.Run("concurrent runs", func(t *testing.T) {
		db := newIntegrationDB(t, reviewMigration, reviewNotificationMigration)
		userID := listUser(t, db, 1)
		insertReviewRecord(t, db, uuid.NewString(), userID, morning.Add(-firstReviewDelay))
		start := make(chan struct{})
		results := make(chan error, 2)
		logs := make(chan string, 2)
		for range 2 {
			go func() {
				<-start
				results <- New(db).LogDueReviewNotifications(t.Context(), morning, notificationAppURL, func(format string, args ...any) {
					logs <- fmt.Sprintf(format, args...)
				})
			}()
		}
		close(start)
		for range 2 {
			if err := <-results; err != nil {
				t.Error(err)
			}
		}
		if len(logs) != 1 {
			t.Fatalf("並行実行のログ件数 = %d, want 1", len(logs))
		}
		if got := <-logs; got != notificationLog(userID, 1) {
			t.Fatalf("ログ = %q, want %q", got, notificationLog(userID, 1))
		}
		assertNotificationRows(t, db, 1)
	})
}

func insertNotificationReview(t *testing.T, db *sql.DB, userID string, createdAt, now time.Time, savedStages int) {
	t.Helper()
	id := uuid.NewString()
	insertReviewRecord(t, db, id, userID, createdAt)
	if _, err := db.Exec(`
		INSERT INTO learning_record_reviews (learning_record_id, user_id, stage, result, saved_at)
		SELECT $1, $2, stage, 'できた', $3 FROM generate_series(1, $4) AS stage
	`, id, userID, now, savedStages); err != nil {
		t.Fatal(err)
	}
}

func assertNotificationLogs(t *testing.T, service *Service, now time.Time, want ...string) {
	t.Helper()
	var got []string
	err := service.LogDueReviewNotifications(t.Context(), now, notificationAppURL, func(format string, args ...any) {
		got = append(got, fmt.Sprintf(format, args...))
	})
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("ログ = %q, err = %v, want %q", got, err, want)
	}
}

func assertNotificationRows(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	err := db.QueryRow("SELECT count(*) FROM review_notification_days").Scan(&got)
	if err != nil || got != want {
		t.Fatalf("通知記録の行数 = %d, err = %v, want %d", got, err, want)
	}
}

func notificationLog(userID string, count int) string {
	return fmt.Sprintf("review notification logged: user_id=%s due_count=%d app_url=%s", userID, count, notificationAppURL)
}
