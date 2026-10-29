package learningrecord

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestSaveRejectsInvalidInputBeforeDB(t *testing.T) {
	// DB接続なしでも入力エラーが返ることで、検索より先の検証を確認する。
	got, err := New(nil).Save(context.Background(), "", SaveInput{})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("ValidationErrorを期待: %v", err)
	}
	if !reflect.DeepEqual(got, Record{}) {
		t.Fatalf("入力不正なら空の記録を期待: %+v", got)
	}
}
