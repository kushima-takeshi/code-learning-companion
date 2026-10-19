package primport

import (
	"io"
	"time"
)

type ImportInput struct {
	UserID string
	URL    string
	File   *UploadFile
}

type UploadFile struct {
	Filename string
	Reader   io.Reader
}

type storedFile struct {
	Path          string  `json:"path"`
	Status        string  `json:"status"`
	Patch         *string `json:"patch"`
	ContentStored bool    `json:"content_stored"`
	ContentKey    string  `json:"content_key,omitempty"`
}

type ImportResult struct {
	SnapshotID string
	Warning    string
}

type SnapshotSummary struct {
	ID          string
	HTMLURL     string
	Owner       string
	Repo        string
	PRNumber    int
	Title       string
	AuthorLogin string
	State       string
	CreatedAt   time.Time
}
