package pricing

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"workflow/domain"
	"workflow/internal/oa/platform"
)

const actorID int64 = 42

func reviewer() platform.Actor {
	return platform.Actor{ID: actorID, Name: "OA reviewer", Actions: map[string]bool{"catalog.view": true, "catalog.manage": true, "erp.manage": true}}
}

func openMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	return db, mock
}

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
			db, mock := openMock(t)
			if !tc.ambiguous && !tc.scoped {
				mock.ExpectQuery("SELECT d.id").WithArgs("成本 QA 1").WillReturnError(sql.ErrNoRows)
			}
			if tc.state == "ready" {
				mock.ExpectQuery("SELECT d.id.*evidence_hash").WithArgs("成本 QA 1", sqlmock.AnyArg()).WillReturnError(sql.ErrNoRows)
			}
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
			result := (app{db: db, native: native}).resolveSKU(req, "成本 QA 1")
			if result.State != tc.state {
				t.Fatalf("expected %s, got %+v", tc.state, result)
			}
			if tc.state == "ready" {
				if result.Suggested == nil || *result.Suggested != 0 || !validToken(result.Token) || post != 1 {
					t.Fatalf("bad ready row: %+v", result)
				}
			} else if result.Suggested != nil || result.Token != "" || !tc.legacy && post != 0 {
				t.Fatalf("unsafe row: %+v", result)
			}
		})
	}
}

func TestSaveBatchTransaction(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "rollback on duplicate successor"}[conflict], func(t *testing.T) {
			db, mock := openMock(t)
			request := confirmation{RequestID: "fixed-batch-request-id", Reason: "review"}
			price := 1.2345
			item := row{SKU: "ABC", Suggested: &price, State: "ready", Token: strings.Repeat("a", 64), evidenceHash: "price fingerprint", Review: &review{ID: 5}}
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO oa_sku_price_reviews").WithArgs(actorID, "OA reviewer", request.RequestID, "digest", request.Reason).WillReturnResult(sqlmock.NewResult(8, 1))
			mock.ExpectExec("INSERT INTO oa_sku_price_reviews").WithArgs(int64(8), item.SKU, int64(5), item.evidenceHash, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(9, 1))
			if conflict {
				mock.ExpectExec("INSERT INTO oa_sku_price_reviews").WithArgs(int64(8), "XYZ", int64(0), item.evidenceHash, sqlmock.AnyArg()).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate successor"})
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			second := item
			second.SKU = "XYZ"
			second.Review = nil
			selected := []row{item}
			if conflict {
				selected = append(selected, second)
			}
			if err := (app{db: db}).saveBatch(t.Context(), reviewer(), request, "digest", selected); (err != nil) != conflict {
				t.Fatalf("saveBatch error: %v", err)
			}
		})
	}
}

func TestIdempotentConfirmationAndPermissions(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		allowed, match, forbidden bool
		status                    int
	}{
		{name: "original receipt", allowed: true, match: true, status: 200},
		{name: "task scope revoked", allowed: true, match: true, forbidden: true, status: 403},
		{name: "same key different content", allowed: true, status: 409},
		{name: "requires ERP management", status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := openMock(t)
			actor := reviewer()
			if !tc.allowed {
				delete(actor.Actions, "erp.manage")
			}
			body := confirmation{RequestID: "fixed-batch-request-id", Reason: "review", Items: []selection{{SKU: "ABC", Token: strings.Repeat("a", 64)}}}
			if tc.allowed {
				hash := "different"
				if tc.match {
					hash = digest(body)
				}
				mock.ExpectQuery("SELECT id,request_hash").WithArgs(actorID, body.RequestID).
					WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "operator_id", "operator_name", "reason", "created_at"}).AddRow(8, hash, actor.ID, actor.Name, body.Reason, time.Now()))
				mock.ExpectQuery("SELECT id,evidence").WithArgs(int64(8)).
					WillReturnRows(sqlmock.NewRows([]string{"id", "evidence"}).AddRow(9, []byte(`{"sku_code":"ABC","task_id":7,"suggested_cost":1.2345,"state":"ready"}`)))
			}
			raw, _ := json.Marshal(body)
			mux := http.NewServeMux()
			Mount(mux, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !tc.match || r.Method != http.MethodGet || r.URL.Path != "/v1/tasks/7" {
					t.Error("idempotent receipt must only check current task scope")
					w.WriteHeader(500)
					return
				}
				if tc.forbidden {
					w.WriteHeader(403)
					return
				}
				w.WriteHeader(200)
			}))
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, platform.WithActor(httptest.NewRequest(http.MethodPost, basePath+"confirm", strings.NewReader(string(raw))), actor))
			if res.Code != tc.status {
				t.Fatalf("expected %d, got %d %s", tc.status, res.Code, res.Body)
			}
			if tc.status == 200 && (!strings.Contains(res.Body.String(), `"accepted_cost":1.2345`) || !strings.Contains(res.Body.String(), `"batch_id":8`)) {
				t.Fatalf("missing original receipt: %s", res.Body)
			}
		})
	}
}
