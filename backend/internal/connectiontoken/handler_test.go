package connectiontoken

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/config"
)

func TestManagementMissingWebSessionDoesNotUseDB(t *testing.T) {
	h := managementHTTP(t, nil)
	for _, endpoint := range []struct{ method, path string }{
		{"POST", "/connection-tokens"},
		{"GET", "/connection-tokens"},
		{"POST", "/connection-tokens/00000000-0000-0000-0000-000000000001/revoke"},
	} {
		checkHTTPError(t, managementRequest(h, endpoint.method, endpoint.path, "", nil), 401, "unauthorized")
	}
	// Even accidental direct invocation of handlers fails closed without a user.
	for _, method := range []http.HandlerFunc{NewHandler(nil).Issue, NewHandler(nil).List, NewHandler(nil).Revoke} {
		res := httptest.NewRecorder()
		method(res, httptest.NewRequest("POST", "/", nil))
		checkHTTPError(t, res, 401, "unauthorized")
	}
}

func TestManagementRejectsInvalidTrustedOrigin(t *testing.T) {
	mux := http.NewServeMux()
	login := auth.NewHandler(config.Config{}, auth.NewStore(nil))
	if err := NewHandler(nil).RegisterRoutes(mux, login.RequireAuth, "://bad"); err == nil {
		t.Fatal("不正な信頼originは起動時に拒否")
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest("GET", "/connection-tokens", nil))
	if res.Code != 404 {
		t.Fatal("設定不正ならルートを一部だけ登録しない")
	}
}
