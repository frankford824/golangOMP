package office

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"workflow/internal/oa/platform"
)

type handoverApproval struct {
	ID           string `json:"id"`
	TemplateName string `json:"template_name"`
	Current      bool   `json:"current"`
}
type handoverTemplate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type handoverSubordinate struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	EmployeeNo *int   `json:"employee_no"`
	InScope    bool   `json:"-"`
}
type financeHandover struct {
	ApprovalCount  int                   `json:"pending_approval_count"`
	FutureCount    int                   `json:"future_approval_count"`
	TemplateCount  int                   `json:"bound_template_count"`
	Approvals      []handoverApproval    `json:"pending_approvals"`
	Payments       int                   `json:"pending_payments"`
	PaymentCovered bool                  `json:"payment_covered"`
	Templates      []handoverTemplate    `json:"bound_templates"`
	Subordinates   []handoverSubordinate `json:"subordinates"`
	OutOfScope     int                   `json:"out_of_scope_subordinates"`
	CanTransfer    bool                  `json:"can_transfer_approvals"`
	Ready          bool                  `json:"ready"`
}

func lockPersonnelFinance(ctx context.Context, tx *sql.Tx) error {
	var version int
	return tx.QueryRowContext(ctx, "SELECT version FROM oa_office_org_managers WHERE org_kind='lock' AND org_id=1 FOR UPDATE").Scan(&version)
}

const handoverWaitingDocuments = `SELECT payload FROM oa_office_documents d WHERE state='waiting'
	AND (d.approver=? OR JSON_CONTAINS(JSON_EXTRACT(d.payload,'$.route'),CAST(? AS JSON))
		OR JSON_CONTAINS(JSON_EXTRACT(d.payload,'$.approval_steps[*].approver_ids[*]'),CAST(? AS JSON)))
	ORDER BY id`

func (app office) financeHandoverSummary(r *http.Request, tx *sql.Tx, id int64) (financeHandover, error) {
	ctx, actor := r.Context(), platform.ActorFrom(r)
	out := financeHandover{
		Approvals: []handoverApproval{}, Templates: []handoverTemplate{}, Subordinates: []handoverSubordinate{},
		CanTransfer: financeAdmin(actor),
	}
	rows, err := tx.QueryContext(ctx, handoverWaitingDocuments, id, id, id)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		pending, future := waitingApprover(doc, id)
		if pending {
			out.ApprovalCount++
		} else if future {
			out.FutureCount++
		}
		if out.CanTransfer && (pending || future) {
			out.Approvals = append(out.Approvals, handoverApproval{ID: doc.ID, TemplateName: doc.Template.Name, Current: pending})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()

	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(EXISTS(SELECT 1 FROM oa_office_finance_actions g JOIN users u ON u.id=g.user_id
			WHERE g.can_pay AND u.status='active' AND g.user_id<>? AND g.user_id<>d.applicant)),0)=COUNT(*)
		FROM oa_office_documents d JOIN oa_office_finance_actions f ON f.user_id=? AND f.can_pay
		WHERE d.state IN ('approved','paying') AND d.applicant<>?`, id, id, id).Scan(&out.Payments, &out.PaymentCovered); err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT published FROM oa_office_templates WHERE published IS NOT NULL ORDER BY id")
	if err != nil {
		return out, err
	}
	var raw []byte
	for rows.Next() {
		var t template
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return out, err
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			rows.Close()
			return out, err
		}
		if templateHasApprover(t, id) {
			out.TemplateCount++
			if out.CanTransfer || permit(actor, "finance.templates") {
				out.Templates = append(out.Templates, handoverTemplate{ID: t.ID, Name: t.Name})
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()

	subordinates, err := reportingSubordinates(ctx, tx, actor, id)
	if err != nil {
		return out, err
	}
	for _, subordinate := range subordinates {
		if subordinate.InScope {
			out.Subordinates = append(out.Subordinates, subordinate)
		} else {
			out.OutOfScope++
		}
	}
	out.Ready = out.ApprovalCount == 0 && out.FutureCount == 0 && out.PaymentCovered && out.TemplateCount == 0 && len(subordinates) == 0
	return out, nil
}

func waitingApprover(doc document, id int64) (bool, bool) {
	return doc.pendingFor(id), doc.hasUnresolvedApprover(id)
}

func templateHasApprover(t template, id int64) bool {
	value := fmt.Sprintf("fixed:%d", id)
	if t.Extra == value || slices.Contains(t.Nodes, value) {
		return true
	}
	for _, node := range t.ApprovalNodes {
		if slices.Contains(node.Approvers, value) {
			return true
		}
	}
	for _, rule := range t.AmountRules {
		if slices.Contains(rule.Approvers, value) {
			return true
		}
	}
	return false
}

func reportingSubordinates(ctx context.Context, tx *sql.Tx, actor platform.Actor, id int64) ([]handoverSubordinate, error) {
	rows, err := tx.QueryContext(ctx, directoryPeople+`SELECT id,name,employee_no,department_id,team_id,protected
		FROM directory WHERE status='active' AND id<>? AND (manager_id=? OR manager_override_id=?) ORDER BY id`,
		append(directoryDates(), id, id, id)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []handoverSubordinate{}
	for rows.Next() {
		var item handoverSubordinate
		var p person
		var protected bool
		if err := rows.Scan(&item.ID, &item.Name, &item.EmployeeNo, &p.DepartmentID, &p.TeamID, &protected); err != nil {
			return nil, err
		}
		if protected {
			p.Management = "super_admin"
		}
		item.InScope = scopePerson(actor, p)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (app office) canClosePersonnel(w http.ResponseWriter, r *http.Request, tx *sql.Tx, id int64) bool {
	check, err := app.financeHandoverSummary(r, tx, id)
	if err != nil {
		internal(w)
		return false
	}
	if !check.Ready {
		platform.Error(w, 409, "请先完成费用审批、模板、付款经办和直属下属交接")
		return false
	}
	return true
}

func (app office) applyFinanceHandover(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), platform.ActorFrom(r)
	id := number(r.PathValue("id"))
	p, err := app.readPerson(r, id)
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if !scopePerson(actor, p) {
		deny(w)
		return
	}
	var input struct {
		Kind     string `json:"kind"`
		TargetID int64  `json:"target_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.TargetID <= 0 || input.TargetID == id || !slices.Contains([]string{"approvals", "subordinates"}, input.Kind) {
		bad(w, "请选择适用的交接操作及接手人")
		return
	}
	if input.Kind == "approvals" && !financeAdmin(actor) {
		deny(w)
		return
	}
	tx, err := app.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	if err := lockPersonnelFinance(ctx, tx); err != nil {
		internal(w)
		return
	}
	if err := lockManagers(ctx, tx); err != nil {
		internal(w)
		return
	}
	if _, err := app.allowedManager(r, actor, input.TargetID); err != nil {
		profileFailure(w, err)
		return
	}
	var count int
	switch input.Kind {
	case "approvals":
		count, err = transferFinanceApprovals(r, tx, id, input.TargetID)
	case "subordinates":
		count, err = replaceSubordinateManagers(r, tx, id, input.TargetID)
	}
	if errors.Is(err, errHandoverRecipient) || errors.Is(err, errManagerCycle) {
		bad(w, err.Error())
		return
	}
	if errors.Is(err, errHandoverScope) {
		deny(w)
		return
	}
	if err != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]int{"count": count})
}

var errHandoverRecipient = errors.New("接手人是申请人或同节点其他审批人，请更换接手人")
var errHandoverScope = errors.New("有直属下属超出当前人员管理范围")

func transferFinanceApprovals(r *http.Request, tx *sql.Tx, from, to int64) (int, error) {
	ctx := r.Context()
	// The personnel-finance lock serializes routing mutations. Select affected IDs first, then lock only the documents being changed.
	rows, err := tx.QueryContext(ctx, handoverWaitingDocuments, from, from, from)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		pending, future := waitingApprover(doc, from)
		if pending || future {
			ids = append(ids, doc.ID)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	count := 0
	for _, id := range ids {
		doc, err := scanDocument(tx.QueryRowContext(ctx, "SELECT payload FROM oa_office_documents WHERE id=? FOR UPDATE", id))
		if err != nil {
			return 0, err
		}
		pending, future := waitingApprover(doc, from)
		if doc.State != "waiting" || !pending && !future {
			continue
		}
		if doc.Applicant == to || !doc.transferPendingApprover(from, to) {
			return 0, errHandoverRecipient
		}
		entry := newFinanceHistory(platform.ActorFrom(r), "personnel_handover")
		entry.NodeName = "离职审批交接"
		entry.BeforeState, entry.AfterState = doc.State, doc.State
		entry.Comment = fmt.Sprintf("审批人 #%d → #%d", from, to)
		doc.History = append(doc.History, entry)
		doc.Version++
		if err := persistDocument(r, tx, doc); err != nil {
			return 0, err
		}
		if err := appendDocumentEvent(ctx, tx, doc, entry); err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func replaceSubordinateManagers(r *http.Request, tx *sql.Tx, from, to int64) (int, error) {
	ctx, actor := r.Context(), platform.ActorFrom(r)
	subordinates, err := reportingSubordinates(ctx, tx, actor, from)
	if err != nil {
		return 0, err
	}
	for _, item := range subordinates {
		if !item.InScope {
			return 0, errHandoverScope
		}
		if item.ID == to {
			return 0, errManagerCycle
		}
		var draft profileUpdate
		err := tx.QueryRowContext(ctx, directoryPeople+`SELECT profile_version,joined,job_title,job_level,cost_center
			FROM directory WHERE id=?`, append(directoryDates(), item.ID)...).
			Scan(&draft.Version, &draft.Joined, &draft.JobTitle, &draft.JobLevel, &draft.CostCenter)
		if err != nil {
			return 0, err
		}
		draft.ManagerID = to
		if err := writeProfile(ctx, tx, item.ID, actor.ID, draft); err != nil {
			return 0, err
		}
	}
	if err := checkManagerGraph(ctx, tx); err != nil {
		return 0, err
	}
	return len(subordinates), nil
}

func (app office) deactivatePerson(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), platform.ActorFrom(r)
	p, err := app.readPerson(r, number(r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if !scopePerson(actor, p) || p.Management == "super_admin" || p.ID == actor.ID {
		deny(w)
		return
	}
	tx, err := app.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	if err := lockPersonnelFinance(ctx, tx); err != nil {
		internal(w)
		return
	}
	if err := lockManagers(ctx, tx); err != nil {
		internal(w)
		return
	}
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM users WHERE id=?", p.ID).Scan(&status); err != nil {
		internal(w)
		return
	}
	today := businessDate().Format(time.DateOnly)
	if status == "disabled" {
		// A completed native call can outlive a disconnected OA request. Link its pending intent to
		// a fresh observation instead of assigning the source change an unproven time or actor.
		var operationID string
		err := tx.QueryRowContext(ctx, `SELECT JSON_UNQUOTE(JSON_EXTRACT(i.payload,'$.operation_id'))
			FROM oa_office_person_events i WHERE i.user_id=? AND i.kind='account_disable_requested'
			AND NOT EXISTS(SELECT 1 FROM oa_office_person_events e WHERE e.user_id=i.user_id
				AND e.kind IN ('account_disabled','account_disabled_observed') AND JSON_EXTRACT(e.payload,'$.operation_id')=JSON_EXTRACT(i.payload,'$.operation_id'))
			ORDER BY i.id DESC LIMIT 1`, p.ID).Scan(&operationID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			internal(w)
			return
		}
		if operationID != "" {
			if err := recordPersonEvent(ctx, tx, p.ID, change{Kind: "account_disabled_observed", OperationID: operationID, ActorID: actor.ID, EffectiveDate: today}); err != nil {
				internal(w)
				return
			}
		}
		if tx.Commit() != nil {
			internal(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !app.canClosePersonnel(w, r, tx, p.ID) {
		return
	}
	operationID := identifier()
	if err := recordPersonEvent(ctx, app.db, p.ID, change{Kind: "account_disable_requested", OperationID: operationID, ActorID: actor.ID, EffectiveDate: today}); err != nil {
		internal(w)
		return
	}
	_, ok := app.nativePersonnelRequest(w, r, http.MethodPost, fmt.Sprintf("/v1/users/%d/deactivate", p.ID), "{}")
	if !ok {
		return
	}
	if err := recordPersonEvent(ctx, tx, p.ID, change{Kind: "account_disabled", OperationID: operationID, ActorID: actor.ID, EffectiveDate: today}); err != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (app office) nativePersonnelRequest(w http.ResponseWriter, r *http.Request, method, path, body string) (int, bool) {
	if app.native == nil {
		platform.Error(w, 503, "主站人员服务暂不可用")
		return 0, false
	}
	req, err := http.NewRequestWithContext(r.Context(), method, path, strings.NewReader(body))
	if err != nil {
		internal(w)
		return 0, false
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	response := &personnelNativeResponse{header: make(http.Header)}
	app.native.ServeHTTP(response, req)
	switch response.status {
	case 200, 204:
		return response.status, true
	case 400, 401, 403, 404, 409, 422:
		platform.Error(w, response.status, "主站拒绝人员变更，请核对账号状态和授权")
	default:
		platform.Error(w, 503, "主站人员服务暂不可用")
	}
	return 0, false
}

type personnelNativeResponse struct {
	header http.Header
	status int
}

func (r *personnelNativeResponse) Header() http.Header            { return r.header }
func (r *personnelNativeResponse) WriteHeader(status int)         { r.status = status }
func (r *personnelNativeResponse) Write(body []byte) (int, error) { return len(body), nil }
