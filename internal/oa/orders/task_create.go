package orders

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"workflow/internal/oa/platform"
)

var requestKey = regexp.MustCompile("^[A-Za-z0-9_-]{8,80}$")
var coreClient = &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !allowed(actor) || !(actor.SuperAdmin || actor.Actions["task.create"]) {
		platform.Error(w, 403, "当前账号无任务创建权限")
		return
	}
	if s.db == nil || strings.TrimRight(os.Getenv("OA_CORE_URL"), "/") != "http://127.0.0.1:18087" {
		platform.Error(w, 503, "沙箱任务服务暂时不可用")
		return
	}
	var request struct {
		RequestID string                     `json:"request_id"`
		Shop      string                     `json:"shop_id"`
		SKU       string                     `json:"sku"`
		From      string                     `json:"from"`
		To        string                     `json:"to"`
		Task      map[string]json.RawMessage `json:"task"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF || !requestKey.MatchString(request.RequestID) || request.Shop == "" || len(request.Shop) > 128 || request.SKU == "" || len(request.SKU) > 128 || request.Task == nil {
		platform.Error(w, 400, "请填写店铺、SKU 和任务内容")
		return
	}
	values := r.URL.Query()
	values.Set("shop_id", request.Shop)
	values.Set("sku", request.SKU)
	values.Set("from", request.From)
	values.Set("to", request.To)
	r.URL.RawQuery = values.Encode()
	params, err := filters(r, "sku", time.Now().In(beijing))
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
		platform.Error(w, 403, "当前账号无该店铺权限")
		return
	}
	metricRows, err := s.ch.rows(r.Context(), productsSQL, params)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(metricRows) == 0 {
		platform.Error(w, 403, "该期间没有可访问的商品成交")
		return
	}
	idempotency, _ := json.Marshal("oa-orders-" + request.RequestID)
	request.Task["client_create_id"] = idempotency
	payload, err := json.Marshal(request.Task)
	if err != nil {
		platform.Error(w, 400, "任务内容无效")
		return
	}
	snapshot, err := json.Marshal(metricRows)
	if err != nil {
		platform.Error(w, 503, "订单指标暂时不可用")
		return
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(append([]byte(request.Shop+"\n"+request.SKU+"\n"+params.Get("param_from")+"\n"+params.Get("param_until")+"\n"), payload...)))
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO oa_order_task_sources(actor_id,request_id,payload_hash,shop_id,sku,period_start,period_end,metrics) VALUES(?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE request_id=request_id`, actor.ID, request.RequestID, digest, request.Shop, request.SKU, params.Get("param_from"), inclusivePeriodEnd(params.Get("param_until")), snapshot)
	if err != nil {
		platform.Error(w, 503, "任务来源暂时无法保存")
		return
	}
	var existingHash string
	var existingTask sql.NullInt64
	err = s.db.QueryRowContext(r.Context(), `SELECT payload_hash,task_id FROM oa_order_task_sources WHERE actor_id=? AND request_id=?`, actor.ID, request.RequestID).Scan(&existingHash, &existingTask)
	if err != nil {
		platform.Error(w, 503, "任务来源暂时无法读取")
		return
	}
	if existingHash != digest {
		platform.Error(w, 409, "该请求编号已用于其他任务")
		return
	}
	if existingTask.Valid {
		platform.JSON(w, 200, map[string]int64{"task_id": existingTask.Int64})
		return
	}
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "http://127.0.0.1:18087/v1/tasks", bytes.NewReader(payload))
	if err != nil {
		platform.Error(w, 503, "沙箱任务服务暂时不可用")
		return
	}
	upstream.Header.Set("Authorization", r.Header.Get("Authorization"))
	upstream.Header.Set("Content-Type", "application/json")
	response, err := coreClient.Do(upstream)
	if err != nil {
		platform.Error(w, 503, "任务状态待确认，请使用原请求编号重试")
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		platform.Error(w, 503, "任务状态待确认，请使用原请求编号重试")
		return
	}
	if response.StatusCode != 201 && response.StatusCode != 200 {
		if response.StatusCode == 400 || response.StatusCode == 403 || response.StatusCode == 409 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(response.StatusCode)
			_, _ = w.Write(body)
			return
		}
		platform.Error(w, 503, "任务状态待确认，请使用原请求编号重试")
		return
	}
	var reply struct {
		Data struct {
			ID     int64 `json:"id"`
			TaskID int64 `json:"task_id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &reply) != nil {
		platform.Error(w, 503, "任务状态待确认，请使用原请求编号重试")
		return
	}
	id := reply.Data.ID
	if id <= 0 {
		id = reply.Data.TaskID
	}
	if id <= 0 {
		platform.Error(w, 503, "任务状态待确认，请使用原请求编号重试")
		return
	}
	_, err = s.db.ExecContext(r.Context(), `UPDATE oa_order_task_sources SET task_id=? WHERE actor_id=? AND request_id=? AND (task_id IS NULL OR task_id=?)`, id, actor.ID, request.RequestID, id)
	if err != nil {
		platform.Error(w, 503, "任务已提交，请使用原请求编号重试来源登记")
		return
	}
	platform.JSON(w, 201, map[string]int64{"task_id": id})
}

const taskSourceSchema = `CREATE TABLE IF NOT EXISTS oa_order_task_sources(
 actor_id BIGINT NOT NULL,request_id VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 payload_hash CHAR(64) NOT NULL,shop_id VARCHAR(128) NOT NULL,sku VARCHAR(128) NOT NULL,
 period_start DATE NOT NULL,period_end DATE NOT NULL,metrics JSON NOT NULL,task_id BIGINT NULL,
 PRIMARY KEY(actor_id,request_id),KEY idx_oa_order_task_source(task_id,shop_id,sku)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

func inclusivePeriodEnd(until string) string {
	date, _ := time.ParseInLocation(time.DateOnly, until, beijing)
	return date.AddDate(0, 0, -1).Format(time.DateOnly)
}
