package connectiontoken

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireTokenRejectsMalformedCredentialsBeforeDB(t *testing.T) {
	for _, values := range [][]string{
		nil, {""}, {"Basic abc"}, {"Bearer"}, {"Bearer a b"},
		{"Bearer clc_bad"}, {"Bearer " + strings.Repeat("x", 68)},
		{"Bearer clc_" + strings.Repeat("z", 64)},
		{"Bearer a", "Bearer b"},
	} {
		req := httptest.NewRequest("GET", "/learning-records", nil)
		for _, value := range values {
			req.Header.Add("Authorization", value)
		}
		res := httptest.NewRecorder()
		New(nil).RequireToken(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("rejected requests must not reach handler")
		})).ServeHTTP(res, req)
		checkHTTPError(t, res, 401, "unauthorized")
		if res.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatal("missing Bearer challenge")
		}
	}
	if _, ok := TokenFromContext(context.Background()); ok {
		t.Fatal("empty context must not authenticate")
	}
}
