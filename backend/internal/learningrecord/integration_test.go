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
