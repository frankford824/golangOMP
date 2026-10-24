package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"workflow/domain"
	"workflow/repo"
)

type CostMaintenanceService interface {
	ListCostMaintenance(context.Context, repo.CostMaintenanceFilter) (*domain.CostMaintenanceList, *domain.AppError)
	MaintainCost(context.Context, int64, domain.CostMaintenanceRequest) (*domain.CostMaintenanceRow, *domain.AppError)
	CostMaintenanceLogs(context.Context, repo.CostMaintenanceFilter) ([]*domain.CostMaintenanceLog, domain.PaginationMeta, *domain.AppError)
}

func WithTaskCostMaintenanceRepo(r repo.CostMaintenanceRepo) TaskServiceOption {
	return func(s *taskService) { s.costMaintenance = r }
}

func maintenanceToken(r *domain.CostMaintenanceRow) string {
	// Includes specifications, price, identity and sync revision. A background
	// refresh can never silently move the baseline of an open editor.
	raw, _ := json.Marshal([]interface{}{r.ID, r.ProductName, r.VariantJSON, r.ProductIID, r.CostPrice, r.Revision, r.UpdatedAt, r.DetailWidth, r.DetailHeight, r.DetailArea, r.DetailSize})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func hydrateMaintenanceRow(r *domain.CostMaintenanceRow) {
	r.Token = maintenanceToken(r)
	item := &domain.TaskSKUItem{VariantJSON: r.VariantJSON}
	r.StyleCode = firstNonEmptyString(taskSKUItemERPIID(item), taskSKUItemProductIID(item), r.ProductIID)
	r.WidthCM, r.HeightCM, r.AreaM2 = taskSKUItemVariantDimensions(item)
	if r.AreaM2 == nil && r.WidthCM != nil && r.HeightCM != nil {
		a := *r.WidthCM * *r.HeightCM / 10000
		r.AreaM2 = &a
	}
	if r.WidthCM == nil {
		r.WidthCM = r.DetailWidth
	}
	if r.HeightCM == nil {
		r.HeightCM = r.DetailHeight
	}
	if r.AreaM2 == nil {
		r.AreaM2 = r.DetailArea
	}
	r.SizeText = firstNonEmptyString(taskSKUItemVariantString(item, "size_text"), r.DetailSize)
	r.CostInput = skuCostInput(item)
	if in := r.CostInput; in != nil {
		switch in.AreaMode {
		case "flat", "layout":
			w, h := in.WidthM*100, in.HeightM*100
			r.WidthCM = &w
			r.HeightCM = &h
			a := in.WidthM * in.HeightM
			if in.AreaMode == "flat" {
				a *= float64(in.Pieces)
			}
			r.AreaM2 = &a
		case "total":
			r.AreaM2 = &in.AreaM2
			r.WidthCM = nil
			r.HeightCM = nil
		case "faces":
			a := 0.0
			for _, f := range in.Faces {
				a += f.WidthM * f.HeightM * float64(f.Count)
			}
			r.AreaM2 = &a
			r.WidthCM = nil
			r.HeightCM = nil
		}
	} else if r.AreaM2 == nil && r.WidthCM != nil && r.HeightCM != nil {
		a := *r.WidthCM * *r.HeightCM / 10000
		r.AreaM2 = &a
	}
	r.Reviewed = maintenanceReviewMatches(r)
}

// Human review is independent of ERP convergence. Read the latest committed
// audit snapshot so existing confirmations survive reloads without a backfill.
// Runtime timestamps, revisions and delivery states do not change that review;
// a change to any reviewed business value does.
func maintenanceReviewMatches(r *domain.CostMaintenanceRow) bool {
	var log struct {
		Action string                     `json:"action"`
		After  *domain.CostMaintenanceRow `json:"after"`
	}
	if json.Unmarshal(r.ReviewJSON, &log) != nil || log.After == nil || (log.Action != "confirm" && log.Action != "modify") || !domain.ValidObservedCost(r.CostPrice) || !domain.EqualCost(r.CostPrice, log.After.CostPrice) {
		return false
	}
	values := func(v *domain.CostMaintenanceRow) []byte {
		raw, _ := json.Marshal([]interface{}{v.ID, v.TaskID, v.SKUCode, v.ProductName, v.StyleCode, v.WidthCM, v.HeightCM, v.AreaM2, v.SizeText, v.CostInput})
		return raw
	}
	return bytes.Equal(values(r), values(log.After))
}
func normalizeMaintenanceFilter(f repo.CostMaintenanceFilter) repo.CostMaintenanceFilter {
	f.Keyword = strings.TrimSpace(f.Keyword)
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 50
	}
	if f.Page < 1 {
		f.Page = 1
	}
	return f
}
func (s *taskService) ListCostMaintenance(ctx context.Context, f repo.CostMaintenanceFilter) (*domain.CostMaintenanceList, *domain.AppError) {
	f = normalizeMaintenanceFilter(f)
	if f.ReviewState == "" {
		f.ReviewState = "pending"
	}
	if f.ReviewState != "pending" && f.ReviewState != "reviewed" && f.ReviewState != "all" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "确认状态无效", nil)
	}
	// Match the visible human-audit predicate before choosing the page. Bounded
	// keyset batches avoid empty pages after a run of already-reviewed SKUs.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result := &domain.CostMaintenanceList{Data: []*domain.CostMaintenanceRow{}, RefreshedAt: time.Now().UTC()}
	batch := f
	if f.ReviewState != "all" {
		batch.PageSize = 100
	}
	for {
		rows, err := s.costMaintenance.List(ctx, batch)
		if err != nil {
			return nil, infraError("list completed SKU costs", err)
		}
		for _, r := range rows {
			hydrateMaintenanceRow(r)
			reviewed := r.Reviewed && r.Status != "conflict"
			if f.ReviewState == "pending" && reviewed || f.ReviewState == "reviewed" && !reviewed {
				continue
			}
			if len(result.Data) == f.PageSize {
				result.NextBeforeID = result.Data[len(result.Data)-1].SortID
				return result, nil
			}
			result.Data = append(result.Data, r)
		}
		if len(rows) <= batch.PageSize {
			break
		}
		batch.BeforeID = rows[len(rows)-1].SortID
	}
	return result, nil
}
func (s *taskService) CostMaintenanceLogs(ctx context.Context, f repo.CostMaintenanceFilter) ([]*domain.CostMaintenanceLog, domain.PaginationMeta, *domain.AppError) {
	f = normalizeMaintenanceFilter(f)
	if f.Page > 200 {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "请缩小日志查询范围", nil)
	}
	rows, total, err := s.costMaintenance.ListLogs(ctx, f)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list maintenance audit", err)
	}
	return rows, buildPaginationMeta(f.Page, f.PageSize, total), nil
}

func validateMaintenanceRequest(p domain.CostMaintenanceRequest) *domain.AppError {
	bad := func(msg string) *domain.AppError { return domain.NewAppError(domain.ErrCodeInvalidRequest, msg, nil) }
	if p.Token == "" || p.Revision < 1 {
		return bad("数据版本缺失，请刷新后重试")
	}
	if p.Action != "confirm" && p.Action != "modify" {
		return bad("请选择确认或修改")
	}
	if p.Action == "confirm" {
		if p.ProductName != nil || p.StyleCode != nil || p.WidthCM != nil || p.HeightCM != nil || p.AreaM2 != nil || p.CostInput != nil || p.CostPrice != nil {
			return bad("确认不能携带修改字段")
		}
		return nil
	}
	switch p.ReasonCode {
	case "area_incorrect", "style_binding_incorrect", "cost_incorrect", "other":
	default:
		return bad("请选择修改原因")
	}
	if utf8.RuneCountInString(p.ReasonDetail) > 200 || p.ReasonCode == "other" && strings.TrimSpace(p.ReasonDetail) == "" {
		return bad("其他原因需填写说明，最多200字")
	}
	if p.ProductName != nil && (strings.TrimSpace(*p.ProductName) == "" || erpProductNameTooLong(*p.ProductName)) {
		return bad(erpProductNameLimitMessage())
	}
	if p.StyleCode != nil && (strings.TrimSpace(*p.StyleCode) == "" || utf8.RuneCountInString(*p.StyleCode) > 128) {
		return bad("请填写有效款式编码")
	}
	for _, n := range []*float64{p.WidthCM, p.HeightCM, p.AreaM2} {
		if n != nil && (!validCostNumber(*n) || *n <= 0) {
			return bad("尺寸与面积必须是有效正数")
		}
	}
	if p.CostPrice != nil && !domain.ValidObservedCost(p.CostPrice) {
		return bad("成本价必须为有效非负金额")
	}
	if p.CostInput != nil {
		c, _ := calculateCostModel(&domain.CostModel{Basis: "area", UnitPrice: 1, Multiplier: 1}, *p.CostInput)
		if len(c.Missing) > 0 {
			return bad(strings.Join(c.Missing, "；"))
		}
	}
	return nil
}

func (s *taskService) MaintainCost(ctx context.Context, id int64, p domain.CostMaintenanceRequest) (*domain.CostMaintenanceRow, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionCatalogManage) || !domain.ActorHasPermission(actor, domain.PermissionERPManage) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "需要成本维护和ERP管理权限", nil)
	}
	if err := validateMaintenanceRequest(p); err != nil {
		return nil, err
	}
	if s.costSync == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "成本同步未启用，暂不能确认或改价", nil)
	}
	// Locate only, then verify every editable value under a canonical row lock.
	var before *domain.CostMaintenanceRow
	err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error { var e error; before, e = s.costMaintenance.LockRow(ctx, tx, id); return e })
	if err != nil {
		return nil, infraError("read maintenance SKU", err)
	}
	if before == nil {
		return nil, domain.ErrNotFound
	}
	hydrateMaintenanceRow(before)
	if before.Token != p.Token || before.Revision != p.Revision {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "记录已更新，请刷新后核对；本次未修改", nil)
	}
	edit, appErr := s.costSync.PrepareManualEdit(ctx, before.SKUCode, &domain.CostEditBaseline{Revision: p.Revision, ERPCost: p.ERPCost})
	if appErr != nil {
		return nil, appErr
	}
	defer edit.Close()
	task, err := s.taskRepo.GetByID(ctx, before.TaskID)
	if err != nil {
		return nil, infraError("read maintenance task", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionCatalogManage, task.AccessSubject()) {
		return nil, domain.ErrPermissionDenied
	}
	detail, err := s.taskRepo.GetDetailByTaskID(ctx, task.ID)
	if err != nil {
		return nil, infraError("read maintenance detail", err)
	}
	if detail == nil {
		return nil, domain.ErrNotFound
	}
	items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, task.ID)
	if err != nil {
		return nil, infraError("read maintenance items", err)
	}
	var item *domain.TaskSKUItem
	for _, v := range items {
		if v.ID == id {
			c := *v
			item = &c
			break
		}
	}
	if item == nil {
		return nil, domain.ErrNotFound
	}
	oldVariant := string(item.VariantJSON)
	oldName := item.ProductNameSnapshot
	if item.ProductIID == "" {
		item.ProductIID = before.ProductIID
	}
	if p.Action == "modify" {
		if p.ProductName != nil {
			item.ProductNameSnapshot = strings.TrimSpace(*p.ProductName)
			item.ProductShortName = item.ProductNameSnapshot
		}
		if p.StyleCode != nil {
			// Both explicit style identities must agree; a stale ERP identity must not
			// take precedence over the newly selected product style during pricing.
			v := map[string]interface{}{}
			if len(item.VariantJSON) > 0 {
				if err = json.Unmarshal(item.VariantJSON, &v); err != nil {
					return nil, infraError("decode style", err)
				}
			}
			style := strings.TrimSpace(*p.StyleCode)
			v["product_i_id"] = style
			v["i_id"] = style
			v["erp_i_id"] = style
			delete(v, "erp_product_i_id")
			item.VariantJSON, _ = json.Marshal(v)
			item.ProductIID = style
			binding, e := s.costRuleBindingRepo.GetActiveByNormalizedIID(ctx, domain.NormalizeIID(style))
			if e != nil {
				return nil, infraError("validate style binding", e)
			}
			if binding == nil {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "此款式尚未明确绑定计价方案，请先在方案与绑定中维护", nil)
			}
		}
		if p.WidthCM != nil || p.HeightCM != nil || p.AreaM2 != nil || p.CostInput != nil {
			item.VariantJSON, appErr = setTaskSKUItemSpecInVariantJSON(item.VariantJSON, UpdateTaskSKUItemInfoParams{Width: p.WidthCM, Height: p.HeightCM, Area: p.AreaM2, CostInput: p.CostInput})
			if appErr != nil {
				return nil, appErr
			}
			obj := map[string]interface{}{}
			if err = json.Unmarshal(item.VariantJSON, &obj); err != nil {
				return nil, infraError("decode specification", err)
			}
			if p.CostInput != nil {
				calc, _ := calculateCostModel(&domain.CostModel{Basis: "area", UnitPrice: 1, Multiplier: 1}, *p.CostInput)
				obj["area"] = calc.AreaM2
				if p.CostInput.AreaMode == "flat" || p.CostInput.AreaMode == "layout" {
					obj["width"] = p.CostInput.WidthM * 100
					obj["height"] = p.CostInput.HeightM * 100
				}
			}
			if p.WidthCM != nil || p.HeightCM != nil {
				w, h := before.WidthCM, before.HeightCM
				if p.WidthCM != nil {
					w = p.WidthCM
				}
				if p.HeightCM != nil {
					h = p.HeightCM
				}
				if w != nil && h != nil {
					obj["size_text"] = fmt.Sprintf("%g×%gcm", *w, *h)
				}
			}
			item.VariantJSON, _ = json.Marshal(obj)
		}
		if p.CostPrice != nil {
			item.CostPrice = cloneFloat64Ptr(p.CostPrice)
		}
	}
	if !domain.ValidObservedCost(item.CostPrice) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请先填写成本价再确认", nil)
	}
	reason := "已核对当前编码成本"
	if p.Action == "modify" {
		reason = p.ReasonCode + "：" + strings.TrimSpace(p.ReasonDetail)
	}
	item.ManualCostOverride = true
	item.ManualCostOverrideReason = reason
	item.RequiresManualReview = false
	item.OverrideActor = formatOverrideActor(actor.ID)
	now := time.Now().UTC()
	item.OverrideAt = &now
	businessChanged := oldName != item.ProductNameSnapshot || oldVariant != string(item.VariantJSON)
	if p.Action == "modify" && !businessChanged && domain.EqualCost(before.CostPrice, item.CostPrice) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "没有变更内容，请直接确认当前成本", nil)
	}
	// Preserve the explicit reviewed price while refreshing rule provenance.
	if businessChanged {
		if _, appErr = s.applyTaskSKUItemCostPreview(ctx, detail, item); appErr != nil {
			return nil, appErr
		}
		item.RequiresManualReview = false
	}
	syncDetail := len(items) == 1 || task.PrimarySKUCode == item.SKUCode || task.SKUCode == item.SKUCode
	err = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		locked, e := s.costMaintenance.LockRow(ctx, tx, id)
		if e != nil {
			return e
		}
		if locked == nil || maintenanceToken(locked) != p.Token {
			return domain.NewAppError(domain.ErrCodeConflict, "记录已更新，请刷新后核对；本次未修改", nil)
		}
		if e = edit.Lock(ctx, tx); e != nil {
			return e
		}
		if e = s.costMaintenance.Save(ctx, tx, item, syncDetail, businessChanged); e != nil {
			return e
		}
		if e = s.traceTaskCostUpdate(ctx, tx, task, detail, item, actor.ID, "cost_maintenance", p.Action); e != nil {
			return e
		}
		if e = s.costMaintenance.Project(ctx, tx, item); e != nil {
			return e
		}
		if e = edit.Stage(ctx, tx, actor.ID, reason); e != nil {
			return e
		}
		after, e := s.costMaintenance.LockRow(ctx, tx, id)
		if e != nil {
			return e
		}
		if after == nil {
			return domain.ErrNotFound
		}
		hydrateMaintenanceRow(after)
		after.Reviewed = true
		payload := map[string]interface{}{"sku_code": item.SKUCode, "sku_item_id": id, "action": p.Action, "reason_code": p.ReasonCode, "reason_detail": strings.TrimSpace(p.ReasonDetail), "before": before, "after": after}
		if e = s.costMaintenance.AppendLog(ctx, tx, task.ID, id, item.SKUCode, p.Action, p.ReasonCode, actor.ID, payload); e != nil {
			return e
		}
		_, e = s.taskEventRepo.Append(ctx, tx, task.ID, domain.CostMaintenanceEvent, &actor.ID, payload)
		return e
	})
	if err != nil {
		if e, ok := err.(*domain.AppError); ok {
			return nil, e
		}
		return nil, infraError("save maintenance SKU", err)
	}
	s.costSync.Notify()
	// The canonical commit is already durable. Return its visible pending state;
	// only the worker's direct ERP readback can claim convergence.
	before.ProductName = item.ProductNameSnapshot
	before.VariantJSON = item.VariantJSON
	before.CostPrice = item.CostPrice
	before.Status = "pending"
	before.Token = ""
	hydrateMaintenanceRow(before)
	before.Reviewed = true
	var current *domain.CostMaintenanceRow
	if readErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error { var e error; current, e = s.costMaintenance.LockRow(ctx, tx, id); return e }); readErr == nil && current != nil {
		hydrateMaintenanceRow(current)
		return current, nil
	}
	// Never turn a committed write into an apparent failure because of a
	// post-commit read issue. The caller refreshes the canonical list next.
	before.Token = ""
	return before, nil
}
