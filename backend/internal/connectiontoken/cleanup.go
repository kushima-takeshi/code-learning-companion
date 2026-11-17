package connectiontoken

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// UnusableRetention is how long an unusable token is kept.
// Cleanup deletes only after this duration, not at the exact instant.
const UnusableRetention = 30 * 24 * time.Hour

// CleanupResult counts rows removed by one call.
type CleanupResult struct {
	Reconnections int
	Tokens        int
}

// CleanupUnusableTokens deletes tokens that have been unusable for more than
// UnusableRetention, then nothing else. One transaction deletes referencing
// connection_reconnections rows first, then the tokens. Learning records and
// users stay. A second call deletes nothing. now is the caller's clock.
//
// A token is unusable from the earlier of expires_at and revoked_at. When it
// is the new token of a reconnection whose state is not completed, that
// reconnection's expires_at counts too. Deleting an old token also deletes
// every unfinished replacement issued from it, even when that replacement's
// own expiry is still inside 90 days. A completed replacement stays.
func (s *Service) CleanupUnusableTokens(ctx context.Context, now time.Time) (CleanupResult, error) {
	cutoff := now.UTC().Truncate(time.Microsecond).Add(-UnusableRetention)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("cleanup unusable tokens: %w", err)
	}
	defer tx.Rollback()

	ids, err := lockedIDs(ctx, tx, `
		SELECT t.id
		FROM connection_tokens t
		WHERE LEAST(
			t.expires_at,
			COALESCE(t.revoked_at, 'infinity'::timestamptz),
			COALESCE((
				SELECT r.expires_at
				FROM connection_reconnections r
				WHERE r.new_token_id = t.id AND r.state <> 'completed'
			), 'infinity'::timestamptz)
		) < $1
		FOR UPDATE OF t
	`, cutoff)
	if err != nil {
		return CleanupResult{}, err
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return CleanupResult{}, fmt.Errorf("cleanup unusable tokens: %w", err)
		}
		return CleanupResult{}, nil
	}
	extra, err := lockedIDs(ctx, tx, fmt.Sprintf(`
		SELECT t.id
		FROM connection_tokens t
		JOIN connection_reconnections r ON r.new_token_id = t.id
		WHERE r.old_token_id IN (%s) AND r.state <> 'completed'
		FOR UPDATE OF t
	`, numberedPlaceholders(len(ids), 1)), argsFor(ids)...)
	if err != nil {
		return CleanupResult{}, err
	}
	ids = appendUnique(ids, extra)

	reconnections, err := deleteByIDs(ctx, tx, `
		DELETE FROM connection_reconnections
		WHERE old_token_id IN (%s) OR new_token_id IN (%s)
	`, ids, true)
	if err != nil {
		return CleanupResult{}, err
	}
	tokens, err := deleteByIDs(ctx, tx, `
		DELETE FROM connection_tokens
		WHERE id IN (%s)
	`, ids, false)
	if err != nil {
		return CleanupResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CleanupResult{}, fmt.Errorf("cleanup unusable tokens: %w", err)
	}
	return CleanupResult{Reconnections: reconnections, Tokens: tokens}, nil
}

// deleteByIDs repeats the id list when bothOldAndNew is set, so one statement
// can match either side of a reconnection.
func deleteByIDs(ctx context.Context, tx *sql.Tx, query string, ids []string, bothOldAndNew bool) (int, error) {
	lists := 1
	if bothOldAndNew {
		lists = 2
	}
	args := make([]any, 0, len(ids)*lists)
	for n := 0; n < lists; n++ {
		for _, id := range ids {
			args = append(args, id)
		}
	}
	placeholders := make([]any, lists)
	for n := 0; n < lists; n++ {
		placeholders[n] = numberedPlaceholders(len(ids), n*len(ids)+1)
	}
	result, err := tx.ExecContext(ctx, fmt.Sprintf(query, placeholders...), args...)
	if err != nil {
		return 0, fmt.Errorf("cleanup unusable tokens: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cleanup unusable tokens: %w", err)
	}
	return int(count), nil
}

func lockedIDs(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("cleanup unusable tokens: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("cleanup unusable tokens: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cleanup unusable tokens: %w", err)
	}
	return ids, nil
}

func argsFor(ids []string) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

func appendUnique(ids, extra []string) []string {
	seen := make(map[string]struct{}, len(ids)+len(extra))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	for _, id := range extra {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func numberedPlaceholders(n, start int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = fmt.Sprintf("$%d", start+i)
	}
	return strings.Join(parts, ",")
}
