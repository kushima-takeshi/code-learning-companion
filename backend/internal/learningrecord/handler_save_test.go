package learningrecord

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/auth"
)

func recordRequest(h http.Handler, method, path, secret, contentType, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func inputJSON(t *testing.T, input SaveInput) string {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Decode independently of the production response type to check the wire contract.
func checkRecordResponse(t *testing.T, res *httptest.ResponseRecorder, input SaveInput) string {
	t.Helper()
	if res.Code != 200 || res.Header().Get("Cache-Control") != "no-store" || res.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("response: %d %v %s", res.Code, res.Header(), res.Body.String())
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &fields); err != nil || len(fields) != 5 {
		t.Fatalf("response fields: %s %v", res.Body.String(), err)
	}
	for _, key := range []string{"id", "request_id", "theme", "questions", "created_at"} {
		if fields[key] == nil {
			t.Fatalf("missing field %s: %s", key, res.Body.String())
		}
	}
	var got struct {
		ID        string     `json:"id"`
		RequestID string     `json:"request_id"`
		Theme     string     `json:"theme"`
		Questions []Question `json:"questions"`
		CreatedAt time.Time  `json:"created_at"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(got.ID); err != nil || got.CreatedAt.IsZero() || got.RequestID != input.RequestID || got.Theme != input.Theme || !reflect.DeepEqual(got.Questions, input.Questions) {
		t.Fatalf("record content: %+v (%v)", got, err)
	}
	return got.ID
}

func TestLearningSaveHTTPIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	owner, other := listUser(t, db, 1), listUser(t, db, 2)
	ctx := context.Background()
	token, err := tokens.Issue(ctx, owner, "owner")
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := tokens.Issue(ctx, other, "other")
	if err != nil {
		t.Fatal(err)
	}
	secret, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(db).CreateSession(ctx, other, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: "session", Value: secret}
	for _, count := range []int{1, 10} {
		input := validSaveInputForTest(count)
		input.RequestID = uuid.NewString()
		body := inputJSON(t, input)
		res := recordRequest(h, "POST", "/learning-records", token.Secret, "application/json; charset=utf-8", body, cookie)
		id := checkRecordResponse(t, res, input)
		saved, err := New(db).Get(ctx, owner, id)
		if err != nil || saved.UserID != owner || !reflect.DeepEqual(saved.Questions, input.Questions) {
			t.Fatalf("stored record: %+v %v", saved, err)
		}
		retry := recordRequest(h, "POST", "/learning-records", token.Secret, "application/json", body, nil)
		if checkRecordResponse(t, retry, input) != id || retry.Body.String() != res.Body.String() {
			t.Fatal("retry must return the original record")
		}
		input.Theme = "different content"
		checkListError(t, recordRequest(h, "POST", "/learning-records", token.Secret, "application/json", inputJSON(t, input), nil), 409, "request_conflict")
		unchanged, err := New(db).Get(ctx, owner, id)
		if err != nil || !reflect.DeepEqual(unchanged, saved) {
			t.Fatal("conflict changed original record", err)
		}
		// The request ID belongs to the authenticated owner, not to a global namespace.
		otherRes := recordRequest(h, "POST", "/learning-records", otherToken.Secret, "application/json", body, nil)
		if checkRecordResponse(t, otherRes, SaveInput{RequestID: saved.RequestID, Theme: saved.Theme, Questions: saved.Questions}) == id {
			t.Fatal("owners share a record")
		}
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM learning_records WHERE user_id=$1 AND request_id=$2`, owner, input.RequestID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("duplicate records: %d %v", n, err)
		}
	}
	var expires time.Time
	if err := db.QueryRow(`SELECT expires_at FROM connection_tokens WHERE id=$1`, token.Token.ID).Scan(&expires); err != nil || !expires.Equal(token.Token.ExpiresAt) {
		t.Fatal("save changed expiry", err)
	}
}

func TestLearningSaveInvalidHTTPIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	owner := listUser(t, db, 1)
	token, err := tokens.Issue(context.Background(), owner, "owner")
	if err != nil {
		t.Fatal(err)
	}
	valid := inputJSON(t, validSaveInputForTest(1))
	for _, tc := range []struct {
		name, ct, body string
		status         int
		code           string
	}{
		{"missing content type", "", valid, 415, "unsupported_media_type"},
		{"plain text", "text/plain", valid, 415, "unsupported_media_type"},
		{"broken content type", "application/json; charset", valid, 415, "unsupported_media_type"},
		{"empty", "application/json", "", 400, "invalid_json"},
		{"broken JSON", "application/json", "{", 400, "invalid_json"},
		{"wrong type", "application/json", `{"theme":42}`, 400, "invalid_json"},
		{"null", "application/json", "null", 400, "invalid_json"},
		{"array", "application/json", "[]", 400, "invalid_json"},
		{"multiple JSON", "application/json", valid + " null", 400, "invalid_json"},
		{"unknown owner", "application/json", strings.TrimSuffix(valid, "}") + `,"user_id":"someone"}`, 400, "invalid_json"},
		{"unknown nested", "application/json", strings.Replace(valid, `"hint_used":false`, `"hint_used":false,"extra":true`, 1), 400, "invalid_json"},
		{"oversize", "application/json", valid + strings.Repeat(" ", MaxSaveInputBytes), 413, "request_too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkListError(t, recordRequest(h, "POST", "/learning-records", token.Secret, tc.ct, tc.body, nil), tc.status, tc.code)
		})
	}
	for _, input := range []SaveInput{{}, validSaveInputForTest(0), validSaveInputForTest(11)} {
		res := recordRequest(h, "POST", "/learning-records", token.Secret, "application/json", inputJSON(t, input), nil)
		var result struct {
			Error   string       `json:"error"`
			Details []FieldError `json:"details"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || res.Code != 400 || result.Error != "validation_failed" || len(result.Details) == 0 || res.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("validation: %d %s %v", res.Code, res.Body.String(), err)
		}
	}
	input := validSaveInputForTest(1)
	input.Questions[0].Answer.HintUsed = nil
	res := recordRequest(h, "POST", "/learning-records", token.Secret, "application/json", inputJSON(t, input), nil)
	if !strings.Contains(res.Body.String(), `"path":"questions[0].answer.hint_used"`) || !strings.Contains(res.Body.String(), `"code":"required"`) || res.Code != 400 {
		t.Fatalf("field details: %d %s", res.Code, res.Body.String())
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM learning_records`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("invalid input saved: %d %v", n, err)
	}
	// Exact byte limit is accepted, including when the length is not known in advance.
	valid += strings.Repeat(" ", MaxSaveInputBytes-len(valid))
	req := httptest.NewRequest("POST", "/learning-records", io.NopCloser(strings.NewReader(valid)))
	req.Header.Set("Authorization", "Bearer "+token.Secret)
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, req)
	checkRecordResponse(t, res, validSaveInputForTest(1))
}

func TestLearningSaveConcurrentHTTPIntegration(t *testing.T) {
	db, h, tokens := newListHTTP(t)
	owner := listUser(t, db, 1)
	token, err := tokens.Issue(context.Background(), owner, "owner")
	if err != nil {
		t.Fatal(err)
	}
	input := validSaveInputForTest(1)
	body := inputJSON(t, input)
	const count = 6
	results := make(chan *httptest.ResponseRecorder, count)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range count {
		wg.Go(func() {
			<-start
			results <- recordRequest(h, "POST", "/learning-records", token.Secret, "application/json", body, nil)
		})
	}
	close(start)
	wg.Wait()
	close(results)
	var id string
	for res := range results {
		got := checkRecordResponse(t, res, input)
		if id != "" && got != id {
			t.Fatal("concurrent retry created another ID")
		}
		id = got
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM learning_records`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("concurrent records: %d %v", n, err)
	}
}
