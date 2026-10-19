package primport

import (
	"bytes"
	"context"
)

type savedConversation struct {
	ID        string
	Filename  string
	ObjectKey string
}

func (s *Service) saveConversation(
	ctx context.Context,
	userID string,
	file *UploadFile,
	markdown []byte,
	warning string,
) (*savedConversation, error) {
	if file == nil || warning != "" {
		return nil, nil
	}

	id, err := newUUID()
	if err != nil {
		return nil, err
	}

	key := userID + "/conversation-uploads/" + id + "/conversation.md"

	err = s.store.Put(
		ctx,
		key,
		bytes.NewReader(markdown),
		"text/markdown; charset=utf-8",
	)
	if err != nil {
		return nil, err
	}

	return &savedConversation{
		ID:        id,
		Filename:  file.Filename,
		ObjectKey: key,
	}, nil
}
