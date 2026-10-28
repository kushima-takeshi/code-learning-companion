package learningrecord

import (
	"errors"
	"slices"
	"testing"
)

func TestSaveInputValidateMultipleErrors(t *testing.T) {
	input := SaveInput{
		RequestID: "11111111-1111-4111-8111-111111111111",
		Theme:     " \t ",
	}

	err := input.Validate()

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("ValidationErrorを期待しましたが、結果は %v", err)
	}

	if len(validationErr.Details) != 2 {
		t.Fatalf("エラーは2件のはずですが、結果は %+v",
			validationErr.Details)
	}

	got := make(map[string]string)
	for _, detail := range validationErr.Details {
		got[detail.Path] = detail.Code
	}

	if got["theme"] != "required" {
		t.Errorf("テーマのエラーが違います: %v", got)
	}

	if got["questions"] != "invalid_count" {
		t.Errorf("問題数のエラーが違います: %v", got)
	}
}

func TestSaveInputValidateValid(t *testing.T) {
	hintUsed := false

	input := SaveInput{
		RequestID: "11111111-1111-4111-8111-111111111111",
		Theme:     "Goのdefer",
		Questions: []Question{
			{
				Type:   "free_text",
				Prompt: "deferはいつ実行されますか？",
				Answer: Answer{
					Text:     "関数から戻るときです。",
					HintUsed: &hintUsed,
				},
				Feedback: Feedback{
					Understanding: "できた",
					Explanation:   "実行タイミングを説明できています。",
				},
			},
			{
				Type:   "free_text",
				Prompt: "複数のdeferはどの順番で実行されますか？",
				Answer: Answer{
					Text:     "後から登録した順です。",
					HintUsed: &hintUsed,
				},
				Feedback: Feedback{
					Understanding: "できた",
					Explanation:   "逆順に実行されることを説明できています。",
				},
			},
		},
	}

	if err := input.Validate(); err != nil {
		t.Fatalf("正常な入力で検証エラー: %v", err)
	}
}

func TestSaveInputValidateInvalidUnderstanding(t *testing.T) {
	hintUsed := false

	input := SaveInput{
		RequestID: "11111111-1111-4111-8111-111111111111",
		Theme:     "Goのdefer",
		Questions: []Question{
			{
				Type:   "free_text",
				Prompt: "deferはいつ実行されますか？",
				Answer: Answer{
					Text:     "関数から戻るときです。",
					HintUsed: &hintUsed,
				},
				Feedback: Feedback{
					Understanding: "できた",
					Explanation:   "実行タイミングを説明できています。",
				},
			},
			{
				Type:   "free_text",
				Prompt: "複数のdeferはどの順番で実行されますか？",
				Answer: Answer{
					Text:     "後から登録した順です。",
					HintUsed: &hintUsed,
				},
				Feedback: Feedback{
					Understanding: "できた",
					Explanation:   "逆順に実行されることを説明できています。",
				},
			},
		},
	}

	input.Questions[1].Feedback.Understanding = "正解"

	err := input.Validate()

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("ValidationErrorを期待しましたが、結果は %v", err)
	}

	if len(validationErr.Details) != 1 {
		t.Fatalf("エラーは1件のはずですが、結果は %+v",
			validationErr.Details)
	}

	detail := validationErr.Details[0]

	if detail.Path != "questions[1].feedback.understanding" {
		t.Errorf("エラーの場所が違います: %s", detail.Path)
	}

	if detail.Code != "invalid_value" {
		t.Errorf("エラーコードが違います: %s", detail.Code)
	}

	wantAllowed := []string{
		"できた", "一部できた", "もう一度", "判定保留",
	}
	if !slices.Equal(detail.AllowedValues, wantAllowed) {
		t.Errorf("許可値が違います: %v", detail.AllowedValues)
	}
}

func TestSaveInputValidateMissingHintUsed(t *testing.T) {
	hintUsed := false

	input := SaveInput{
		RequestID: "11111111-1111-4111-8111-111111111111",
		Theme:     "Goのdefer",
		Questions: []Question{
			{
				Type:   "free_text",
				Prompt: "deferはいつ実行されますか？",
				Answer: Answer{
					Text:     "関数から戻るときです。",
					HintUsed: &hintUsed,
				},
				Feedback: Feedback{
					Understanding: "できた",
					Explanation:   "実行タイミングを説明できています。",
				},
			},
			{
				Type:   "free_text",
				Prompt: "複数のdeferはどの順番で実行されますか？",
				Answer: Answer{
					Text:     "後から登録した順です。",
					HintUsed: &hintUsed,
				},
				Feedback: Feedback{
					Understanding: "できた",
					Explanation:   "逆順に実行されることを説明できています。",
				},
			},
		},
	}

	input.Questions[1].Answer.HintUsed = nil

	err := input.Validate()

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("ValidationErrorを期待しましたが、結果は %v", err)
	}

	if len(validationErr.Details) != 1 {
		t.Fatalf("エラーは1件のはずですが、結果は %+v",
			validationErr.Details)
	}

	detail := validationErr.Details[0]

	if detail.Path != "questions[1].answer.hint_used" {
		t.Errorf("エラーの場所が違います: %s", detail.Path)
	}

	if detail.Code != "required" {
		t.Errorf("エラーコードが違います: %s", detail.Code)
	}

	if len(detail.AllowedValues) != 0 {
		t.Errorf("必須項目エラーに許可値は不要です: %v",
			detail.AllowedValues)
	}
}
