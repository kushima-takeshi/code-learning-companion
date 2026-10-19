package primport

import (
	"context"
	"strings"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
)

func (s *Service) saveSnapshot(
	ctx context.Context,
	userID string,
	snapshotID string,
	snapshot *github.PRSnapshot,
	keys snapshotKeys,
) ([]storedFile, []string, error) {
	putKeys := make([]string, 0)

	textObjects := []struct {
		key         string
		body        string
		contentType string
	}{
		{keys.Body, snapshot.Body, "text/markdown; charset=utf-8"},
		{keys.Diff, snapshot.Diff, "text/plain; charset=utf-8"},
	}

	for _, object := range textObjects {
		err := s.store.Put(
			ctx,
			object.key,
			strings.NewReader(object.body),
			object.contentType,
		)
		if err != nil {
			return nil, putKeys, err
		}

		putKeys = append(putKeys, object.key)
	}

	storedFiles, contentKeys, err := s.saveFileContents(
		ctx,
		userID,
		snapshotID,
		snapshot.Files,
	)
	putKeys = append(putKeys, contentKeys...)
	if err != nil {
		return nil, putKeys, err
	}

	jsonObjects := []struct {
		key   string
		value any
	}{
		{keys.Files, storedFiles},
		{keys.Commits, snapshot.Commits},
		{keys.ReviewComments, snapshot.ReviewComments},
	}

	for _, object := range jsonObjects {
		if err := s.putJSON(ctx, object.key, object.value); err != nil {
			return nil, putKeys, err
		}

		putKeys = append(putKeys, object.key)
	}

	return storedFiles, putKeys, nil
}
