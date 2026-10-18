package primport

import "io"

type ImportInput struct {
	UserID string
	URL    string
	File   *UploadFile
}

type UploadFile struct {
	Filename string
	Reader   io.Reader
}
