package orders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestDateCoverageAndVerifiedZero(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, beijing)
	for _, tc := range []struct {
		path, from, until string
		valid             bool
	}{
		{"/?from=2026-06-01&to=2026-11-18", "2026-06-01", "2026-11-19", true},
		{"/?from=2026-06-01&to=2026-06-01", "2026-06-01", "2026-06-02", true},
		{"/?from=2026-05-31&to=2026-06-01", "", "", false},
		{"/?from=2026-11-18&to=2026-11-19", "", "", false},
		{"/?from=2026-11-18&to=2026-11-17", "", "", false},
	} {
		params, err := filters(httptest.NewRequest(http.MethodGet, tc.path, nil), "summary", now)
		if (err == nil) != tc.valid || params.Get("param_from") != tc.from || params.Get("param_until") != tc.until {
			t.Errorf("%s: %v %v", tc.path, params, err)
		}
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "[('known','2026-11-18'),('unknown','2026-11-18')]"
		if r.URL.Query().Get("param_zero_pairs") != want {
			t.Error("expected exact shop-day candidates")
		}
		_, _ = w.Write([]byte(`{"data":[{"day":"2026-11-18","shop_id":"known"}]}`))
	}))
	defer fixture.Close()
	service := &server{ch: useEndpoint(t, fixture)}
	rows := []row{
		{"day": []byte(`"2026-11-18"`), "shop_id": []byte(`"known"`), "verified_gross": []byte("null")},
		{"day": []byte(`"2026-11-18"`), "shop_id": []byte(`"unknown"`), "verified_gross": []byte("null")},
	}
	if err := service.markVerifiedZero(context.Background(), url.Values{}, rows); err != nil {
		t.Fatal(err)
	}
	if string(rows[0]["verified_gross"]) != `"0.00"` || string(rows[1]["verified_gross"]) != "null" {
		t.Error("zero and unknown mismatch")
	}
}

func TestHourlyFilterUsesAtMostTwoBeijingDates(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, beijing)
	for _, test := range []struct {
		start string
		valid bool
	}{
		{"2026-11-17", true}, {"2026-11-16", false},
	} {
		request := httptest.NewRequest(http.MethodGet, "/?from="+test.start+"&to=2026-11-18", nil)
		_, err := filters(request, "hourly", now)
		if (err == nil) != test.valid {
			t.Errorf("hourly from=%s: %v", test.start, err)
		}
	}
}
