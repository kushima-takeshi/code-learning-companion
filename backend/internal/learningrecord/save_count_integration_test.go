package learningrecord

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestSaveIntegrationQuestionCounts(t *testing.T) {
	for _, count := range []int{1, 10} {
		t.Run(fmt.Sprintf("%d問を保存して取得", count), func(t *testing.T) {
			db := newIntegrationDB(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var ownerID string
			if err := db.QueryRowContext(ctx, `INSERT INTO users (github_user_id, github_login) VALUES (1, 'test-owner') RETURNING id`).Scan(&ownerID); err != nil {
				t.Fatal(err)
			}

			want := validSaveInputForTest(count)
			data, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			input, err := DecodeSaveInput(data)
			if err != nil {
				t.Fatalf("合意した件数のJSONが拒否された: %v", err)
			}
			service := New(db)
			saved, err := service.Save(ctx, ownerID, input)
			if err != nil {
				t.Fatalf("保存に失敗: %v", err)
			}
			record, err := service.Get(ctx, ownerID, saved.ID)
			if err != nil {
				t.Fatalf("取得に失敗: %v", err)
			}
			got := SaveInput{RequestID: record.RequestID, Theme: record.Theme, Questions: record.Questions}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("JSONから保存・取得した内容が元の入力と不一致")
			}
		})
	}
}
