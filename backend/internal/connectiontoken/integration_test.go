package connectiontoken

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestMigrationIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	owner := createUser(t, db)
	insert := `INSERT INTO connection_tokens (user_id, name, token_hash, expires_at) VALUES ($1, 'PC', $2, now() + interval '30 days')`
	hash := make([]byte, 32)
	if _, err := db.ExecContext(ctx, insert, owner, hash); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, owner string
		hash        []byte
		code        string
	}{
		{name: "ハッシュの重複を拒否", owner: owner, hash: hash, code: "23505"},
		{name: "存在しない所有者を拒否", owner: uuid.NewString(), hash: append([]byte{1}, make([]byte, 31)...), code: "23503"},
		{name: "不正なハッシュ長を拒否", owner: owner, hash: []byte{1}, code: "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.ExecContext(ctx, insert, tc.owner, tc.hash)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("DBエラー%sを期待: %v", tc.code, err)
			}
		})
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, owner); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM connection_tokens`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("所有者削除後のトークン件数: %d", count)
	}
	applyMigration(t, db, "000005_connection_tokens.down.sql")
	var relation sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('connection_tokens')::text`).Scan(&relation); err != nil {
		t.Fatal(err)
	}
	if relation.Valid {
		t.Fatal("down後にテーブルが残っています")
	}
	applyMigration(t, db, "000005_connection_tokens.up.sql")
}

func newIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("CONNECTIONTOKEN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("統合テスト用のDB接続先が未指定です")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { admin.Close() })
	schema := pgx.Identifier{"connectiontoken_test_" + uuid.NewString()}.Sanitize()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("スキーマ削除に失敗: %v", err)
		}
	})
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { db.Close() })
	applyMigration(t, db, "000002_auth.up.sql")
	applyMigration(t, db, "000005_connection_tokens.up.sql")
	return db
}

func applyMigration(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, string(data)); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func createUser(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var id string
	if err := db.QueryRowContext(ctx, `INSERT INTO users (github_user_id, github_login) VALUES ((SELECT COALESCE(max(github_user_id),0)+1 FROM users), 'test-user') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
