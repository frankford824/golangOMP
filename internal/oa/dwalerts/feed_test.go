package dwalerts

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeedRequiresAdministrator(t *testing.T) {
	w := httptest.NewRecorder()
	List(w, httptest.NewRequest(http.MethodGet, "/oa-api/data-alerts", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d", w.Code)
	}
}
