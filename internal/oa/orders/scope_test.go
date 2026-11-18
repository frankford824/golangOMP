package orders

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"workflow/internal/oa/platform"
)

func TestOrdersReadSeparatesAttributionPermissionsAndEmptyData(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		query := strings.TrimSuffix(string(body), " FORMAT JSON")
		var rows string
		switch query {
		case personScope:
			rows = "[{\"shop_id\":\"s1\",\"person_id\":\"p1\"},{\"shop_id\":\"\",\"person_id\":\"p1\"}]"
		case freshnessSQL:
			rows = "[{\"source_committed_at\":\"2026-11-18 09:00:00\"}]"
		case productsSQL:
			rows = "[{\"sku\":\"all-products\"}]"
		case myProductsSQL:
			if r.URL.Query().Get("param_person") != "p1" {
				t.Error("personal products omitted person parameter")
			}
			rows = "[{\"sku\":\"own-products\"}]"
		case orderDetailSQL:
			if r.URL.Query().Get("param_brush") != "0" {
				t.Error("order detail omitted explicit brush exclusion")
			}
			rows = "[]"
		default:
			t.Errorf("unexpected query %q", query)
			rows = "[]"
		}
		fmt.Fprintf(w, "{\"data\":%s}", rows)
	}))
	defer endpoint.Close()
	s := &server{ch: useEndpoint(t, endpoint), cache: map[string]cached{}, active: map[string]*flight{}}
	employee := 42
	tests := []struct {
		endpoint, query string
		employee        *int
		status          int
		scope, sku      string
	}{
		{"products", "", &employee, 200, "", "all-products"},
		{"products", "?mine=1", &employee, 200, "", "own-products"},
		{"products", "?mine=1", nil, 200, "unassigned", ""},
		{"products", "?shop_id=s2", &employee, 403, "", ""},
		{"detail", "?include_brush=0", &employee, 200, "", ""},
	}
	for _, tc := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/oa-api/orders/"+tc.endpoint+tc.query, nil)
		actor := platform.Actor{ID: 100, EmployeeNo: tc.employee, Actions: map[string]bool{"orders.view": true}}
		s.read(recorder, platform.WithActor(request, actor), tc.endpoint)
		if recorder.Code != tc.status {
			t.Fatalf("%s%s status = %d, want %d", tc.endpoint, tc.query, recorder.Code, tc.status)
		}
		if tc.status != 200 {
			continue
		}
		var output struct{ Data result }
		if err := json.Unmarshal(recorder.Body.Bytes(), &output); err != nil {
			t.Fatal(err)
		}
		sku := ""
		if len(output.Data.Rows) > 0 {
			_ = json.Unmarshal(output.Data.Rows[0]["sku"], &sku)
		}
		if output.Data.ScopeState != tc.scope || sku != tc.sku {
			t.Errorf("%s%s scope = %q sku = %q, want %q %q", tc.endpoint, tc.query, output.Data.ScopeState, sku, tc.scope, tc.sku)
		}
	}
}
