package learningrecord

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestSaveInputValidateQuestionCount(t *testing.T) {
	for _, count := range []int{0, 1, 2, 3, 4, 10, 11} {
		t.Run(fmt.Sprintf("%d問", count), func(t *testing.T) {
			input := validSaveInputForTest(count)
			err := input.Validate()
			if count >= 1 && count <= 10 {
				if err != nil {
					t.Fatalf("1〜10問は受け付ける: %v", err)
				}
				return
			}
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("ValidationErrorを期待: %v", err)
			}
			want := []FieldError{{Path: "questions", Code: "invalid_count"}}
			if !reflect.DeepEqual(validationErr.Details, want) {
				t.Fatalf("件数エラー: got %+v, want %+v", validationErr.Details, want)
			}
		})
	}
}

func validSaveInputForTest(count int) SaveInput {
	hintUsed := false
	input := SaveInput{
		RequestID: "11111111-1111-4111-8111-111111111111",
		Theme:     "Goのdefer",
		Questions: make([]Question, count),
	}
	for i := range input.Questions {
		input.Questions[i] = Question{
			Type: "free_text", Prompt: "deferはいつ実行されますか？",
			Answer:   Answer{Text: "関数から戻るときです。", HintUsed: &hintUsed},
			Feedback: Feedback{Understanding: "できた", Explanation: "実行タイミングを説明できています。"},
		}
	}
	return input
}
