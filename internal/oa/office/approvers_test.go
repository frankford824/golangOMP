package office

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/internal/oa/platform"
)

func TestApproverLookupForFinancialDelegation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := office{db: db, source: db}
	for _, admin := range []bool{false, true} {
		want := http.StatusForbidden
		if admin {
			want = http.StatusOK
			mock.ExpectQuery("SELECT id,display_name,employee_no,.* FROM users WHERE status='active'").
				WithArgs(true, int64(9), int64(7), int64(33)).WillReturnRows(
				sqlmock.NewRows([]string{"id", "display_name", "employee_no", "department"}).AddRow(33, "示例经办人", 26, "设计部"))
			mock.ExpectQuery("SELECT id,display_name,employee_no,.* FROM users WHERE status='active'").
				WithArgs(true, int64(9), int64(7), "%33%", "%33%", "%33%").
				WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "employee_no", "department"}))
		} else {
			mock.ExpectQuery("SELECT level FROM oa_office_delegates").WithArgs(int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"level"}))
		}
		req := httptest.NewRequest("GET", "/oa-api/finance/approvers?ids=33&q=33", nil)
		req = platform.WithActor(req, platform.Actor{ID: 7, DepartmentID: 9, SuperAdmin: admin})
		w := httptest.NewRecorder()
		app.approvers(w, req)
		if w.Code != want {
			t.Fatalf("super admin=%t: got %d, want %d", admin, w.Code, want)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
