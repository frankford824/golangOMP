package orders

import (
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
	cost, costTask, costOK := s.currentCost(r, sku)
	if costOK {
		output["cost_context_available"] = true
		output["current_cost"] = cost
	}
	tasks := []taskLink{}
	if s.db != nil {
		tasks = s.linkedTasks(r, sku, data.Rows)
	}
	seen := map[int64]bool{}
	for _, task := range tasks {
		seen[task.ID] = true
	}
	if costTask.ID > 0 && !seen[costTask.ID] {
		tasks = append(tasks, costTask)
		seen[costTask.ID] = true
	}
	output["tasks"] = append(tasks, s.historicalTasks(r, sku, seen)...)
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
		if _, ok := s.nativeTask(r, *item.TaskID); !ok {
			platform.Error(w, 403, "当前账号无该任务权限")
			return
		}
		if s.db == nil {
			platform.Error(w, 503, "反馈服务暂时不可用")
			return
		}
		var exists bool
		err = s.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM oa_order_task_links WHERE task_id=? AND shop_id=? AND sku=?)", *item.TaskID, item.Shop, item.SKU).Scan(&exists)
		if err != nil {
			platform.Error(w, 503, "任务关联暂时不可用")
			return
		}
		if !exists {
			platform.Error(w, 400, "请先关联该订单商品")
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
