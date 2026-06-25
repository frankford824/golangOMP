package task_ai_summary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/aiagent"
	"workflow/service/task_aggregator"
)

type DetailProvider interface {
	Get(ctx context.Context, taskID int64) (*task_aggregator.Detail, error)
}

type TaskEventProvider interface {
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskEvent, *domain.AppError)
}

type SummaryGenerator interface {
	GenerateTaskSummary(ctx context.Context, evidence any) (*aiagent.TaskSummary, error)
}

type Service struct {
	detail     DetailProvider
	taskEvents TaskEventProvider
	costEvents repo.TaskCostOverrideEventRepo
	generator  SummaryGenerator
}

func NewService(
	detail DetailProvider,
	taskEvents TaskEventProvider,
	costEvents repo.TaskCostOverrideEventRepo,
	generator SummaryGenerator,
) *Service {
	return &Service{
		detail:     detail,
		taskEvents: taskEvents,
		costEvents: costEvents,
		generator:  generator,
	}
}

func (s *Service) Generate(ctx context.Context, taskID int64) (*aiagent.TaskSummary, *domain.AppError) {
	if s == nil || s.detail == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "task AI summary service is not configured", nil)
	}
	if s.generator == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "AI 摘要服务未配置", nil)
	}
	detail, err := s.detail.Get(ctx, taskID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if detail == nil || detail.Task == nil {
		return nil, domain.ErrNotFound
	}

	var taskEvents []*domain.TaskEvent
	var costEvents []*domain.TaskCostOverrideAuditEvent
	notes := make([]string, 0, 2)
	if s.taskEvents != nil {
		loaded, appErr := s.taskEvents.ListByTaskID(ctx, taskID)
		if appErr != nil {
			notes = append(notes, "任务事件加载失败："+appErr.Message)
		} else {
			taskEvents = loaded
		}
	}
	if s.costEvents != nil {
		loaded, err := s.costEvents.ListByTaskID(ctx, taskID)
		if err != nil {
			notes = append(notes, "成本审计事件加载失败："+err.Error())
		} else {
			costEvents = loaded
		}
	}

	evidence := buildEvidence(detail, taskEvents, costEvents, notes)
	summary, err := s.generator.GenerateTaskSummary(ctx, evidence)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "AI 摘要生成失败："+err.Error(), nil)
	}
	return summary, nil
}

type evidence struct {
	GeneratedAt        string                 `json:"generated_at"`
	Task               taskEvidence           `json:"task"`
	People             []personEvidence       `json:"people"`
	Workflow           workflowEvidence       `json:"workflow"`
	Modules            []moduleEvidence       `json:"modules"`
	RecentEvents       []eventEvidence        `json:"recent_events"`
	CostOverrideEvents []costOverrideEvidence `json:"cost_override_events"`
	SKUs               []skuEvidence          `json:"skus"`
	Assets             []assetEvidence        `json:"assets"`
	References         []referenceEvidence    `json:"references"`
	Notes              []string               `json:"notes,omitempty"`
}

type taskEvidence struct {
	ID                    int64    `json:"id"`
	TaskNo                string   `json:"task_no"`
	TaskType              string   `json:"task_type"`
	SourceMode            string   `json:"source_mode"`
	ProductName           string   `json:"product_name"`
	PrimarySKU            string   `json:"primary_sku"`
	Status                string   `json:"status"`
	Priority              string   `json:"priority"`
	BusinessLane          string   `json:"business_lane"`
	OwnerDepartment       string   `json:"owner_department"`
	OwnerTeam             string   `json:"owner_team"`
	DeadlineAt            string   `json:"deadline_at,omitempty"`
	CreatedAt             string   `json:"created_at"`
	UpdatedAt             string   `json:"updated_at"`
	DemandText            string   `json:"demand_text,omitempty"`
	ChangeRequest         string   `json:"change_request,omitempty"`
	DesignRequirement     string   `json:"design_requirement,omitempty"`
	Category              string   `json:"category,omitempty"`
	CategoryCode          string   `json:"category_code,omitempty"`
	Material              string   `json:"material,omitempty"`
	SpecText              string   `json:"spec_text,omitempty"`
	Process               string   `json:"process,omitempty"`
	CostPriceMode         string   `json:"cost_price_mode,omitempty"`
	CostPrice             *float64 `json:"cost_price,omitempty"`
	EstimatedCost         *float64 `json:"estimated_cost,omitempty"`
	RequiresManualReview  bool     `json:"requires_manual_review"`
	ManualCostOverride    bool     `json:"manual_cost_override"`
	FilingStatus          string   `json:"filing_status,omitempty"`
	FilingErrorMessage    string   `json:"filing_error_message,omitempty"`
	ERPSyncRequired       bool     `json:"erp_sync_required"`
	LastFilingAttemptAt   string   `json:"last_filing_attempt_at,omitempty"`
	LastFiledAt           string   `json:"last_filed_at,omitempty"`
	WarehouseRejectReason string   `json:"warehouse_reject_reason,omitempty"`
}

type personEvidence struct {
	Role string `json:"role"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Note string `json:"note,omitempty"`
}

type workflowEvidence struct {
	MainStatus              string           `json:"main_status"`
	Design                  subStatus        `json:"design"`
	Audit                   subStatus        `json:"audit"`
	Procurement             subStatus        `json:"procurement"`
	Warehouse               subStatus        `json:"warehouse"`
	Customization           subStatus        `json:"customization"`
	CanPrepareWarehouse     bool             `json:"can_prepare_warehouse"`
	CanClose                bool             `json:"can_close"`
	WarehouseBlockingReason []reasonEvidence `json:"warehouse_blocking_reasons"`
	CannotCloseReasons      []reasonEvidence `json:"cannot_close_reasons"`
}

type subStatus struct {
	Code   string `json:"code"`
	Label  string `json:"label,omitempty"`
	Source string `json:"source,omitempty"`
}

type reasonEvidence struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type moduleEvidence struct {
	ModuleKey    string `json:"module_key"`
	ModuleName   string `json:"module_name"`
	State        string `json:"state"`
	ClaimedBy    string `json:"claimed_by,omitempty"`
	PoolTeamCode string `json:"pool_team_code,omitempty"`
	EnteredAt    string `json:"entered_at,omitempty"`
	TerminalAt   string `json:"terminal_at,omitempty"`
	Data         string `json:"data,omitempty"`
}

type eventEvidence struct {
	Time    string `json:"time"`
	Stage   string `json:"stage"`
	Type    string `json:"type"`
	Actor   string `json:"actor,omitempty"`
	Summary string `json:"summary,omitempty"`
	Payload string `json:"payload,omitempty"`
}

type costOverrideEvidence struct {
	Time                  string   `json:"time"`
	Type                  string   `json:"type"`
	Actor                 string   `json:"actor,omitempty"`
	CategoryCode          string   `json:"category_code,omitempty"`
	PreviousEstimatedCost *float64 `json:"previous_estimated_cost,omitempty"`
	PreviousCostPrice     *float64 `json:"previous_cost_price,omitempty"`
	OverrideCost          *float64 `json:"override_cost,omitempty"`
	ResultCostPrice       *float64 `json:"result_cost_price,omitempty"`
	Reason                string   `json:"reason,omitempty"`
	Note                  string   `json:"note,omitempty"`
	GovernanceStatus      string   `json:"governance_status,omitempty"`
}

type skuEvidence struct {
	SKUCode                  string   `json:"sku_code"`
	SequenceNo               int      `json:"sequence_no,omitempty"`
	ProductName              string   `json:"product_name,omitempty"`
	ProductID                string   `json:"product_id,omitempty"`
	ERPProductID             string   `json:"erp_product_id,omitempty"`
	SKUStatus                string   `json:"sku_status,omitempty"`
	FilingStatus             string   `json:"filing_status,omitempty"`
	ERPSyncStatus            string   `json:"erp_sync_status,omitempty"`
	FilingErrorMessage       string   `json:"filing_error_message,omitempty"`
	CostPrice                *float64 `json:"cost_price,omitempty"`
	EstimatedCost            *float64 `json:"estimated_cost,omitempty"`
	CostRuleName             string   `json:"cost_rule_name,omitempty"`
	CostRuleSource           string   `json:"cost_rule_source,omitempty"`
	RequiresManualReview     bool     `json:"requires_manual_review"`
	ManualCostOverride       bool     `json:"manual_cost_override"`
	ManualCostOverrideReason string   `json:"manual_cost_override_reason,omitempty"`
}

type assetEvidence struct {
	ID                 int64  `json:"id"`
	AssetID            int64  `json:"asset_id,omitempty"`
	SKU                string `json:"sku,omitempty"`
	Type               string `json:"type"`
	VersionNo          int    `json:"version_no"`
	Filename           string `json:"filename"`
	UploadStatus       string `json:"upload_status,omitempty"`
	PreviewStatus      string `json:"preview_status,omitempty"`
	FlowReviewStatus   string `json:"flow_review_status,omitempty"`
	UsableState        string `json:"usable_state,omitempty"`
	WarehouseReady     bool   `json:"warehouse_ready"`
	ApprovedForFlow    bool   `json:"approved_for_flow"`
	UploadedBy         string `json:"uploaded_by,omitempty"`
	UploadedAt         string `json:"uploaded_at,omitempty"`
	Remark             string `json:"remark,omitempty"`
	CurrentVersionRole string `json:"current_version_role,omitempty"`
}

type referenceEvidence struct {
	RefID    string `json:"ref_id"`
	Filename string `json:"filename,omitempty"`
	Source   string `json:"source,omitempty"`
	Status   string `json:"status,omitempty"`
}

func buildEvidence(detail *task_aggregator.Detail, taskEvents []*domain.TaskEvent, costEvents []*domain.TaskCostOverrideAuditEvent, notes []string) evidence {
	task := detail.Task
	var taskDetail *domain.TaskDetail
	if detail != nil {
		taskDetail = detail.TaskDetail
	}
	out := evidence{
		GeneratedAt:        time.Now().Format(time.RFC3339),
		Task:               buildTaskEvidence(task, taskDetail),
		People:             buildPeopleEvidence(detail),
		Workflow:           buildWorkflowEvidence(detail),
		Modules:            buildModuleEvidence(detail),
		RecentEvents:       buildEventEvidence(detail, taskEvents),
		CostOverrideEvents: buildCostOverrideEvidence(costEvents),
		SKUs:               buildSKUEvidence(detail),
		Assets:             buildAssetEvidence(detail),
		References:         buildReferenceEvidence(detail),
		Notes:              notes,
	}
	return out
}

func buildTaskEvidence(task *domain.Task, detail *domain.TaskDetail) taskEvidence {
	if task == nil {
		return taskEvidence{}
	}
	out := taskEvidence{
		ID:                    task.ID,
		TaskNo:                task.TaskNo,
		TaskType:              string(task.TaskType),
		SourceMode:            string(task.SourceMode),
		ProductName:           task.ProductNameSnapshot,
		PrimarySKU:            firstNonEmpty(task.PrimarySKUCode, task.SKUCode),
		Status:                string(task.TaskStatus),
		Priority:              string(task.Priority),
		BusinessLane:          string(task.BusinessLane),
		OwnerDepartment:       task.OwnerDepartment,
		OwnerTeam:             firstNonEmpty(task.OwnerOrgTeam, task.OwnerTeam),
		CreatedAt:             timeString(&task.CreatedAt),
		UpdatedAt:             timeString(&task.UpdatedAt),
		ERPSyncRequired:       false,
		WarehouseRejectReason: truncateText(task.WarehouseRejectReason, 300),
	}
	out.DeadlineAt = timeString(task.DeadlineAt)
	if detail == nil {
		return out
	}
	out.DemandText = truncateText(detail.DemandText, 220)
	out.ChangeRequest = truncateText(detail.ChangeRequest, 160)
	out.DesignRequirement = truncateText(detail.DesignRequirement, 220)
	out.Category = firstNonEmpty(detail.CategoryName, detail.Category, detail.CategoryCode)
	out.CategoryCode = detail.CategoryCode
	out.Material = firstNonEmpty(detail.Material, detail.MaterialMode)
	out.SpecText = truncateText(detail.SpecText, 120)
	out.Process = truncateText(detail.Process, 120)
	out.CostPriceMode = detail.CostPriceMode
	out.CostPrice = detail.CostPrice
	out.EstimatedCost = detail.EstimatedCost
	out.RequiresManualReview = detail.RequiresManualReview
	out.ManualCostOverride = detail.ManualCostOverride
	out.FilingStatus = string(detail.FilingStatus)
	out.FilingErrorMessage = truncateText(detail.FilingErrorMessage, 220)
	out.ERPSyncRequired = detail.ERPSyncRequired
	out.LastFilingAttemptAt = timeString(detail.LastFilingAttemptAt)
	out.LastFiledAt = timeString(firstTime(detail.LastFiledAt, detail.FiledAt))
	return out
}

func buildPeopleEvidence(detail *task_aggregator.Detail) []personEvidence {
	if detail == nil || detail.Task == nil {
		return []personEvidence{}
	}
	task := detail.Task
	people := make([]personEvidence, 0, 5)
	people = appendPerson(people, "发起人", int64Ptr(task.CreatorID), detail.CreatorName, "任务创建人")
	people = appendPerson(people, "需求人", task.RequesterID, detail.RequesterName, "业务需求提出人")
	people = appendPerson(people, "设计", task.DesignerID, detail.DesignerName, "设计执行人")
	people = appendPerson(people, "当前处理人", task.CurrentHandlerID, detail.CurrentHandlerName, "系统当前指向的处理人")
	people = appendPerson(people, "任务负责人", detail.AssigneeID, detail.AssigneeName, "聚合读模型中的承接人")
	return people
}

func appendPerson(people []personEvidence, role string, id *int64, name string, note string) []personEvidence {
	if id == nil && strings.TrimSpace(name) == "" {
		return append(people, personEvidence{Role: role, Name: "系统暂无记录", Note: note})
	}
	out := personEvidence{Role: role, Name: firstNonEmpty(name, "系统暂无记录"), Note: note}
	if id != nil {
		out.ID = strconv.FormatInt(*id, 10)
	}
	return append(people, out)
}

func buildWorkflowEvidence(detail *task_aggregator.Detail) workflowEvidence {
	if detail == nil {
		return workflowEvidence{}
	}
	workflow := detail.Workflow
	return workflowEvidence{
		MainStatus:              string(workflow.MainStatus),
		Design:                  buildSubStatus(workflow.SubStatus.Design),
		Audit:                   buildSubStatus(workflow.SubStatus.Audit),
		Procurement:             buildSubStatus(workflow.SubStatus.Procurement),
		Warehouse:               buildSubStatus(workflow.SubStatus.Warehouse),
		Customization:           buildSubStatus(workflow.SubStatus.Customization),
		CanPrepareWarehouse:     workflow.CanPrepareWarehouse,
		CanClose:                workflow.CanClose || workflow.Closable,
		WarehouseBlockingReason: buildReasons(workflow.WarehouseBlockingReasons),
		CannotCloseReasons:      buildReasons(workflow.CannotCloseReasons),
	}
}

func buildSubStatus(item domain.TaskSubStatusItem) subStatus {
	return subStatus{Code: string(item.Code), Label: item.Label, Source: string(item.Source)}
}

func buildReasons(reasons []domain.WorkflowReason) []reasonEvidence {
	out := make([]reasonEvidence, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, reasonEvidence{Code: string(reason.Code), Message: reason.Message})
	}
	return out
}

func buildModuleEvidence(detail *task_aggregator.Detail) []moduleEvidence {
	if detail == nil {
		return []moduleEvidence{}
	}
	out := make([]moduleEvidence, 0, len(detail.Modules))
	for _, module := range detail.Modules {
		if module.TaskModule == nil {
			continue
		}
		m := module.TaskModule
		row := moduleEvidence{
			ModuleKey:    m.ModuleKey,
			ModuleName:   moduleLabel(m.ModuleKey),
			State:        string(m.State),
			ClaimedBy:    int64String(m.ClaimedBy),
			PoolTeamCode: stringValue(m.PoolTeamCode),
			EnteredAt:    timeString(&m.EnteredAt),
			TerminalAt:   timeString(m.TerminalAt),
		}
		out = append(out, row)
	}
	return out
}

func buildEventEvidence(detail *task_aggregator.Detail, taskEvents []*domain.TaskEvent) []eventEvidence {
	out := make([]eventEvidence, 0, len(taskEvents)+20)
	for _, event := range taskEvents {
		if event == nil {
			continue
		}
		actor := firstNonEmpty(event.OperatorName, event.CreatorName)
		if actor == "" {
			actor = int64String(firstInt64(event.OperatorID, event.CreatorID))
		}
		out = append(out, eventEvidence{
			Time:    timeString(&event.CreatedAt),
			Stage:   "任务事件",
			Type:    event.EventType,
			Actor:   actor,
			Summary: taskEventSummary(event.EventType, event.Payload),
			Payload: compactJSON(event.Payload, 180),
		})
	}
	if detail != nil {
		for _, event := range detail.Events {
			if event == nil {
				continue
			}
			actor := actorNameFromSnapshot(event.ActorSnapshot)
			if actor == "" {
				actor = int64String(event.ActorID)
			}
			out = append(out, eventEvidence{
				Time:    timeString(&event.CreatedAt),
				Stage:   moduleLabel(event.ModuleKey),
				Type:    string(event.EventType),
				Actor:   actor,
				Summary: moduleEventSummary(event),
				Payload: compactJSON(event.Payload, 180),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Time > out[j].Time
	})
	if len(out) > 12 {
		return out[:12]
	}
	return out
}

func buildCostOverrideEvidence(events []*domain.TaskCostOverrideAuditEvent) []costOverrideEvidence {
	out := make([]costOverrideEvidence, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		out = append(out, costOverrideEvidence{
			Time:                  timeString(&event.OverrideAt),
			Type:                  string(event.EventType),
			Actor:                 event.OverrideActor,
			CategoryCode:          event.CategoryCode,
			PreviousEstimatedCost: event.PreviousEstimatedCost,
			PreviousCostPrice:     event.PreviousCostPrice,
			OverrideCost:          event.OverrideCost,
			ResultCostPrice:       event.ResultCostPrice,
			Reason:                truncateText(event.OverrideReason, 120),
			Note:                  truncateText(event.Note, 120),
			GovernanceStatus:      string(event.GovernanceStatus),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	if len(out) > 6 {
		return out[:6]
	}
	return out
}

func buildSKUEvidence(detail *task_aggregator.Detail) []skuEvidence {
	if detail == nil || detail.Task == nil {
		return []skuEvidence{}
	}
	out := make([]skuEvidence, 0, len(detail.SKUItems)+1)
	for _, item := range detail.SKUItems {
		if item == nil {
			continue
		}
		out = append(out, skuEvidence{
			SKUCode:                  item.SKUCode,
			SequenceNo:               item.SequenceNo,
			ProductName:              item.ProductNameSnapshot,
			ProductID:                int64String(item.ProductID),
			ERPProductID:             stringValue(item.ERPProductID),
			SKUStatus:                string(item.SKUStatus),
			FilingStatus:             string(item.FilingStatus),
			ERPSyncStatus:            string(item.ERPSyncStatus),
			FilingErrorMessage:       truncateText(item.FilingErrorMessage, 180),
			CostPrice:                item.CostPrice,
			EstimatedCost:            item.EstimatedCost,
			CostRuleName:             item.CostRuleName,
			CostRuleSource:           item.CostRuleSource,
			RequiresManualReview:     item.RequiresManualReview,
			ManualCostOverride:       item.ManualCostOverride,
			ManualCostOverrideReason: truncateText(item.ManualCostOverrideReason, 120),
		})
	}
	if len(out) > 8 {
		out = out[:8]
	}
	if len(out) == 0 && strings.TrimSpace(detail.Task.SKUCode) != "" {
		out = append(out, skuEvidence{
			SKUCode:     detail.Task.SKUCode,
			ProductName: detail.Task.ProductNameSnapshot,
		})
	}
	return out
}

func buildAssetEvidence(detail *task_aggregator.Detail) []assetEvidence {
	if detail == nil {
		return []assetEvidence{}
	}
	out := make([]assetEvidence, 0, len(detail.AssetVersions))
	for _, version := range detail.AssetVersions {
		if version == nil {
			continue
		}
		out = append(out, assetEvidence{
			ID:                 version.ID,
			AssetID:            version.AssetID,
			SKU:                version.ScopeSKUCode,
			Type:               string(version.AssetType),
			VersionNo:          version.VersionNo,
			Filename:           truncateText(version.OriginalFilename, 120),
			UploadStatus:       string(version.UploadStatus),
			PreviewStatus:      string(version.PreviewStatus),
			FlowReviewStatus:   string(version.FlowReviewStatus),
			UsableState:        string(version.UsableState),
			WarehouseReady:     version.WarehouseReady,
			ApprovedForFlow:    version.ApprovedForFlow,
			UploadedBy:         firstNonEmpty(version.UploadedByName, strconv.FormatInt(version.UploadedBy, 10)),
			UploadedAt:         timeString(version.UploadedAt),
			Remark:             truncateText(version.Remark, 120),
			CurrentVersionRole: version.CurrentVersionRole,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UploadedAt == out[j].UploadedAt {
			return out[i].ID > out[j].ID
		}
		return out[i].UploadedAt > out[j].UploadedAt
	})
	if len(out) > 8 {
		return out[:8]
	}
	return out
}

func buildReferenceEvidence(detail *task_aggregator.Detail) []referenceEvidence {
	if detail == nil {
		return []referenceEvidence{}
	}
	out := make([]referenceEvidence, 0, len(detail.References))
	for _, ref := range detail.References {
		row := referenceEvidence{
			RefID:    firstNonEmpty(ref.RefID, ref.AssetID),
			Filename: ref.Filename,
			Source:   ref.Source,
			Status:   ref.Status,
		}
		out = append(out, row)
	}
	if len(out) > 8 {
		return out[:8]
	}
	return out
}

func taskEventSummary(eventType string, payload json.RawMessage) string {
	if eventType == "" {
		return ""
	}
	switch eventType {
	case "task_created":
		return "任务创建"
	case "task_assigned":
		return "任务指派"
	case "design_submitted":
		return "设计提交"
	case "audit_approved":
		return "审核通过"
	case "audit_rejected":
		return "审核打回"
	case "warehouse_received":
		return "仓库接收"
	case "task_closed":
		return "任务关闭"
	default:
		if payloadSummary := compactJSON(payload, 180); payloadSummary != "" {
			return eventType + "：" + payloadSummary
		}
		return eventType
	}
}

func moduleEventSummary(event *domain.TaskModuleEvent) string {
	if event == nil {
		return ""
	}
	from := ""
	to := ""
	if event.FromState != nil {
		from = string(*event.FromState)
	}
	if event.ToState != nil {
		to = string(*event.ToState)
	}
	action := string(event.EventType)
	if from != "" || to != "" {
		return fmt.Sprintf("%s 状态 %s -> %s", action, firstNonEmpty(from, "空"), firstNonEmpty(to, "空"))
	}
	return action
}

func moduleLabel(key string) string {
	switch key {
	case domain.ModuleKeyBasicInfo:
		return "基础信息"
	case domain.ModuleKeyDesign:
		return "设计"
	case domain.ModuleKeyAudit:
		return "审核"
	case domain.ModuleKeyWarehouse:
		return "仓库"
	case domain.ModuleKeyCustomization:
		return "定制"
	case domain.ModuleKeyProcurement:
		return "采购"
	case domain.ModuleKeyRetouch:
		return "P图"
	case "":
		return "任务"
	default:
		return key
	}
}

func actorNameFromSnapshot(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, key := range []string{"name", "display_name", "displayName", "username", "real_name", "nickname"} {
		if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func compactJSON(raw json.RawMessage, limit int) string {
	if len(raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return truncateText(string(raw), limit)
	}
	return truncateText(buf.String(), limit)
}

func truncateText(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

func timeString(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func firstTime(times ...*time.Time) *time.Time {
	for _, t := range times {
		if t != nil && !t.IsZero() {
			return t
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func int64Ptr(v int64) *int64 {
	return &v
}

func firstInt64(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func int64String(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}
