package handler

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
	"workflow/domain"
	"workflow/repo"
	assetworkbench "workflow/service/asset_workbench"
)

type incomeMiddlewareRepo struct {
	repo.AssetWorkbenchRepo
	visible bool
	fail    error
}

func (r *incomeMiddlewareRepo) GetMembership(context.Context, string, int64) (*domain.AppMembership, error) {
	return &domain.AppMembership{Status: domain.AppMembershipStatusActive}, nil
}
func (r *incomeMiddlewareRepo) GetProfileByUserID(context.Context, int64) (*domain.AssetWorkbenchProfile, error) {
	return &domain.AssetWorkbenchProfile{SettlementVisible: &r.visible}, r.fail
}

type incomeMiddlewareTx struct{}

func (incomeMiddlewareTx) RunInTx(ctx context.Context, fn func(repo.Tx) error) error { return fn(nil) }
func TestWorkbenchMiddlewareAppliesIncomeProjectionBeforeReadAndCreateResponses(t *testing.T) {
	for _, visible := range []bool{false, true} {
		r := &incomeMiddlewareRepo{visible: visible}
		svc := assetworkbench.NewService(assetworkbench.Config{}, assetworkbench.WithRepository(r, incomeMiddlewareTx{}))
		h := NewAssetWorkbenchHandler(svc)
		router := gin.New()
		router.POST("/client", h.RequireActiveMembership(), func(c *gin.Context) { respondCreated(c, gin.H{"gross_total": 123.45, "submission_id": 99}) })
		request := httptest.NewRequest("POST", "/client", nil)
		request = request.WithContext(domain.WithRequestActor(request.Context(), domain.RequestActor{ID: 22, Roles: []domain.Role{domain.RoleAssetSubmitter}, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		expected := `"gross_total":0`
		if visible {
			expected = `"gross_total":123.45`
		}
		if recorder.Code != 201 || !strings.Contains(recorder.Body.String(), expected) {
			t.Fatal(recorder.Code, recorder.Body.String())
		}
	}
}

func TestIncomeResponseProjectionCoversNestedSnapshotsAndPreservesFacts(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled-or-admin", true: "hidden-client"}[hidden], func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(workbenchIncomeHiddenKey, hidden)
			original := map[string]interface{}{"id": int64(9007199254740993), "gross_total": 12.34, "item_count": 3, "snapshot": json.RawMessage(`{"base_unit_price":12.34,"base_price_rule_id":99,"items":[{"gross_amount":12.34,"amount":"¥12.34","unit_price_snapshot":12.34}]}`)}
			respondCreated(c, original)
			var response map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			var data map[string]json.RawMessage
			_ = json.Unmarshal(response["data"], &data)
			if string(data["id"]) != "9007199254740993" || string(data["item_count"]) != "3" {
				t.Fatalf("identity/count altered: %s", w.Body.String())
			}
			expected := "12.34"
			if hidden {
				expected = "0"
			}
			if string(data["gross_total"]) != expected {
				t.Fatal(w.Body.String())
			}
			var snapshot map[string]json.RawMessage
			_ = json.Unmarshal(data["snapshot"], &snapshot)
			if string(snapshot["base_unit_price"]) != expected || string(snapshot["base_price_rule_id"]) != "99" {
				t.Fatal(w.Body.String())
			}
			var items []map[string]json.RawMessage
			_ = json.Unmarshal(snapshot["items"], &items)
			if string(items[0]["gross_amount"]) != expected || string(items[0]["unit_price_snapshot"]) != expected {
				t.Fatal(w.Body.String())
			}
			if hidden && string(items[0]["amount"]) != `""` {
				t.Fatal(w.Body.String())
			}
			if original["gross_total"] != 12.34 {
				t.Fatal("stored/service facts mutated")
			}
		})
	}
}
func TestIncomeResponseProjectionAlsoProtectsPaginationAndErrors(t *testing.T) {
	for _, kind := range []string{"ok", "page", "error"} {
		t.Run(kind, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(workbenchIncomeHiddenKey, true)
			payload := map[string]interface{}{"amount": 999.12, "unit_price": 99.12}
			switch kind {
			case "ok":
				respondOK(c, payload)
			case "page":
				respondOKWithPagination(c, []interface{}{payload}, gin.H{"total": 1})
			case "error":
				c.JSON(400, incomeSafeResponse(c, gin.H{"error": payload}))
			}
			var result interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if containsMoneyJSON(result) {
				t.Fatal(w.Body.String())
			}
		})
	}
}
func containsMoneyJSON(v interface{}) bool {
	switch x := v.(type) {
	case float64:
		return x == 999.12 || x == 99.12
	case map[string]interface{}:
		for _, v := range x {
			if containsMoneyJSON(v) {
				return true
			}
		}
	case []interface{}:
		for _, v := range x {
			if containsMoneyJSON(v) {
				return true
			}
		}
	}
	return false
}
