package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/go-beginner-kit/internal/study"
)

func request(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestRecordFlow(t *testing.T) {
	h := newHandler(study.NewStore())
	w := request(h, "GET", "/records", "")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("initial list = %d %s", w.Code, w.Body.String())
	}
	w = request(h, "POST", "/records", `{"title":" Go入門 ","minutes":25}`)
	if w.Code != 201 || w.Header().Get("Location") != "/records/1" {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatal("missing JSON content type")
	}
	w = request(h, "GET", "/records/1", "")
	var got study.Record
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || got.ID != 1 || got.Title != "Go入門" || got.Minutes != 25 {
		t.Fatalf("get = %d %#v", w.Code, got)
	}
	w = request(h, "GET", "/records", "")
	var list []study.Record
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(list) != 1 || list[0] != got {
		t.Fatalf("list = %d %#v", w.Code, list)
	}
}

func TestInvalidRequests(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"invalid JSON", "POST", "/records", `{`, 400},
		{"empty body", "POST", "/records", "", 400},
		{"null", "POST", "/records", `null`, 400},
		{"empty title", "POST", "/records", `{"title":" ","minutes":25}`, 400},
		{"zero", "POST", "/records", `{"title":"Go","minutes":0}`, 400},
		{"fraction", "POST", "/records", `{"title":"Go","minutes":1.5}`, 400},
		{"string minutes", "POST", "/records", `{"title":"Go","minutes":"25"}`, 400},
		{"unknown field", "POST", "/records", `{"title":"Go","minutes":25,"typo":1}`, 400},
		{"extra JSON", "POST", "/records", `{"title":"Go","minutes":25} {}`, 400},
		{"oversize", "POST", "/records", `{"title":"` + strings.Repeat("a", (1<<20)+1) + `","minutes":25}`, 400},
		{"bad id", "GET", "/records/abc", "", 400},
		{"nonpositive id", "GET", "/records/0", "", 400},
		{"missing record", "GET", "/records/999", "", 404},
		{"unknown path", "GET", "/unknown", "", 404},
		{"unsupported method", "DELETE", "/records", "", 405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := study.NewStore()
			w := request(newHandler(s), tt.method, tt.path, tt.body)
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d, body %s", w.Code, tt.want, w.Body.String())
			}
			if len(s.List()) != 0 {
				t.Fatal("invalid request changed the store")
			}
		})
	}
}
