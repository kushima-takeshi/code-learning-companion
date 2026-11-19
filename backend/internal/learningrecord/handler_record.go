package learningrecord

import "time"

// recordResponse exposes the saved content without internal ownership information.
type recordResponse struct {
	ID         string     `json:"id"`
	RequestID  string     `json:"request_id"`
	Theme      string     `json:"theme"`
	Repository string     `json:"repository,omitempty"`
	Questions  []Question `json:"questions"`
	CreatedAt  time.Time  `json:"created_at"`
}

func publicRecord(record Record) recordResponse {
	return recordResponse{
		ID: record.ID, RequestID: record.RequestID, Theme: record.Theme,
		Repository: record.Repository, Questions: record.Questions, CreatedAt: record.CreatedAt,
	}
}
