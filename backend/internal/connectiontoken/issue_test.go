package connectiontoken

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIssueRejectsBlankName(t *testing.T) {
	for _, name := range []string{"", " \t\n "} {
		got, err := New(nil).Issue(context.Background(), uuid.NewString(), name)
		if !errors.Is(err, ErrInvalidName) {
			t.Fatalf("接続名の検証エラーを期待: %v", err)
		}
		if !reflect.DeepEqual(got, IssuedToken{}) {
			t.Fatal("入力不正なら発行結果は空")
		}
	}
}

func TestIssueIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	service := New(db)
	service.now = func() time.Time { return now }
	issued, err := service.Issue(ctx, owner, " 自宅PC ")
	if err != nil {
		t.Fatal(err)
	}
	if issued.Token.ID == "" || issued.Token.UserID != owner || issued.Token.Name != "自宅PC" {
		t.Fatal("発行メタデータが不正")
	}
	if !issued.Token.CreatedAt.Equal(now) || !issued.Token.ExpiresAt.Equal(now.Add(30*24*time.Hour)) || issued.Token.RevokedAt != nil {
		t.Fatal("発行時刻・30日の期限・失効状態が不正")
	}
	if !strings.HasPrefix(issued.Secret, "clc_") || len(issued.Secret) != 68 {
		t.Fatal("発行トークンの形式が不正")
	}
	var hash []byte
	if err := db.QueryRowContext(ctx, `SELECT token_hash FROM connection_tokens WHERE id=$1`, issued.Token.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256([]byte(issued.Secret))
	if !bytes.Equal(hash, wantHash[:]) || bytes.Equal(hash, []byte(issued.Secret)) {
		t.Fatal("DBは原本でなくSHA-256ハッシュを保存する")
	}
	second, err := service.Issue(ctx, owner, "ノートPC")
	if err != nil {
		t.Fatal(err)
	}
	if second.Secret == issued.Secret || second.Token.ID == issued.Token.ID {
		t.Fatal("別の発行では別のトークンとIDを作る")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM connection_tokens WHERE user_id=$1`, owner).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("発行件数: got %d, want 2", count)
	}
}

func TestIssueIntegrationFailureReturnsNoSecret(t *testing.T) {
	db := newIntegrationDB(t)
	got, err := New(db).Issue(context.Background(), uuid.NewString(), "PC")
	if err == nil {
		t.Fatal("存在しない所有者では発行できない")
	}
	if !reflect.DeepEqual(got, IssuedToken{}) {
		t.Fatal("保存失敗ではトークン原本もメタデータも返さない")
	}
}
