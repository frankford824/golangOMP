package orders

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"workflow/internal/oa/office"
	"workflow/internal/oa/platform"
)

var beijing = time.FixedZone("Asia/Shanghai", 8*3600)
var firstDay = time.Date(2026, 6, 1, 0, 0, 0, 0, beijing)

type result struct {
	Cache     string `json:"-"`
	Rows      []row  `json:"rows"`
	Freshness row    `json:"freshness"`
	Stale     bool   `json:"stale"`
	HasMore   bool   `json:"has_more"`
}

type cached struct {
	result  result
	fetched time.Time
}
type flight struct {
	done   chan struct{}
	result result
	err    error
}
type server struct {
	ch            *client
	db            *sql.DB
	native        http.Handler
	mu            sync.Mutex
	cache         map[string]cached
	active        map[string]*flight
	freshnessMu   sync.Mutex
	freshnessAt   time.Time
	freshnessRows []row
}

var sharedClient = sync.OnceValue(newClient)

func Mount(mux *http.ServeMux, db *sql.DB, native http.Handler) {
	s := &server{ch: sharedClient(), db: db, native: native, cache: make(map[string]cached), active: make(map[string]*flight)}
	s.ch.keepAlive()
	for _, name := range []string{"hourly", "summary", "products", "rankings", "personal", "sku", "shops", "cost-history", "refunds", "ledger", "detail", "compare"} {
		mux.HandleFunc("GET /oa-api/orders/"+name, func(w http.ResponseWriter, r *http.Request) { s.read(w, r, name) })
	}
	mux.HandleFunc("POST /oa-api/orders/feedback", s.saveFeedback)
	mux.HandleFunc("POST /oa-api/orders/task-links", s.saveTaskLink)
}

func allowed(actor platform.Actor) bool {
	if actor.ID <= 0 {
		return false
	}
	if actor.SuperAdmin || actor.Actions["orders.view"] {
		return true
	}
	for _, role := range actor.Roles {
		if role == "ops" || role == "operations" {
			return true
		}
	}
	return false
}

func (s *server) scope(ctx context.Context, actor platform.Actor, params url.Values, own bool) (bool, error) {
	params.Set("param_all", "0")
	params.Set("param_shops", "[]")
	params.Set("param_person", "")
	if actor.SuperAdmin && !own {
		params.Set("param_all", "1")
		return true, nil
	}
	set, person, err := s.baselineScope(ctx, actor.EmployeeNo)
	if err != nil {
		return false, err
	}
	overrides := map[string]bool{}
	if s.db != nil {
		overrides, err = office.ShopOverrides(ctx, s.db, actor.ID)
	}
	if err != nil {
		return false, err
	}
	for shop, active := range overrides {
		if active {
			set[shop] = true
		} else {
			delete(set, shop)
		}
	}
	if len(set) == 0 || own && person == "" {
		return false, nil
	}
	shops := make([]string, 0, len(set))
	for shop := range set {
		shops = append(shops, shop)
	}
	sort.Strings(shops)
	quoted := make([]string, 0, len(shops))
	for _, shop := range shops {
		quoted = append(quoted, "'"+strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(shop)+"'")
	}
	params.Set("param_shops", "["+strings.Join(quoted, ",")+"]")
	params.Set("param_person", person)
	return true, nil
}

func (s *server) baselineScope(ctx context.Context, employee *int) (map[string]bool, string, error) {
	return baselineScope(ctx, s.ch, employee)
}

func BaselineShops(ctx context.Context, employee *int) (map[string]bool, error) {
	shops, _, err := baselineScope(ctx, sharedClient(), employee)
	return shops, err
}

func baselineScope(ctx context.Context, ch *client, employee *int) (map[string]bool, string, error) {
	shops := map[string]bool{}
	if employee == nil || *employee < 0 {
		return shops, "", nil
	}
	rows, err := ch.rows(ctx, personScope, url.Values{"param_employee": {strconv.Itoa(*employee)}, "param_at": {time.Now().In(beijing).Format(time.DateTime)}})
	if err != nil {
		return nil, "", err
	}
	var person string
	for _, item := range rows {
		var shop, id string
		if json.Unmarshal(item["shop_id"], &shop) != nil || json.Unmarshal(item["person_id"], &id) != nil || id == "" || person != "" && id != person {
			return nil, "", errUnavailable
		}
		person = id
		if shop != "" {
			shops[shop] = true
		}
	}
	return shops, person, nil
}

func filters(r *http.Request, name string, now time.Time) (url.Values, error) {
	query := r.URL.Query()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, beijing)
	from, to := today, today
	if name == "personal" {
		from = today.AddDate(0, 0, 1-today.Day())
	}
	for field, value := range map[string]*time.Time{"from": &from, "to": &to} {
		if raw := query.Get(field); raw != "" {
			parsed, err := time.ParseInLocation(time.DateOnly, raw, beijing)
			if err != nil {
				return nil, errors.New("日期格式应为 YYYY-MM-DD")
			}
			*value = parsed
		}
	}
	if from.Before(firstDay) || to.Before(from) || to.After(today) || to.Sub(from) > 366*24*time.Hour {
		return nil, errors.New("日期需为 2026-06-01 至今，区间最长 367 天")
	}
	if name == "hourly" && to.After(from.AddDate(0, 0, 1)) {
		return nil, errors.New("分时趋势限连续两日")
	}
	params := url.Values{"param_from": {from.Format(time.DateOnly)}, "param_until": {to.AddDate(0, 0, 1).Format(time.DateOnly)}}
	for _, field := range []string{"shop_id", "platform", "sku", "product"} {
		value := query.Get(field)
		if len(value) > 128 {
			return nil, errors.New("筛选值过长")
		}
		key := field
		if key == "shop_id" {
			key = "shop"
		}
		params.Set("param_"+key, value)
	}
	if (name == "sku" || name == "cost-history") && params.Get("param_sku") == "" {
		return nil, errors.New("请选择 SKU")
	}
	offset := 0
	if raw := query.Get("offset"); raw != "" {
		var err error
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 || offset > 10000 {
			return nil, errors.New("分页位置无效")
		}
	}
	params.Set("param_offset", strconv.Itoa(offset))
	params.Set("param_brush", "0")
	if query.Get("include_brush") == "1" {
		params.Set("param_brush", "1")
	}
	return params, nil
}

func (s *server) read(w http.ResponseWriter, r *http.Request, name string) {
	actor := platform.ActorFrom(r)
	if !allowed(actor) {
		platform.Error(w, 403, "当前账号无订单经营权限")
		return
	}
	params, err := filters(r, name, time.Now().In(beijing))
	if err != nil {
		platform.Error(w, 400, err.Error())
		return
	}
	group := r.URL.Query().Get("group")
	queries := map[string]string{"hourly": hourlySQL, "summary": summarySQL, "products": productsSQL, "personal": personalSQL, "sku": skuSQL, "shops": shopNamesSQL, "cost-history": historySQL, "refunds": refundsSQL, "ledger": ledgerSQL, "detail": orderDetailSQL, "compare": compareSQL}
	query := queries[name]
	if number := r.URL.Query().Get("online_order_no"); name == "detail" && number != "" {
		if len(number) > 128 {
			platform.Error(w, 400, "平台订单号过长")
			return
		}
		params.Set("param_order_no", number)
		query = exactOrderDetailSQL
	}
	if name == "rankings" {
		query = map[string]string{"": shopRankingSQL, "shop": shopRankingSQL, "platform": platformRankingSQL, "product": productRankingSQL, "person": personRankingSQL, "growth": growthSQL}[group]
		if query == "" {
			platform.Error(w, 400, "排行类型无效")
			return
		}
		if (group == "person" || name == "ledger") && !actor.SuperAdmin {
			platform.Error(w, 403, "当前账号无人员排行权限")
			return
		}
	}
	if name == "ledger" && !actor.SuperAdmin {
		platform.Error(w, 403, "当前账号无台账权限")
		return
	}
	if group == "growth" {
		from, _ := time.ParseInLocation(time.DateOnly, params.Get("param_from"), beijing)
		until, _ := time.ParseInLocation(time.DateOnly, params.Get("param_until"), beijing)
		baseline := from.Add(-until.Sub(from))
		if baseline.Before(firstDay) {
			platform.Error(w, 400, "同长度基期超出订单数据范围")
			return
		}
		params.Set("param_baseline", baseline.Format(time.DateOnly))
	}
	if name == "compare" {
		day, e := time.ParseInLocation(time.DateOnly, r.URL.Query().Get("adjusted_on"), beijing)
		days, n := strconv.Atoi(r.URL.Query().Get("days"))
		now := time.Now().In(beijing)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, beijing)
		if e != nil || n != nil || days < 1 || days > 90 || day.AddDate(0, 0, -days).Before(firstDay) || day.AddDate(0, 0, days).After(today) || params.Get("param_shop") == "" || params.Get("param_sku") == "" {
			platform.Error(w, 400, "请选择店铺、SKU 和完整的同长度比较期")
			return
		}
		params.Set("param_adjusted", day.Format(time.DateOnly))
		params.Set("param_from", day.AddDate(0, 0, -days).Format(time.DateOnly))
		params.Set("param_until", day.AddDate(0, 0, days).Format(time.DateOnly))
	}
	if params.Get("param_brush") == "1" {
		switch query {
		case summarySQL:
			query = brushSummarySQL
		case shopRankingSQL:
			query = brushShopRankSQL
		case platformRankingSQL:
			query = brushPlatformRankSQL
		case growthSQL:
			platform.Error(w, 400, "增长排行使用剔除刷单的固定口径")
			return
		}
	}
	own := name == "personal" || (name == "products" && r.URL.Query().Get("mine") == "1")
	if own && name == "products" {
		query = myProductsSQL
	}
	permitted, err := s.scope(r.Context(), actor, params, own)
	if err != nil {
		writeError(w, err)
		return
	}
	if !permitted {
		platform.JSON(w, 200, result{Rows: []row{}})
		return
	}
	key := fmt.Sprintf("%d/%s/%s/%s", actor.ID, name, group, params.Encode())
	data, err := s.load(r.Context(), key, query, params)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("X-OA-Order-Cache", data.Cache)
	if name == "sku" {
		s.skuDetails(w, r, actor, params.Get("param_sku"), data)
		return
	}
	if name == "personal" {
		platform.JSON(w, 200, personalView(data, params, time.Now().In(beijing)))
		return
	}
	platform.JSON(w, 200, data)
}

func writeError(w http.ResponseWriter, err error) {
	code := 503
	if errors.Is(err, errTimeout) {
		code = 504
	}
	platform.Error(w, code, err.Error())
}

func (s *server) load(ctx context.Context, key, query string, params url.Values) (result, error) {
	s.mu.Lock()
	previous := s.cache[key]
	if time.Since(previous.fetched) < 30*time.Second {
		s.mu.Unlock()
		data := previous.result
		data.Cache = "hit"
		return data, nil
	}
	pending := s.active[key]
	if pending != nil {
		s.mu.Unlock()
		select {
		case <-pending.done:
			data := pending.result
			data.Cache = "coalesced"
			return data, pending.err
		case <-ctx.Done():
			return result{}, errTimeout
		}
	}
	pending = &flight{done: make(chan struct{})}
	s.active[key] = pending
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.active, key); close(pending.done); s.mu.Unlock() }()
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 12*time.Second)
	defer cancel()
	freshness, err := s.freshness(readCtx)
	var rows []row
	if err == nil && len(freshness) == 1 {
		rows, err = s.ch.rows(readCtx, query, params)
		if err == nil && query == summarySQL {
			err = s.markVerifiedZero(readCtx, params, rows)
		}
	} else if err == nil {
		err = errUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if errors.Is(err, errDenied) {
			clear(s.cache)
		} else if time.Since(previous.fetched) < 5*time.Minute && (errors.Is(err, errUnavailable) || errors.Is(err, errTimeout)) {
			pending.result = previous.result
			pending.result.Stale = true
			pending.result.Cache = "stale"
			return pending.result, nil
		}
		pending.err = err
		return result{}, err
	}
	pending.result = result{Cache: "miss", Rows: rows, Freshness: freshness[0], HasMore: len(rows) == 101 && (query == productsSQL || query == skuSQL || query == myProductsSQL || query == personRankingSQL || query == productRankingSQL || query == shopRankingSQL || query == platformRankingSQL || query == brushShopRankSQL || query == brushPlatformRankSQL || query == growthSQL || query == ledgerSQL || query == orderDetailSQL || query == exactOrderDetailSQL)}
	if pending.result.HasMore {
		pending.result.Rows = rows[:100]
	}
	if len(s.cache) >= 512 {
		clear(s.cache)
	}
	s.cache[key] = cached{result: pending.result, fetched: time.Now()}
	return pending.result, nil
}

func (s *server) markVerifiedZero(ctx context.Context, params url.Values, rows []row) error {
	quoted := strings.NewReplacer("\\", "\\\\", "'", "\\'")
	pairs := []string{}
	candidates := map[string]row{}
	for _, item := range rows {
		if string(item["verified_gross"]) != "null" {
			continue
		}
		var shop, day string
		if json.Unmarshal(item["shop_id"], &shop) != nil || json.Unmarshal(item["day"], &day) != nil {
			return errUnavailable
		}
		candidates[shop+"\x00"+day] = item
		pairs = append(pairs, "('"+quoted.Replace(shop)+"','"+quoted.Replace(day)+"')")
	}
	if len(pairs) > 32 {
		for _, item := range candidates {
			item["verified_status"] = json.RawMessage(`"zero_or_unverified"`)
		}
		return nil
	}
	if len(pairs) == 0 {
		return nil
	}
	values := make(url.Values, len(params)+1)
	for k, v := range params {
		values[k] = v
	}
	values.Set("param_zero_pairs", "["+strings.Join(pairs, ",")+"]")
	found, err := s.ch.rows(ctx, verifiedZeroSQL, values)
	if err != nil {
		return err
	}
	for _, item := range found {
		var shop, day string
		if json.Unmarshal(item["shop_id"], &shop) != nil || json.Unmarshal(item["day"], &day) != nil {
			return errUnavailable
		}
		match := candidates[shop+"\x00"+day]
		if match != nil {
			match["verified_gross"] = json.RawMessage(`"0.00"`)
			match["cost_verified_total"] = json.RawMessage(`"0.00"`)
		}
	}
	return nil
}

func (s *server) freshness(ctx context.Context) ([]row, error) {
	s.freshnessMu.Lock()
	defer s.freshnessMu.Unlock()
	if time.Since(s.freshnessAt) < 12*time.Second {
		return s.freshnessRows, nil
	}
	rows, err := s.ch.rows(ctx, freshnessSQL, nil)
	if err == nil && len(rows) == 1 {
		s.freshnessAt = time.Now()
		s.freshnessRows = rows
	}
	return rows, err
}
