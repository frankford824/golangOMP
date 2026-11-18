package orders

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"workflow/internal/oa/platform"
)

const requestTopic = "sku_cost_request"
const costTopicCondition = `JSON_UNQUOTE(JSON_EXTRACT(payload,'$.kind'))='sku_cost_request'`
const pendingCondition = `JSON_UNQUOTE(JSON_EXTRACT(payload,'$.state'))='pending'`
const completedCondition = `JSON_UNQUOTE(JSON_EXTRACT(payload,'$.state'))='handled'`

// Cost maintenance is shared by SKU across shops. Retain the two native write
// permissions and the native catalog read permission for the resolver queue.
func CanHandleCostRequest(actor platform.Actor) bool {
	return actor.ID > 0 && actor.Actions["catalog.view"] && actor.Actions["catalog.manage"] && actor.Actions["erp.manage"]
}

type costRequest struct {
	Kind       string `json:"kind"`
	State      string `json:"state"`
	Shop       string `json:"shop_id"`
	SKU        string `json:"sku"`
	Reason     string `json:"reason"`
	Sender     string `json:"sender"`
	RequestKey string `json:"request_key,omitempty"`
	Reply      string `json:"reply,omitempty"`
	HandledBy  string `json:"handled_by,omitempty"`
	HandledAt  string `json:"handled_at,omitempty"`
}

type costNotice struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Unread    bool      `json:"unread"`
	costRequest
}

func readCostInput(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		platform.Error(w, 400, "请核对提交内容")
		return false
	}
	return true
}
func costUnavailable(w http.ResponseWriter, err error) {
	log.Printf("[OA-COST-REQUEST] operation failed: %v", err)
	platform.Error(w, 503, "核价消息暂不可用，请稍后重试")
}

func (s *server) requestCostCheck(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !allowed(actor) {
		platform.Error(w, 403, "当前账号无经营查询权限")
		return
	}
	var in struct {
		Shop         string `json:"shop_id"`
		SKU          string `json:"sku"`
		From         string `json:"from"`
		To           string `json:"to"`
		IncludeBrush bool   `json:"include_brush"`
		Reason       string `json:"reason"`
		RequestKey   string `json:"request_key"`
	}
	if !readCostInput(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Shop == "" || in.SKU == "" || in.From == "" || in.To == "" || in.Reason == "" || utf8.RuneCountInString(in.Reason) > 300 || len(in.RequestKey) < 16 || len(in.RequestKey) > 80 {
		platform.Error(w, 400, "请选择店铺和商品，并填写不超过 300 字的核价原因")
		return
	}
	q := url.Values{"from": {in.From}, "to": {in.To}, "sku": {in.SKU}, "shop_id": {in.Shop}}
	if in.IncludeBrush {
		q.Set("include_brush", "1")
	}
	check := r.Clone(r.Context())
	check.URL.RawQuery = q.Encode()
	params, err := filters(check, "sku", time.Now().In(beijing))
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
		platform.Error(w, 403, "当前账号没有该店铺的经营权限")
		return
	}
	hits, err := s.ch.rows(r.Context(), `SELECT 1 AS found`+realLines+rankedOrderIDs+` AND sku_id={sku:String} LIMIT 1`, params)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(hits) == 0 {
		platform.Error(w, 404, "当前店铺和日期范围未查到此商品，请刷新后重试")
		return
	}
	n := costRequest{Kind: requestTopic, State: "pending", Shop: in.Shop, SKU: in.SKU, Reason: in.Reason, Sender: actor.Name, RequestKey: in.RequestKey}
	id, existing, err := s.storeCostRequest(r.Context(), actor.ID, n)
	if err != nil {
		costUnavailable(w, err)
		return
	}
	platform.JSON(w, 200, map[string]any{"id": id, "existing": existing})
}

func (s *server) storeCostRequest(ctx context.Context, actor int64, n costRequest) (int64, bool, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return 0, false, err
	}
	defer conn.Close()
	// Serialize retries for this person, shop and SKU on the same MySQL connection.
	digest := sha256.Sum256([]byte(strconv.FormatInt(actor, 10) + "\x00" + n.Shop + "\x00" + n.SKU))
	lock := "oa:cost:" + hex.EncodeToString(digest[:24])
	var held int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,2)", lock).Scan(&held); err != nil || held != 1 {
		if err == nil {
			err = errors.New("cost request busy")
		}
		return 0, false, err
	}
	defer func() {
		release, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(release, "DO RELEASE_LOCK(?)", lock)
	}()
	var id int64
	err = conn.QueryRowContext(ctx, `SELECT id FROM oa_office_notifications WHERE user_id=? AND `+costTopicCondition+` AND BINARY JSON_UNQUOTE(JSON_EXTRACT(payload,'$.sku'))=BINARY ? AND BINARY JSON_UNQUOTE(JSON_EXTRACT(payload,'$.shop_id'))=BINARY ? AND (`+pendingCondition+` OR JSON_UNQUOTE(JSON_EXTRACT(payload,'$.request_key'))=?) ORDER BY id DESC LIMIT 1`, actor, n.SKU, n.Shop, n.RequestKey).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	body, err := json.Marshal(n)
	if err != nil {
		return 0, false, err
	}
	row, err := conn.ExecContext(ctx, "INSERT INTO oa_office_notifications(user_id,payload) VALUES(?,?)", actor, body)
	if err != nil {
		return 0, false, err
	}
	id, err = row.LastInsertId()
	return id, false, err
}

func (s *server) costMessages(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if actor.ID <= 0 {
		platform.Error(w, 403, "请登录")
		return
	}
	reviewer := CanHandleCostRequest(actor)
	queue := r.URL.Query().Get("queue")
	if queue == "" {
		queue = "mine"
	}
	var where string
	var args []any
	switch queue {
	case "mine":
		where = "user_id=?"
		args = []any{actor.ID}
	case "replies":
		where = "user_id=? AND read_at IS NULL AND " + completedCondition
		args = []any{actor.ID}
	case "pending":
		if !reviewer {
			platform.Error(w, 403, "当前账号无成本维护权限")
			return
		}
		where = pendingCondition
	default:
		platform.Error(w, 400, "核价列表类型无效")
		return
	}
	var pending, unread int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(? AND `+pendingCondition+`),0),COALESCE(SUM(user_id=? AND read_at IS NULL AND `+completedCondition+`),0) FROM oa_office_notifications WHERE `+costTopicCondition+` AND (? OR user_id=?)`, reviewer, actor.ID, reviewer, actor.ID).Scan(&pending, &unread); err != nil {
		costUnavailable(w, err)
		return
	}
	resp := map[string]any{"pending_count": pending, "reply_count": unread, "can_handle": reviewer}
	if r.URL.Query().Get("summary") == "1" {
		platform.JSON(w, 200, resp)
		return
	}
	if raw := r.URL.Query().Get("before"); raw != "" {
		before, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			platform.Error(w, 400, "分页位置无效")
			return
		}
		where += " AND id<?"
		args = append(args, before)
	}
	rows, err := s.db.QueryContext(r.Context(), "SELECT id,created_at,read_at IS NULL,payload FROM oa_office_notifications WHERE "+costTopicCondition+" AND "+where+" ORDER BY id DESC LIMIT 21", args...)
	if err != nil {
		costUnavailable(w, err)
		return
	}
	defer rows.Close()
	items := []costNotice{}
	for rows.Next() {
		var item costNotice
		var raw []byte
		if err = rows.Scan(&item.ID, &item.CreatedAt, &item.Unread, &raw); err == nil {
			err = json.Unmarshal(raw, &item.costRequest)
		}
		if err != nil {
			costUnavailable(w, err)
			return
		}
		item.RequestKey = ""
		item.Unread = queue != "pending" && item.State == "handled" && item.Unread
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		costUnavailable(w, err)
		return
	}
	resp["has_more"] = len(items) > 20
	if len(items) > 20 {
		items = items[:20]
	}
	resp["items"] = items
	platform.JSON(w, 200, resp)
}

func (s *server) resolveCostMessage(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !CanHandleCostRequest(actor) {
		platform.Error(w, 403, "当前账号无成本维护权限")
		return
	}
	var in struct {
		Reply string `json:"reply"`
	}
	if !readCostInput(w, r, &in) {
		return
	}
	in.Reply = strings.TrimSpace(in.Reply)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 || in.Reply == "" || utf8.RuneCountInString(in.Reply) > 300 {
		platform.Error(w, 400, "请填写不超过 300 字的核对结果")
		return
	}
	row, err := s.db.ExecContext(r.Context(), `UPDATE oa_office_notifications SET payload=JSON_SET(payload,'$.state','handled','$.reply',?,'$.handled_by',?,'$.handled_at',?),read_at=NULL WHERE id=? AND `+costTopicCondition+" AND "+pendingCondition, in.Reply, actor.Name, time.Now().In(beijing).Format(time.RFC3339), id)
	if err != nil {
		costUnavailable(w, err)
		return
	}
	count, err := row.RowsAffected()
	if err != nil {
		costUnavailable(w, err)
		return
	}
	if count != 1 {
		platform.Error(w, 409, "这条请求已处理或不存在，请刷新列表")
		return
	}
	platform.JSON(w, 200, map[string]any{"id": id, "state": "handled"})
}

func (s *server) readCostMessage(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		platform.Error(w, 400, "消息编号无效")
		return
	}
	_, err = s.db.ExecContext(r.Context(), "UPDATE oa_office_notifications SET read_at=COALESCE(read_at,CURRENT_TIMESTAMP) WHERE id=? AND user_id=? AND "+costTopicCondition+" AND "+completedCondition, id, actor.ID)
	if err != nil {
		costUnavailable(w, err)
		return
	}
	platform.JSON(w, 200, map[string]bool{"read": true})
}
