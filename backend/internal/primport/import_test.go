package primport

import (
	"context"
	"errors"
	"testing"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/github"
)

type unexpectedReader struct {
	t *testing.T
}

func (r unexpectedReader) Read(p []byte) (int, error) {
	r.t.Fatal("URL が不正なのにファイルが読み取られました")
	return 0, nil
}

func TestImportInvalidURL(t *testing.T) {
	service := New(nil, nil, nil)

	result, err := service.Import(
		context.Background(),
		ImportInput{
			UserID: "test-user",
			URL:    "not-a-pr-url",
			File: &UploadFile{
				Filename: "conversation.md",
				Reader:   unexpectedReader{t: t},
			},
		},
	)

	if !errors.Is(err, github.ErrInvalidURL) {
		t.Fatalf("error = %v, want invalid_url", err)
	}

	if result != nil {
		t.Fatalf("result = %+v, want nil", result)
	}
}
