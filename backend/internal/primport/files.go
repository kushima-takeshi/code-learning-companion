package primport

import (
	"context"
	"fmt"
	"strings"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
)

func (s *Service) saveFileContents(
	ctx context.Context,
	userID string,
	snapshotID string,
	files []github.ChangedFile,
) ([]storedFile, []string, error) {
	storedFiles := make([]storedFile, 0, len(files))
	putKeys := make([]string, 0)

	for i, file := range files {
		record := storedFile{
			Path:   file.Filename,
			Status: file.Status,
			Patch:  file.Patch,
		}

		if file.Content != nil {
			key := fmt.Sprintf(
				"%s/pr-snapshots/%s/contents/%d.txt",
				userID,
				snapshotID,
				i,
			)

			err := s.store.Put(
				ctx,
				key,
				strings.NewReader(*file.Content),
				"text/plain; charset=utf-8",
			)
			if err != nil {
				return nil, putKeys, err
			}

			putKeys = append(putKeys, key)
			record.ContentStored = true
			record.ContentKey = key
		}

		storedFiles = append(storedFiles, record)
	}

	return storedFiles, putKeys, nil
}
