package github

import (
	"strings"
	"testing"
)

func TestReadFileContentSizeLimit(t *testing.T) {
	t.Run("上限ちょうどなら取得する", func(t *testing.T) {
		input := strings.Repeat("a", maxFileContentBytes)

		content, reason, err := readFileContent(
			strings.NewReader(input),
		)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content == nil {
			t.Fatal("content should not be nil")
		}
		if *content != input {
			t.Error("content does not match input")
		}
		if reason != "" {
			t.Errorf("unexpected skip reason: %s", reason)
		}
	})

	t.Run("上限を1バイト超えたら省略する", func(t *testing.T) {
		input := strings.Repeat("a", maxFileContentBytes+1)

		content, reason, err := readFileContent(
			strings.NewReader(input),
		)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != nil {
			t.Error("content should be nil")
		}
		if reason != "too_large" {
			t.Errorf("skip reason = %q, want too_large", reason)
		}
	})
}

func TestReadFileContentBinary(t *testing.T) {
	input := "hello\x00world"

	content, reason, err := readFileContent(
		strings.NewReader(input),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != nil {
		t.Error("content should be nil")
	}
	if reason != "binary" {
		t.Errorf("skip reason = %q, want binary", reason)
	}
}
