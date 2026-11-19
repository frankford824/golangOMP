package office

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"workflow/internal/oa/platform"
)

const handledFinanceActions = "'approve','return','reject','payment','void'"

const financeDoneScope = `EXISTS (SELECT 1 FROM oa_office_document_events e
 WHERE e.document_id=CONVERT(d.id USING ascii) COLLATE ascii_bin
 AND e.operator_id=? AND e.action IN (` + handledFinanceActions + `))`

// A CC must have an explicit recipient in an internally authored notification.
const financeCCScope = `EXISTS (SELECT 1 FROM oa_office_notifications n
 WHERE n.user_id=? AND JSON_VALUE(n.payload,'$.kind')='cc'
 AND BINARY JSON_VALUE(n.payload,'$.document_id')=BINARY d.id)`

func hasFinanceReadEvent(ctx context.Context, db *sql.DB, actorID int64, docID string) (bool, error) {
	var allowed bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM oa_office_documents d
 WHERE d.id=? AND (`+financeDoneScope+` OR `+financeCCScope+`))`, docID, actorID, actorID).Scan(&allowed)
	return allowed, err
}

// Amounts in query projections are base-10 strings of cents so that both
// JavaScript and spreadsheet software preserve values beyond their exact integers.
type financeTotals struct {
	Count     int64  `json:"count"`
	Requested string `json:"requested_cents"`
	Approved  string `json:"approved_cents"`
	Paid      string `json:"paid_cents"`
	Unpaid    string `json:"unpaid_cents"`
	InFlight  string `json:"in_flight_cents"`
}

type financeGroup struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	financeTotals
}

type financeRow struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	ApplicantID   string `json:"applicant_id"`
	ApplicantName string `json:"applicant_name"`
	ApplicantNo   string `json:"applicant_employee_no"`
	Department    string `json:"department_name"`
	Team          string `json:"team_name"`
	Site          string `json:"site"`
	Template      string `json:"template_name"`
	ExpenseType   string `json:"expense_type"`
	Channel       string `json:"channel"`
	SubmittedDate string `json:"submitted_date"`
	ApprovedDate  string `json:"approved_date"`
	PaidDate      string `json:"paid_date"`
	Requested     string `json:"requested_cents"`
	Approved      string `json:"approved_cents"`
	Paid          string `json:"paid_cents"`
	Purpose       string `json:"purpose_summary,omitempty"`
	Payee         string `json:"payee,omitempty"`
	ApprovalState string `json:"approval_status,omitempty"`
	PaymentState  string `json:"payment_status,omitempty"`
	PayerID       string `json:"payment_operator_id,omitempty"`
	PayerName     string `json:"payment_operator_name,omitempty"`
}

type financeFilters struct {
	sort, order                                                                                                        string
	page, size                                                                                                         int
	start, end, basis, group, state, keyword, template, department, team, person, site, channel, min, max, expenseType string
}

var financeDimensions = map[string][2]string{
	"department": {"COALESCE(NULLIF(department_id,''),department_name)", "department_name"},
	"team":       {"CONCAT(department_id, ':', department_name, ':', team_name)", "team_name"},
	"person":     {"applicant", "applicant_name"},
	"site":       {"site", "site"},
	"template":   {"COALESCE(NULLIF(template_id,''),template_name)", "template_name"},
	"channel":    {"channel", "channel"},
	"status":     {"state", "state"},
	"payee":      {"payee", "payee"},
}

var financeDateColumns = map[string]string{
	"submitted": "submitted_date", "approved": "approved_date", "paid": "paid_date",
}

func parseFinanceFilters(r *http.Request, report bool) (financeFilters, string) {
	q := r.URL.Query()
	f := financeFilters{
		basis: q.Get("basis"), group: q.Get("group_by"), state: q.Get("state"), keyword: strings.TrimSpace(q.Get("q")),
		template: q.Get("template"), expenseType: q.Get("expense_type"), department: q.Get("department"), team: q.Get("team"), person: q.Get("person"), site: q.Get("site"), channel: q.Get("channel"),
		sort: q.Get("sort"), order: q.Get("order"), start: q.Get("start"), end: q.Get("end"), min: q.Get("min_amount_cents"), max: q.Get("max_amount_cents"),
	}
	var valid bool
	f.page, valid = activityParam(r, "page", 1, 1_000_000)
	if !valid {
		return f, "页码无效"
	}
	sizeKey := "size"
	if q.Get(sizeKey) == "" {
		sizeKey = "page_size"
	}
	f.size, valid = activityParam(r, sizeKey, 15, 100)
	if !valid {
		return f, "每页条数无效"
	}
	if f.sort != "" && f.sort != "date" && f.sort != "amount" {
		return f, "排序字段无效"
	}
	if f.order != "" && f.order != "asc" && f.order != "desc" {
		return f, "排序方向无效"
	}
	switch f.expenseType {
	case "", "reimbursement", "payment":
	default:
		return f, "费用类型无效"
	}
	switch f.state {
	case "", "waiting", "returned", "rejected", "approved", "paid", "withdrawn", "voided":
	default:
		return f, "单据状态无效"
	}
	for _, value := range []string{f.keyword, f.template, f.department, f.team, f.person, f.site, f.channel} {
		if len(value) > 240 {
			return f, "筛选内容过长"
		}
	}
	if f.person != "" {
		n, err := strconv.ParseInt(f.person, 10, 64)
		if err != nil || n <= 0 {
			return f, "人员编号无效"
		}
	}
	var amounts [2]int64
	for i, value := range [2]string{f.min, f.max} {
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 0 {
			return f, "金额范围无效"
		}
		amounts[i] = parsed
	}
	if f.max != "" && amounts[0] > amounts[1] {
		return f, "金额范围无效"
	}
	if f.basis == "" {
		f.basis = "submitted"
		if report {
			f.basis = "paid"
		}
	}
	if financeDateColumns[f.basis] == "" {
		return f, "统计日期口径无效"
	}
	if f.group == "" {
		f.group = "department"
	}
	if _, ok := financeDimensions[f.group]; !ok {
		return f, "汇总维度无效"
	}
	if f.start == "" && f.end == "" && report {
		period := q.Get("period")
		if period == "" {
			period = businessDate().Format("2006-01")
		}
		layout := map[int]string{4: "2006", 7: "2006-01", 10: time.DateOnly}[len(period)]
		start, err := time.Parse(layout, period)
		if err != nil || layout == "" {
			return f, "统计期间无效"
		}
		end := start
		switch layout {
		case "2006":
			end = start.AddDate(1, 0, -1)
		case "2006-01":
			end = start.AddDate(0, 1, -1)
		}
		f.start, f.end = start.Format(time.DateOnly), end.Format(time.DateOnly)
	}
	if f.start != "" || f.end != "" {
		start, err := time.Parse(time.DateOnly, f.start)
		end, endErr := time.Parse(time.DateOnly, f.end)
		if err != nil || endErr != nil || start.Format(time.DateOnly) != f.start || end.Format(time.DateOnly) != f.end ||
			start.Year() < 1000 || end.Before(start) {
			return f, "请选择有效的起止日期"
		}
	}
	return f, ""
}

// Newly written and historical event timestamps are UTC. The paid_date column
// is already a business date. For legacy rows, approval can be recovered from
// the last approval event only for states that prove approval was completed.
const financeProjection = `WITH extracted AS (
 SELECT d.*,
 COALESCE(JSON_VALUE(payload,'$.applicant_name'),'') AS applicant_name,
 COALESCE(JSON_VALUE(payload,'$.applicant_employee_no'),'') AS applicant_no,
 COALESCE(JSON_VALUE(payload,'$.department_id'),'') AS department_id,
 COALESCE(JSON_VALUE(payload,'$.department_name'),'') AS department_name,
 COALESCE(JSON_VALUE(payload,'$.team_name'),'') AS team_name,
 COALESCE(JSON_VALUE(payload,'$.site'),'') AS site,
 COALESCE(JSON_VALUE(payload,'$.template.id'),'') AS template_id,
 COALESCE(JSON_VALUE(payload,'$.template.name'),'') AS template_name,
 CASE JSON_VALUE(payload,'$.template.type')
   WHEN '报销' THEN 'reimbursement' WHEN 'reimbursement' THEN 'reimbursement'
   WHEN '付款' THEN 'payment' WHEN 'payment' THEN 'payment' ELSE '' END AS expense_type,
 COALESCE(JSON_VALUE(payload,'$.channel'),'') AS channel,
 COALESCE(JSON_VALUE(payload,'$.holder'),'') AS payee,
 COALESCE(LEFT(JSON_UNQUOTE(JSON_EXTRACT(payload,'$.reason')),240),'') AS purpose,
 COALESCE(JSON_VALUE(payload,'$.amount_cents' RETURNING DECIMAL(30,0)),0) AS request_amount,
 JSON_VALUE(payload,'$.approved_amount_cents' RETURNING DECIMAL(30,0)) AS approved_amount,
 JSON_VALUE(payload,'$.payment.amount_cents' RETURNING DECIMAL(30,0)) AS paid_amount,
 DATE(DATE_ADD(REPLACE(LEFT(COALESCE(JSON_VALUE(payload,'$.submitted_at'), JSON_VALUE(payload,'$.history[0].at')),19),'T',' '),INTERVAL 8 HOUR)) AS submitted_date,
 DATE(DATE_ADD(REPLACE(LEFT(COALESCE(JSON_VALUE(payload,'$.approved_at'),
  CASE WHEN d.state IN ('approved','paid','voided') THEN
   (SELECT h.event_at FROM JSON_TABLE(d.payload,'$.history[*]' COLUMNS (
     pos FOR ORDINALITY, event_action VARCHAR(24) PATH '$.action', event_at VARCHAR(40) PATH '$.at'
   )) h WHERE h.event_action='approve' ORDER BY h.pos DESC LIMIT 1) END),19),'T',' '),INTERVAL 8 HOUR)) AS approved_date
 FROM oa_office_documents d WHERE `

const financeTotalsSQL = `COUNT(*), COALESCE(SUM(request_amount),0),
 COALESCE(SUM(CASE WHEN state IN ('approved','paid','voided') THEN COALESCE(approved_amount,request_amount) ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN state='paid' THEN COALESCE(paid_amount,0) ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN state='approved' THEN COALESCE(approved_amount,request_amount) ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN state IN ('waiting','returned') THEN request_amount ELSE 0 END),0)`

const financeRowSQL = `id,state,applicant,applicant_name,applicant_no,department_name,team_name,site,template_name,expense_type,channel,
 COALESCE(DATE_FORMAT(submitted_date,'%Y-%m-%d'),''),COALESCE(DATE_FORMAT(approved_date,'%Y-%m-%d'),''),COALESCE(DATE_FORMAT(paid_date,'%Y-%m-%d'),''),
 request_amount,
 COALESCE(CAST(CASE WHEN state IN ('approved','paid','voided') THEN COALESCE(approved_amount,request_amount) END AS CHAR),''),
 COALESCE(CAST(CASE WHEN state='paid' THEN paid_amount END AS CHAR),'')`

const financeReportRowSQL = `,purpose,payee,
 CASE WHEN state IN ('approved','paid','voided') THEN 'approved' ELSE state END,
 CASE state WHEN 'paid' THEN 'paid' WHEN 'approved' THEN 'pending' WHEN 'voided' THEN 'voided' ELSE 'not_due' END,
 CASE WHEN state='paid' THEN COALESCE(CAST((SELECT e.operator_id FROM oa_office_document_events e
  WHERE e.document_id=CONVERT(matched.id USING ascii) COLLATE ascii_bin AND e.action='payment'
  ORDER BY e.document_version DESC LIMIT 1) AS CHAR),
  JSON_UNQUOTE(JSON_EXTRACT(matched.payload,REPLACE(JSON_UNQUOTE(
    JSON_SEARCH(matched.payload,'one','payment',NULL,'$.history[*].action')),'.action','.actor_id'))), '') ELSE '' END,
 CASE WHEN state='paid' THEN COALESCE((SELECT JSON_VALUE(e.payload,'$.operator_name') FROM oa_office_document_events e
  WHERE e.document_id=CONVERT(matched.id USING ascii) COLLATE ascii_bin AND e.action='payment'
  ORDER BY e.document_version DESC LIMIT 1),'') ELSE '' END`

func financeQuery(scope string, args []any, f financeFilters, report bool) (string, []any) {
	query := financeProjection + scope + "), matched AS (SELECT * FROM extracted WHERE 1=1"
	if f.start != "" {
		column := financeDateColumns[f.basis]
		query += " AND " + column + " BETWEEN ? AND ?"
		args = append(args, f.start, f.end)
	}
	for _, filter := range []struct{ column, value string }{
		{"state", f.state}, {"template_name", f.template}, {"expense_type", f.expenseType}, {"department_name", f.department}, {"team_name", f.team},
		{"site", f.site}, {"channel", f.channel},
	} {
		if filter.value != "" {
			query += " AND " + filter.column + "=?"
			args = append(args, filter.value)
		}
	}
	if f.person != "" {
		applicantID, _ := strconv.ParseInt(f.person, 10, 64)
		query += " AND applicant=?"
		args = append(args, applicantID)
	}
	if f.min != "" {
		query += " AND request_amount>=CAST(? AS DECIMAL(30,0))"
		args = append(args, f.min)
	}
	if f.max != "" {
		query += " AND request_amount<=CAST(? AS DECIMAL(30,0))"
		args = append(args, f.max)
	}
	if f.keyword != "" {
		fields := "id, applicant, applicant_name, applicant_no, template_name, department_name, team_name, site"
		if report {
			fields += ",purpose,payee"
		}
		query += " AND LOCATE(?, CONCAT_WS(' '," + fields + "))>0"
		args = append(args, f.keyword)
	}
	return query + ") ", args
}

func financeTotalTargets(t *financeTotals) []any {
	return []any{&t.Count, &t.Requested, &t.Approved, &t.Paid, &t.Unpaid, &t.InFlight}
}

func readFinanceGroups(ctx context.Context, tx *sql.Tx, query string, args []any, dimension string) ([]financeGroup, error) {
	by := financeDimensions[dimension]
	rows, err := tx.QueryContext(ctx, query+"SELECT "+by[0]+", MAX(NULLIF("+by[1]+",'')), "+financeTotalsSQL+
		" FROM matched GROUP BY "+by[0]+" ORDER BY COUNT(*) DESC, "+by[0], args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []financeGroup{}
	for rows.Next() {
		var g financeGroup
		var label sql.NullString
		if err = rows.Scan(append([]any{&g.Key, &label}, financeTotalTargets(&g.financeTotals)...)...); err != nil {
			return nil, err
		}
		g.Label = label.String
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// Legacy readers requested the unpaginated contract by passing only the
// original query parameter. Any other parameter selects the newer contract.
func legacyFinanceQuery(r *http.Request, originalParam string) bool {
	q := r.URL.Query()
	return len(q) == 0 || (len(q) == 1 && q.Has(originalParam))
}

func (app office) queryFinanceDocuments(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	q := r.URL.Query().Get("queue")
	scope := "d.applicant=?"
	args := []any{actor.ID}
	switch q {
	case "", "mine":
	case "todo":
		scope = "d.state='waiting' AND " + financePendingScope + " AND d.applicant<>?"
		args = append(args, actor.ID, actor.ID)
	case "pay":
		if !permit(actor, "finance.pay") {
			deny(w)
			return
		}
		scope = "d.state='approved' AND d.applicant<>?"
	case "done":
		scope = financeDoneScope
	case "cc":
		scope = financeCCScope
	case "all", "ledger":
		if !financeAdmin(actor) && !permit(actor, "finance.report") {
			deny(w)
			return
		}
		scope, args = "1=1", nil
	default:
		bad(w, "队列无效")
		return
	}
	originalQueue := q == "" || q == "mine" || q == "todo" || q == "pay"
	if originalQueue && legacyFinanceQuery(r, "queue") {
		app.legacyFinanceDocuments(w, r, scope, args, false)
		return
	}
	f, problem := parseFinanceFilters(r, false)
	if problem != "" {
		bad(w, problem)
		return
	}
	app.respondFinanceQuery(w, r, scope, args, f, originalQueue, false, false)
}

func (app office) report(w http.ResponseWriter, r *http.Request) {
	if !permit(platform.ActorFrom(r), "finance.report") {
		deny(w)
		return
	}
	f, problem := parseFinanceFilters(r, true)
	if problem != "" {
		bad(w, problem)
		return
	}
	if legacyFinanceQuery(r, "period") {
		app.legacyFinanceDocuments(w, r, "d.state='paid' AND d.paid_date BETWEEN ? AND ?", []any{f.start, f.end}, true)
		return
	}
	format := r.URL.Query().Get("format")
	if format != "" && format != "xlsx" {
		bad(w, "导出格式无效")
		return
	}
	app.respondFinanceQuery(w, r, "1=1", nil, f, false, true, format == "xlsx")
}

func (app office) legacyFinanceDocuments(w http.ResponseWriter, r *http.Request, scope string, args []any, report bool) {
	rows, err := app.db.QueryContext(r.Context(), "SELECT d.payload FROM oa_office_documents d WHERE "+scope+" ORDER BY d.paid_date DESC,d.id", args...)
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	actor := platform.ActorFrom(r)
	items := []document{}
	var amount int64
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			internal(w)
			return
		}
		if report {
			if doc.Payment != nil {
				amount += doc.Payment.Amount
				doc.Payment.BankReference = ""
			}
			for i := range doc.History {
				doc.History[i].BankReference = ""
			}
		}
		items = append(items, doc.visible(actor, false))
	}
	if rows.Err() != nil {
		internal(w)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	platform.JSON(w, 200, map[string]any{"items": items, "amount_cents": amount, "count": len(items)})
}

func (app office) respondFinanceQuery(w http.ResponseWriter, r *http.Request, scope string, args []any, f financeFilters, documents, report, export bool) {
	ctx := r.Context()
	query, args := financeQuery(scope, args, f, report)
	tx, err := app.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	w.Header().Set("Cache-Control", "private, no-store")
	result := map[string]any{"page": f.page, "size": f.size, "page_size": f.size}
	var total int64
	var totals financeTotals
	var groups []financeGroup
	if report {
		err = tx.QueryRowContext(ctx, query+"SELECT "+financeTotalsSQL+" FROM matched", args...).Scan(financeTotalTargets(&totals)...)
		total = totals.Count
		if err == nil {
			groups, err = readFinanceGroups(ctx, tx, query, args, f.group)
		}
		result["summary"], result["groups"] = totals, groups
		result["amount_cents"], result["basis"], result["group_by"] = totals.Paid, f.basis, f.group
		result["start"], result["end"] = f.start, f.end
	} else {
		err = tx.QueryRowContext(ctx, query+"SELECT COUNT(*) FROM matched", args...).Scan(&total)
		result["amount_cents"] = 0 // Previous queue responses included this unused field.
	}
	if err != nil {
		internal(w)
		return
	}
	result["count"], result["total"] = total, total
	if export && (total > 1048575 || len(groups) > 1048575) {
		bad(w, "记录超过 Excel 工作表上限，请缩小查询范围")
		return
	}
	column := financeDateColumns[f.basis]
	selectSQL := financeRowSQL
	if report {
		selectSQL += financeReportRowSQL
	}
	if documents {
		selectSQL = "payload"
	}
	if f.sort == "amount" {
		column = "request_amount"
		if r.URL.Query().Get("queue") == "pay" {
			column = "COALESCE(approved_amount,request_amount)"
		}
	}
	direction := " DESC"
	if f.order == "asc" {
		direction = " ASC"
	}
	order := column + direction + ", id DESC"
	if !export {
		order += " LIMIT ? OFFSET ?"
		args = append(args, f.size, (f.page-1)*f.size)
	}
	rows, err := tx.QueryContext(ctx, query+"SELECT "+selectSQL+" FROM matched ORDER BY "+order, args...)
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	if export {
		book, err := buildFinanceWorkbook(rows, f, totals, groups)
		if err != nil {
			internal(w)
			return
		}
		defer book.Close()
		if err := tx.Commit(); err != nil {
			internal(w)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename=oa-finance-report.xlsx")
		_ = book.Write(w)
		return
	}
	if documents {
		items := []document{}
		actor := platform.ActorFrom(r)
		for rows.Next() {
			doc, err := scanDocument(rows)
			if err != nil {
				internal(w)
				return
			}
			items = append(items, doc.visible(actor, false))
		}
		result["items"] = items
	} else {
		items := []financeRow{}
		for rows.Next() {
			row, err := scanFinanceRow(rows, report)
			if err != nil {
				internal(w)
				return
			}
			items = append(items, row)
		}
		result["items"] = items
	}
	if rows.Err() != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, result)
}

func scanFinanceRow(rows *sql.Rows, report bool) (financeRow, error) {
	var x financeRow
	targets := []any{&x.ID, &x.State, &x.ApplicantID, &x.ApplicantName, &x.ApplicantNo, &x.Department, &x.Team,
		&x.Site, &x.Template, &x.ExpenseType, &x.Channel, &x.SubmittedDate, &x.ApprovedDate, &x.PaidDate, &x.Requested, &x.Approved, &x.Paid}
	if report {
		targets = append(targets, &x.Purpose, &x.Payee, &x.ApprovalState, &x.PaymentState, &x.PayerID, &x.PayerName)
	}
	err := rows.Scan(targets...)
	return x, err
}

func buildFinanceWorkbook(rows *sql.Rows, f financeFilters, t financeTotals, groups []financeGroup) (*excelize.File, error) {
	book := excelize.NewFile()
	ok := false
	defer func() {
		if !ok {
			_ = book.Close()
		}
	}()
	const summary, group, detail = "汇总", "分组", "单据明细"
	if err := book.SetSheetName("Sheet1", summary); err != nil {
		return nil, err
	}
	for _, sheet := range []string{group, detail} {
		if _, err := book.NewSheet(sheet); err != nil {
			return nil, err
		}
	}
	basis := map[string]string{"submitted": "首次提交日期", "approved": "完成审批日期", "paid": "实际付款日期"}[f.basis]
	dimension := map[string]string{"department": "部门", "team": "团队", "person": "人员", "site": "店铺", "template": "模板", "channel": "付款渠道", "status": "单据状态", "payee": "收款方"}[f.group]
	totalHeader := []any{"单据数", "申请金额(分)", "核定金额(分)", "实付金额(分)", "未付金额(分)", "在途金额(分)"}
	totalValues := func(t financeTotals) []any {
		return []any{t.Count, t.Requested, t.Approved, t.Paid, t.Unpaid, t.InFlight}
	}
	for i, row := range [][]any{
		{"日期口径", basis, "开始日期", f.start, "结束日期（含）", f.end, "分组维度", dimension},
		{"状态", financeStateLabel(f.state), "搜索", f.keyword, "费用类型", financeExpenseTypeLabel(f.expenseType), "模板", f.template, "部门", f.department},
		{"团队", f.team, "人员 ID", f.person, "店铺", f.site, "渠道", f.channel},
		{"申请金额下限(分)", f.min, "申请金额上限(分)", f.max},
		{"未付：筛选范围内当前已批准待付款。"},
		{"在途：筛选范围内当前待审批或退回待修改，按申请金额。"},
		{"申请、核定与实付金额均以分为单位并保留原始精度。"},
		totalHeader, totalValues(t),
	} {
		if err := book.SetSheetRow(summary, fmt.Sprintf("A%d", i+1), &row); err != nil {
			return nil, err
		}
	}
	gs, err := book.NewStreamWriter(group)
	if err != nil {
		return nil, err
	}
	if err = gs.SetRow("A1", append([]any{dimension}, totalHeader...)); err != nil {
		return nil, err
	}
	for i, g := range groups {
		label := g.Label
		if f.group == "status" {
			label = financeStateLabel(label)
		}
		if label == "" {
			label = "未记录"
		}
		if err = gs.SetRow(fmt.Sprintf("A%d", i+2), append([]any{label}, totalValues(g.financeTotals)...)); err != nil {
			return nil, err
		}
	}
	if err = gs.Flush(); err != nil {
		return nil, err
	}
	ds, err := book.NewStreamWriter(detail)
	if err != nil {
		return nil, err
	}
	if err = ds.SetRow("A1", []any{"单号", "状态", "申请人 ID", "申请人", "员工编号", "部门", "团队", "店铺", "模板", "费用类型", "付款渠道", "首次提交日期", "完成审批日期", "实际付款日期", "申请金额(分)", "核定金额(分)", "实付金额(分)", "用途摘要", "收款对象", "审批状态", "付款状态", "付款经办人ID", "付款经办人"}); err != nil {
		return nil, err
	}
	for i := 2; rows.Next(); i++ {
		x, err := scanFinanceRow(rows, true)
		if err != nil {
			return nil, err
		}
		if err = ds.SetRow(fmt.Sprintf("A%d", i), []any{x.ID, financeStateLabel(x.State), x.ApplicantID, x.ApplicantName, x.ApplicantNo, x.Department,
			x.Team, x.Site, x.Template, financeExpenseTypeLabel(x.ExpenseType), x.Channel, x.SubmittedDate, x.ApprovedDate, x.PaidDate, x.Requested, x.Approved, x.Paid, x.Purpose, x.Payee, approvalStateLabel(x.ApprovalState), paymentStateLabel(x.PaymentState), x.PayerID, x.PayerName}); err != nil {
			return nil, err
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = ds.Flush(); err != nil {
		return nil, err
	}
	ok = true
	return book, nil
}

func financeStateLabel(state string) string {
	labels := map[string]string{"waiting": "待审批", "returned": "退回待修改", "rejected": "已拒绝", "approved": "待付款", "paid": "已付款", "withdrawn": "已撤回", "voided": "已作废"}
	return labels[state]
}

func approvalStateLabel(state string) string {
	if state == "approved" {
		return "已批准"
	}
	return financeStateLabel(state)
}

func paymentStateLabel(state string) string {
	return map[string]string{"paid": "已付款", "pending": "待付款", "voided": "已作废", "not_due": "未进入付款"}[state]
}

func financeExpenseTypeLabel(kind string) string {
	switch kind {
	case "reimbursement":
		return "报销"
	case "payment":
		return "付款"
	default:
		return ""
	}
}
