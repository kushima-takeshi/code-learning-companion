package learningrecord

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestParseListQuery(t *testing.T) {
	cases := []struct {
		query         string
		limit, offset int
	}{
		{"", 20, 0}, {"limit=1", 1, 0}, {"limit=100&offset=2", 100, 2},
		{"offset=" + strconv.Itoa(int(^uint(0)>>1)), 20, int(^uint(0) >> 1)},
	}
	if strconv.IntSize > 32 {
		largeOffset := int64(1) << 31
		cases = append(cases, struct {
			query         string
			limit, offset int
		}{"offset=2147483648", 20, int(largeOffset)})
	}
	for _, tc := range cases {
		limit, offset, err := parseListQuery(tc.query)
		if err != nil || limit != tc.limit || offset != tc.offset {
			t.Fatalf("%q: %d %d %v", tc.query, limit, offset, err)
		}
	}
	for _, query := range []string{
		"limit=0", "limit=101", "limit=-1", "limit=", "limit=1.5", "limit=abc",
		"offset=-1", "offset=", "offset=999999999999999999999999",
		"offset=" + strconv.FormatUint(uint64(^uint(0)>>1)+1, 10),
		"offset=+1", "limit=20&limit=10", "offset=0&offset=1", "user_id=someone",
		"cursor=anything", "limit=%zz", "limit=20;offset=0", "offset=%2B1",
	} {
		if _, _, err := parseListQuery(query); err != ErrInvalidPagination {
			t.Fatalf("%q must be rejected: %v", query, err)
		}
	}
}

func TestListHandlerRequiresTokenContext(t *testing.T) {
	res := httptest.NewRecorder()
	NewHandler(nil).List(res, httptest.NewRequest("GET", "/learning-records", nil))
	checkListError(t, res, 401, "unauthorized")
}

func checkListError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if res.Code != status || res.Header().Get("Cache-Control") != "no-store" || res.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("response: %d %v %s", res.Code, res.Header(), res.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil || len(got) != 1 || got["error"] != code {
		t.Fatalf("error response: %s %v", res.Body.String(), err)
	}
	if strings.Contains(res.Body.String(), "SQL") {
		t.Fatal("must not expose SQL errors")
	}
}
