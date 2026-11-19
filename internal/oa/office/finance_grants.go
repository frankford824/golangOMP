package office

import (
	"fmt"
	"net/http"
	"strings"

	"workflow/internal/oa/platform"
)

func financeAdmin(actor platform.Actor) bool {
	return permit(actor, "finance.admin")
}

type financeGrant struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Department string `json:"department"`
	EmployeeNo *int   `json:"employee_no"`
	Status     string `json:"status"`
	Level      string `json:"level"`
	Pay        bool   `json:"pay"`
	Report     bool   `json:"report"`
}

func (app office) financeGrants(w http.ResponseWriter, r *http.Request) {
	if !financeAdmin(platform.ActorFrom(r)) {
		deny(w)
		return
	}
	if len(r.URL.Query()) != 0 {
		app.financeGrantsPage(w, r, "")
		return
	}
	items, err := app.readFinanceGrants(r)
	if err != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"items": items})
}

func (app office) readFinanceGrants(r *http.Request) ([]financeGrant, error) {
	ctx := r.Context()
	rows, err := app.db.QueryContext(ctx, `
		WITH recipients AS (
			SELECT user_id FROM oa_office_delegates WHERE level IN ('draft','publish')
			UNION SELECT user_id FROM oa_office_finance_actions WHERE can_pay OR can_report
		)
		SELECT a.user_id,COALESCE(d.level,'none'),COALESCE(f.can_pay,FALSE),COALESCE(f.can_report,FALSE)
		FROM recipients a
		LEFT JOIN oa_office_delegates d ON d.user_id=a.user_id
		LEFT JOIN oa_office_finance_actions f ON f.user_id=a.user_id
		ORDER BY a.user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []financeGrant{}
	for rows.Next() {
		item := financeGrant{Status: "missing"}
		if err := rows.Scan(&item.ID, &item.Level, &item.Pay, &item.Report); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if app.source == nil {
		return nil, fmt.Errorf("finance grant identity source unavailable")
	}
	const batchSize = 256
	for start := 0; start < len(items); start += batchSize {
		if err := app.fillFinanceGrantPeople(r, items[start:min(start+batchSize, len(items))]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (app office) fillFinanceGrantPeople(r *http.Request, items []financeGrant) error {
	args := make([]any, len(items))
	byID := make(map[int64]*financeGrant, len(items))
	for i := range items {
		args[i] = items[i].ID
		byID[items[i].ID] = &items[i]
	}
	rows, err := app.source.QueryContext(r.Context(), "SELECT id,display_name,COALESCE(department,''),employee_no,status FROM users WHERE id IN ("+strings.Repeat("?,", len(items)-1)+"?)", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name, department, status string
		var employeeNo *int
		if err := rows.Scan(&id, &name, &department, &employeeNo, &status); err != nil {
			return err
		}
		if item := byID[id]; item != nil {
			item.Name, item.Department, item.EmployeeNo, item.Status = name, department, employeeNo, status
		}
	}
	return rows.Err()
}
