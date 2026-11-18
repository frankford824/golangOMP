package office

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"time"

	"workflow/internal/oa/platform"
)

func (app office) handoverItems(w http.ResponseWriter, r *http.Request) {
	p, err := app.readPerson(r, number(r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if !scopePerson(platform.ActorFrom(r), p) {
		deny(w)
		return
	}
	type task struct {
		ID     int64  `json:"id"`
		Number string `json:"task_no"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	rows, err := app.source.QueryContext(r.Context(), "SELECT id,task_no,product_name_snapshot,task_status FROM tasks WHERE (current_handler_id=? OR designer_id=?) AND task_status NOT IN ('Completed','Cancelled','Archived') ORDER BY id DESC LIMIT 200", p.ID, p.ID)
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	items := []task{}
	for rows.Next() {
		var item task
		if err = rows.Scan(&item.ID, &item.Number, &item.Title, &item.Status); err != nil {
			internal(w)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		internal(w)
		return
	}
	shops := []string{}
	today := businessDate().Format(time.DateOnly)
	if app.baseline != nil {
		base, err := app.baseline(r.Context(), p.EmployeeNo)
		if err != nil {
			platform.Error(w, 503, "暂时无法核对店铺归属")
			return
		}
		overrides := shopOverrides(p.Terms, today)
		for shop := range base {
			if _, local := overrides[shop]; !local {
				shops = append(shops, shop)
			}
		}
	}
	for _, term := range p.Terms {
		if term.From <= today && (term.PendingHandover || term.To == "" || term.To > today) && !slices.Contains(shops, term.ShopID) {
			shops = append(shops, term.ShopID)
		}
	}
	slices.Sort(shops)
	platform.JSON(w, 200, map[string]any{"items": items, "shops": shops})
}
