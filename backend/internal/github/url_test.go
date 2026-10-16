package github

import (
	"errors"
	"testing"
)

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{
			name:    "末尾にスラッシュ",
			raw:     "https://github.com/cli/cli/pull/123/",
			wantErr: false,
		},
		{
			name:    "末尾にdiff",
			raw:     "https://github.com/cli/cli/pull/123.diff",
			wantErr: false,
		},
		{
			name:    "クエリ付き",
			raw:     "https://github.com/cli/cli/pull/123?utm=example",
			wantErr: false,
		},
		{
			name:    "HTTP",
			raw:     "http://github.com/cli/cli/pull/123",
			wantErr: true,
		},
		{
			name:    "別ホスト",
			raw:     "https://example.com/cli/cli/pull/123",
			wantErr: true,
		},
		{
			name:    "pullがない",
			raw:     "https://github.com/cli/cli/123",
			wantErr: true,
		},
		{
			name:    "番号ではない",
			raw:     "https://github.com/cli/cli/pull/abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, number, err := ParsePRURL(tt.raw)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if !errors.Is(err, ErrInvalidURL) {
					t.Fatalf("error = %v, want %v", err, ErrInvalidURL)
				}
				return
			}

			if owner != "cli" || repo != "cli" || number != 123 {
				t.Fatalf(
					"got (%q, %q, %d), want (cli, cli, 123)",
					owner, repo, number,
				)
			}
		})
	}
}
