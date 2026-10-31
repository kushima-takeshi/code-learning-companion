package connectiontoken

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAuthenticateRejectsMalformedTokenBeforeDB(t *testing.T) {
	for _, secret := range []string{"", "invalid", strings.Repeat("a", 64), "clc_" + strings.Repeat("g", 64), "clc_" + strings.Repeat("a", 65)} {
		got, err := New(nil).Authenticate(context.Background(), secret)
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("形式不正は無効なトークンとして拒否: %v", err)
		}
		if !reflect.DeepEqual(got, Token{}) {
			t.Fatal("無効なトークンでは所有者情報を返さない")
		}
	}
}

func TestAuthenticateIntegrationExpiry(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	service := New(db)
	service.now = func() time.Time { return now }
	issued, err := service.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		at    time.Time
		valid bool
	}{
		{name: "発行直後", at: issued.Token.CreatedAt, valid: true},
		{name: "期限直前", at: issued.Token.ExpiresAt.Add(-time.Microsecond), valid: true},
		{name: "期限ちょうど", at: issued.Token.ExpiresAt},
		{name: "期限後", at: issued.Token.ExpiresAt.Add(time.Microsecond)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now = tc.at
			got, err := service.Authenticate(ctx, issued.Secret)
			if tc.valid {
				if err != nil {
					t.Fatalf("期限内は認証できる: %v", err)
				}
				if !reflect.DeepEqual(got, issued.Token) {
					t.Fatal("照合結果は元の所有者・期限を持つ")
				}
			} else {
				if !errors.Is(err, ErrInvalidToken) {
					t.Fatalf("期限切れは拒否: %v", err)
				}
				if !reflect.DeepEqual(got, Token{}) {
					t.Fatal("期限切れでは情報を返さない")
				}
			}
		})
	}
	var expires time.Time
	if err := db.QueryRowContext(ctx, `SELECT expires_at FROM connection_tokens WHERE id=$1`, issued.Token.ID).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	if !expires.Equal(issued.Token.ExpiresAt) {
		t.Fatal("照合・利用で期限を自動延長しない")
	}
}

func TestAuthenticateIntegrationPreservesStoredExpiry(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	createdAt := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	now := createdAt
	service := New(db)
	service.now = func() time.Time { return now }
	issued, err := service.Issue(ctx, owner, "既存の接続")
	if err != nil {
		t.Fatal(err)
	}
	// Model a token persisted before the lifetime changed to 90 days.
	storedExpiry := createdAt.Add(30 * 24 * time.Hour)
	if _, err := db.ExecContext(ctx, `UPDATE connection_tokens SET expires_at=$1 WHERE id=$2`, storedExpiry, issued.Token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Issue(ctx, owner, "新しい接続"); err != nil {
		t.Fatal(err)
	}

	now = storedExpiry.Add(-time.Microsecond)
	got, err := service.Authenticate(ctx, issued.Secret)
	if err != nil || got.ID != issued.Token.ID || got.UserID != owner || !got.ExpiresAt.Equal(storedExpiry) {
		t.Fatalf("既存トークンは保存済み期限の直前まで有効: got %+v, err %v", got, err)
	}
	for _, at := range []time.Time{storedExpiry, createdAt.Add(31 * 24 * time.Hour)} {
		now = at
		got, err := service.Authenticate(ctx, issued.Secret)
		if !errors.Is(err, ErrInvalidToken) || !reflect.DeepEqual(got, Token{}) {
			t.Fatalf("既存の30日期限を90日まで延長しない: at %v, got %+v, err %v", at, got, err)
		}
	}
	var expires time.Time
	if err := db.QueryRowContext(ctx, `SELECT expires_at FROM connection_tokens WHERE id=$1`, issued.Token.ID).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	if !expires.Equal(storedExpiry) {
		t.Fatal("新規発行・照合で既存トークンの保存済み期限を書き換えない")
	}
}

func TestAuthenticateIntegrationUnknownAndRevoked(t *testing.T) {
	db := newIntegrationDB(t)
	owner := createUser(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	service := New(db)
	issued, err := service.Issue(ctx, owner, "PC")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE connection_tokens SET revoked_at=now() WHERE id=$1`, issued.Token.ID); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{issued.Secret, "clc_" + strings.Repeat("0", 64)} {
		got, err := service.Authenticate(ctx, secret)
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("未発行・失効済みは拒否: %v", err)
		}
		if !reflect.DeepEqual(got, Token{}) {
			t.Fatal("無効なら所有者情報を返さない")
		}
	}
}

func TestAuthenticateIntegrationDBError(t *testing.T) {
	db := newIntegrationDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := New(db).Authenticate(context.Background(), "clc_"+strings.Repeat("0", 64))
	if err == nil || errors.Is(err, ErrInvalidToken) {
		t.Fatalf("DB障害は認証不成立と区別する: %v", err)
	}
	if !reflect.DeepEqual(got, Token{}) {
		t.Fatal("DB障害時は情報を返さない")
	}
}
