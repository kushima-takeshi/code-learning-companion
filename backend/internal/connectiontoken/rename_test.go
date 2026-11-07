package connectiontoken

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRenameIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner, other := createUser(t, db), createUser(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	service := New(db)
	issued, err := service.Issue(ctx, owner, "codex")
	if err != nil {
		t.Fatal(err)
	}
	kept, err := service.Issue(ctx, owner, "残す接続")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := service.Issue(ctx, other, "別ユーザー")
	if err != nil {
		t.Fatal(err)
	}

	renamed, err := service.Rename(ctx, owner, issued.Token.ID, "  cursor  ")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != issued.Token.ID || renamed.UserID != owner || renamed.Name != "cursor" || renamed.RevokedAt != nil {
		t.Fatalf("表示名だけを変える: %+v", renamed)
	}
	if !renamed.CreatedAt.Equal(issued.Token.CreatedAt) || !renamed.ExpiresAt.Equal(issued.Token.ExpiresAt) {
		t.Fatal("作成日時と期限は変えない")
	}
	authenticated, err := service.Authenticate(ctx, issued.Secret)
	if err != nil || authenticated.Name != "cursor" || authenticated.ID != issued.Token.ID {
		t.Fatalf("原本はそのまま照合できる: %+v %v", authenticated, err)
	}
	var hash []byte
	if err := db.QueryRowContext(ctx, `SELECT token_hash FROM connection_tokens WHERE id=$1`, issued.Token.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(issued.Secret))
	if !bytes.Equal(hash, sum[:]) {
		t.Fatal("名前変更でハッシュを書き換えない")
	}

	again, err := service.Rename(ctx, owner, issued.Token.ID, "cursor")
	if err != nil || again.Name != "cursor" || !again.ExpiresAt.Equal(issued.Token.ExpiresAt) {
		t.Fatalf("同じ名前への変更も成功する: %+v %v", again, err)
	}
	if _, err := service.Rename(ctx, owner, issued.Token.ID, "  "); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("空白だけの名前は拒否: %v", err)
	}
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM connection_tokens WHERE id=$1`, issued.Token.ID).Scan(&name); err != nil || name != "cursor" {
		t.Fatalf("拒否した変更は保存しない: %s %v", name, err)
	}

	for _, tc := range []struct{ name, userID, tokenID string }{
		{name: "他人のトークン", userID: other, tokenID: issued.Token.ID},
		{name: "存在しないトークン", userID: owner, tokenID: uuid.NewString()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Rename(ctx, tc.userID, tc.tokenID, "乗っ取り")
			if !errors.Is(err, ErrTokenNotFound) {
				t.Fatalf("存在なしと他人は同じエラーで拒否: %v", err)
			}
		})
	}
	var foreignName, keptName string
	if err := db.QueryRowContext(ctx, `SELECT name FROM connection_tokens WHERE id=$1`, foreign.Token.ID).Scan(&foreignName); err != nil || foreignName != "別ユーザー" {
		t.Fatalf("他人の接続名を変えない: %s %v", foreignName, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT name FROM connection_tokens WHERE id=$1`, kept.Token.ID).Scan(&keptName); err != nil || keptName != "残す接続" {
		t.Fatalf("別の接続には影響しない: %s %v", keptName, err)
	}

	if _, err := service.Rename(ctx, owner, kept.Token.ID, "cursor"); err != nil {
		t.Fatal(err)
	}
	list, err := service.List(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]int{}
	for _, token := range list {
		names[token.Name]++
	}
	if names["cursor"] != 2 {
		t.Fatalf("同名の接続を拒否しない: %+v", list)
	}

	revoked, err := service.Issue(ctx, owner, "失効済み")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Revoke(ctx, owner, revoked.Token.ID); err != nil {
		t.Fatal(err)
	}
	revokedRenamed, err := service.Rename(ctx, owner, revoked.Token.ID, "失効後の名前")
	if err != nil || revokedRenamed.Name != "失効後の名前" || revokedRenamed.RevokedAt == nil {
		t.Fatalf("失効済みでも所有者なら表示名を変え、失効日時は残す: %+v %v", revokedRenamed, err)
	}
	var revokedAt time.Time
	if err := db.QueryRowContext(ctx, `SELECT revoked_at FROM connection_tokens WHERE id=$1`, revoked.Token.ID).Scan(&revokedAt); err != nil || !revokedAt.Equal(*revokedRenamed.RevokedAt) {
		t.Fatalf("名前変更で失効日時を動かさない: %v %v", revokedAt, err)
	}

	past := New(db)
	past.now = func() time.Time { return time.Now().Add(-Lifetime - time.Hour) }
	expired, err := past.Issue(ctx, owner, "期限切れ")
	if err != nil {
		t.Fatal(err)
	}
	expiredRenamed, err := service.Rename(ctx, owner, expired.Token.ID, "期限後の名前")
	if err != nil || expiredRenamed.Name != "期限後の名前" || !expiredRenamed.ExpiresAt.Equal(expired.Token.ExpiresAt) {
		t.Fatalf("期限切れでも所有者なら表示名を変え、期限は残す: %+v %v", expiredRenamed, err)
	}

	unicode, err := service.Rename(ctx, owner, issued.Token.ID, "  コーデックス  ")
	if err != nil || unicode.Name != "コーデックス" {
		t.Fatalf("前後の空白を除いた名前を保存する: %+v %v", unicode, err)
	}
}

func TestRenameIntegrationDBError(t *testing.T) {
	db := newIntegrationDB(t)
	service := New(db)
	if _, err := service.Rename(context.Background(), uuid.NewString(), uuid.NewString(), "  "); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("空の名前はDBを見る前に拒否: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := service.Rename(context.Background(), uuid.NewString(), uuid.NewString(), "cursor")
	if err == nil || errors.Is(err, ErrTokenNotFound) || errors.Is(err, ErrInvalidName) {
		t.Fatalf("DB障害は存在なしや名前不正と区別する: %v", err)
	}
}
