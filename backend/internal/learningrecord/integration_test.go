package learningrecord

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestGetIntegrationOwner(t *testing.T) {
	db := newIntegrationDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var ownerID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO users (github_user_id, github_login)
		VALUES (1, 'test-owner')
		RETURNING id
	`).Scan(&ownerID)
	if err != nil {
		t.Fatal(err)
	}

	hintUsed := false
	question := Question{
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
	}

	want := Record{
		UserID:    ownerID,
		RequestID: uuid.NewString(),
		Theme:     "Goのdefer",
		Questions: []Question{question, question},
	}

	questionsJSON, err := json.Marshal(want.Questions)
	if err != nil {
		t.Fatal(err)
	}

	err = db.QueryRowContext(ctx, `
		INSERT INTO learning_records (
			user_id, request_id, theme, questions
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`,
		want.UserID,
		want.RequestID,
		want.Theme,
		string(questionsJSON),
	).Scan(&want.ID, &want.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}

	got, err := New(db).Get(ctx, ownerID, want.ID)
	if err != nil {
		t.Fatalf("本人の取得に失敗: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("取得結果が違います:\ngot:  %+v\nwant: %+v", got, want)
	}

	t.Run("他人の記録は取得できない", func(t *testing.T) {
		var otherID string
		err := db.QueryRowContext(ctx, `
			INSERT INTO users (github_user_id, github_login)
			VALUES (2, 'test-other')
			RETURNING id
		`).Scan(&otherID)
		if err != nil {
			t.Fatal(err)
		}

		record, err := New(db).Get(ctx, otherID, want.ID)

		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("該当なしのエラーを期待しましたが、結果は %v", err)
		}

		if !reflect.DeepEqual(record, Record{}) {
			t.Fatalf("他人へ記録が返っています: %+v", record)
		}
	})

	t.Run("存在しない記録は取得できない", func(t *testing.T) {
		missingID := uuid.NewString()

		record, err := New(db).Get(ctx, ownerID, missingID)

		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("該当なしのエラーを期待しましたが、結果は %v", err)
		}

		if !reflect.DeepEqual(record, Record{}) {
			t.Fatalf("存在しない記録の取得でデータが返っています: %+v",
				record)
		}
	})
}

func TestInsertRecordIntegrationRoundTrip(t *testing.T) {
	db := newIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var ownerID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO users (github_user_id, github_login)
		VALUES (1, 'test-owner')
		RETURNING id
	`).Scan(&ownerID)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte(`{
		"request_id": "11111111-1111-4111-8111-111111111111",
		"theme": "Goのdefer",
		"questions": [
			{
				"type": "free_text",
				"prompt": "コードの \"defer\" はいつ実行されますか？",
				"answer": {
					"text": "関数から戻るときです。\n終了処理に使います。",
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
					"explanation": "ヒントを使って確認できました。"
				}
			}
		]
	}`)

	input, err := DecodeSaveInput(data)
	if err != nil {
		t.Fatal(err)
	}

	service := New(db)
	saved, err := service.insertRecord(ctx, ownerID, input)
	if err != nil {
		t.Fatalf("保存に失敗: %v", err)
	}

	got, err := service.Get(ctx, ownerID, saved.ID)
	if err != nil {
		t.Fatalf("取得に失敗: %v", err)
	}

	reloaded := SaveInput{
		RequestID: got.RequestID,
		Theme:     got.Theme,
		Questions: got.Questions,
	}

	reloadedJSON, err := json.Marshal(reloaded)
	if err != nil {
		t.Fatal(err)
	}

	var wantContent, gotContent any
	if err := json.Unmarshal(data, &wantContent); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(reloadedJSON, &gotContent); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(gotContent, wantContent) {
		t.Fatalf("保存前後で内容が変わっています:\ngot:  %#v\nwant: %#v",
			gotContent, wantContent)
	}
}

func TestGetByRequestIDIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ownerID, otherID := uuid.NewString(), uuid.NewString()
	for i, userID := range []string{ownerID, otherID} {
		_, err := db.ExecContext(ctx, `
			INSERT INTO users (id, github_user_id, github_login)
			VALUES ($1, $2, $3)
		`, userID, i+1, "test-user-"+userID)
		if err != nil {
			t.Fatal(err)
		}
	}

	hintUsed := false
	question := Question{
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
	}
	input := SaveInput{
		RequestID: uuid.NewString(),
		Theme:     "Goのdefer",
		Questions: []Question{question, question},
	}
	service := New(db)
	saved, err := service.insertRecord(ctx, ownerID, input)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		userID     string
		requestID  string
		wantNoRows bool
	}{
		{name: "本人の保存済み要求", userID: ownerID, requestID: input.RequestID},
		{name: "本人の未保存要求", userID: ownerID, requestID: uuid.NewString(), wantNoRows: true},
		{name: "他人の保存済み要求", userID: otherID, requestID: input.RequestID, wantNoRows: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.getByRequestID(ctx, tc.userID, tc.requestID)
			if tc.wantNoRows {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("該当なしのエラーを期待しましたが、結果は %v", err)
				}
				if !reflect.DeepEqual(got, Record{}) {
					t.Fatalf("該当なしの場合は空の記録を期待しましたが、結果は %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("保存済み要求の検索に失敗: %v", err)
			}
			if !reflect.DeepEqual(got, saved) {
				t.Fatalf("保存済み記録と検索結果が違います:\ngot:  %+v\nwant: %+v", got, saved)
			}
		})
	}
}

func newIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := os.Getenv("LEARNINGRECORD_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("統合テスト用のDB接続先が未指定です")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}

	admin := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { admin.Close() })

	schema := pgx.Identifier{
		"learningrecord_test_" + uuid.NewString(),
	}.Sanitize()

	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("テスト用スキーマの削除に失敗: %v", err)
		}
	})

	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { db.Close() })

	for _, name := range []string{
		"000002_auth.up.sql",
		"000004_learning_records.up.sql",
	} {
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}

		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("%sの適用に失敗: %v", name, err)
		}
	}

	return db
}
