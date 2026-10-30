package learningrecord

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (input SaveInput) Validate() error {
	var details []FieldError

	if _, err := uuid.Parse(input.RequestID); err != nil {
		details = append(details, FieldError{
			Path: "request_id",
			Code: "invalid_uuid",
		})
	}

	if strings.TrimSpace(input.Theme) == "" {
		details = append(details, FieldError{
			Path: "theme",
			Code: "required",
		})
	}

	if len(input.Questions) < 1 || len(input.Questions) > 10 {
		details = append(details, FieldError{
			Path: "questions",
			Code: "invalid_count",
		})
	}

	for i, question := range input.Questions {
		path := fmt.Sprintf("questions[%d]", i)

		if question.Type != "free_text" {
			details = append(details, FieldError{
				Path:          path + ".type",
				Code:          "invalid_value",
				AllowedValues: []string{"free_text"},
			})
		}

		if strings.TrimSpace(question.Prompt) == "" {
			details = append(details, FieldError{
				Path: path + ".prompt",
				Code: "required",
			})
		}

		if strings.TrimSpace(question.Answer.Text) == "" {
			details = append(details, FieldError{
				Path: path + ".answer.text",
				Code: "required",
			})
		}

		if question.Answer.HintUsed == nil {
			details = append(details, FieldError{
				Path: path + ".answer.hint_used",
				Code: "required",
			})
		}

		switch question.Feedback.Understanding {
		case "できた", "一部できた", "もう一度", "判定保留":
			// 許可された値なので、そのまま受け付ける。
		default:
			details = append(details, FieldError{
				Path: path + ".feedback.understanding",
				Code: "invalid_value",
				AllowedValues: []string{
					"できた",
					"一部できた",
					"もう一度",
					"判定保留",
				},
			})
		}

		if strings.TrimSpace(question.Feedback.Explanation) == "" {
			details = append(details, FieldError{
				Path: path + ".feedback.explanation",
				Code: "required",
			})
		}
	}

	if len(details) > 0 {
		return &ValidationError{Details: details}
	}

	return nil
}
