CREATE TABLE pr_snapshots (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    html_url TEXT NOT NULL,
    owner TEXT NOT NULL,
    repo TEXT NOT NULL,
    pr_number INTEGER NOT NULL,
    title TEXT NOT NULL,
    author_login TEXT NOT NULL,
    state TEXT NOT NULL,
    head_sha TEXT NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    body_key TEXT NOT NULL,
    diff_key TEXT NOT NULL,
    files_key TEXT NOT NULL,
    commits_key TEXT NOT NULL,
    review_comments_key TEXT NOT NULL,
    changed_files JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX pr_snapshots_user_id_idx
    ON pr_snapshots (user_id, created_at DESC);

CREATE TABLE conversation_uploads (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    pr_snapshot_id UUID NOT NULL REFERENCES pr_snapshots (id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    object_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX conversation_uploads_snapshot_idx
    ON conversation_uploads (pr_snapshot_id);
