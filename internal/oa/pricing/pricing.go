// Package pricing resolves read-only SKU cost evidence from the main site.
package pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"workflow/domain"
	"workflow/internal/oa/platform"
)

const batchLimit = 30
const basePath = "/oa-api/pricing/"

type app struct {
	native http.Handler
}

func Mount(mux *http.ServeMux, native http.Handler) {
	app := app{native: native}
	mux.HandleFunc("POST "+basePath+"preview", app.preview)
}

func CanView(actor platform.Actor) bool {
	return actor.SuperAdmin || actor.Actions["catalog.view"]
}

type row struct {
	SKU          string                         `json:"sku_code"`
	Product      string                         `json:"product_name,omitempty"`
	TaskID       int64                          `json:"task_id,omitempty"`
	SKUItemID    int64                          `json:"sku_item_id,omitempty"`
	Current      *float64                       `json:"current_cost"`
	Suggested    *float64                       `json:"suggested_cost"`
	Rule         *domain.CostRulePreviewMatch   `json:"rule,omitempty"`
	Input        *domain.CostRulePreviewRequest `json:"input,omitempty"`
	Calculation  *domain.CostCalculation        `json:"calculation,omitempty"`
	MatchMode    string                         `json:"match_mode,omitempty"`
	AppliedRules []domain.CostRulePreviewMatch  `json:"applied_rules,omitempty"`
	State        string                         `json:"state"`
	Reason       string                         `json:"reason"`
}

func decode(w http.ResponseWriter, r *http.Request, input any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 24<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(input) != nil || d.Decode(new(any)) != io.EOF {
		platform.Error(w, 400, "请求内容或格式无效")
		return false
	}
	return true
}

func normalize(values []string) ([]string, bool) {
	if len(values) < 1 || len(values) > batchLimit {
		return nil, false
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		sku := strings.ToUpper(strings.TrimSpace(value))
		if sku == "" || utf8.RuneCountInString(sku) > 128 || strings.ContainsFunc(sku, unicode.IsControl) {
			return nil, false
		}
		if !seen[sku] {
			seen[sku] = true
			out = append(out, sku)
		}
	}
	return out, true
}

func (a app) preview(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !CanView(actor) {
		platform.Error(w, 403, "当前账号没有商品成本查看权限")
		return
	}
	var request struct {
		SKUs []string `json:"sku_codes"`
	}
	if !decode(w, r, &request) {
		return
	}
	skus, ok := normalize(request.SKUs)
	if !ok {
		platform.Error(w, 400, "请输入 1 到 30 个有效的商品编码")
		return
	}
	platform.JSON(w, 200, map[string]any{"rows": a.resolve(r, skus)})
}

func (a app) resolve(r *http.Request, skus []string) []row {
	ctx, cancel := context.WithTimeout(r.Context(), 48*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	rows := make([]row, len(skus))
	jobs := make(chan int, len(skus))
	for i := range skus {
		jobs <- i
	}
	close(jobs)
	var workers sync.WaitGroup
	for range min(4, len(skus)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				rows[i] = a.resolveSKU(r, skus[i])
			}
		}()
	}
	workers.Wait()
	return rows
}

func (a app) resolveSKU(r *http.Request, sku string) row {
	result := row{SKU: sku, State: "unavailable"}
	finish := func(state, reason string) row { result.State, result.Reason = state, reason; return result }
	canonical, state := a.loadCanonical(r, sku)
	switch state {
	case "not_found":
		return finish(state, "主站没有找到这个编码的已完成商品成本记录")
	case "ambiguous":
		return finish(state, "主站有多条同编码成本记录，无法唯一确认")
	case "incomplete":
		return finish(state, "主站匹配结果超出核价查询范围，请单独核查编码")
	case "unavailable":
		return finish(state, "主站成本记录暂不可用")
	}
	result.Current, result.Product, result.TaskID, result.SKUItemID = canonical.CostPrice, canonical.ProductName, canonical.TaskID, canonical.ID
	var task struct {
		SKUItems []domain.TaskSKUItem `json:"sku_items"`
		Detail   *domain.TaskDetail   `json:"task_detail"`
	}
	status := a.readCore(r, http.MethodGet, "/v1/tasks/"+strconv.FormatInt(canonical.TaskID, 10), nil, &task)
	if status == http.StatusForbidden || status == http.StatusNotFound {
		return finish("unavailable", "当前账号无法读取对应任务的核价依据")
	}
	if status != http.StatusOK {
		return finish("unavailable", "主站任务核价依据暂不可用")
	}
	var item *domain.TaskSKUItem
	for i := range task.SKUItems {
		candidate := &task.SKUItems[i]
		if candidate.ID == canonical.ID && strings.EqualFold(candidate.SKUCode, sku) {
			item = candidate
			break
		}
	}
	if item == nil {
		return finish("unavailable", "任务中缺少与当前成本记录一致的 SKU 明细")
	}
	if canonical.Reviewed {
		return finish("protected", "主站人工审核已确认当前成本")
	}
	if canonical.Status == "conflict" {
		return finish("protected", "主站成本存在 ERP 冲突，需先处理")
	}

	if item.ManualCostOverride || item.CostPriceMode == string(domain.CostPriceModeManual) ||
		(task.Detail != nil && len(task.SKUItems) == 1 && (task.Detail.ManualCostOverride || task.Detail.CostPriceMode == string(domain.CostPriceModeManual))) {
		return finish("protected", "编码现价由人工维护")
	}
	if !equalPrice(item.CostPrice, canonical.CostPrice) {
		return finish("unavailable", "主站成本与任务快照不同步，请刷新后重试")
	}
	var detail *domain.TaskDetail
	if len(task.SKUItems) == 1 {
		detail = task.Detail
	}
	input, ok := prepareInput(canonical, item, detail)
	if !ok {
		return finish("needs_review", "缺少与编码一致的显式款式身份，无法按绑定规则计算")
	}
	result.Input = &input
	var quote domain.CostRulePreviewResponse
	if a.readCore(r, http.MethodPost, "/v1/cost-rules/preview", &input, &quote) != http.StatusOK {
		return finish("unavailable", "主站规则试算暂不可用")
	}
	result.Rule, result.Calculation, result.MatchMode, result.AppliedRules = quote.MatchedRule, quote.Calculation, quote.MatchMode, quote.AppliedRules
	if quote.RequiresManualReview || !domain.ValidObservedCost(quote.EstimatedCost) ||
		quote.MatchedRule == nil || quote.MatchedRule.RuleID <= 0 || quote.MatchedRule.RuleVersion <= 0 ||
		(quote.MatchMode != string(domain.CostRuleMatchModeBindingERPIID) && quote.MatchMode != string(domain.CostRuleMatchModeBindingProductIID)) {
		reason := quote.Explanation
		if reason == "" {
			reason = "缺少有效的绑定规则或完整计价输入，需人工核查"
		}
		return finish("needs_review", reason)
	}
	result.Suggested = quote.EstimatedCost
	return finish("ready", quote.Explanation)
}

func equalPrice(a, b *float64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func prepareInput(r domain.CostMaintenanceRow, item *domain.TaskSKUItem, detail *domain.TaskDetail) (domain.CostRulePreviewRequest, bool) {
	var variant map[string]json.RawMessage
	_ = json.Unmarshal(item.VariantJSON, &variant)
	get := func(keys ...string) string {
		for _, key := range keys {
			var value string
			if json.Unmarshal(variant[key], &value) == nil && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
		return ""
	}
	erp, product := get("erp_i_id", "erp_product_i_id"), strings.TrimSpace(item.ProductIID)
	if product == "" {
		product = get("product_i_id", "i_id")
	}
	style := product
	if erp != "" {
		style = erp
	}
	if style == "" || !strings.EqualFold(style, strings.TrimSpace(r.StyleCode)) {
		return domain.CostRulePreviewRequest{}, false
	}
	metres := func(v *float64) *float64 {
		if v == nil {
			return nil
		}
		value := *v / 100
		return &value
	}
	process := get("process", "craft_text")
	if process == "" && detail != nil {
		process = detail.Process
	}
	return domain.CostRulePreviewRequest{
		ERPIID: erp, ProductIID: product, Width: metres(r.WidthCM), Height: metres(r.HeightCM),
		Area: r.AreaM2, Input: r.CostInput, Notes: r.SizeText, Process: process,
		Quantity: item.Quantity,
	}, true
}

func (a app) loadCanonical(r *http.Request, sku string) (result domain.CostMaintenanceRow, state string) {
	var before int64
	for range 5 {
		path := "/v1/cost-management/sku-costs"
		q := url.Values{"keyword": {sku}, "review_state": {"all"}, "page_size": {"100"}}
		if before > 0 {
			q.Set("before_id", strconv.FormatInt(before, 10))
		}
		var page struct {
			Data []domain.CostMaintenanceRow `json:"data"`
			Next int64                       `json:"next_before_id"`
		}
		if a.readCore(r, http.MethodGet, path+"?"+q.Encode(), nil, &page) != http.StatusOK {
			return result, "unavailable"
		}
		for _, entry := range page.Data {
			if strings.EqualFold(strings.TrimSpace(entry.SKUCode), sku) {
				if result.ID > 0 && result.ID != entry.ID {
					return result, "ambiguous"
				}
				result = entry
			}
		}
		if page.Next == 0 {
			if result.ID > 0 && result.TaskID > 0 {
				return result, "ok"
			}
			return result, "not_found"
		}
		if before > 0 && page.Next >= before {
			return result, "incomplete"
		}
		before = page.Next
	}
	return result, "incomplete"
}

type responseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *responseWriter) Header() http.Header    { return w.header }
func (w *responseWriter) WriteHeader(status int) { w.status = status }
func (w *responseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(p)
}
func (a app) readCore(r *http.Request, method, path string, payload any, out any) int {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return 500
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(r.Context(), method, path, body)
	if err != nil {
		return 500
	}
	request.Header.Set("Authorization", r.Header.Get("Authorization"))
	response := &responseWriter{header: make(http.Header)}
	a.native.ServeHTTP(response, request)
	if response.status != 200 {
		return response.status
	}
	raw := response.body.Bytes()
	if strings.HasPrefix(path, "/v1/cost-management/sku-costs?") {
		if json.Unmarshal(raw, out) != nil {
			return 502
		}
	} else {
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(raw, &envelope) != nil || len(envelope.Data) == 0 || json.Unmarshal(envelope.Data, out) != nil {
			return 502
		}
	}
	return 200
}
