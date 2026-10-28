CREATE TABLE learning_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    request_id UUID NOT NULL,
    theme TEXT NOT NULL,
    questions JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT learning_records_user_request_unique
        UNIQUE (user_id, request_id),

    CONSTRAINT learning_records_questions_array
        CHECK (jsonb_typeof(questions) = 'array')
);

CREATE INDEX learning_records_user_created_idx
    ON learning_records (user_id, created_at DESC);
