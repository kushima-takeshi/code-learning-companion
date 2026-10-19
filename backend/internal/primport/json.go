package primport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

func (s *Service) putJSON(
	ctx context.Context,
	key string,
	value any,
) error {
	content, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode JSON for %q: %w", key, err)
	}

	return s.store.Put(
		ctx,
		key,
		bytes.NewReader(content),
		"application/json",
	)
}
