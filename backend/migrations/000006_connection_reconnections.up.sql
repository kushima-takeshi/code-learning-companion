-- Composite keys keep both sides of a replacement under the same owner.
ALTER TABLE connection_tokens ADD CONSTRAINT connection_tokens_owner_id_key UNIQUE (user_id, id);

CREATE TABLE connection_reconnections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    old_token_id UUID NOT NULL,
    new_token_id UUID NOT NULL UNIQUE,
    state TEXT NOT NULL CHECK (state IN ('pending', 'confirmed', 'completed', 'superseded', 'expired')),
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    confirmed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    FOREIGN KEY (user_id, old_token_id) REFERENCES connection_tokens (user_id, id),
    FOREIGN KEY (user_id, new_token_id) REFERENCES connection_tokens (user_id, id),
    CHECK (old_token_id <> new_token_id),
    CHECK (expires_at = created_at + interval '24 hours'),
    CHECK (confirmed_at IS NULL OR (confirmed_at >= created_at AND confirmed_at < expires_at)),
    CHECK ((state IN ('confirmed', 'completed')) = (confirmed_at IS NOT NULL) OR state IN ('superseded', 'expired')),
    CHECK ((state = 'completed') = (completed_at IS NOT NULL)),
    CHECK (completed_at IS NULL OR (completed_at >= confirmed_at AND completed_at < expires_at))
);

-- A completed replacement also prevents a delayed start retry from issuing again.
CREATE UNIQUE INDEX connection_reconnections_current_idx ON connection_reconnections (old_token_id)
    WHERE state IN ('pending', 'confirmed', 'completed');
