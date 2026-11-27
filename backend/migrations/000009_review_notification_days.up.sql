CREATE TABLE review_notification_days (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    notified_on DATE NOT NULL,
    due_count INTEGER NOT NULL CHECK (due_count > 0),
    logged_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, notified_on)
);
