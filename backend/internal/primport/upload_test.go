package primport

import (
	"io"
	"strings"
	"testing"
)

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errInjected }

type countingReader struct {
	reader io.Reader
	bytes  int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.bytes += n
	return n, err
}

func TestReadUpload(t *testing.T) {
	tests := []struct {
		name, filename, body, wantWarning string
		absent, readError                 bool
	}{
		{name: "absent", absent: true},
		{name: "markdown", filename: "notes.md", body: "# 会話\nhello"},
		{name: "uppercase_extension", filename: "notes.MD", body: "hello"},
		{name: "empty_markdown", filename: "notes.md", wantWarning: "empty_file"},
		{name: "exactly_1MiB", filename: "notes.md", body: strings.Repeat("a", 1024*1024)},
		{name: "over_1MiB", filename: "notes.md", body: strings.Repeat("a", 1024*1024+1), wantWarning: "file_too_large"},
		{name: "wrong_extension", filename: "notes.txt", body: "hello", wantWarning: "invalid_file_extension"},
		{name: "nul", filename: "notes.md", body: "hello\x00world", wantWarning: "file_contains_nul"},
		{name: "read_failure", filename: "notes.md", readError: true, wantWarning: "file_read_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var file *UploadFile
			if !tt.absent {
				file = &UploadFile{Filename: tt.filename, Reader: strings.NewReader(tt.body)}
				if tt.readError {
					file.Reader = errorReader{}
				}
				if tt.wantWarning == "invalid_file_extension" {
					file.Reader = unexpectedReader{t}
				}
			}
			got, warning := readUpload(file)
			if warning != tt.wantWarning {
				t.Fatalf("warning=%q, want %q", warning, tt.wantWarning)
			}
			if tt.absent || tt.wantWarning != "" {
				if got != nil {
					t.Fatal("discarded/absent file returned data")
				}
			} else if string(got) != tt.body {
				t.Fatal("file content changed")
			}
		})
	}
}

func TestReadUploadBoundsMemoryRead(t *testing.T) {
	reader := &countingReader{reader: strings.NewReader(strings.Repeat("a", 2*1024*1024))}
	_, warning := readUpload(&UploadFile{Filename: "large.md", Reader: reader})
	if warning != "file_too_large" || reader.bytes != 1024*1024+1 {
		t.Fatalf("warning=%q, bytes read=%d", warning, reader.bytes)
	}
}
