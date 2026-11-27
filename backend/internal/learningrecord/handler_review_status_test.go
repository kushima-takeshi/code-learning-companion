package learningrecord

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWebLearningRecordReviewStatusHTTPIntegration(t *testing.T) {
	db, bearerHandler, tokens := newListHTTP(t, reviewMigration)
	now := time.Date(2026, 12, 3, 10, 15, 0, 0, time.UTC)
	handler := newReviewHTTP(t, db, func() time.Time { return now })
	const detailPath = "/web/learning-records/"

	for i, tc := range []struct {
		name        string
		age         time.Duration
		savedStages int
		delay       time.Duration
		due         bool
	}{
		{name: "before_deadline", age: 72*time.Hour - time.Microsecond, delay: 72 * time.Hour},
		{name: "at_deadline", age: 72 * time.Hour, delay: 72 * time.Hour, due: true},
		{name: "past_deadline_stage_2", age: 337 * time.Hour, savedStages: 1, delay: 336 * time.Hour, due: true},
		{name: "completed", age: 721 * time.Hour, savedStages: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner := listUser(t, db, i+1)
			cookie := learningSession(t, db, owner, time.Now().Add(time.Hour))
			id := uuid.NewString()
			createdAt := now.Add(-tc.age)
			insertReviewRecord(t, db, id, owner, createdAt)
			for stage := 1; stage <= tc.savedStages; stage++ {
				body := fmt.Sprintf(`{"stage":%d,"result":"できた"}`, stage)
				res := postReview(t.Context(), handler, id, body, "https://app.example", cookie)
				if res.Code != http.StatusCreated {
					t.Fatalf("段階%dの保存: %d %s", stage, res.Code, res.Body.String())
				}
			}

			res := listRequest(handler, http.MethodGet, detailPath+id, "", cookie)
			detail := decodeWebRecordJSON[struct {
				Review map[string]json.RawMessage `json:"review"`
			}](t, res)
			review := detail.Review
			due := decodeWebRecordJSON[dueReviewsResponse](t, listRequest(handler, http.MethodGet, dueReviewsPath, "", cookie))
			if tc.savedStages == 3 {
				if len(review) != 1 || string(review["completed"]) != "true" || len(due.Records) != 0 {
					t.Fatalf("完了時の詳細 = %s, 期限一覧 = %+v", res.Body.String(), due.Records)
				}
				return
			}

			wantStage := tc.savedStages + 1
			var dueAt time.Time
			if len(review) != 4 || string(review["completed"]) != "false" ||
				string(review["stage"]) != strconv.Itoa(wantStage) ||
				string(review["due"]) != strconv.FormatBool(tc.due) ||
				json.Unmarshal(review["due_at"], &dueAt) != nil || !dueAt.Equal(createdAt.Add(tc.delay)) {
				t.Fatalf("詳細 = %s, 期待する段階 = %d, 期限 = %s, 期限到来 = %t",
					res.Body.String(), wantStage, createdAt.Add(tc.delay), tc.due)
			}
			if tc.due {
				if len(due.Records) != 1 || due.Records[0].ID != id ||
					due.Records[0].Stage != wantStage || !due.Records[0].DueAt.Equal(dueAt) {
					t.Fatalf("詳細 = %s, 期限一覧 = %+v", res.Body.String(), due.Records)
				}
			} else if len(due.Records) != 0 {
				t.Fatalf("期限前の記録が期限一覧にある: %+v", due.Records)
			}
		})
	}

	t.Run("bearer_detail_matches_save", func(t *testing.T) {
		owner := listUser(t, db, 10)
		token, err := tokens.Issue(t.Context(), owner, "owner")
		if err != nil {
			t.Fatal(err)
		}
		input := validSaveInputForTest(1)
		saved := recordRequest(bearerHandler, http.MethodPost, "/learning-records", token.Secret, "application/json", inputJSON(t, input), nil)
		id := checkRecordResponse(t, saved, input)
		detail := listRequest(bearerHandler, http.MethodGet, "/learning-records/"+id, token.Secret, nil)
		if checkRecordResponse(t, detail, input) != id || detail.Body.String() != saved.Body.String() {
			t.Fatalf("Bearer詳細 = %s, 保存応答 = %s", detail.Body.String(), saved.Body.String())
		}
	})
}
