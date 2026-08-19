package pagination_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/google/uuid"
)

func TestParseClampLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=999", nil)
	page := pagination.Parse(r, pagination.Limits{Default: 50, Max: 100}, pagination.Desc)
	if page.Limit != 100 {
		t.Fatalf("limit=%d", page.Limit)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	id := uuid.New()
	cur := pagination.EncodeCursor(at, id)
	gotAt, gotID, err := pagination.DecodeCursor(cur)
	if err != nil {
		t.Fatal(err)
	}
	if !gotAt.Equal(at) || gotID != id {
		t.Fatalf("got %v %v", gotAt, gotID)
	}
}

func TestTrimPage(t *testing.T) {
	id := uuid.New()
	items := []string{"a", "b", "c"}
	page := pagination.Page{Limit: 2}
	res := pagination.TrimPage(page, items, func(s string) (time.Time, uuid.UUID) {
		return time.Unix(1, 0), id
	})
	if len(res.Items) != 2 || res.NextCursor == "" {
		t.Fatalf("%+v", res)
	}
}
