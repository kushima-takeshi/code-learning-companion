package learningrecord

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDecodeSaveInputJSONError(t *testing.T) {
	cases := []struct {
		name       string
		data       string
		wantSyntax bool
	}{
		{
			name:       "構文が不正",
			data:       `{"theme":"Goのdefer"`,
			wantSyntax: true,
		},
		{
			name: "テーマの型が不正",
			data: `{
				"request_id": "11111111-1111-4111-8111-111111111111",
				"theme": 123
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input, err := DecodeSaveInput([]byte(tc.data))

			if tc.wantSyntax {
				var syntaxErr *json.SyntaxError
				if !errors.As(err, &syntaxErr) {
					t.Fatalf("構文エラーを期待しましたが、結果は %v", err)
				}
			} else {
				var typeErr *json.UnmarshalTypeError
				if !errors.As(err, &typeErr) {
					t.Fatalf("型エラーを期待しましたが、結果は %v", err)
				}
				if typeErr.Field != "theme" {
					t.Errorf("型エラーの項目が違います: %s", typeErr.Field)
				}
			}

			if input.RequestID != "" ||
				input.Theme != "" ||
				input.Questions != nil {
				t.Errorf("失敗時は空の入力を期待しましたが、結果は %+v",
					input)
			}
		})
	}
}

func TestDecodeSaveInputValid(t *testing.T) {
	data := []byte(`{
		"request_id": "11111111-1111-4111-8111-111111111111",
		"theme": "Goのdefer",
		"questions": [
			{
				"type": "free_text",
				"prompt": "deferはいつ実行されますか？",
				"answer": {
					"text": "関数から戻るときです。",
					"hint_used": false
				},
				"feedback": {
					"understanding": "できた",
					"explanation": "実行タイミングを説明できています。"
				}
			},
			{
				"type": "free_text",
				"prompt": "複数のdeferの実行順は？",
				"answer": {
					"text": "後から登録した順です。",
					"hint_used": true
				},
				"feedback": {
					"understanding": "一部できた",
					"explanation": "ヒントを使って実行順を確認できました。"
				}
			}
		]
	}`)

	input, err := DecodeSaveInput(data)
	if err != nil {
		t.Fatalf("正常なJSONでエラー: %v", err)
	}

	if input.Theme != "Goのdefer" {
		t.Errorf("テーマが違います: %s", input.Theme)
	}

	if len(input.Questions) != 2 {
		t.Fatalf("問題は2件のはずですが、結果は%d件",
			len(input.Questions))
	}

	if input.Questions[0].Answer.Text != "関数から戻るときです。" {
		t.Error("1問目の回答が正しく読み込まれていません")
	}

	for i, wantHintUsed := range []bool{false, true} {
		got := input.Questions[i].Answer.HintUsed
		if got == nil || *got != wantHintUsed {
			t.Errorf("%d問目のヒント有無が違います", i+1)
		}
	}
}

func TestDecodeSaveInputValidationError(t *testing.T) {
	data := []byte(`{
		"request_id": "11111111-1111-4111-8111-111111111111",
		"theme": "Goのdefer"
	}`)

	input, err := DecodeSaveInput(data)

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("ValidationErrorを期待しましたが、結果は %v", err)
	}

	if len(validationErr.Details) != 1 {
		t.Fatalf("エラーは1件のはずですが、結果は %+v",
			validationErr.Details)
	}

	detail := validationErr.Details[0]
	if detail.Path != "questions" || detail.Code != "invalid_count" {
		t.Errorf("問題数のエラーが違います: %+v", detail)
	}

	if input.RequestID != "" ||
		input.Theme != "" ||
		input.Questions != nil {
		t.Errorf("失敗時は空の入力を期待しましたが、結果は %+v", input)
	}
}

func TestAnswerDecodeInvalidHintUsedType(t *testing.T) {
	input := `{"text":"回答","hint_used":"false"}`

	var answer Answer
	err := json.Unmarshal([]byte(input), &answer)

	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("JSONの型エラーを期待しましたが、結果は %v", err)
	}

	if typeErr.Field != "hint_used" {
		t.Errorf("型エラーの項目が違います: %s", typeErr.Field)
	}
}

func TestAnswerDecodeHintUsed(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantNil bool
	}{
		{
			name:  "ヒントなし",
			input: `{"text":"回答","hint_used":false}`,
		},
		{
			name:    "項目なし",
			input:   `{"text":"回答"}`,
			wantNil: true,
		},
		{
			name:    "null",
			input:   `{"text":"回答","hint_used":null}`,
			wantNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var answer Answer
			if err := json.Unmarshal([]byte(tc.input), &answer); err != nil {
				t.Fatalf("JSONの読み込みに失敗: %v", err)
			}

			if tc.wantNil {
				if answer.HintUsed != nil {
					t.Fatal("未指定ならnilになるはずです")
				}
				return
			}

			if answer.HintUsed == nil {
				t.Fatal("falseの指定が未指定として扱われています")
			}
			if *answer.HintUsed {
				t.Fatal("falseを期待しましたが、trueでした")
			}
		})
	}
}
