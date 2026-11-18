package assetworkbench

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"

	"workflow/domain"
)

type monthlyContribution struct {
	workerType string
	pages      int
	errors     int
	units      int64
}

var personnelNumberPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func payrollPersonnelNo(payeeID int64, profile *domain.AssetWorkbenchProfile) string {
	if profile != nil && profile.PersonnelNo != nil && *profile.PersonnelNo != "" {
		return *profile.PersonnelNo
	}
	return strconv.FormatInt(payeeID, 10)
}

func difficultyCoefficient(value *float64, fallback float64) (float64, *domain.AppError) {
	if value == nil {
		return fallback, nil
	}
	coefficient := *value
	rounded := math.Round(coefficient*10000) / 10000
	if math.IsNaN(coefficient) || math.IsInf(coefficient, 0) || coefficient <= 0 ||
		coefficient > 999999.9999 || math.Abs(coefficient-rounded) > 1e-9 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "难度系数须大于 0，最多 6 位整数和 4 位小数。", nil)
	}
	return rounded, nil
}

func defaultDifficultyCoefficient(code string) float64 {
	switch code {
	case "A", "D", "A-视频", "A+视频":
		return 1.5
	case "B":
		return 0.8
	case "C":
		return 0.5
	case "A+小夜灯", "小夜灯":
		return 3.5
	default:
		return 1
	}
}

func fillSettlementReportMetrics(report *SettlementReport, coefficients map[string]float64) {
	const precision = 10000
	report.DifficultyCoefficients = make(map[string]float64, len(report.DifficultyClasses))
	for _, code := range report.DifficultyClasses {
		coefficient, exists := coefficients[code]
		if !exists {
			coefficient = defaultDifficultyCoefficient(code)
		}
		report.DifficultyCoefficients[code] = coefficient
	}
	applyDifficulty := func(metrics []SettlementReportDifficultyMetric) int64 {
		var units int64
		for index := range metrics {
			metric := &metrics[index]
			coefficient := report.DifficultyCoefficients[metric.DifficultyClass]
			subtotal := int64(math.Round(coefficient*precision)) * int64(metric.ItemCount)
			metric.DifficultyCoefficient = coefficient
			metric.CoefficientSubtotal = float64(subtotal) / precision
			units += subtotal
		}
		return units
	}
	people := make(map[int64]monthlyContribution)
	for i := range report.Rows {
		row := &report.Rows[i]
		person := people[row.PayeeUserID]
		person.workerType = row.WorkerType
		person.pages += row.PageCount
		person.errors += row.ErrorCount
		person.units += applyDifficulty(row.DifficultyMetrics)
		people[row.PayeeUserID] = person
	}
	totalUnits := applyDifficulty(report.Totals.DifficultyMetrics)
	report.Totals.MonthlyCoefficient = float64(totalUnits) / precision
	report.Totals.MonthlyContributionRate = boolReportShare(totalUnits > 0)

	// Iterate in sorted report order so group averages are reproducible.
	report.WorkerTypeAverageErrorRates = make(map[string]float64)
	groupSizes := make(map[string]int)
	seen := make(map[int64]bool)
	for _, row := range report.Rows {
		person := people[row.PayeeUserID]
		if seen[row.PayeeUserID] || person.pages <= 0 || person.errors <= 0 ||
			(person.workerType != domain.AssetWorkbenchWorkerTypeFulltime && person.workerType != domain.AssetWorkbenchWorkerTypeParttime) {
			continue
		}
		seen[row.PayeeUserID] = true
		groupSizes[person.workerType]++
		report.WorkerTypeAverageErrorRates[person.workerType] += safeReportRatio(person.errors, person.pages)
	}
	for workerType, sum := range report.WorkerTypeAverageErrorRates {
		report.WorkerTypeAverageErrorRates[workerType] = sum / float64(groupSizes[workerType])
	}
	for index := range report.Rows {
		row := &report.Rows[index]
		person := people[row.PayeeUserID]
		row.MonthlyCoefficient = float64(person.units) / precision
		row.MonthlyContributionRate = safeReportAmountRatio(float64(person.units), float64(totalUnits))
		row.MonthlyErrorRate = safeReportRatio(person.errors, person.pages)
		if rate, ok := report.WorkerTypeAverageErrorRates[person.workerType]; ok {
			row.WorkerTypeAverageErrorRate = &rate
		}
	}
}

type settlementReportLedger struct {
	gross             map[int64]*domain.AssetWorkbenchSettlementItem
	supplements       map[int64]*domain.AssetWorkbenchSettlementItem
	legacyErrors      map[int64]*domain.AssetWorkbenchSettlementItem
	qualityErrors     map[int64]*domain.AssetWorkbenchSettlementItem
	actualDifferences map[string]float64
	manualActual      map[string]float64
	consumedActual    map[string]bool
	reservations      settlementReservations
	other             []*domain.AssetWorkbenchSettlementItem
}

func indexSettlementReportLedger(items []*domain.AssetWorkbenchSettlementItem, actual []*domain.AssetWorkbenchSettlementAdjustment) settlementReportLedger {
	ledger := settlementReportLedger{
		gross:             make(map[int64]*domain.AssetWorkbenchSettlementItem),
		supplements:       make(map[int64]*domain.AssetWorkbenchSettlementItem),
		legacyErrors:      make(map[int64]*domain.AssetWorkbenchSettlementItem),
		qualityErrors:     make(map[int64]*domain.AssetWorkbenchSettlementItem),
		actualDifferences: make(map[string]float64),
		manualActual:      make(map[string]float64),
		consumedActual:    make(map[string]bool),
		reservations:      reservedSettlementSources(items),
	}
	for _, item := range items {
		switch item.ItemType {
		case domain.AssetWorkbenchItemTypeGrossPiecework:
			ledger.gross[derefInt64(item.SubmissionItemID)] = item
		case domain.AssetWorkbenchItemTypeSupplement:
			ledger.supplements[derefInt64(item.SourceRefID)] = item
		case domain.AssetWorkbenchItemTypeErrorDeduction:
			if item.SubmissionItemID == nil {
				ledger.qualityErrors[derefInt64(item.SourceRefID)] = item
			} else {
				ledger.legacyErrors[*item.SubmissionItemID] = item
			}
		case domain.AssetWorkbenchItemTypeQualityDeductionAdjustment:
			key := settlementReportRowKey(item.PayeeUserID, domain.WorkbenchPayrollRowType(item.Snapshot))
			ledger.actualDifferences[key] += signedSettlementAmount(item)
			if item.UnitPrice != nil && *item.UnitPrice > 0 {
				ledger.consumedActual[key] = true
			}
		case domain.AssetWorkbenchItemTypeWelfare, domain.AssetWorkbenchItemTypeAdjustment, domain.AssetWorkbenchItemTypeReversal:
			ledger.other = append(ledger.other, item)
		}
	}
	for _, entry := range actual {
		ledger.manualActual[settlementReportRowKey(entry.PayeeUserID, domain.WorkbenchPayrollRowType(entry.Payload))] = entry.Amount
	}
	return ledger
}

func countPayrollDifficulty(row *SettlementPayrollRow, code string) {
	if row.DifficultyPieceCounts == nil {
		row.DifficultyPieceCounts = make(map[string]int)
	}
	row.DifficultyPieceCounts[settlementReportDifficultyClass(code)]++
}

func payrollSnapshotDifficulty(snapshot json.RawMessage) string {
	var payload struct {
		DifficultyClass string `json:"difficulty_class"`
	}
	_ = json.Unmarshal(snapshot, &payload)
	return payload.DifficultyClass
}

func payrollPricingSnapshot(item *domain.AssetWorkbenchSubmissionItem) json.RawMessage {
	if payrollSnapshotDifficulty(item.PricingSnapshot) == item.DifficultyClass {
		return item.PricingSnapshot
	}
	payload := map[string]json.RawMessage{}
	if json.Unmarshal(item.PricingSnapshot, &payload) != nil || payload == nil {
		payload = make(map[string]json.RawMessage)
	}
	payload["difficulty_class"] = mustJSON(item.DifficultyClass)
	return mustJSON(payload)
}
