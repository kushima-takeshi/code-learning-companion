package connectiontoken

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestManageIntegration(t *testing.T) {
	db := newIntegrationDB(t)
	owner, other := createUser(t, db), createUser(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	service := New(db)
	service.now = func() time.Time { return now }
	empty, err := service.List(ctx, owner)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("未発行なら空の一覧を期待: %v", err)
	}
	first, err := service.Issue(ctx, owner, "自宅PC")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	second, err := service.Issue(ctx, owner, "ノートPC")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := service.Issue(ctx, other, "別ユーザーのPC")
	if err != nil {
		t.Fatal(err)
	}
	list, err := service.List(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list, []Token{second.Token, first.Token}) {
		t.Fatal("一覧は本人の接続だけを新しい順に返す")
	}
	metadata, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{first.Secret, second.Secret, foreign.Secret} {
		if strings.Contains(string(metadata), secret) {
			t.Fatal("一覧は原本を返さない")
		}
	}

	for _, tc := range []struct{ name, userID, tokenID string }{
		{name: "他人のトークン", userID: other, tokenID: first.Token.ID},
		{name: "存在しないトークン", userID: owner, tokenID: uuid.NewString()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := service.Revoke(ctx, tc.userID, tc.tokenID); !errors.Is(err, ErrTokenNotFound) {
				t.Fatalf("存在なしと他人は同じエラーで拒否: %v", err)
			}
		})
	}
	if _, err := service.Authenticate(ctx, first.Secret); err != nil {
		t.Fatal("他人の失効要求で本人の接続を止めない")
	}
	now = now.Add(time.Hour)
	revokedAt := now
	if err := service.Revoke(ctx, owner, first.Token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, first.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("失効後は照合できない: %v", err)
	}
	for _, secret := range []string{second.Secret, foreign.Secret} {
		if _, err := service.Authenticate(ctx, secret); err != nil {
			t.Fatalf("ほかの接続は失効の影響を受けない: %v", err)
		}
	}
	// 繰り返しても成功し、最初の失効日時を保持する。
	now = now.Add(time.Hour)
	if err := service.Revoke(ctx, owner, first.Token.ID); err != nil {
		t.Fatal(err)
	}
	list, err = service.List(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[1].RevokedAt == nil || !list[1].RevokedAt.Equal(revokedAt) {
		t.Fatal("失効済み記録と最初の失効日時を一覧に残す")
	}
}

func TestManageIntegrationDBError(t *testing.T) {
	db := newIntegrationDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	service := New(db)
	got, err := service.List(context.Background(), uuid.NewString())
	if err == nil || got != nil {
		t.Fatal("一覧のDBエラーでは部分結果を返さない")
	}
	err = service.Revoke(context.Background(), uuid.NewString(), uuid.NewString())
	if err == nil || errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("DB障害は存在なしと区別する: %v", err)
	}
}
