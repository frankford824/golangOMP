package office

import (
	"database/sql"
	"net/http"
	"slices"
	"strings"

	"workflow/internal/oa/platform"
)

const financeGrantProjection = "WITH recipients AS (" +
	" SELECT user_id FROM oa_office_delegates WHERE level IN ('draft','publish')" +
	" UNION SELECT user_id FROM oa_office_finance_actions WHERE can_pay OR can_report" +
	"), grants AS (" +
	" SELECT a.user_id AS id, COALESCE(u.display_name,'') AS name,COALESCE(u.department,'') AS department,u.employee_no," +
	" COALESCE(u.status,'missing') AS status,COALESCE(d.level,'none') AS level," +
	" COALESCE(f.can_pay,FALSE) AS pay,COALESCE(f.can_report,FALSE) AS report" +
	" FROM recipients a LEFT JOIN oa_office_delegates d ON d.user_id=a.user_id" +
	" LEFT JOIN oa_office_finance_actions f ON f.user_id=a.user_id LEFT JOIN users u ON u.id=a.user_id)"

func (app office) financeGrantsPage(w http.ResponseWriter, r *http.Request, kind string) {
	q := r.URL.Query()
	page, okPage := activityParam(r, "page", 1, 1_000_000)
	size, okSize := activityParam(r, "page_size", 15, 100)
	if q.Get("page_size") == "" {
		size, okSize = activityParam(r, "size", 15, 100)
	}
	status, text, order, sort := q.Get("status"), strings.TrimSpace(q.Get("q")), q.Get("order"), q.Get("sort")
	if kind == "" {
		kind = q.Get("kind")
	}
	sortColumn := map[string]string{"": "id", "id": "id", "name": "name", "status": "status", "department": "department", "employee_no": "employee_no", "level": "level"}[sort]
	if !okPage || !okSize || len(text) > 240 || sortColumn == "" || !slices.Contains([]string{"", "asc", "desc"}, order) ||
		!slices.Contains([]string{"", "active", "inactive", "deleted", "missing"}, status) ||
		!slices.Contains([]string{"", "template", "finance", "pay", "report"}, kind) {
		bad(w, "授权筛选、排序或分页参数无效")
		return
	}
	filter := " FROM grants WHERE 1=1"
	var args []any
	if exact := q.Get("id"); exact != "" {
		id := number(exact)
		if id <= 0 || decimal(id) != exact {
			bad(w, "账号编号无效")
			return
		}
		filter += " AND id=?"
		args = append(args, id)
	}
	if text != "" {
		filter += " AND LOCATE(?,CONCAT_WS(' ',name,id,employee_no,department))>0"
		args = append(args, text)
	}
	if status != "" {
		filter += " AND status=?"
		args = append(args, status)
	}
	switch kind {
	case "template":
		filter += " AND level IN ('draft','publish')"
	case "finance":
		filter += " AND (pay OR report)"
	case "pay":
		filter += " AND pay"
	case "report":
		filter += " AND report"
	}
	ctx := r.Context()
	tx, err := app.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	var total int64
	var hasActivePayer bool
	if tx.QueryRowContext(ctx, financeGrantProjection+" SELECT COUNT(*)"+filter, args...).Scan(&total) != nil ||
		tx.QueryRowContext(ctx, financeGrantProjection+" SELECT EXISTS (SELECT 1 FROM grants WHERE status='active' AND pay)").Scan(&hasActivePayer) != nil {
		internal(w)
		return
	}
	direction := " ASC"
	if order == "desc" {
		direction = " DESC"
	}
	rows, err := tx.QueryContext(ctx, financeGrantProjection+" SELECT id,name,department,employee_no,status,level,pay,report"+filter+
		" ORDER BY "+sortColumn+direction+",id ASC LIMIT ? OFFSET ?", append(args, size, (page-1)*size)...)
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	items := []financeGrant{}
	for rows.Next() {
		var item financeGrant
		if rows.Scan(&item.ID, &item.Name, &item.Department, &item.EmployeeNo, &item.Status, &item.Level, &item.Pay, &item.Report) != nil {
			internal(w)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	platform.JSON(w, 200, map[string]any{"items": items, "total": total, "page": page, "size": size, "page_size": size, "has_active_payer": hasActivePayer, "level": "publish"})
}
