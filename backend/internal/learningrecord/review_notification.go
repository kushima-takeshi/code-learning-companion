package learningrecord

import (
	"context"
	"fmt"
	"time"
)

type reviewNotification struct {
	userID   string
	dueCount int
}

// LogDueReviewNotifications logs only the users newly recorded for this JST day.
func (s *Service) LogDueReviewNotifications(ctx context.Context, now time.Time, appURL string, logf func(string, ...any)) error {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return fmt.Errorf("load review notification time zone: %w", err)
	}
	localNow := now.In(tokyo)
	if localNow.Hour() < 8 {
		return nil
	}

	notifications, err := s.recordDueReviewNotifications(ctx, now, localNow.Format(time.DateOnly))
	if err != nil {
		return err
	}
	for _, notification := range notifications {
		logf("review notification logged: user_id=%s due_count=%d app_url=%s", notification.userID, notification.dueCount, appURL)
	}
	return nil
}

func (s *Service) recordDueReviewNotifications(ctx context.Context, now time.Time, notifiedOn string) ([]reviewNotification, error) {
	const query = `
		INSERT INTO review_notification_days (user_id, notified_on, due_count, logged_at)
		SELECT record.user_id, $3::text::date, count(*), $2
		FROM learning_records AS record
		CROSS JOIN LATERAL (` + nextReviewStage + `) AS next
		WHERE record.created_at <= $1 AND ` + reviewDueAt + ` <= $2
		GROUP BY record.user_id
		ORDER BY record.user_id
		ON CONFLICT (user_id, notified_on) DO NOTHING
		RETURNING user_id, due_count
	`
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin review notification recording: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query, now.Add(-firstReviewDelay), now, notifiedOn)
	if err != nil {
		return nil, fmt.Errorf("record review notifications: %w", err)
	}
	defer rows.Close()

	var notifications []reviewNotification
	for rows.Next() {
		var notification reviewNotification
		if err := rows.Scan(&notification.userID, &notification.dueCount); err != nil {
			return nil, fmt.Errorf("scan review notification: %w", err)
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read review notifications: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit review notifications: %w", err)
	}
	return notifications, nil
}
