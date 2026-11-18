package orders

import (
	"database/sql"
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"workflow/internal/oa/platform"
)

func personalView(data result, params map[string][]string, now time.Time) map[string]any {
	output := map[string]any{"rows": data.Rows, "freshness": data.Freshness, "stale": data.Stale, "forecast": nil}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, beijing)
	nextMonth := monthStart.AddDate(0, 1, 0)
	total, complete := new(big.Rat), new(big.Rat)
	values := make(map[string]*big.Rat)
	for _, item := range data.Rows {
		var day, amount string
		if json.Unmarshal(item["day"], &day) != nil || json.Unmarshal(item["item_pay_total"], &amount) != nil {
			continue
		}
		number, ok := new(big.Rat).SetString(amount)
		if !ok {
			continue
		}
		values[day] = number
		if day >= monthStart.Format(time.DateOnly) && day <= now.Format(time.DateOnly) {
			total.Add(total, number)
		}
	}
	output["month_actual"] = total.FloatString(2)
	var committed string
	_ = json.Unmarshal(data.Freshness["source_committed_at"], &committed)
	if now.Day() < 4 || data.Stale || committed < now.Format(time.DateOnly) || params["param_from"][0] > monthStart.Format(time.DateOnly) || params["param_until"][0] < now.Format(time.DateOnly) {
		return output
	}
	for day := monthStart; day.Before(monthStart.AddDate(0, 0, now.Day()-1)); day = day.AddDate(0, 0, 1) {
		number, ok := values[day.Format(time.DateOnly)]
		if !ok {
			return output
		}
		complete.Add(complete, number)
	}
	forecast := complete.Mul(complete, big.NewRat(int64(nextMonth.Sub(monthStart)/(24*time.Hour)), int64(now.Day()-1)))
	output["forecast"] = forecast.FloatString(2)
	return output
}

type taskLink struct {
	ID     int64  `json:"id"`
	Number string `json:"task_no"`
	Status string `json:"status"`
}
type feedback struct {
	ID       int64  `json:"id"`
	Shop     string `json:"shop_id"`
	SKU      string `json:"sku"`
	Note     string `json:"note"`
	Adjusted string `json:"adjusted_on"`
	TaskID   *int64 `json:"task_id"`
}

func (s *server) skuDetails(w http.ResponseWriter, r *http.Request, actor platform.Actor, sku string, data result) {
	output := map[string]any{"rows": data.Rows, "freshness": data.Freshness, "stale": data.Stale, "has_more": data.HasMore, "current_cost": nil, "tasks": []taskLink{}, "feedback": []feedback{}}
	if len(data.Rows) == 0 || data.Stale {
		platform.JSON(w, 200, output)
		return
	}
	output["cost_context_available"] = false
	if s.source != nil {
		var cost sql.NullString
		err := s.source.QueryRowContext(r.Context(), `SELECT CAST(cost_price AS CHAR) FROM omp_sku_records WHERE sku_code=? AND BINARY sku_code=BINARY ?`, sku, sku).Scan(&cost)
		output["cost_context_available"] = err == nil || err == sql.ErrNoRows
		if cost.Valid && err == nil {
			output["current_cost"] = cost.String
		}
		if actor.SuperAdmin || actor.Actions["task.view"] {
			rows, err := s.source.QueryContext(r.Context(), `SELECT DISTINCT t.id,t.task_no,t.task_status FROM tasks t LEFT JOIN task_sku_items i ON i.task_id=t.id AND i.sku_code=? AND BINARY i.sku_code=BINARY ? WHERE ((t.sku_code=? AND BINARY t.sku_code=BINARY ?) OR i.task_id IS NOT NULL) AND (?=1 OR t.creator_id=? OR t.designer_id=? OR t.current_handler_id=?) ORDER BY t.id DESC LIMIT 100`, sku, sku, sku, sku, actor.SuperAdmin, actor.ID, actor.ID, actor.ID)
			if err == nil {
				links := []taskLink{}
				for rows.Next() {
					var link taskLink
					if rows.Scan(&link.ID, &link.Number, &link.Status) == nil {
						links = append(links, link)
					}
				}
				rows.Close()
				if rows.Err() == nil {
					output["tasks"] = links
				}
			}
		}
	}
	if s.db != nil {
		items := []feedback{}
		shops := map[string]bool{}
		for _, item := range data.Rows {
			var shop string
			_ = json.Unmarshal(item["shop_id"], &shop)
			shops[shop] = true
		}
		rows, err := s.db.QueryContext(r.Context(), `SELECT id,shop_id,sku,feedback_note,DATE_FORMAT(adjusted_on,'%Y-%m-%d'),task_id FROM oa_order_feedback WHERE sku=? AND BINARY sku=BINARY ? AND (?=1 OR actor_id=?) ORDER BY id DESC LIMIT 100`, sku, sku, actor.SuperAdmin, actor.ID)
		if err == nil {
			for rows.Next() {
				var item feedback
				if rows.Scan(&item.ID, &item.Shop, &item.SKU, &item.Note, &item.Adjusted, &item.TaskID) == nil && shops[item.Shop] {
					items = append(items, item)
				}
			}
			rows.Close()
			if rows.Err() == nil {
				output["feedback"] = items
			}
		}
	}
	platform.JSON(w, 200, output)
}

func (s *server) saveFeedback(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !allowed(actor) {
		platform.Error(w, 403, "当前账号无订单经营权限")
		return
	}
	var item feedback
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&item) != nil || item.Shop == "" || len(item.Shop) > 128 || item.SKU == "" || len(item.SKU) > 128 || item.Note == "" || len(item.Note) > 5000 {
		platform.Error(w, 400, "请填写店铺、SKU 和反馈内容")
		return
	}
	adjusted, err := time.ParseInLocation(time.DateOnly, item.Adjusted, beijing)
	if err != nil || adjusted.Before(firstDay) || adjusted.After(time.Now()) {
		platform.Error(w, 400, "请选择有效的实际调整日期")
		return
	}
	params := url.Values{"param_shop": {item.Shop}}
	permitted, err := s.scope(r.Context(), actor, params, false)
	if err != nil {
		writeError(w, err)
		return
	}
	if !permitted {
		platform.Error(w, 403, "当前账号无该店铺权限")
		return
	}
	shops, err := s.ch.rows(r.Context(), `SELECT shop_id FROM jst.v_shops WHERE shop_id={shop:String} AND ({all:UInt8}=1 OR shop_id IN {shops:Array(String)}) LIMIT 1`, params)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(shops) != 1 {
		platform.Error(w, 403, "当前账号无该店铺权限")
		return
	}
	if item.TaskID != nil {
		if s.source == nil || !(actor.SuperAdmin || actor.Actions["task.view"]) {
			platform.Error(w, 403, "当前账号无该任务权限")
			return
		}
		var taskID int64
		err = s.source.QueryRowContext(r.Context(), `SELECT t.id FROM tasks t WHERE t.id=? AND ((t.sku_code=? AND BINARY t.sku_code=BINARY ?) OR EXISTS(SELECT 1 FROM task_sku_items i WHERE i.task_id=t.id AND i.sku_code=? AND BINARY i.sku_code=BINARY ?)) AND (?=1 OR t.creator_id=? OR t.designer_id=? OR t.current_handler_id=?)`, *item.TaskID, item.SKU, item.SKU, item.SKU, item.SKU, actor.SuperAdmin, actor.ID, actor.ID, actor.ID).Scan(&taskID)
		if err != nil {
			platform.Error(w, 403, "任务不可访问或 SKU 不匹配")
			return
		}
	}
	if s.db == nil {
		platform.Error(w, 503, "反馈服务暂时不可用")
		return
	}
	saved, err := s.db.ExecContext(r.Context(), `INSERT INTO oa_order_feedback(actor_id,shop_id,sku,feedback_note,adjusted_on,task_id) VALUES(?,?,?,?,?,?)`, actor.ID, item.Shop, item.SKU, item.Note, item.Adjusted, item.TaskID)
	if err != nil {
		platform.Error(w, 503, "反馈暂时无法保存")
		return
	}
	item.ID, _ = saved.LastInsertId()
	platform.JSON(w, 201, item)
}
