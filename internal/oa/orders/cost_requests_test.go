package orders

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/internal/oa/platform"
)

func TestCostRequestsReusePendingAndDeliverOnlyOneReply(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := server{db: db}
	n := costRequest{Kind: requestTopic, State: "pending", Shop: "s", SKU: "x", Sender: "测试运营", Reason: "请核对规格", RequestKey: "fixture-request-123"}
	for _, repeat := range []bool{false, true} {
		mock.ExpectQuery("SELECT GET_LOCK").WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(1))
		rows := sqlmock.NewRows([]string{"id"})
		if repeat {
			rows.AddRow(51)
		}
		mock.ExpectQuery("SELECT id FROM oa_office_notifications WHERE user_id=").WithArgs(int64(1), "x", "s", n.RequestKey).WillReturnRows(rows)
		if !repeat {
			mock.ExpectExec("INSERT INTO oa_office_notifications").WithArgs(int64(1), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(51, 1))
		}
		mock.ExpectExec("DO RELEASE_LOCK").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
		id, existing, err := s.storeCostRequest(context.Background(), 1, n)
		if err != nil || id != 51 || existing != repeat {
			t.Fatalf("save id=%d existing=%t err=%v", id, existing, err)
		}
	}
	for _, code := range []int{403, 200, 409} {
		r := httptest.NewRequest("POST", "/unused", strings.NewReader(`{"reply":"已核对材质与现价"}`))
		r.SetPathValue("id", "51")
		actor := platform.Actor{ID: 8, Name: "测试维护", Actions: map[string]bool{"catalog.view": true, "catalog.manage": true, "erp.manage": code != 403}}
		if code != 403 {
			var n int64
			if code == 200 {
				n = 1
			}
			mock.ExpectExec("UPDATE oa_office_notifications SET payload=JSON_SET").WithArgs("已核对材质与现价", actor.Name, sqlmock.AnyArg(), int64(51)).WillReturnResult(sqlmock.NewResult(0, n))
		}
		w := httptest.NewRecorder()
		s.resolveCostMessage(w, platform.WithActor(r, actor))
		if w.Code != code {
			t.Fatalf("handle=%d want %d", w.Code, code)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestCostsAreAuthorizedAndBoundToSelectedProduct(t *testing.T) {
	s := server{}
	for _, test := range []struct {
		input   string
		allowed bool
		status  int
	}{
		{`{"sku":"test"}`, false, 403},
		{`{"shop_id":"s","sku":"x","reason":"test","from":"2026-06-04","to":"2026-06-01","request_key":"fixture-request-123"}`, true, 400},
		{`{"shop_id":"s","sku":"x","reason":"test","from":"2026-06-01","to":"2026-06-01","request_key":"fixture-request-123"}`, true, 403},
	} {
		r := httptest.NewRequest("POST", "/unused", strings.NewReader(test.input))
		w := httptest.NewRecorder()
		s.requestCostCheck(w, platform.WithActor(r, platform.Actor{ID: 1, Actions: map[string]bool{"orders.view": test.allowed}}))
		if w.Code != test.status {
			t.Fatalf("code=%d want %d", w.Code, test.status)
		}
	}
}
