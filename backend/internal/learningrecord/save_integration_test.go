package learningrecord

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSaveIntegration(t *testing.T) {
	cases := []struct {
		name   string
		resend bool
		change func(*SaveInput)
	}{
		{name: "新規保存"},
		{name: "同じ内容の再送", resend: true},
		{name: "テーマが異なる再送", resend: true, change: func(in *SaveInput) { in.Theme = "別のテーマ" }},
		{name: "回答が異なる再送", resend: true, change: func(in *SaveInput) { in.Questions[0].Answer.Text = "変更した回答" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newIntegrationDB(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var ownerID string
			if err := db.QueryRowContext(ctx, `INSERT INTO users (github_user_id, github_login) VALUES (1, 'test-owner') RETURNING id`).Scan(&ownerID); err != nil {
				t.Fatal(err)
			}
			service := New(db)
			input := saveTestInput()
			saved, err := service.Save(ctx, ownerID, input)
			if err != nil {
				t.Fatalf("新規保存に失敗: %v", err)
			}
			if saved.ID == "" || saved.CreatedAt.IsZero() || saved.UserID != ownerID {
				t.Fatalf("保存結果の付加情報が不正: %+v", saved)
			}
			want := SaveInput{RequestID: saved.RequestID, Theme: saved.Theme, Questions: saved.Questions}
			if !reflect.DeepEqual(want, input) {
				t.Fatalf("保存結果と入力が不一致: %+v", saved)
			}

			// 比較用の初回結果と、変更する送信内容を分離する。
			if tc.change != nil {
				input.Questions = append([]Question(nil), input.Questions...)
				tc.change(&input)
			}
			if tc.resend {
				got, err := service.Save(ctx, ownerID, input)
				if tc.change != nil {
					if !errors.Is(err, ErrRequestConflict) {
						t.Fatalf("競合エラーを期待: %v", err)
					}
					if !reflect.DeepEqual(got, Record{}) {
						t.Fatalf("競合時は空の記録を期待: %+v", got)
					}
				} else {
					if err != nil {
						t.Fatalf("再送に失敗: %v", err)
					}
					if !reflect.DeepEqual(got, saved) {
						t.Fatalf("再送結果が初回と不一致: got %+v, want %+v", got, saved)
					}
				}
			}
			reloaded, err := service.Get(ctx, ownerID, saved.ID)
			if err != nil {
				t.Fatalf("読み戻しに失敗: %v", err)
			}
			if !reflect.DeepEqual(reloaded, saved) {
				t.Fatalf("DBの記録が初回結果と不一致: got %+v, want %+v", reloaded, saved)
			}
			var count int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM learning_records WHERE user_id = $1`, ownerID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("保存件数: got %d, want 1", count)
			}
		})
	}
}

func TestSaveIntegrationDBError(t *testing.T) {
	db := newIntegrationDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := New(db).Save(context.Background(), uuid.NewString(), saveTestInput())
	if err == nil || errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrRequestConflict) {
		t.Fatalf("該当なし・競合とは別のDBエラーを期待: %v", err)
	}
	if !reflect.DeepEqual(got, Record{}) {
		t.Fatalf("DBエラーなら空の記録を期待: %+v", got)
	}
}

func saveTestInput() SaveInput {
	hintUsed := false
	return SaveInput{
		RequestID: uuid.NewString(),
		Theme:     "Goのdefer",
		Questions: []Question{
			{Type: "free_text", Prompt: "deferはいつ実行されますか？", Answer: Answer{Text: "関数から戻るときです。\n終了処理に使います。", HintUsed: &hintUsed}, Feedback: Feedback{Understanding: "できた", Explanation: "実行タイミングを説明できています。"}},
			{Type: "free_text", Prompt: "複数のdeferの実行順は？", Answer: Answer{Text: "後から登録した順です。", HintUsed: &hintUsed}, Feedback: Feedback{Understanding: "できた", Explanation: "逆順に実行されます。"}},
		},
	}
}
