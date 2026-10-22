package primport

import (
	"context"
	"errors"
	"time"
)

func (s *Service) Import(
	ctx context.Context,
	input ImportInput,
) (*ImportResult, error) {
	// prepare performs the following steps in order without saving anything.
	// 1. Parse the PR URL. Return invalid_url if it is invalid.
	// 2. Read and validate the optional conversation file. Continue with a warning if invalid.
	// 3. Fetch the PR data from GitHub into memory. Return on failure.
	prepared, err := s.prepare(ctx, input)
	if err != nil {
		return nil, err
	}

	// 4. Generate the snapshot ID and object keys.
	snapshotID, err := newUUID()
	if err != nil {
		return nil, err
	}
	keys := buildSnapshotKeys(input.UserID, snapshotID)

	// 5. Save the PR data to MinIO.
	files, savedObjectKeys, err := s.saveSnapshot(
		ctx,
		input.UserID,
		snapshotID,
		prepared.Snapshot,
		keys,
	)
	if err != nil {
		return nil, s.cleanupAfterFailure(ctx, savedObjectKeys, err)
	}

	// 6. Save the conversation file if present and valid.
	conversation, err := s.saveConversation(
		ctx,
		input.UserID,
		input.File,
		prepared.Markdown,
		prepared.Warning,
	)
	if err != nil {
		return nil, s.cleanupAfterFailure(ctx, savedObjectKeys, err)
	}

	if conversation != nil {
		savedObjectKeys = append(savedObjectKeys, conversation.ObjectKey)
	}

	// 7. Insert the records in a single database transaction.
	err = s.saveRecords(
		ctx,
		input.UserID,
		snapshotID,
		prepared,
		keys,
		files,
		conversation,
	)
	if err != nil {
		return nil, s.cleanupAfterFailure(ctx, savedObjectKeys, err)
	}

	changedFiles := make([]string, 0, len(files))

	for _, file := range files {
		changedFiles = append(changedFiles, file.Path)
	}
	result := &ImportResult{
		SnapshotID:   snapshotID,
		Warning:      prepared.Warning,
		Title:        prepared.Snapshot.Title,
		HTMLURL:      prepared.Snapshot.HTMLURL,
		HeadSHA:      prepared.Snapshot.HeadSHA,
		ChangedFiles: changedFiles,
	}

	if conversation != nil {
		result.ConversationFilename = &conversation.Filename
	}

	return result, nil
}

// 8. Delete the saved objects on failure.
func (s *Service) cleanupAfterFailure(
	ctx context.Context,
	keys []string,
	cause error,
) error {
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		30*time.Second,
	)
	defer cancel()

	result := cause
	for _, key := range keys {
		if err := s.store.Delete(cleanupCtx, key); err != nil {
			result = errors.Join(result, err)
		}
	}

	return result
}
