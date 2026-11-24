ALTER TABLE learning_records
    ADD CONSTRAINT learning_records_id_user_unique UNIQUE (id, user_id);

CREATE TABLE learning_record_reviews (
    learning_record_id UUID NOT NULL,
    user_id UUID NOT NULL,
    stage INTEGER NOT NULL CHECK (stage BETWEEN 1 AND 3),
    result TEXT NOT NULL CHECK (result IN ('できた', '一部できた', 'もう一度', '判定保留')),
    saved_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (learning_record_id, stage),
    FOREIGN KEY (learning_record_id, user_id)
        REFERENCES learning_records (id, user_id) ON DELETE CASCADE
);
