package office

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"workflow/internal/oa/platform"
)

type financeNoticePayload struct {
	DocumentID string `json:"document_id"`
	State      string `json:"state"`
	Title      string `json:"title"`
	Kind       string `json:"kind"`
}

type financeNotice struct {
	ID        int64                `json:"id"`
	Type      string               `json:"notification_type"`
	Payload   financeNoticePayload `json:"payload"`
	CreatedAt string               `json:"created_at"`
}

type financeActivity struct {
	Todos       []financeNotice `json:"todos"`
	Items       []financeNotice `json:"items"`
	TodoCount   int64           `json:"todo_count"`
	ResultCount int64           `json:"result_count"`
}

// Called with the document row locked, in the same transaction as the state
// change. Submit retries, stale decisions, and identical payments return before
// recording an outcome. Intermediate approvals are actionable only to the next
// assignee and do not create an inbox result.
func recordFinanceResult(ctx context.Context, tx *sql.Tx, doc document) error {
	switch doc.State {
	case "approved", "returned", "rejected", "paid":
		payload := financeNoticePayload{DocumentID: doc.ID, State: doc.State, Title: doc.Template.Name, Kind: "result"}
		_, err := tx.ExecContext(ctx, "INSERT INTO oa_office_notifications(user_id,payload) VALUES(?,?)", doc.Applicant, jsonData(payload))
		return err
	default:
		return nil
	}
}

// Current document state is the source of truth for todo badges. A new payer
// sees pending payments immediately and someone whose grant was revoked sees
// none, even when the older event table still contains their notifications.
const financeTodoScope = ` FROM oa_office_documents d
	WHERE (d.state='waiting' AND d.approver=? AND d.applicant<>?)
	   OR (d.state='returned' AND d.applicant=?)
	   OR (? AND d.state='approved' AND d.applicant<>?)`

// Historical rows written before this projection include waiting notices and
// approved notices for payers. Only the applicant's outcome history is shown.
const financeResultScope = ` FROM oa_office_notifications n
	JOIN oa_office_documents d ON d.id=JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.document_id')) AND d.applicant=n.user_id
	WHERE n.user_id=? AND JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.state')) IN ('approved','returned','rejected','paid')`

func activityParam(r *http.Request, key string, fallback, max int) (int, bool) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(value)
	return n, err == nil && n >= 1 && n <= max
}

func (app office) notifications(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	todoPage, todoPageOK := activityParam(r, "todo_page", 1, 1000000)
	resultPage, resultPageOK := activityParam(r, "result_page", 1, 1000000)
	limit, limitOK := activityParam(r, "limit", 20, 100)
	summary := r.URL.Query().Get("summary")
	if !todoPageOK || !resultPageOK || !limitOK || (summary != "" && summary != "1") {
		bad(w, "通知分页参数无效")
		return
	}

	ctx := r.Context()
	tx, err := app.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()

	data := financeActivity{Todos: []financeNotice{}, Items: []financeNotice{}}
	todoArgs := []any{actor.ID, actor.ID, actor.ID, permit(actor, "finance.pay"), actor.ID}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*)"+financeTodoScope, todoArgs...).Scan(&data.TodoCount); err != nil {
		internal(w)
		return
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*)"+financeResultScope, actor.ID).Scan(&data.ResultCount); err != nil {
		internal(w)
		return
	}
	if summary != "1" {
		const todoFields = `SELECT 0, d.id, d.state, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(d.payload,'$.template.name')), ''),
			CASE d.state WHEN 'waiting' THEN 'approve' WHEN 'returned' THEN 'revise' ELSE 'pay' END,
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(d.payload,'$.history[last].at')), '') AS occurred_at`
		data.Todos, err = selectFinanceNotices(ctx, tx, todoFields+financeTodoScope+" ORDER BY occurred_at DESC,d.id LIMIT ? OFFSET ?", append(todoArgs, limit, (todoPage-1)*limit)...)
		if err != nil {
			internal(w)
			return
		}
		const resultFields = `SELECT n.id, d.id, JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.state')),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.title')), ''), 'result',
			DATE_FORMAT(n.created_at,'%Y-%m-%d %H:%i:%s')`
		data.Items, err = selectFinanceNotices(ctx, tx, resultFields+financeResultScope+" ORDER BY n.id DESC LIMIT ? OFFSET ?", actor.ID, limit, (resultPage-1)*limit)
		if err != nil {
			internal(w)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		internal(w)
		return
	}
	platform.JSON(w, http.StatusOK, data)
}

func selectFinanceNotices(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]financeNotice, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notices := []financeNotice{}
	for rows.Next() {
		notice := financeNotice{Type: "oa_finance_document"}
		p := &notice.Payload
		if err := rows.Scan(&notice.ID, &p.DocumentID, &p.State, &p.Title, &p.Kind, &notice.CreatedAt); err != nil {
			return nil, err
		}
		notices = append(notices, notice)
	}
	return notices, rows.Err()
}
