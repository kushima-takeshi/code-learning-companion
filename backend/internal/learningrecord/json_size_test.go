package learningrecord

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeSaveInputSizeBoundary(t *testing.T) {
	const limit = 1 << 20
	want := validSaveInputForTest(1)
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{limit - 1, limit, limit + 1} {
		t.Run(fmt.Sprintf("%dバイト", size), func(t *testing.T) {
			data := append(bytes.Clone(raw), bytes.Repeat([]byte(" "), size-len(raw))...)
			got, err := DecodeSaveInput(data)
			if size > limit {
				if !errors.Is(err, ErrSaveInputTooLarge) {
					t.Fatalf("容量超過エラーを期待: %v", err)
				}
				if !reflect.DeepEqual(got, SaveInput{}) {
					t.Fatalf("容量超過なら空の入力を期待: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("上限以内なら受け付ける: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("読み込んだ内容が元の入力と不一致")
			}
		})
	}
}

func TestDecodeSaveInputSizeCountsUTF8Bytes(t *testing.T) {
	input := validSaveInputForTest(1)
	input.Theme = strings.Repeat("あ", (1<<20)/3)
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= 1<<20 {
		t.Fatal("テストデータが容量上限を超えていません")
	}
	got, err := DecodeSaveInput(data)
	if !errors.Is(err, ErrSaveInputTooLarge) {
		t.Fatalf("文字数でなくバイト数で拒否する: %v", err)
	}
	if !reflect.DeepEqual(got, SaveInput{}) {
		t.Fatal("容量超過なら空の入力を期待")
	}
}

func TestDecodeSaveInputRejectsOversizeBeforeJSON(t *testing.T) {
	data := bytes.Repeat([]byte("x"), (1<<20)+1)
	got, err := DecodeSaveInput(data)
	if !errors.Is(err, ErrSaveInputTooLarge) {
		t.Fatalf("JSON解析より先に容量超過を返す: %v", err)
	}
	if !reflect.DeepEqual(got, SaveInput{}) {
		t.Fatal("容量超過なら空の入力を期待")
	}
}
