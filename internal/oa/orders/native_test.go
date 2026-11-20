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
			_, _ = io.WriteString(w, `{"data":{"id":1,"task_no":"QA-1","task_status":"Completed"}}`)
		default:
			w.WriteHeader(403)
		}
	})}
	if value, _, ok := app.currentCost(request, "SKU-1"); !ok || value == nil || *value != "0.0000" {
		t.Fatalf("wrong cost %v %v", value, ok)
	}
	if task, ok := app.nativeTask(request, 1); !ok || task.Status != "Completed" {
		t.Fatalf("wrong task %+v %v", task, ok)
	}
	if _, ok := app.nativeTask(request, 2); ok {
		t.Fatal("denied task visible")
	}
}

func TestHistoricalTasksRequireExactAuthorizedDetail(t *testing.T) {
	request := httptest.NewRequest("GET", "/oa-api/orders/sku", nil)
	app := &server{native: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/search":
			if r.URL.Query().Get("q") != "SKU-1" || r.URL.Query().Get("mode") != "exact" || r.URL.Query().Get("scope") != "tasks" {
				t.Fatal("invalid search scope")
			}
			_, _ = io.WriteString(w, `{"query":"SKU-1","results":{"tasks":[{"id":1},{"id":2},{"id":3},{"id":3},{"id":4}]},"retrieval":{"mode":"exact"}}`)
		case "/v1/tasks/1":
			t.Fatal("already linked task fetched again")
		case "/v1/tasks/2":
			w.WriteHeader(403)
		case "/v1/tasks/3":
			_, _ = io.WriteString(w, `{"data":{"id":3,"task_no":"T-3","sku_code":"SKU-OTHER","task_status":"Completed","sku_items":[{"sku_code":"SKU-1"}]}}`)
		case "/v1/tasks/4":
			_, _ = io.WriteString(w, `{"data":{"id":4,"task_no":"SKU-1","sku_code":"SKU-10"}}`)
		default:
			t.Fatal("unexpected route")
		}
	})}
	tasks := app.historicalTasks(request, "SKU-1", map[int64]bool{1: true})
	if len(tasks) != 1 || tasks[0].ID != 3 || tasks[0].Status != "Completed" {
		t.Fatalf("unexpected historical tasks: %+v", tasks)
	}
}

func TestCurrentCostDistinguishesUnpricedAbsentAndUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name, rows   string
		status, task int
		ok           bool
	}{
		{"matched with NULL price", `[{"sku_code":"原/码","task_id":12,"cost_price":null}]`, 200, 12, true},
		{"non-unique exact matches", `[{"sku_code":"原/码","task_id":12,"cost_price":1},{"sku_code":"原/码","task_id":13,"cost_price":3}]`, 200, 0, false},
		{"no canonical match", `[{"sku_code":"原/码1","task_id":12,"cost_price":2.45}]`, 200, 0, true},
		{"task not visible", `[{"sku_code":"原/码","task_id":12,"cost_price":2.45}]`, 403, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &server{native: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/cost-management/sku-costs":
					if r.URL.Query().Get("keyword") != "原/码" {
						t.Fatal("wrong SKU")
					}
					io.WriteString(w, `{"data":`+tc.rows+`}`)
				case "/v1/tasks/12":
					w.WriteHeader(tc.status)
					io.WriteString(w, `{"data":{"id":12,"task_no":"T-12"}}`)
				default:
					t.Fatal(r.URL.Path)
				}
			})}
			value, task, ok := app.currentCost(httptest.NewRequest("GET", "/", nil), "原/码")
			if value != nil || task.ID != int64(tc.task) || ok != tc.ok {
				t.Fatalf("price=%v task=%d available=%t", value, task.ID, ok)
			}
		})
	}
}
