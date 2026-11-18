package orders

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/internal/oa/office"
	"workflow/internal/oa/platform"
)

type savedTerms string

func (data *savedTerms) Match(value driver.Value) bool {
	raw, ok := value.(string)
	if !ok || !json.Valid([]byte(raw)) {
		return false
	}
	*data = savedTerms(raw)
	return true
}

func TestShopBindingMutationControlsOrderScopeWithoutPersonnelAttribution(t *testing.T) {
	db, ext, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source, sourceMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		query := strings.TrimSuffix(string(raw), " FORMAT JSON")
		switch query {
		case freshnessSQL:
			fmt.Fprint(w, "{\"data\":[{\"source_committed_at\":\"2026-11-18 09:00:00\"}]}")
		case summarySQL:
			if r.URL.Query().Get("param_all") != "0" || r.URL.Query().Get("param_shops") != "['store-1']" {
				t.Error("expected a store-scoped order result")
			}
			fmt.Fprint(w, "{\"data\":[{\"day\":\"2026-11-18\",\"shop_id\":\"store-1\",\"verified_gross\":\"10.00\"}]}")
		default:
			t.Errorf("unexpected query %q", query)
		}
	}))
	defer endpoint.Close()
	orderServer := &server{ch: useEndpoint(t, endpoint), db: db, cache: map[string]cached{}, active: map[string]*flight{}}
	directory := func(_ context.Context, no *int) (map[string]bool, string, []office.BindingShop, error) {
		if no != nil {
			t.Error("fixture must have no employee number")
		}
		return nil, "", []office.BindingShop{{ID: "store-1", Selectable: true}}, nil
	}
	mux := http.NewServeMux()
	office.Mount(mux, db, source, nil, nil, directory)
	var stored savedTerms
	change := func(body string, prior string, expected int) {
		sourceMock.ExpectQuery("SELECT id,display_name,employee_no").WithArgs(42).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "employee", "department", "team", "deptName", "teamName", "status", "type", "protected"}).AddRow(42, "fixture", nil, 0, 0, "", "", "active", "", false))
		sourceMock.ExpectQuery("WITH grants AS").WithArgs(42, 42).
			WillReturnRows(sqlmock.NewRows([]string{"code", "mode", "dept", "team", "type", "id", "access"}))
		if expected != 400 {
			ext.ExpectBegin()
			ext.ExpectExec("INSERT INTO oa_office_people").WithArgs(42).WillReturnResult(sqlmock.NewResult(42, 1))
			ext.ExpectQuery("SELECT payload FROM oa_office_people WHERE id=.+FOR UPDATE").WithArgs(42).
				WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(prior))
			if expected == 200 {
				ext.ExpectExec("UPDATE oa_office_people SET version=").WithArgs(sqlmock.AnyArg(), &stored, 42).
					WillReturnResult(sqlmock.NewResult(42, 1))
				ext.ExpectCommit()
			} else {
				ext.ExpectRollback()
			}
		}
		req := httptest.NewRequest(http.MethodPost, "/oa-api/people/42/shop-bindings", strings.NewReader(body))
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, platform.WithActor(req, platform.Actor{ID: 5, SuperAdmin: true}))
		if recorder.Code != expected {
			t.Fatalf("mutation status = %d, want %d, body = %s", recorder.Code, expected, recorder.Body)
		}
	}
	read := func(name string, expectedScope string, expectedRows int) {
		ext.ExpectQuery("SELECT payload FROM oa_office_people WHERE id=").WithArgs(42).
			WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(string(stored)))
		req := httptest.NewRequest(http.MethodGet, "/oa-api/orders/"+name, nil)
		recorder := httptest.NewRecorder()
		actor := platform.Actor{ID: 42, Actions: map[string]bool{"orders.view": true}}
		orderServer.read(recorder, platform.WithActor(req, actor), name)
		var result struct{ Data result }
		if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &result) != nil ||
			result.Data.ScopeState != expectedScope || len(result.Data.Rows) != expectedRows {
			t.Fatalf("%s got %d %s", name, recorder.Code, recorder.Body)
		}
	}
	change("{\"kind\":\"bind\",\"version\":0,\"shop_id\":\"invented\"}", "{}", 400)
	change("{\"kind\":\"bind\",\"version\":0,\"shop_id\":\"store-1\"}", "{}", 200)
	read("summary", "", 1)
	read("personal", "unassigned", 0)
	change("{\"kind\":\"unbind\",\"version\":0,\"from_shop_id\":\"store-1\"}", string(stored), 409)
	read("summary", "", 1)
	change("{\"kind\":\"unbind\",\"version\":1,\"from_shop_id\":\"store-1\"}", string(stored), 200)
	read("summary", "unassigned", 0)
	if err := ext.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := sourceMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
