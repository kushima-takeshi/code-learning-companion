package learningrecord

type SaveInput struct {
	RequestID string     `json:"request_id"`
	Theme     string     `json:"theme"`
	Questions []Question `json:"questions"`
}

type Question struct {
	Type     string   `json:"type"`
	Prompt   string   `json:"prompt"`
	Answer   Answer   `json:"answer"`
	Feedback Feedback `json:"feedback"`
}

type Answer struct {
	Text     string `json:"text"`
	HintUsed *bool  `json:"hint_used"`
}

type Feedback struct {
	Understanding string `json:"understanding"`
	Explanation   string `json:"explanation"`
}
