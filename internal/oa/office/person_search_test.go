package office

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/internal/oa/platform"
)

func TestAuthorizedPhoneAndPinyinSearch(t *testing.T) {
	for _, test := range []struct {
		q, name string
		want    bool
	}{
		{"lihaiting", "李海婷", true}, {"lht", "李海婷", true}, {"hait", "李海婷", true}, {"shan", "单敏", true}, {"lǚ", "吕清", true}, {"xht", "李海婷", false},
	} {
		if got := nameMatches(test.name, romanQuery(test.q)); got != test.want {
			t.Errorf("query %s match=%v", test.q, got)
		}
	}
	db, m, _ := sqlmock.New()
	defer db.Close()
	app := office{db: db, source: db}
	for _, q := range []string{"+86 138-0000-1234", "lht"} {
		m.ExpectQuery("SELECT level").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"level"}).AddRow("draft"))
		mock := m.ExpectQuery("SELECT id,display_name,employee_no,.* FROM users WHERE status='active' AND")
		if q == "lht" {
			mock.WithArgs(false, int64(9), int64(7), int64(0))
		} else {
			mock.WithArgs(false, int64(9), int64(7), int64(0), "13800001234", "+8613800001234", "8613800001234", "008613800001234")
		}
		rows := sqlmock.NewRows([]string{"id", "display_name", "employee_no", "department"})
		if q == "lht" {
			for i := 0; i < 60; i++ {
				rows.AddRow(100+i, "陈宇", i, "运营")
			}
		}
		mock.WillReturnRows(rows.AddRow(33, "李海婷", 26, "设计部"))
		req := httptest.NewRequest("POST", "/oa-api/finance/approvers/search", strings.NewReader(`{"q":"`+q+`"}`))
		req = platform.WithActor(req, platform.Actor{ID: 7, DepartmentID: 9})
		w := httptest.NewRecorder()
		app.searchApprovers(w, req)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "李海婷") || strings.Contains(w.Body.String(), "1380000") || strings.Contains(w.Body.String(), "陈宇") {
			t.Fatal("unexpected search result")
		}
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
