package office

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/internal/oa/platform"
)

func TestFinanceAdminsCanPublishWithoutDelegation(t *testing.T) {
	for _, actor := range []platform.Actor{
		{ID: 10, SuperAdmin: true},
		{ID: 11, Actions: map[string]bool{"finance.admin": true}},
	} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		app := office{db: db}
		mock.ExpectQuery("SELECT COALESCE").
			WillReturnRows(sqlmock.NewRows([]string{"template"}))
		req := platform.WithActor(httptest.NewRequest("GET", "/oa-api/finance/templates?draft=1", nil), actor)
		w := httptest.NewRecorder()
		app.templates(w, req)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"level":"publish"`) {
			t.Fatalf("template management: %d %s", w.Code, w.Body.String())
		}
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT version FROM oa_office_org_managers").WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))
		mock.ExpectExec("INSERT INTO oa_office_templates").WithArgs("test-travel").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT version FROM oa_office_templates").WithArgs("test-travel").
			WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))
		mock.ExpectExec("INSERT INTO oa_office_template_versions").
			WithArgs("test-travel", 1, actor.ID, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("UPDATE oa_office_templates SET version").WithArgs(1, sqlmock.AnyArg(), "test-travel").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		req = platform.WithActor(httptest.NewRequest("PUT", "/oa-api/finance/templates/test-travel", strings.NewReader(
			`{"name":"演示差旅","type":"报销","channels":["person"],"evidence":"发票","nodes":["manager"],"version":0,"publish":true}`)), actor)
		req.SetPathValue("id", "test-travel")
		w = httptest.NewRecorder()
		app.saveTemplate(w, req)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"version":1`) {
			t.Fatalf("publish response: %d %s", w.Code, w.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestFinanceGrantListAndInactiveRevocation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := office{db: db, source: db}
	hr := platform.Actor{ID: 31, Roles: []string{"hr_admin", "department_admin"}, Actions: map[string]bool{"people.manage": true}, PeopleDepartments: []int64{companyPeopleScope}}
	request := platform.WithActor(httptest.NewRequest("GET", "/oa-api/finance/grants", nil), hr)
	w := httptest.NewRecorder()
	app.financeGrants(w, request)
	if w.Code != http.StatusForbidden {
		t.Fatalf("personnel role gained finance access: %d", w.Code)
	}

	actor := platform.Actor{ID: 1, Actions: map[string]bool{"finance.admin": true}}
	mock.ExpectQuery("WITH recipients").
		WillReturnRows(sqlmock.NewRows([]string{"id", "level", "pay", "report"}).
			AddRow(12, "publish", true, false).AddRow(13, "none", false, true).AddRow(15, "draft", false, false))
	mock.ExpectQuery("SELECT id,display_name,COALESCE").
		WithArgs(int64(12), int64(13), int64(15)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "department", "employee_no", "status"}).
			AddRow(12, "演示离职用户", "设计部", 12, "inactive").AddRow(13, "演示报表员", "财务部", nil, "active"))
	w = httptest.NewRecorder()
	app.financeGrants(w, platform.WithActor(request, actor))
	var result struct {
		Data struct{ Items []financeGrant }
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Data.Items) != 3 {
		t.Fatalf("unexpected list: %d %s", w.Code, w.Body.String())
	}
	if first := result.Data.Items[0]; first.Name != "演示离职用户" || first.Status != "inactive" || first.Level != "publish" || !first.Pay || result.Data.Items[2].Status != "missing" {
		t.Fatalf("incomplete grants: %#v", result.Data.Items)
	}

	mock.ExpectExec("UPDATE oa_office_delegates SET level='none'").WithArgs(int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT version FROM oa_office_org_managers").WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))
	mock.ExpectExec("UPDATE oa_office_finance_actions SET can_pay=FALSE").WithArgs(int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT version FROM oa_office_org_managers").WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))
	mock.ExpectQuery("SELECT status FROM users").WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("inactive"))
	mock.ExpectRollback()
	for _, test := range []struct {
		handler http.HandlerFunc
		body    string
		status  int
	}{
		{app.saveDelegate, `{"level":"none"}`, 200},
		{app.saveFinanceActions, `{"pay":false,"report":false}`, 200},
		{app.saveFinanceActions, `{"pay":true,"report":false}`, 400},
		{app.saveFinanceActions, `{"pay":false}`, 400},
		{app.saveDelegate, `{"level":"none"}`, 403},
	} {
		currentActor := actor
		if test.status == 403 {
			currentActor = hr
		}
		w = httptest.NewRecorder()
		req := platform.WithActor(httptest.NewRequest("PUT", "/", strings.NewReader(test.body)), currentActor)
		req.SetPathValue("id", "12")
		test.handler(w, req)
		if w.Code != test.status {
			t.Fatalf("grant %s: expected %d, got %d %s", test.body, test.status, w.Code, w.Body.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchCandidatesPageAcrossPinyinMatchesWithinScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := office{db: db, source: db}
	actor := platform.Actor{ID: 7, DepartmentID: 9, PeopleDepartments: []int64{11}, PeopleTeams: []int64{12}}
	for index, cursor := range []string{"0", "150"} {
		mock.ExpectQuery("SELECT level").WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"level"}).AddRow("publish"))
		rows := sqlmock.NewRows([]string{"id", "display_name", "employee_no", "department"})
		if index == 0 {
			for id := 100; id <= 151; id++ {
				name := "李海婷"
				if id == 120 {
					name = "陈宇"
				}
				rows.AddRow(id, name, id, "运营")
			}
		} else {
			rows.AddRow(151, "李浩天", 151, "运营")
		}
		mock.ExpectQuery("OR department_id=.+ OR team_id=.+ AND id>.+ORDER BY id").
			WithArgs(false, int64(9), int64(7), int64(11), int64(12), int64(index*150)).WillReturnRows(rows)
		req := platform.WithActor(httptest.NewRequest("POST", "/oa-api/finance/approvers/search", strings.NewReader(`{"q":"lht","after_id":`+cursor+`}`)), actor)
		w := httptest.NewRecorder()
		app.searchApprovers(w, req)
		var response struct {
			Data struct {
				Items []approverOption
				Next  int64 `json:"next_after_id"`
			}
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
			t.Fatalf("bad candidate page %d: %d %s", index, w.Code, w.Body.String())
		}
		var result []int64
		for _, item := range response.Data.Items {
			result = append(result, item.ID)
		}
		if index == 0 {
			if len(result) != 50 || result[0] != 100 || result[49] != 150 || response.Data.Next != 150 {
				t.Fatalf("first candidate page lost rows: %v cursor %d", result, response.Data.Next)
			}
		} else if !reflect.DeepEqual(result, []int64{151}) || response.Data.Next != 0 {
			t.Fatalf("second candidate page lost rows: %v cursor %d", result, response.Data.Next)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReportPermissionIsIndependentOfPayAndFinanceManage(t *testing.T) {
	for _, actor := range []platform.Actor{
		{ID: 1, SuperAdmin: true},
		{ID: 2, Actions: map[string]bool{"finance.admin": true, "finance.pay": true}},
		{ID: 3, Actions: map[string]bool{"finance.pay": true}},
	} {
		w := httptest.NewRecorder()
		req := platform.WithActor(httptest.NewRequest("GET", "/oa-api/finance/report", nil), actor)
		(office{}).report(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("actor %d without report grant got %d", actor.ID, w.Code)
		}
	}
}
