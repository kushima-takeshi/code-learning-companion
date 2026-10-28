package learningrecord

type FieldError struct {
	Path          string   `json:"path"`
	Code          string   `json:"code"`
	AllowedValues []string `json:"allowed_values,omitempty"`
}

type ValidationError struct {
	Details []FieldError `json:"details"`
}

func (e *ValidationError) Error() string {
	return "validation_failed"
}
