package taskarchive

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/internal/oa/platform"
)

func TestArchiveAuthorizationAndInput(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		allowed     bool
		status      int
	}{
		{"unauthorized", `{"task_ids":[1]}`, false, 403},
		{"empty", `{"task_ids":[]}`, true, 400},
		{"duplicate", `{"task_ids":[9,9]}`, true, 400},
		{"zero", `{"task_ids":[0]}`, true, 400},
		{"string", `{"task_ids":["1"]}`, true, 400},
		{"trailing", `{"task_ids":[1]} {}`, true, 400},
		{"unknown field", `{"task_ids":[1],"status":"Completed"}`, true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invoke(t, New(nil), tc.input, tc.allowed, tc.status)
		})
	}
}

func TestArchiveGlobalScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := platform.Actor{ID: 30, SuperAdmin: true, Actions: map[string]bool{"task.view": true}}
	for _, granted := range []bool{false, true} {
		mock.ExpectQuery("SELECT EXISTS.*auth_user_role_assignments.*scope_mode = 'global'").
			WithArgs(30).WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(granted))
		if GlobalAllowed(context.Background(), db, actor) != granted {
			t.Fatalf("expected archive permission %t", granted)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveLocksInOrderAndAuditsOnlyChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	expectTask(mock, 1, "Completed")
	expectTask(mock, 2, "Cancelled")
	expectTask(mock, 3, "Archived")
	mock.ExpectExec("UPDATE tasks SET task_status").WithArgs("Archived", 1, 2, "Completed", "Cancelled").WillReturnResult(sqlmock.NewResult(0, 2))
	expectEvent(mock, 1, "Completed")
	expectEvent(mock, 2, "Cancelled")
	mock.ExpectCommit()
	got := invoke(t, New(db), `{"task_ids":[3,2,1]}`, true, 200)
	if strings.TrimSpace(got) != `{"data":{"archived":2,"already_archived":1}}` {
		t.Fatalf("unexpected response %s", got)
	}

	mock.ExpectBegin()
	expectTask(mock, 1, "Archived")
	mock.ExpectCommit()
	got = invoke(t, New(db), `{"task_ids":[1]}`, true, 200)
	if strings.TrimSpace(got) != `{"data":{"archived":0,"already_archived":1}}` {
		t.Fatalf("unexpected replay response %s", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveRollsBackForIneligibleOrMissingTask(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		code         int
	}{
		{"ineligible", "InProgress", 409}, {"missing", "", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			expectTask(mock, 1, "Completed")
			expectTask(mock, 2, tc.status)
			mock.ExpectRollback()
			invoke(t, New(db), `{"task_ids":[2,1]}`, true, tc.code)
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestArchiveRollsBackWhenAuditFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	expectTask(mock, 1, "Completed")
	mock.ExpectExec("UPDATE tasks SET task_status").WithArgs("Archived", 1, "Completed", "Cancelled").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO task_event_sequences").WithArgs(1).WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()
	invoke(t, New(db), `{"task_ids":[1]}`, true, 503)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func invoke(t *testing.T, h http.Handler, input string, allowed bool, status int) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/oa-api/tasks/batch-archive", strings.NewReader(input))
	req = platform.WithActor(req, platform.Actor{ID: 30, Actions: map[string]bool{Action: allowed}})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != status {
		t.Fatalf("status = %d, want %d (%s)", w.Code, status, w.Body.String())
	}
	return w.Body.String()
}

func expectTask(mock sqlmock.Sqlmock, id int, status string) {
	rows := sqlmock.NewRows([]string{"task_status"})
	if status != "" {
		rows.AddRow(status)
	}
	mock.ExpectQuery("SELECT task_status FROM tasks WHERE id = .* FOR UPDATE").
		WithArgs(id).WillReturnRows(rows)
}

func expectEvent(mock sqlmock.Sqlmock, id int, from string) {
	mock.ExpectExec("INSERT INTO task_event_sequences").WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT last_sequence FROM task_event_sequences.*FOR UPDATE").
		WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"last_sequence"}).AddRow(1))
	mock.ExpectQuery("SELECT COALESCE.*task_event_logs.*FOR SHARE").
		WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"max_sequence"}).AddRow(2))
	mock.ExpectExec("UPDATE task_event_sequences SET last_sequence").
		WithArgs(3, id).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO task_event_logs").
		WithArgs(sqlmock.AnyArg(), id, 3, "task.status.changed", 30,
			[]byte(`{"from_task_status":"`+from+`","source":"oa_manual_archive","to_task_status":"Archived"}`), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
}
