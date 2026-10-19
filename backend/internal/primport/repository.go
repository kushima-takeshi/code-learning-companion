package primport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func insertSnapshot(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	snapshotID string,
	prepared *preparedImport,
	keys snapshotKeys,
	files []storedFile,
) error {
	changedFiles, err := json.Marshal(files)
	if err != nil {
		return fmt.Errorf("encode changed files: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`
		INSERT INTO pr_snapshots (
			id, user_id, html_url, owner, repo, pr_number,
			title, author_login, state, head_sha,
			body_key, diff_key, files_key, commits_key,
			review_comments_key, changed_files
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16
		)
		`,
		snapshotID,
		userID,
		prepared.Snapshot.HTMLURL,
		prepared.Owner,
		prepared.Repo,
		prepared.Number,
		prepared.Snapshot.Title,
		prepared.Snapshot.Author,
		prepared.Snapshot.State,
		prepared.Snapshot.HeadSHA,
		keys.Body,
		keys.Diff,
		keys.Files,
		keys.Commits,
		keys.ReviewComments,
		string(changedFiles),
	)
	if err != nil {
		return fmt.Errorf("insert PR snapshot: %w", err)
	}

	return nil
}

func insertConversation(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	snapshotID string,
	conversation *savedConversation,
) error {
	_, err := tx.ExecContext(
		ctx,
		`
		INSERT INTO conversation_uploads (
			id,
			user_id,
			pr_snapshot_id,
			filename,
			object_key
		)
		VALUES ($1, $2, $3, $4, $5)
		`,
		conversation.ID,
		userID,
		snapshotID,
		conversation.Filename,
		conversation.ObjectKey,
	)
	if err != nil {
		return fmt.Errorf("insert conversation upload: %w", err)
	}

	return nil
}

func (s *Service) saveRecords(
	ctx context.Context,
	userID string,
	snapshotID string,
	prepared *preparedImport,
	keys snapshotKeys,
	files []storedFile,
	conversation *savedConversation,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin import transaction: %w", err)
	}
	defer tx.Rollback()

	err = insertSnapshot(
		ctx,
		tx,
		userID,
		snapshotID,
		prepared,
		keys,
		files,
	)
	if err != nil {
		return err
	}

	if conversation != nil {
		err = insertConversation(
			ctx,
			tx,
			userID,
			snapshotID,
			conversation,
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit import transaction: %w", err)
	}

	return nil
}
