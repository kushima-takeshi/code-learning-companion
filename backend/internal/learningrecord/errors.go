package learningrecord

import "errors"

var ErrRequestConflict = errors.New("learning record request conflict")

var ErrSaveInputTooLarge = errors.New("save_input_too_large")

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
