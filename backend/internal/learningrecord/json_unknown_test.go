package learningrecord

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeSaveInputRejectsUnknownFields(t *testing.T) {
	cases := []struct {
		name string
		path []string
	}{
		{name: "保存要求の直下"},
		{name: "問題内", path: []string{"questions"}},
		{name: "回答内", path: []string{"questions", "answer"}},
		{name: "フィードバック内", path: []string{"questions", "feedback"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(validSaveInputForTest(1))
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatal(err)
			}
			target := body
			if len(tc.path) > 0 {
				target = body["questions"].([]any)[0].(map[string]any)
				if len(tc.path) > 1 {
					target = target[tc.path[1]].(map[string]any)
				}
			}
			target["memo"] = "保存対象だと思って送った情報"
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}

			got, err := DecodeSaveInput(data)
			if err == nil {
				t.Fatal("未知の項目がある入力は拒否する")
			}
			if !strings.Contains(err.Error(), `unknown field "memo"`) {
				t.Fatalf("修正対象の項目名を含むエラーを期待: %v", err)
			}
			if !reflect.DeepEqual(got, SaveInput{}) {
				t.Fatal("未知項目があれば空の入力を返す")
			}

			// 未定義の項目を除いた同じ内容なら再送できる。
			delete(target, "memo")
			fixed, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			got, err = DecodeSaveInput(fixed)
			if err != nil {
				t.Fatalf("形式を修正した入力が拒否された: %v", err)
			}
			if !reflect.DeepEqual(got, validSaveInputForTest(1)) {
				t.Fatal("修正後の既知項目が変わっています")
			}
		})
	}
}

func TestDecodeSaveInputRequiresSingleJSON(t *testing.T) {
	data, err := json.Marshal(validSaveInputForTest(1))
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{string(data), " null", " garbage"} {
		t.Run(suffix[:min(len(suffix), 20)], func(t *testing.T) {
			got, err := DecodeSaveInput(append(append([]byte(nil), data...), suffix...))
			if err == nil {
				t.Fatal("末尾に別のJSONや不正な文字がある場合は拒否する")
			}
			if !reflect.DeepEqual(got, SaveInput{}) {
				t.Fatal("不正なJSONなら空の入力を返す")
			}
		})
	}
}
