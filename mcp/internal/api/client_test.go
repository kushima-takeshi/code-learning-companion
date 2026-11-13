package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRejectsEmptyToken(t *testing.T) {
	_, err := New("http://localhost:8080", "  ")
	if err == nil || strings.Contains(err.Error(), "clc_") {
		t.Fatalf("error = %v", err)
	}
}

func TestDoJSONSendsBearerAndJSON(t *testing.T) {
	const token = "clc_secret_value"
	var gotAuth, gotType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		if r.Header.Get("Cookie") != "" {
			t.Errorf("cookie = %q", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"records":[],"has_more":false}`))
	}))
	defer server.Close()

	client, err := New(server.URL+"/", token)
	if err != nil {
		t.Fatal(err)
	}
	status, body, err := client.DoJSON(context.Background(), http.MethodPost, "/learning-records", map[string]string{"theme": "loops"})
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || string(body) != `{"records":[],"has_more":false}` {
		t.Fatalf("status %d body %s", status, body)
	}
	if gotAuth != "Bearer "+token {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotType != "application/json" || gotBody != `{"theme":"loops"}` {
		t.Fatalf("type %q body %s", gotType, gotBody)
	}
}

func TestDoJSONGetOmitsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.ContentLength > 0 || r.Header.Get("Content-Type") != "" {
			t.Errorf("method %s length %d type %q", r.Method, r.ContentLength, r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "clc_secret_value")
	if err != nil {
		t.Fatal(err)
	}
	status, body, err := client.DoJSON(context.Background(), http.MethodGet, "learning-records", nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusUnauthorized || string(body) != `{"error":"unauthorized"}` {
		t.Fatalf("status %d body %s", status, body)
	}
}

func TestDoJSONHidesTokenInTransportError(t *testing.T) {
	const token = "clc_secret_value"
	client, err := New("http://127.0.0.1:9", token)
	if err != nil {
		t.Fatal(err)
	}
	client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed for " + token)
	})}
	_, _, err = client.DoJSON(context.Background(), http.MethodGet, "/learning-records", nil)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), token) || !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("error leaked token: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
