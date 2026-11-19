package orders

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"workflow/internal/oa/platform"
)

func TestRankingHTTPValidationAndScope(t *testing.T) {
	s := &server{}
	admin := platform.Actor{ID: 1, SuperAdmin: true}
	operator := platform.Actor{ID: 2, Actions: map[string]bool{"orders.view": true}}
	for _, tc := range []struct {
		actor  platform.Actor
		query  string
		status int
	}{
		{admin, "group=shop&order=", 400},
		{admin, "group=shop&order=ASC", 400},
		{admin, "group=shop&order=asc;invalid", 400},
		{admin, "group=shop&order=desc&order=asc", 400},
		{admin, "group=person&sort=", 400},
		{admin, "group=person&sort=quantity", 400},
		{admin, "group=product&sort=title", 400},
		{admin, "group=growth&include_brush=1", 400},
		{admin, "group=shop&offset=4294967296", 400},
		{operator, "group=person&order=asc", 403},
		{operator, "group=product", 200},
	} {
		req := httptest.NewRequest("GET", "/oa-api/orders/rankings?"+tc.query, nil)
		w := httptest.NewRecorder()
		s.read(w, platform.WithActor(req, tc.actor), "rankings")
		if w.Code != tc.status {
			t.Errorf("%s: got %d, want %d", tc.query, w.Code, tc.status)
		}
		if w.Code == 200 && !strings.Contains(w.Body.String(), `"total_rows":0`) {
			t.Error("empty scope lost its total")
		}
	}
}

func TestRankingHTTPPagesTotalsAndCache(t *testing.T) {
	asc, _ := rankingQuery("product", "amount", "asc", false)
	desc, _ := rankingQuery("product", "", "", false)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		sql := strings.TrimSuffix(string(raw), " FORMAT JSON")
		offset := r.URL.Query().Get("param_offset")
		rows := []map[string]any{}
		switch {
		case sql == freshnessSQL:
			rows = append(rows, map[string]any{"source_committed_at": "2026-11-19 12:00:00", "lag_seconds": 1})
		case sql == "SELECT total_rows FROM ("+asc+") LIMIT 1" && offset == "0":
			rows = append(rows, map[string]any{"total_rows": 102})
		case sql == asc || sql == desc:
			start, size := 0, 101
			switch offset {
			case "0":
			case "100":
				start, size = 100, 2
			case "10100":
				size = 0
			default:
				t.Errorf("unexpected offset %s", offset)
			}
			for i := 0; i < size; i++ {
				key := start + i
				if sql == desc {
					key = 101 - key
				}
				rows = append(rows, map[string]any{"key": key, "total_rows": 102, "title": "宝贝"})
			}
		default:
			t.Error("unexpected ranking query")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
	}))
	defer upstream.Close()
	s := &server{ch: useEndpoint(t, upstream), cache: map[string]cached{}, active: map[string]*flight{}}
	for _, tc := range []struct {
		params, cache string
		size, first   int
		more          bool
	}{
		{"", "miss", 100, 101, true}, {"&order=desc&sort=amount", "hit", 100, 101, true},
		{"&order=asc", "miss", 100, 0, true}, {"&order=asc&offset=100", "miss", 2, 100, false},
		{"&order=asc&offset=10100", "miss", 0, 0, false},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/oa-api/orders/rankings?group=product"+tc.params, nil)
		s.read(w, platform.WithActor(req, platform.Actor{ID: 1, SuperAdmin: true}), "rankings")
		var res struct{ Data result }
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &res) != nil {
			t.Fatalf("%s: %d %s", tc.params, w.Code, w.Body.String())
		}
		if w.Header().Get("X-OA-Order-Cache") != tc.cache || len(res.Data.Rows) != tc.size || res.Data.HasMore != tc.more || res.Data.TotalRows == nil || *res.Data.TotalRows != 102 {
			t.Fatalf("bad page for %s", tc.params)
		}
		if tc.size > 0 {
			var n int
			if json.Unmarshal(res.Data.Rows[0]["key"], &n) != nil || n != tc.first || res.Data.Rows[0]["total_rows"] != nil {
				t.Errorf("incorrect first ranking row for %s", tc.params)
			}
		}
	}
}
