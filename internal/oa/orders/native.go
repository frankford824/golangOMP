package orders

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	"workflow/internal/oa/platform"
)

func (s *server) nativeRead(r *http.Request, target string, out any) bool {
	if s.native == nil {
		return false
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		return false
	}
	request.Header.Set("Authorization", r.Header.Get("Authorization"))
	recorder := httptest.NewRecorder()
	s.native.ServeHTTP(recorder, request)
	return recorder.Code == 200 && json.Unmarshal(recorder.Body.Bytes(), out) == nil
}

func (s *server) nativeTask(r *http.Request, id int64) (taskLink, bool) {
	var payload struct {
		Data struct {
			Task struct {
				ID     int64  `json:"id"`
				Number string `json:"task_no"`
				Status string `json:"task_status"`
			} `json:"task"`
		} `json:"data"`
	}
	returnValue := s.nativeRead(r, fmt.Sprintf("/v1/tasks/%d", id), &payload)
	return taskLink{ID: payload.Data.Task.ID, Number: payload.Data.Task.Number, Status: payload.Data.Task.Status}, returnValue && payload.Data.Task.ID == id
}

func (s *server) currentCost(r *http.Request, sku string) (*string, bool) {
	query := url.Values{"keyword": {sku}, "review_state": {"all"}, "page_size": {"100"}}
	var cursor int64
	for range 20 {
		var response struct {
			Data []struct {
				TaskID int64       `json:"task_id"`
				SKU    string      `json:"sku_code"`
				Cost   json.Number `json:"cost_price"`
			} `json:"data"`
			Next int64 `json:"next_before_id"`
		}
		if !s.nativeRead(r, "/v1/cost-management/sku-costs?"+query.Encode(), &response) {
			return nil, false
		}
		for _, record := range response.Data {
			if record.SKU != sku {
				continue
			}
			if _, ok := s.nativeTask(r, record.TaskID); !ok {
				return nil, false
			}
			if record.Cost == "" {
				return nil, true
			}
			value := record.Cost.String()
			return &value, true
		}
		if response.Next == 0 {
			return nil, true
		}
		if response.Next < 0 || cursor != 0 && response.Next >= cursor {
			return nil, false
		}
		cursor = response.Next
		query.Set("before_id", strconv.FormatInt(cursor, 10))
	}
	return nil, false
}
func (s *server) linkedTasks(r *http.Request, sku string, data []row) []taskLink {
	result := []taskLink{}
	shops := map[string]bool{}
	for _, item := range data {
		var shop string
		if json.Unmarshal(item["shop_id"], &shop) == nil {
			shops[shop] = true
		}
	}
	rows, err := s.db.QueryContext(r.Context(), "SELECT task_id,shop_id FROM oa_order_task_links WHERE sku=? ORDER BY task_id DESC LIMIT 100", sku)
	if err != nil {
		return result
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for rows.Next() {
		var id int64
		var shop string
		if rows.Scan(&id, &shop) == nil && shops[shop] && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if item, ok := s.nativeTask(r, id); ok {
			result = append(result, item)
		}
	}
	return result
}

func (s *server) saveTaskLink(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !allowed(actor) {
		platform.Error(w, 403, "当前账号无经营权限")
		return
	}
	if s.db == nil {
		platform.Error(w, 503, "订单任务关联暂未启用")
		return
	}
	var body struct {
		Shop   string `json:"shop_id"`
		SKU    string `json:"sku"`
		From   string `json:"from"`
		To     string `json:"to"`
		TaskID int64  `json:"task_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF || strings.TrimSpace(body.Shop) == "" || len(body.Shop) > 128 || strings.TrimSpace(body.SKU) == "" || len(body.SKU) > 128 || body.From == "" || body.To == "" || body.TaskID <= 0 {
		platform.Error(w, 400, "请填写店铺、SKU、观察期间和任务")
		return
	}
	nested := r.Clone(r.Context())
	nested.URL.RawQuery = url.Values{"shop_id": {body.Shop}, "sku": {body.SKU}, "from": {body.From}, "to": {body.To}}.Encode()
	params, err := filters(nested, "sku", time.Now().In(beijing))
	if err != nil {
		platform.Error(w, 400, err.Error())
		return
	}
	ok, err := s.scope(r.Context(), actor, params, false)
	if err != nil {
		writeError(w, err)
		return
	}
	if !ok {
		platform.Error(w, 403, "当前账号无该经营范围权限")
		return
	}
	if _, ok = s.nativeTask(r, body.TaskID); !ok {
		platform.Error(w, 403, "当前账号无法读取该任务")
		return
	}
	metrics, err := s.ch.rows(r.Context(), skuSQL, params)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(metrics) == 0 {
		platform.Error(w, 403, "该期间没有可访问的商品成交")
		return
	}
	snapshot, err := json.Marshal(metrics)
	if err != nil {
		writeError(w, errUnavailable)
		return
	}
	res, err := s.db.ExecContext(r.Context(), `INSERT INTO oa_order_task_links(shop_id,sku,task_id,actor_id,period_from,period_to,metrics)
 VALUES(?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE task_id=task_id`, body.Shop, body.SKU, body.TaskID, actor.ID, body.From, body.To, snapshot)
	if err != nil {
		platform.Error(w, 503, "任务关联暂时不可用")
		return
	}
	count, _ := res.RowsAffected()
	status := 200
	if count == 1 {
		status = 201
	}
	platform.JSON(w, status, map[string]int64{"task_id": body.TaskID})
}
