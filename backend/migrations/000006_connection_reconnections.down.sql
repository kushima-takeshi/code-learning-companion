-- Dropping the deadline checks must not reactivate abandoned replacement tokens.
UPDATE connection_tokens t SET revoked_at = COALESCE(t.revoked_at, now())
FROM connection_reconnections r WHERE r.new_token_id = t.id AND r.state <> 'completed';
DROP TABLE connection_reconnections;
ALTER TABLE connection_tokens DROP CONSTRAINT connection_tokens_owner_id_key;
