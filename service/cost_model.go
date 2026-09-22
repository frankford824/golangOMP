package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	"workflow/domain"
)

func parseCostModel(raw string) (*domain.CostModel, error) {
	if len(raw) > 32768 {
		return nil, fmt.Errorf("计价方案不能超过32KB")
	}
	var m domain.CostModel
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return nil, fmt.Errorf("计价方案格式错误：%w", err)
	}
	if err := d.Decode(new(interface{})); err != io.EOF {
		return nil, fmt.Errorf("计价方案只能包含一个JSON对象")
	}
	if m.Version != 1 || strings.TrimSpace(m.Material) == "" {
		return nil, fmt.Errorf("请填写材质，方案版本必须为1")
	}
	if m.Basis != "area" && m.Basis != "piece" && m.Basis != "set" && m.Basis != "manual" {
		return nil, fmt.Errorf("计费方式必须为面积、件、套或人工报价")
	}
	for _, n := range []float64{m.UnitPrice, m.Multiplier, m.Minimum, m.SmallAreaThreshold, m.SmallAreaSurcharge} {
		if !validCostNumber(n) {
			return nil, fmt.Errorf("计价参数必须是有限的非负数")
		}
	}
	if m.Multiplier <= 0 {
		return nil, fmt.Errorf("基础价格系数必须大于0")
	}
	if len(m.ThicknessPrices) > 32 || len(m.Processes) > 8 {
		return nil, fmt.Errorf("厚度档位最多32项，工艺最多8项")
	}
	seen := map[float64]bool{}
	for _, p := range m.ThicknessPrices {
		if p.ThicknessMM <= 0 || !validCostNumber(p.ThicknessMM) || !validCostNumber(p.UnitPrice) || seen[p.ThicknessMM] {
			return nil, fmt.Errorf("厚度必须为不重复的正数，单价不能为负")
		}
		seen[p.ThicknessMM] = true
	}
	codes := map[string]bool{}
	for _, p := range m.Processes {
		if p.Code != "slot" && p.Code != "punch" && p.Code != "laminate" && p.Code != "double_sided" {
			return nil, fmt.Errorf("不支持的工艺：%s", p.Code)
		}
		if codes[p.Code] {
			return nil, fmt.Errorf("工艺不能重复")
		}
		codes[p.Code] = true
		if p.Unit != "sku" && p.Unit != "piece" && p.Unit != "hole" && p.Unit != "metre" && p.Unit != "area" {
			return nil, fmt.Errorf("不支持的工艺收费单位")
		}
		if !validCostNumber(p.UnitPrice) || !validCostNumber(p.Multiplier) || p.Multiplier <= 0 {
			return nil, fmt.Errorf("工艺价格必须非负，系数必须大于0")
		}
	}
	return &m, nil
}
func validCostNumber(n float64) bool {
	return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 && n <= 1e9
}

func costReadyForFiling(cost *float64, review, manual bool) bool {
	return cost != nil && validCostNumber(*cost) && (!review || manual)
}

func calculateCostModel(m *domain.CostModel, in domain.CostInput) (*domain.CostCalculation, *float64) {
	c := &domain.CostCalculation{Status: "missing_input", Input: in, Lines: []domain.CostLine{}, Missing: []string{}}
	missing := func(s string) { c.Missing = append(c.Missing, s) }
	if m.Basis == "manual" {
		c.Status = "manual_quote"
		missing("此方案使用人工报价")
		return c, nil
	}
	for _, n := range []float64{in.WidthM, in.HeightM, in.DepthM, in.AreaM2, in.ThicknessMM, in.ProcessLengthM} {
		if !validCostNumber(n) {
			missing("规格必须为有限的非负数")
			return c, nil
		}
	}
	if in.Pieces < 1 || in.Pieces > 100000 || in.HoleCount < 0 || in.HoleCount > 100000 {
		missing("每SKU片数必须为1至100000，孔数不能为负")
		return c, nil
	}
	area := 0.0
	needsArea := m.Basis == "area"
	for _, p := range m.Processes {
		if in.Processes[p.Code] && p.Unit == "area" {
			needsArea = true
		}
	}
	if needsArea {
		switch in.AreaMode {
		case "flat":
			if in.WidthM <= 0 || in.HeightM <= 0 {
				missing("请填写单片长、宽")
			}
			area = in.WidthM * in.HeightM * float64(in.Pieces)
		case "layout":
			if in.WidthM <= 0 || in.HeightM <= 0 {
				missing("请填写一个SKU的展开总长、总宽")
			}
			area = in.WidthM * in.HeightM
		case "total":
			if in.AreaM2 <= 0 {
				missing("请填写一个SKU的总面积")
			}
			area = in.AreaM2
		case "faces":
			if len(in.Faces) == 0 || len(in.Faces) > 100 {
				missing("请填写1至100行面清单")
			}
			for _, f := range in.Faces {
				if !validCostNumber(f.WidthM) || !validCostNumber(f.HeightM) || f.WidthM <= 0 || f.HeightM <= 0 || f.Count <= 0 || f.Count > 100000 {
					missing("面清单长宽及片数必须为正数")
					break
				}
				area += f.WidthM * f.HeightM * float64(f.Count)
			}
		default:
			missing("请选择平面、分面、展开或总面积")
		}
	}
	price := m.UnitPrice
	if len(m.ThicknessPrices) > 0 {
		found := false
		for _, p := range m.ThicknessPrices {
			if math.Abs(p.ThicknessMM-in.ThicknessMM) < 0.000001 {
				price = p.UnitPrice
				found = true
				break
			}
		}
		if !found {
			missing("厚度未命中价格档位")
		}
	}
	c.AreaM2 = area
	q := 1.0
	unit := "套"
	if m.Basis == "area" {
		q = math.Max(area, m.Minimum)
		unit = "㎡"
	} else if m.Basis == "piece" {
		q = math.Max(float64(in.Pieces), m.Minimum)
		unit = "件"
	} else {
		q = math.Max(1, m.Minimum)
	}
	c.BillableQuantity = q
	add := func(name string, qty float64, u string, p, mult float64) {
		c.Lines = append(c.Lines, domain.CostLine{Name: name, Quantity: qty, Unit: u, UnitPrice: p, Multiplier: mult, Amount: roundCostAmount(qty * p * mult)})
	}
	add("基础费用", q, unit, price, m.Multiplier)
	if m.Basis == "area" && m.SmallAreaThreshold > 0 && area < m.SmallAreaThreshold {
		add("小面积加价", q, "㎡", m.SmallAreaSurcharge, m.Multiplier)
	}
	configured := map[string]bool{}
	for _, p := range m.Processes {
		configured[p.Code] = true
		on, provided := in.Processes[p.Code]
		if !provided {
			missing("请确认工艺：" + costProcessName(p.Code))
			continue
		}
		if !on {
			continue
		}
		pq := 1.0
		switch p.Unit {
		case "piece":
			pq = float64(in.Pieces)
		case "hole":
			pq = float64(in.HoleCount)
		case "metre":
			pq = in.ProcessLengthM
		case "area":
			pq = area
		}
		if pq <= 0 {
			missing("请补充" + costProcessName(p.Code) + "计费数量")
		}
		add(costProcessName(p.Code), pq, p.Unit, p.UnitPrice, p.Multiplier)
	}
	for code, on := range in.Processes {
		if on && !configured[code] {
			missing("已选择工艺但未配置价格：" + costProcessName(code))
		}
	}
	if len(c.Missing) > 0 {
		return c, nil
	}
	total := 0.0
	for _, line := range c.Lines {
		total += line.Amount
	}
	total = roundCostAmount(total)
	if !validCostNumber(total) || total > maxAutomaticEstimatedCost {
		missing("金额超过自动计算上限，请人工复核")
		return c, nil
	}
	c.Status = "calculated"
	return c, &total
}
func costProcessName(code string) string {
	switch code {
	case "slot":
		return "开槽"
	case "punch":
		return "打孔"
	case "laminate":
		return "覆膜"
	case "double_sided":
		return "双面"
	}
	return code
}

func costModelPreview(req domain.CostRulePreviewRequest, rules []*domain.CostRule) (costPreviewComputation, bool) {
	models := []*domain.CostRule{}
	for _, r := range rules {
		if r != nil && r.RuleType == domain.CostRuleTypeModel {
			models = append(models, r)
		}
	}
	if len(models) == 0 {
		return costPreviewComputation{}, false
	}
	r := models[0]
	resp := &domain.CostRulePreviewResponse{MatchedRule: previewMatchFromRule(r), MatchedRuleID: previewMatchRuleID(r), MatchedRuleVersion: previewMatchRuleVersion(r), AppliedRules: []domain.CostRulePreviewMatch{}, RuleSource: r.Source, GovernanceStatus: previewGovernanceStatus(r), RequiresManualReview: true, ERPIID: req.ERPIID, ProductIID: req.ProductIID}
	out := costPreviewComputation{Response: resp, MatchedRule: r}
	if len(models) != 1 {
		resp.Explanation = "同一规则组存在多套生效计价方案，请停用重复方案"
		resp.Calculation = &domain.CostCalculation{Status: "rule_conflict", Lines: []domain.CostLine{}, Missing: []string{resp.Explanation}}
		return out, true
	}
	m, err := parseCostModel(r.FormulaExpression)
	if err != nil {
		resp.Explanation = err.Error()
		return out, true
	}
	in := domain.CostInput{Pieces: 1, Processes: map[string]bool{}}
	if req.Input != nil {
		in = *req.Input
	} else {
		// Legacy structured fields are per-SKU. Never parse free text for a model.
		if req.Area != nil && *req.Area > 0 {
			in.AreaMode = "total"
			in.AreaM2 = *req.Area
		} else if req.Width != nil && req.Height != nil {
			in.AreaMode = "flat"
			in.WidthM = *req.Width
			in.HeightM = *req.Height
		}
		// Legacy quantity can mean order quantity. Only cost_input.pieces is
		// authoritative for the number of physical pieces within one SKU.
		if m.Basis == "piece" {
			in.Pieces = 0
		}
	}
	resp.Calculation, resp.EstimatedCost = calculateCostModel(m, in)
	resp.RequiresManualReview = resp.EstimatedCost == nil
	resp.AppliedRules = append(resp.AppliedRules, *previewMatchFromRule(r))
	if resp.RequiresManualReview {
		resp.Explanation = strings.Join(resp.Calculation.Missing, "；")
	} else {
		resp.Explanation = fmt.Sprintf("%s：一个SKU销售单位，计费量 %.4f，成本 ¥%.3f。总面积/展开面积不重复乘片数。", r.RuleName, resp.Calculation.BillableQuantity, *resp.EstimatedCost)
	}
	return out, true
}

func skuCostInput(item *domain.TaskSKUItem) *domain.CostInput {
	if item == nil {
		return nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(item.VariantJSON, &obj) != nil {
		return nil
	}
	if raw := obj["cost_input"]; len(raw) > 0 {
		var in domain.CostInput
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil {
			return &domain.CostInput{AreaMode: "invalid"}
		}
		return &in
	}
	return nil
}

func skuCostCalculationSnapshot(item *domain.TaskSKUItem) interface{} {
	if item == nil {
		return nil
	}
	return taskSKUItemVariantObject(item.VariantJSON)["cost_calculation"]
}
