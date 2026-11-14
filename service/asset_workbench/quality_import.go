package assetworkbench

import (
	"context"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type QualityImportRowOverride struct {
	Sheet          int    `json:"sheet"`
	Row            int    `json:"row"`
	PayrollRowType string `json:"payroll_row_type"`
}

func applyQualityImportOverrides(records []ImportErrorRecordInput, overrides []QualityImportRowOverride) *domain.AppError {
	type position struct{ sheet, row int }
	byPosition := make(map[position]string, len(overrides))
	for _, override := range overrides {
		key := position{override.Sheet, override.Row}
		_, duplicate := byPosition[key]
		if _, valid := normalizeQualityPayrollRowType(override.PayrollRowType); duplicate || !valid || strings.TrimSpace(override.PayrollRowType) == "" {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "工资条类型选择无效或存在重复。", nil)
		}
		byPosition[key] = override.PayrollRowType
	}
	for idx := range records {
		row := &records[idx]
		key := position{row.excelSheet, row.excelRow}
		if rowType, ok := byPosition[key]; ok {
			row.PayrollRowType = rowType
			delete(byPosition, key)
		}
	}
	if len(byPosition) != 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "工资条选择对应的行不存在，请重新校对表格。", nil)
	}
	return nil
}

type QualityImportReviewRow struct {
	Sheet            int    `json:"sheet"`
	Row              int    `json:"row"`
	PayeeName        string `json:"payee_name"`
	OccurredDate     string `json:"occurred_date"`
	DifficultyClass  string `json:"difficulty_class"`
	ErrorCount       int    `json:"error_count"`
	IssueDescription string `json:"issue_description"`
	OrderNo          string `json:"order_no"`
	FileName         string `json:"file_name"`
	PayrollRowType   string `json:"payroll_row_type"`
	NeedsPayrollType bool   `json:"needs_payroll_type"`
	MatchStatus      string `json:"match_status"`
	Message          string `json:"message"`
	Blocking         bool   `json:"blocking"`
}

type QualityImportReview struct {
	Rows []QualityImportReviewRow `json:"rows"`
}

func (s *Service) validateQualityImport(actor domain.RequestActor, params ImportErrorRecordsParams) (string, *domain.AppError) {
	if appErr := s.requireRepo(); appErr != nil {
		return "", appErr
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return "", domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers or settlement roles can import error records.", nil)
	}
	month := strings.TrimSpace(params.BusinessMonth)
	if _, err := time.Parse("2006-01", month); err != nil || month > s.businessMonth(s.nowFn().UTC()) {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择当前或历史结算月。", nil)
	}
	if len(params.Records) == 0 || len(params.Records) > 50000 {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "质检记录须在 1 至 50000 行之间。", nil)
	}
	return month, nil
}

func (s *Service) checkQualityImportMonth(ctx context.Context, month string) *domain.AppError {
	states, err := s.repo.ListSettlementMonthStates(ctx)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to check settlement month state.", nil)
	}
	if states[month] {
		return domain.NewAppError(domain.ErrCodeConflict, "该月已有生成或确认的结算批次，质检导入已锁定。", nil)
	}
	return nil
}

func (s *Service) PreviewQualityImport(ctx context.Context, actor domain.RequestActor, params ImportErrorRecordsParams) (*QualityImportReview, *domain.AppError) {
	month, appErr := s.validateQualityImport(actor, params)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.checkQualityImportMonth(ctx, month); appErr != nil {
		return nil, appErr
	}
	_, rows, appErr := s.reviewQualityImport(ctx, month, params.Records)
	if appErr != nil {
		return nil, appErr
	}
	return &QualityImportReview{Rows: rows}, nil
}

func (s *Service) reviewQualityImport(ctx context.Context, month string, inputs []ImportErrorRecordInput) ([]errorRecordImportMatch, []QualityImportReviewRow, *domain.AppError) {
	var items []*domain.AssetWorkbenchSubmissionItem
	var difficulties map[string]string
	references := make(map[int64]qualityWorkLookup)
	type payeeKey struct {
		id   int64
		name string
	}
	payees := make(map[payeeKey]qualityImportPayee)
	matches := make([]errorRecordImportMatch, len(inputs))
	rows := make([]QualityImportReviewRow, len(inputs))
	for idx, input := range inputs {
		row := QualityImportReviewRow{
			Sheet: input.excelSheet, Row: input.excelRow, PayeeName: input.PayeeName,
			OrderNo: input.OrderNo, FileName: input.FileName, OccurredDate: input.OccurredDate,
			DifficultyClass: input.DifficultyClass, ErrorCount: input.ErrorCount, IssueDescription: input.IssueDescription,
		}
		if row.Row == 0 {
			row.Row = idx + 1
		}
		rowType, valid := normalizeQualityPayrollRowType(input.PayrollRowType)
		switch {
		case input.ErrorCount < 0:
			row.Message = "出错张数须为非负整数。"
		case rowType == domain.AssetWorkbenchPayrollRowTypeSupplementPiecework && strings.TrimSpace(input.DifficultyClass) == "":
			row.Message = "补录出错记录必须填写出错分类。"
		}
		if row.Message != "" {
			row.Blocking = true
			rows[idx] = row
			continue
		}

		var match errorRecordImportMatch
		var appErr *domain.AppError
		qualityInput := isQualityErrorImportInput(input)
		if qualityInput {
			if difficulties == nil {
				difficulties, appErr = s.errorImportDifficultyIndex(ctx)
				if appErr != nil {
					return nil, nil, appErr
				}
			}
			key := payeeKey{name: normalizeHumanName(input.PayeeName)}
			if input.PayeeUserID != nil {
				key.id = *input.PayeeUserID
			}
			match, appErr = s.matchQualityErrorRecord(input, difficulties, func() (qualityImportPayee, *domain.AppError) {
				if result, ok := payees[key]; ok {
					return result, nil
				}
				id, candidates, reason, appErr := s.matchErrorImportPayee(ctx, input)
				result := qualityImportPayee{id: id, candidates: candidates, reason: reason}
				if appErr == nil {
					payees[key] = result
				}
				return result, appErr
			})
			if appErr != nil {
				if appErr.Code != domain.ErrCodeInvalidRequest {
					return nil, nil, appErr
				}
				row.Message = "出错日期无效，请修正 Excel 后重新选择。"
				row.Blocking = true
				rows[idx] = row
				continue
			}
		} else {
			if items == nil {
				var err error
				items, err = s.repo.ListSubmissionItemsByMonth(ctx, month)
				if err != nil {
					return nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load submission items for error import matching.", nil)
				}
			}
			match = matchImportedErrorRecord(items, input)
		}

		row.PayrollRowType = rowType
		row.MatchStatus = match.Status
		if match.OccurredDate != nil {
			row.OccurredDate = match.OccurredDate.Format("2006-01-02")
		}
		switch {
		case !valid:
			row.NeedsPayrollType = true
			row.Blocking = true
			row.Message = "工资条类型无效，请重新选择。"
		case match.Status != domain.AssetWorkbenchErrorMatchStatusMatched:
			row.Message = qualityImportUnmatchedMessage(match.Reason)
		case strings.TrimSpace(input.PayrollRowType) != "":
			row.Message = "按手动选择"
		case match.PayeeUserID == nil || (strings.TrimSpace(input.OrderNo) == "" && strings.TrimSpace(input.FileName) == "") || !qualityInput:
			row.Message = "无作品线索，默认日常计件"
		default:
			payeeID := *match.PayeeUserID
			lookup, ok := references[payeeID]
			if !ok {
				refs, err := s.repo.ListQualityImportWorkReferences(ctx, month, payeeID)
				if err != nil {
					return nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to resolve quality import work references.", nil)
				}
				lookup = indexQualityWorkReferences(refs)
				references[payeeID] = lookup
			}
			workID, supplement, message := lookup.resolve(input.OrderNo, input.FileName)
			if workID == 0 {
				row.PayrollRowType = ""
				row.NeedsPayrollType = true
				row.Blocking = true
				row.Message = message
			} else {
				if supplement {
					row.PayrollRowType = domain.AssetWorkbenchPayrollRowTypeSupplementPiecework
				}
				if workID > 0 {
					match.SubmissionItemID = &workID
				}
				row.Message = "已按作品匹配"
			}
		}
		matches[idx] = match
		rows[idx] = row
	}
	return matches, rows, nil
}

func qualityImportUnmatchedMessage(reason string) string {
	switch reason {
	case "difficulty_class_not_found":
		return "出错分类未匹配，本行不计扣款"
	case "payee_name_ambiguous":
		return "人员重名，本行不计扣款"
	default:
		return "未匹配到人员或作品，本行不计扣款"
	}
}

type qualityImportPayee struct {
	id         *int64
	candidates []int64
	reason     string
}

type qualityWorkLookup map[string]map[int64]bool

func indexQualityWorkReferences(refs []repo.AssetWorkbenchQualityWorkReference) qualityWorkLookup {
	index := make(qualityWorkLookup)
	for _, ref := range refs {
		for _, value := range []string{ref.OrderNo, ref.OriginalName, ref.DisplayName, ref.RelativePath} {
			key := qualityWorkKey(value)
			if key == "" {
				continue
			}
			if index[key] == nil {
				index[key] = make(map[int64]bool)
			}
			index[key][ref.WorkID] = ref.Supplement
		}
	}
	return index
}

func qualityWorkKey(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
}

func (index qualityWorkLookup) resolve(orderNo, fileName string) (int64, bool, string) {
	order := index[qualityWorkKey(orderNo)]
	file := index[qualityWorkKey(fileName)]
	if (qualityWorkKey(orderNo) != "" && len(order) == 0) || (qualityWorkKey(fileName) != "" && len(file) == 0) {
		return 0, false, "该人员在所选月份无匹配作品，请核对线索或手动选择工资条。"
	}
	var workID int64
	var supplement bool
	if len(order) == 0 {
		order, file = file, nil
	}
	for id, isSupplement := range order {
		if _, found := file[id]; len(file) > 0 && !found {
			continue
		}
		if workID != 0 {
			return 0, false, "匹配到多件作品，请补全文件名或手动选择工资条。"
		}
		workID, supplement = id, isSupplement
	}
	if workID == 0 {
		return 0, false, "订单号和文件名指向不同作品，请核对或手动选择工资条。"
	}
	return workID, supplement, ""
}

func qualityImportRowError(row QualityImportReviewRow) *domain.AppError {
	location := fmt.Sprintf("第 %d 行", row.Row)
	if row.Sheet > 0 {
		location = fmt.Sprintf("工作表 %d 第 %d 行", row.Sheet, row.Row)
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, location+"："+row.Message, row)
}
