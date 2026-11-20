package pricing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"workflow/domain"
	"workflow/internal/oa/platform"
)

func TestResolveRejectsUnsafeInputs(t *testing.T) {
	for _, tc := range []struct {
		name, state                                 string
		reviewed, manual, ambiguous, scoped, legacy bool
	}{
		{name: "ambiguous", state: "ambiguous", ambiguous: true},
		{name: "out of task scope", state: "unavailable", scoped: true},
		{name: "core confirmed", state: "protected", reviewed: true},
		{name: "manual", state: "protected", manual: true},
		{name: "unbound rule", state: "needs_review", legacy: true},
		{name: "explicit zero", state: "ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var post int
			native := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Fatal("native call lost identity")
				}
				switch r.URL.Path {
				case "/v1/cost-management/sku-costs":
					if r.URL.Query().Get("review_state") != "all" || r.URL.Query().Get("keyword") != "成本 QA 1" {
						t.Fatal("unexpected canonical lookup")
					}
					cost, width := 9.1234, 15.0
					record := domain.CostMaintenanceRow{ID: 11, TaskID: 7, SKUCode: "成本 QA 1", CostPrice: &cost, WidthCM: &width, StyleCode: "ERP-7", SizeText: "15 cm", Token: "fixture", Reviewed: tc.reviewed}
					rows := []domain.CostMaintenanceRow{record}
					if tc.ambiguous {
						record.ID++
						rows = append(rows, record)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
				case "/v1/tasks/7":
					if tc.scoped {
						w.WriteHeader(403)
						return
					}
					price := 9.1234
					platform.JSON(w, 200, map[string]any{"sku_items": []domain.TaskSKUItem{{ID: 11, SKUCode: "成本 QA 1", CostPrice: &price, ProductIID: "PRODUCT-7", VariantJSON: json.RawMessage(`{"erp_i_id":"ERP-7"}`), ManualCostOverride: tc.manual}}})
				case "/v1/cost-rules/preview":
					post++
					if r.Method != http.MethodPost {
						t.Fatal("wrong rule preview method")
					}
					var input domain.CostRulePreviewRequest
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
						t.Fatal(err)
					}
					if input.ERPIID != "ERP-7" || input.ProductIID != "PRODUCT-7" || input.Width == nil || *input.Width != .15 {
						t.Fatalf("wrong native pricing evidence: %+v", input)
					}
					mode := "binding_erp_i_id"
					if tc.legacy {
						mode = "legacy_alias"
					}
					estimate := 0.0
					platform.JSON(w, 200, domain.CostRulePreviewResponse{EstimatedCost: &estimate, MatchMode: mode, MatchedRule: &domain.CostRulePreviewMatch{RuleID: 123, RuleVersion: 4}})
				default:
					t.Fatal("unexpected native call")
				}
			})
			req := httptest.NewRequest(http.MethodPost, basePath+"preview", nil)
			req.Header.Set("Authorization", "Bearer fixture")
			result := (app{native: native}).resolveSKU(req, "成本 QA 1", false)
			if result.State != tc.state {
				t.Fatalf("expected %s, got %+v", tc.state, result)
			}
			if tc.state == "ready" {
				if result.Suggested == nil || *result.Suggested != 0 || post != 1 {
					t.Fatalf("bad ready row: %+v", result)
				}
			} else if result.Suggested != nil || !tc.legacy && post != 0 {
				t.Fatalf("unsafe row: %+v", result)
			}
		})
	}
}

func TestPreviewPermissionAndOnlyMountedOperation(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("unauthorized request unexpectedly reached the main site")
	}))
	for _, tc := range []struct {
		path   string
		status int
	}{
		{basePath + "preview", http.StatusForbidden},
		{basePath + "confirm", http.StatusNotFound},
	} {
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, httptest.NewRequest(http.MethodPost, tc.path, nil))
		if res.Code != tc.status {
			t.Fatalf("%s: expected %d, got %d", tc.path, tc.status, res.Code)
		}
	}
}

func TestOrderCodesKeepTheirOriginalSpelling(t *testing.T) {
	codes, ok := normalize([]string{"款/a,03", "款/A,03", "款/a,03"}, true)
	if !ok || len(codes) != 2 || codes[0] != "款/a,03" || codes[1] != "款/A,03" || sameSKU(codes[0], codes[1], true) {
		t.Fatalf("order codes changed: %v", codes)
	}
	old, ok := normalize(codes, false)
	if !ok || len(old) != 1 || old[0] != "款/A,03" || !sameSKU(codes[0], old[0], false) {
		t.Fatal("legacy batch changed")
	}
}
