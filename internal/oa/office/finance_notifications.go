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
	Unread    bool                 `json:"unread"`
}

type financeActivity struct {
	Todos             []financeNotice `json:"todos"`
	Items             []financeNotice `json:"items"`
	ApproveCount      int64           `json:"approve_count"`
	PayCount          int64           `json:"pay_count"`
	TodoCount         int64           `json:"todo_count"`
	ResultCount       int64           `json:"result_count"`
	UnreadResultCount int64           `json:"unread_result_count"`
}

// Called with the document row locked, in the same transaction as the state
// change. Submit retries, stale decisions, and identical payments return before
// recording an outcome. Intermediate approvals are actionable only to the next
// assignee and do not create an inbox result.
func recordFinanceResult(ctx context.Context, tx *sql.Tx, doc document) error {
	switch doc.State {
	case "approved", "returned", "rejected", "paid", "voided":
		payload := struct {
			financeNoticePayload
			NotifyUnread bool `json:"notify_unread"`
		}{
			financeNoticePayload{DocumentID: doc.ID, State: doc.State, Title: doc.Template.Name, Kind: "result"},
			doc.State != "returned",
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO oa_office_notifications(user_id,payload) VALUES(?,?)", doc.Applicant, jsonData(payload))
		return err
	default:
		return nil
	}
}

// A CC is a separate append-only recipient record for each completed phase.
func recordFinanceCC(ctx context.Context, tx *sql.Tx, doc document) error {
	var recipients []int64
	switch doc.State {
	case "approved":
		recipients = doc.Template.CCAfterApproval
	case "paid":
		recipients = doc.Template.CCAfterPayment
	}
	for _, id := range recipients {
		payload := map[string]any{
			"document_id": doc.ID, "document_version": doc.Version, "state": doc.State,
			"title": doc.Template.Name, "kind": "cc", "notify_unread": true,
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO oa_office_notifications(user_id,payload) VALUES(?,?)", id, jsonData(payload)); err != nil {
			return err
		}
	}
	return nil
}

// Current document state is the source of truth for todo badges. A new payer
// sees pending payments immediately and someone whose grant was revoked sees
// none, even when the older event table still contains their notifications.
const financeTodoScope = ` FROM oa_office_documents d
	WHERE (d.state='waiting' AND ` + financePendingScope + ` AND d.applicant<>?)
	   OR (d.state='returned' AND d.applicant=?)
	   OR (? AND d.state='approved' AND d.applicant<>?)`

// Historical rows written before this projection include waiting notices and
// approved notices for payers. Show applicant outcomes and explicit CC only.
const financeResultJoin = ` oa_office_notifications n
	JOIN oa_office_documents d ON d.id=JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.document_id')) AND (d.applicant=n.user_id OR JSON_VALUE(n.payload,'$.kind')='cc')`
const financeResultFilter = `
	WHERE n.user_id=? AND JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.state')) IN ('approved','returned','rejected','paid','voided')`
const financeResultScope = " FROM" + financeResultJoin + financeResultFilter

// Only results created with unread tracking qualify. Return notices appear as
// a revision todo and stay in history without adding a second badge.
const financeResultUnread = `n.read_at IS NULL AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.notify_unread'))='true', FALSE)`

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
	todoArgs := []any{actor.ID, actor.ID, actor.ID, actor.ID, permit(actor, "finance.pay"), actor.ID}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(d.state='waiting'),0),COALESCE(SUM(d.state='approved'),0)"+financeTodoScope, todoArgs...).Scan(&data.TodoCount, &data.ApproveCount, &data.PayCount); err != nil {
		internal(w)
		return
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM("+financeResultUnread+"),0)"+financeResultScope, actor.ID).Scan(&data.ResultCount, &data.UnreadResultCount); err != nil {
		internal(w)
		return
	}
	if summary != "1" {
		const todoFields = `SELECT 0, d.id, d.state, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(d.payload,'$.template.name')), ''),
			CASE d.state WHEN 'waiting' THEN 'approve' WHEN 'returned' THEN 'revise' ELSE 'pay' END,
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(d.payload,'$.history[last].at')), '') AS occurred_at, FALSE`
		data.Todos, err = selectFinanceNotices(ctx, tx, todoFields+financeTodoScope+" ORDER BY occurred_at DESC,d.id LIMIT ? OFFSET ?", append(todoArgs, limit, (todoPage-1)*limit)...)
		if err != nil {
			internal(w)
			return
		}
		const resultFields = `SELECT n.id, d.id, JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.state')),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(n.payload,'$.title')), ''), CASE WHEN JSON_VALUE(n.payload,'$.kind')='cc' THEN 'cc' ELSE 'result' END,
			DATE_FORMAT(n.created_at,'%Y-%m-%d %H:%i:%s'), ` + financeResultUnread
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
		if err := rows.Scan(&notice.ID, &p.DocumentID, &p.State, &p.Title, &p.Kind, &notice.CreatedAt, &notice.Unread); err != nil {
			return nil, err
		}
		notices = append(notices, notice)
	}
	return notices, rows.Err()
}

func (app office) readFinanceResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		bad(w, "通知编号无效")
		return
	}
	const query = "UPDATE" + financeResultJoin + " SET n.read_at=CURRENT_TIMESTAMP" + financeResultFilter +
		" AND n.id=? AND " + financeResultUnread
	if _, err := app.db.ExecContext(r.Context(), query, platform.ActorFrom(r).ID, id); err != nil {
		internal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
