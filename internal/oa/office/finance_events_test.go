package office

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/internal/oa/platform"
)

type documentEventMatches func(history) bool

func (matches documentEventMatches) Match(value driver.Value) bool {
	data, ok := value.(string)
	var event history
	return ok && json.Unmarshal([]byte(data), &event) == nil && matches(event)
}

func TestFinanceDocumentTransitions(t *testing.T) {
	payer := platform.Actor{ID: 3, Name: "出纳", Actions: map[string]bool{"finance.pay": true}}
	admin := platform.Actor{ID: 4, Name: "财务审批", Actions: map[string]bool{"finance.admin": true}}
	applicant := platform.Actor{ID: 2, Name: "申请人"}
	approvedAmount := int64(8000)
	date := businessDate().Format(time.DateOnly)
	receipt := strings.Repeat("a", 32)
	cases := []struct {
		name, state, action, input string
		actor                      platform.Actor
		current, status            int
		afterState                 string
		approvedAmount             *int64
		eventAmount                int64
		paymentReceipt             bool
	}{
		{"first-node withdrawal", "waiting", "withdraw", `{"version":2}`, applicant, 0, 200, "withdrawn", nil, 10000, false},
		{"withdrawal after approval", "waiting", "withdraw", `{"version":2}`, applicant, 1, 409, "", nil, 0, false},
		{"stale withdrawal", "waiting", "withdraw", `{"version":1}`, applicant, 0, 409, "", nil, 0, false},
		{"authorized void", "approved", "void", `{"version":2,"reason":"申请撤销"}`, payer, 1, 200, "voided", &approvedAmount, 8000, false},
		{"void requires reason", "approved", "void", `{"version":2,"reason":" "}`, payer, 1, 400, "", &approvedAmount, 0, false},
		{"paid cannot be voided", "paid", "void", `{"version":2,"reason":"申请撤销"}`, admin, 1, 409, "", &approvedAmount, 0, false},
		{"authorized reduction", "waiting", "decision", `{"version":2,"current":1,"decision":"approve","approved_amount_cents":8000,"adjustment_reason":"核减"}`, admin, 1, 200, "approved", nil, 8000, false},
		{"non-finance reviewer cannot reduce", "waiting", "decision", `{"version":2,"current":0,"decision":"approve","approved_amount_cents":8000,"adjustment_reason":"核减"}`, payer, 0, 403, "", nil, 0, false},
		{"approved amount payment", "approved", "payment", jsonData(payment{Amount: 8000, Date: date, Receipt: receipt, BankReference: " REF-001 "}), payer, 2, 200, "paid", &approvedAmount, 8000, true},
		{"reject paying original instead of approved", "approved", "payment", jsonData(payment{Amount: 10000, Date: date, Receipt: receipt, BankReference: "REF-001"}), payer, 2, 400, "", &approvedAmount, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			doc := document{
				ID: "test-doc", Applicant: applicant.ID, Current: tc.current, Version: 2,
				application: application{Amount: 10000}, State: tc.state,
				Route: []int64{payer.ID, admin.ID}, RouteNodes: []string{"主管", "财务"},
				ApprovedAmount: tc.approvedAmount,
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT payload FROM oa_office_documents WHERE id=.+FOR UPDATE").
				WithArgs(doc.ID).WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(jsonData(doc)))
			if tc.paymentReceipt {
				mock.ExpectQuery("SELECT owner_id,document_id FROM oa_office_files").
					WithArgs(receipt).WillReturnRows(sqlmock.NewRows([]string{"owner_id", "document_id"}).AddRow(payer.ID, nil))
				mock.ExpectExec("UPDATE oa_office_files SET document_id").
					WithArgs(doc.ID, receipt).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if tc.status == 200 {
				mock.ExpectExec("UPDATE oa_office_documents SET").WillReturnResult(sqlmock.NewResult(0, 1))
				eventAction := tc.action
				if eventAction == "decision" {
					eventAction = "approve"
				}
				mock.ExpectExec("INSERT INTO oa_office_document_events").WithArgs(doc.ID, 3, tc.actor.ID, eventAction, sqlmock.AnyArg(),
					documentEventMatches(func(e history) bool {
						if e.OperatorID != tc.actor.ID || e.OperatorName != tc.actor.Name || e.NodeName == "" ||
							e.CreatedAt == nil || e.BeforeState != tc.state || e.AfterState != tc.afterState ||
							e.BeforeAmount == nil || *e.BeforeAmount != doc.payableAmount() ||
							e.AfterAmount == nil || *e.AfterAmount != tc.eventAmount {
							return false
						}
						if tc.paymentReceipt {
							return e.BankReference == "REF-001" && e.Receipt == receipt && e.PaymentDate == date
						}
						if tc.action == "decision" {
							return e.AdjustmentReason == "核减"
						}
						return true
					})).WillReturnResult(sqlmock.NewResult(0, 1))
				if tc.afterState == "approved" || tc.afterState == "paid" || tc.afterState == "voided" {
					mock.ExpectExec("INSERT INTO oa_office_notifications").WillReturnResult(sqlmock.NewResult(0, 1))
				}
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			app := office{db: db}
			mux := http.NewServeMux()
			app.mountFinance(mux)
			request := httptest.NewRequest(http.MethodPost, "/oa-api/finance/documents/"+doc.ID+"/"+tc.action, strings.NewReader(tc.input))
			result := httptest.NewRecorder()
			mux.ServeHTTP(result, platform.WithActor(request, tc.actor))
			if result.Code != tc.status {
				t.Fatalf("wanted %d, got %d: %s", tc.status, result.Code, result.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFinanceDetailPreservesLegacyHistoryAlongsideAppendOnlyEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := platform.Actor{ID: 2, Name: "申请人"}
	original := history{Actor: actor.ID, Action: "submit", At: time.Now().Add(-time.Hour).UTC()}
	event := newFinanceHistory(actor, "withdraw")
	doc := document{ID: "test-doc", Applicant: actor.ID, Version: 2, History: []history{original, event}}
	doc.History[1].Action = "changed-mirror"
	mock.ExpectQuery("SELECT payload FROM oa_office_document_events").
		WithArgs(doc.ID, doc.Version).WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(jsonData(event)))
	app := office{db: db}
	if err := app.readDocumentEvents(t.Context(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.History) != 2 || doc.History[0].OperatorName != "" || doc.History[0].CreatedAt != nil ||
		doc.History[1].OperatorName != actor.Name || doc.History[1].Action != "withdraw" {
		t.Fatalf("unexpected history: %+v", doc.History)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinanceInlinePreviewChecksLinkedDocumentEvenForUploader(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OA_FILES_DIR", dir)
	id := strings.Repeat("a", 32)
	if err := os.WriteFile(filepath.Join(dir, id), []byte("%PDF-1.7\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		actor  int64
		status int
	}{{3, 403}, {4, 200}} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		app := office{db: db}
		doc := document{ID: "test-doc", Applicant: 4}
		mock.ExpectQuery("SELECT owner_id,name,mime,document_id FROM oa_office_files").
			WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"owner_id", "name", "mime", "document_id"}).AddRow(3, "invoice.pdf", "application/pdf", doc.ID))
		mock.ExpectQuery("SELECT payload FROM oa_office_documents").
			WithArgs(doc.ID).WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(jsonData(doc)))
		request := httptest.NewRequest(http.MethodGet, "/oa-api/finance/files/"+id+"?inline=1", nil)
		request.SetPathValue("id", id)
		result := httptest.NewRecorder()
		app.download(result, platform.WithActor(request, platform.Actor{ID: tc.actor}))
		if result.Code != tc.status {
			t.Fatalf("wanted %d, got %d: %s", tc.status, result.Code, result.Body.String())
		}
		if tc.status == 200 && (!strings.HasPrefix(result.Header().Get("Content-Disposition"), "inline;") ||
			result.Header().Get("Content-Type") != "application/pdf" || result.Header().Get("Cache-Control") != "private, no-store") {
			t.Fatalf("unexpected headers: %v", result.Header())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestFinanceAuditFailureRollsBackDocumentMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := platform.Actor{ID: 2}
	doc := document{ID: "test-doc", Applicant: actor.ID, State: "waiting", Version: 1}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT payload FROM oa_office_documents").WithArgs(doc.ID).
		WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(jsonData(doc)))
	mock.ExpectExec("UPDATE oa_office_documents SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO oa_office_document_events").WillReturnError(errors.New("audit write unavailable"))
	mock.ExpectRollback()

	app := office{db: db}
	request := httptest.NewRequest(http.MethodPost, "/oa-api/finance/documents/"+doc.ID+"/withdraw", strings.NewReader(`{"version":1}`))
	request.SetPathValue("id", doc.ID)
	request.SetPathValue("operation", "withdraw")
	result := httptest.NewRecorder()
	app.mutateDocument(result, platform.WithActor(request, actor))
	if result.Code != 500 {
		t.Fatalf("wanted 500, got %d: %s", result.Code, result.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinanceLimitedDetailRemovesPaymentAndReviewEvidence(t *testing.T) {
	doc := document{
		application: application{Account: "12345678", Reason: "private purpose", Files: []string{"invoice"}},
		ID:          "test-doc", State: "paid", Route: []int64{17},
		History: []history{{Comment: "private review"}},
		Payment: &payment{Amount: 1000, Date: "2026-11-18", Receipt: "private receipt", BankReference: "private reference"},
	}
	view := doc.limitedDetail()
	if view.ID != doc.ID || view.State != "paid" || view.Payment.Amount != 1000 || view.Payment.Date != doc.Payment.Date ||
		view.Account != "" || view.Reason != "" || len(view.Files) > 0 || len(view.Route) > 0 || len(view.History) > 0 ||
		view.Payment.Receipt != "" || view.Payment.BankReference != "" {
		t.Fatalf("unexpected limited projection: %+v", view)
	}
}

func TestFinanceAdminCanReadApprovedMetadataWithoutPaymentGrant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := platform.Actor{ID: 4, Actions: map[string]bool{"finance.admin": true}}
	doc := document{
		ID: "test-doc", Applicant: 2, State: "approved", Version: 3,
		application: application{Amount: 1000, Account: "sensitive", Files: []string{"invoice"}},
	}
	mock.ExpectQuery("SELECT payload FROM oa_office_documents").WithArgs(doc.ID).
		WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow(jsonData(doc)))
	app := office{db: db}
	request := httptest.NewRequest(http.MethodGet, "/oa-api/finance/documents/"+doc.ID+"?reveal=1", nil)
	request.SetPathValue("id", doc.ID)
	result := httptest.NewRecorder()
	app.detail(result, platform.WithActor(request, actor))
	var response struct{ Data document }
	if result.Code != 200 || json.Unmarshal(result.Body.Bytes(), &response) != nil || response.Data.ID != doc.ID ||
		response.Data.Amount != 1000 || response.Data.Account != "" || len(response.Data.Files) > 0 {
		t.Fatalf("unexpected metadata response: %d %s", result.Code, result.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
