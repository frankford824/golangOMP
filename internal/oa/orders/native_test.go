package orders

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeReadsUseCallerAndExactSKU(t *testing.T) {
	request := httptest.NewRequest("GET", "/oa-api/orders/sku", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	request.Header.Set("X-User-ID", "12345")
	app := &server{native: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer fixture" || r.Header.Get("X-User-ID") != "" {
			t.Fatalf("unexpected native request %s", r.Method)
		}
		switch r.URL.Path {
		case "/v1/cost-management/sku-costs":
			if r.URL.Query().Get("keyword") != "SKU-1" || r.URL.Query().Get("review_state") != "all" {
				t.Fatal("missing search parameters")
			}
			if r.URL.Query().Get("before_id") == "42" {
				_, _ = io.WriteString(w, `{"data":[{"sku_code":"SKU-1","task_id":1,"cost_price":0.0000}]}`)
			} else {
				_, _ = io.WriteString(w, `{"data":[{"sku_code":"SKU-10","cost_price":44.00}],"next_before_id":42}`)
			}
		case "/v1/tasks/1":
			_, _ = io.WriteString(w, `{"data":{"task":{"id":1,"task_no":"QA-1","task_status":"Completed"}}}`)
		default:
			w.WriteHeader(403)
		}
	})}
	if value, ok := app.currentCost(request, "SKU-1"); !ok || value == nil || *value != "0.0000" {
		t.Fatalf("wrong cost %v %v", value, ok)
	}
	if task, ok := app.nativeTask(request, 1); !ok || task.Status != "Completed" {
		t.Fatalf("wrong task %+v %v", task, ok)
	}
	if _, ok := app.nativeTask(request, 2); ok {
		t.Fatal("denied task visible")
	}
}
