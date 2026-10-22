package primport

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
)

const maxUploadBytes = 1024 * 1024

func readUpload(file *UploadFile) ([]byte, string) {
	if file == nil {
		return nil, ""
	}

	if strings.ToLower(filepath.Ext(file.Filename)) != ".md" {
		return nil, "invalid_file_extension"
	}

	content, err := io.ReadAll(
		io.LimitReader(file.Reader, maxUploadBytes+1),
	)
	if err != nil {
		return nil, "file_read_failed"
	}

	if len(content) == 0 {
		return nil, "empty_file"
	}

	if len(content) > maxUploadBytes {
		return nil, "file_too_large"
	}

	if bytes.Contains(content, []byte{0}) {
		return nil, "file_contains_nul"
	}

	return content, ""
}
