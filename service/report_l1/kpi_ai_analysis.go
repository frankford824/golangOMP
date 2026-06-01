package report_l1

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/aiagent"
)

const CodeAIAnalysisNotConfigured = "kpi_ai_analysis_not_configured"

type KPIAnalysisGenerator interface {
	GenerateKPIAnalysis(ctx context.Context, evidence any) (*aiagent.KPIAnalysis, error)
}

type KPIAIAnalysisParams struct {
	From time.Time
	To   time.Time
}

type kpiAnalysisEvidence struct {
	Period       kpiAnalysisPeriod       `json:"period"`
	Metrics      kpiAnalysisMetrics      `json:"metrics"`
	People       []kpiAnalysisPerson     `json:"people"`
	TaskSamples  []kpiAnalysisTaskSample `json:"task_samples"`
	RecentAssets []kpiAnalysisAsset      `json:"recent_assets"`
	Evidence     []string                `json:"evidence"`
	GeneratedAt  time.Time               `json:"generated_at"`
}

type kpiAnalysisPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type kpiAnalysisMetrics struct {
	TaskCreates    int `json:"task_creates"`
	DesignClaims   int `json:"design_claims"`
	DesignSubmits  int `json:"design_submits"`
	AuditApproves  int `json:"audit_approves"`
	AuditRejects   int `json:"audit_rejects"`
	FinalAssets    int `json:"final_assets"`
	ReferenceFiles int `json:"reference_files"`
}

type kpiAnalysisPerson struct {
	Name          string `json:"name"`
	Role          string `json:"role"`
	Department    string `json:"department,omitempty"`
	Team          string `json:"team,omitempty"`
	TaskCreates   int    `json:"task_creates,omitempty"`
	DesignClaims  int    `json:"design_claims,omitempty"`
	DesignSubmits int    `json:"design_submits,omitempty"`
	AuditApproves int    `json:"audit_approves,omitempty"`
	AuditRejects  int    `json:"audit_rejects,omitempty"`
	FinalAssets   int    `json:"final_assets,omitempty"`
}

type kpiAnalysisTaskSample struct {
	TaskID       int64    `json:"task_id"`
	TaskNo       string   `json:"task_no"`
	TaskName     string   `json:"task_name,omitempty"`
	TaskType     string   `json:"task_type,omitempty"`
	BusinessLane string   `json:"business_lane,omitempty"`
	CategoryName string   `json:"category_name,omitempty"`
	Timeline     []string `json:"timeline"`
	Assets       []string `json:"assets,omitempty"`
}

type kpiAnalysisAsset struct {
	Time     string `json:"time"`
	TaskNo   string `json:"task_no"`
	Uploader string `json:"uploader,omitempty"`
	Type     string `json:"type"`
	FileName string `json:"file_name,omitempty"`
}

type personAccumulator struct {
	kpiAnalysisPerson
	roles map[string]struct{}
}

type taskAccumulator struct {
	kpiAnalysisTaskSample
	lastAt time.Time
}

func (s *Service) KPIAIAnalysis(ctx context.Context, actor domain.RequestActor, params KPIAIAnalysisParams) (*aiagent.KPIAnalysis, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/l1/kpi-ai-analysis"); err != nil {
		return nil, err
	}
	if params.From.IsZero() || params.To.IsZero() || params.From.After(params.To) {
		return nil, domain.NewAppError(CodeInvalidDateRange, "from must be before or equal to to", nil)
	}
	if s.kpiAnalysisRepo == nil {
		return nil, domain.NewAppError(CodeAIAnalysisNotConfigured, "AI 绩效分析服务尚未配置", nil)
	}

	filter := repo.KPIAnalysisFilter{
		From:  params.From,
		To:    params.To.AddDate(0, 0, 1),
		Limit: 260,
	}
	events, err := s.kpiAnalysisRepo.ListTaskEvents(ctx, filter)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	assets, err := s.kpiAnalysisRepo.ListTaskAssets(ctx, repo.KPIAnalysisFilter{From: filter.From, To: filter.To, Limit: 140})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}

	evidence := buildKPIAnalysisEvidence(params.From, params.To, events, assets)
	if s.kpiAnalysisGenerator == nil {
		return fallbackKPIAnalysis(evidence, nil), nil
	}
	analysis, err := s.kpiAnalysisGenerator.GenerateKPIAnalysis(ctx, evidence)
	if err != nil || analysis == nil {
		return fallbackKPIAnalysis(evidence, err), nil
	}
	if analysis.Evidence == nil {
		analysis.Evidence = []string{}
	}
	return analysis, nil
}

func fallbackKPIAnalysis(evidence kpiAnalysisEvidence, cause error) *aiagent.KPIAnalysis {
	metrics := evidence.Metrics
	overview := fmt.Sprintf("%s 至 %s：系统记录任务创建 %d 条，设计接单 %d 次，设计提交 %d 次，审核通过 %d 次，审核打回 %d 次，最终成品图 %d 个。AI 深度解读暂时不可用，当前先展示系统统计分析。",
		evidence.Period.From,
		evidence.Period.To,
		metrics.TaskCreates,
		metrics.DesignClaims,
		metrics.DesignSubmits,
		metrics.AuditApproves,
		metrics.AuditRejects,
		metrics.FinalAssets,
	)
	highlights := []aiagent.KPIAnalysisHighlight{
		{Title: "运营任务创建", Value: fmt.Sprintf("%d", metrics.TaskCreates), Note: "按任务创建和批量子项创建动作统计"},
		{Title: "设计接单", Value: fmt.Sprintf("%d", metrics.DesignClaims), Note: "按指派、改派和批量指派动作统计"},
		{Title: "设计提交", Value: fmt.Sprintf("%d", metrics.DesignSubmits), Note: "按设计提交动作统计"},
		{Title: "审核通过", Value: fmt.Sprintf("%d", metrics.AuditApproves), Note: fmt.Sprintf("审核通过率 %s", fallbackRate(metrics.AuditApproves, metrics.AuditApproves+metrics.AuditRejects))},
		{Title: "审核打回", Value: fmt.Sprintf("%d", metrics.AuditRejects), Note: "用于观察返工压力"},
		{Title: "最终成品图", Value: fmt.Sprintf("%d", metrics.FinalAssets), Note: "按最终成品图资产记录统计"},
	}

	lines := append([]string{}, evidence.Evidence...)
	lines = append(lines, "AI 深度解读暂时不可用，已切换为系统统计分析。")
	if cause != nil && strings.TrimSpace(cause.Error()) != "" {
		lines = append(lines, "本次未使用 AI 文本结论，避免把临时模型异常展示给一线用户。")
	}

	return &aiagent.KPIAnalysis{
		Headline:       "绩效统计已生成，AI 深度解读暂时不可用",
		Overview:       overview,
		Highlights:     highlights,
		PeopleInsights: fallbackPeopleInsights(evidence.People),
		TaskSamples:    fallbackTaskSamples(evidence.TaskSamples),
		Risks:          fallbackKPIRisks(metrics),
		Actions:        fallbackKPIActions(metrics),
		Evidence:       lines,
		Confidence:     "medium",
		GeneratedAt:    time.Now().UTC(),
		Model:          "system",
		Provider:       "system_fallback",
	}
}

func fallbackRate(numerator, denominator int) string {
	if denominator <= 0 {
		return "暂无数据"
	}
	return fmt.Sprintf("%.1f%%", float64(numerator)*100/float64(denominator))
}

func fallbackPeopleInsights(people []kpiAnalysisPerson) []aiagent.KPIAnalysisPersonInsight {
	out := make([]aiagent.KPIAnalysisPersonInsight, 0, min(len(people), 5))
	for _, person := range people {
		if len(out) >= 5 {
			break
		}
		total := person.TaskCreates + person.DesignClaims + person.DesignSubmits + person.AuditApproves + person.AuditRejects + person.FinalAssets
		if total <= 0 {
			continue
		}
		metric := fmt.Sprintf("创建%d，接单%d，提交%d，通过%d，打回%d，成品图%d",
			person.TaskCreates,
			person.DesignClaims,
			person.DesignSubmits,
			person.AuditApproves,
			person.AuditRejects,
			person.FinalAssets,
		)
		signal := "本周期有可追踪业务动作"
		if person.AuditRejects > 0 {
			signal = "存在审核打回记录，建议复盘打回原因"
		} else if person.DesignSubmits > 0 {
			signal = "有设计提交产出"
		} else if person.TaskCreates > 0 {
			signal = "有任务发起动作"
		}
		out = append(out, aiagent.KPIAnalysisPersonInsight{
			Role:   person.Role,
			Name:   person.Name,
			Metric: metric,
			Signal: signal,
			Action: "结合任务明细继续核对异常单据",
		})
	}
	return out
}

func fallbackTaskSamples(tasks []kpiAnalysisTaskSample) []aiagent.KPIAnalysisTaskSample {
	out := make([]aiagent.KPIAnalysisTaskSample, 0, min(len(tasks), 4))
	for _, task := range tasks {
		if len(out) >= 4 {
			break
		}
		timeline := append([]string{}, task.Timeline...)
		if len(timeline) == 0 && len(task.Assets) > 0 {
			timeline = append(timeline, task.Assets...)
		}
		if len(timeline) > 4 {
			timeline = timeline[len(timeline)-4:]
		}
		observation := "该任务有近期可追踪动作。"
		if len(task.Assets) > 0 {
			observation = "该任务包含近期设计资产记录，可作为抽查样本。"
		}
		out = append(out, aiagent.KPIAnalysisTaskSample{
			TaskNo:      task.TaskNo,
			TaskName:    task.TaskName,
			TaskType:    task.TaskType,
			Timeline:    timeline,
			Observation: observation,
		})
	}
	return out
}

func fallbackKPIRisks(metrics kpiAnalysisMetrics) []aiagent.KPIAnalysisRisk {
	risks := []aiagent.KPIAnalysisRisk{}
	if metrics.AuditRejects > 0 {
		risks = append(risks, aiagent.KPIAnalysisRisk{
			Level:  "medium",
			Title:  "存在审核打回",
			Reason: fmt.Sprintf("本周期审核打回 %d 次，需要按人员和任务复盘返工原因。", metrics.AuditRejects),
		})
	}
	if metrics.DesignClaims > 0 && metrics.DesignSubmits < metrics.DesignClaims {
		risks = append(risks, aiagent.KPIAnalysisRisk{
			Level:  "low",
			Title:  "设计接单与提交存在差额",
			Reason: fmt.Sprintf("接单 %d 次，提交 %d 次，可能包含进行中任务或未及时提交的任务。", metrics.DesignClaims, metrics.DesignSubmits),
		})
	}
	if metrics.FinalAssets == 0 && metrics.DesignSubmits > 0 {
		risks = append(risks, aiagent.KPIAnalysisRisk{
			Level:  "low",
			Title:  "成品图记录不足",
			Reason: "已有设计提交，但最终成品图资产统计为 0，需要核对上传入口和资产归类。",
		})
	}
	if len(risks) == 0 {
		risks = append(risks, aiagent.KPIAnalysisRisk{
			Level:  "low",
			Title:  "暂无明显异常",
			Reason: "系统统计未发现突出的打回或资产缺口。",
		})
	}
	return risks
}

func fallbackKPIActions(metrics kpiAnalysisMetrics) []aiagent.KPIAnalysisAction {
	actions := []aiagent.KPIAnalysisAction{
		{Owner: "运营主管", Action: "查看任务创建量和平均发起间隔，确认任务分配是否集中在少数人员。", Timing: "当天"},
		{Owner: "设计主管", Action: "按接单未提交和审核打回筛选任务，优先处理影响交付的单据。", Timing: "当天"},
		{Owner: "审核主管", Action: "复盘打回样本，沉淀常见问题口径。", Timing: "本周期"},
	}
	if metrics.AuditRejects == 0 {
		actions[2].Action = "抽查审核通过任务和最终成品图，确认可用状态标记准确。"
	}
	return actions
}

func buildKPIAnalysisEvidence(from, to time.Time, events []domain.KPIAnalysisEvent, assets []domain.KPIAnalysisAsset) kpiAnalysisEvidence {
	sort.Slice(events, func(i, j int) bool { return events[i].CreatedAt.Before(events[j].CreatedAt) })
	sort.Slice(assets, func(i, j int) bool { return assets[i].CreatedAt.After(assets[j].CreatedAt) })

	people := map[string]*personAccumulator{}
	tasks := map[int64]*taskAccumulator{}
	metrics := kpiAnalysisMetrics{}

	for _, event := range events {
		personName, department, team := eventActor(event)
		role := eventRole(event.EventType)
		person := ensurePerson(people, personName, role, department, team)
		task := ensureTask(tasks, event)
		line := fmt.Sprintf("%s %s %s", formatLocalDateTime(event.CreatedAt), personName, eventLabel(event.EventType))
		task.Timeline = append(task.Timeline, line)
		task.lastAt = event.CreatedAt

		switch event.EventType {
		case "task.created", "task.batch_items_created":
			metrics.TaskCreates++
			person.TaskCreates++
		case "task.assigned", "task.reassigned", "task.batch_assigned":
			metrics.DesignClaims++
			person.DesignClaims++
		case "task.design.submitted":
			metrics.DesignSubmits++
			person.DesignSubmits++
		case "task.audit.approved":
			metrics.AuditApproves++
			person.AuditApproves++
		case "task.audit.rejected":
			metrics.AuditRejects++
			person.AuditRejects++
		}
	}

	recentAssets := make([]kpiAnalysisAsset, 0, min(len(assets), 15))
	for _, asset := range assets {
		if asset.AssetType == "final" {
			metrics.FinalAssets++
		}
		if asset.AssetType == "reference" {
			metrics.ReferenceFiles++
		}
		uploader := strings.TrimSpace(asset.UploadedByName)
		if uploader == "" && asset.UploadedBy > 0 {
			uploader = fmt.Sprintf("人员#%d", asset.UploadedBy)
		}
		person := ensurePerson(people, uploader, "设计", "", "")
		if asset.AssetType == "final" {
			person.FinalAssets++
		}
		if task := ensureTaskFromAsset(tasks, asset); task != nil {
			name := firstNonEmpty(asset.OriginalName, asset.FileName)
			task.Assets = append(task.Assets, fmt.Sprintf("%s %s 上传%s %s", formatLocalDateTime(asset.CreatedAt), uploader, assetTypeLabel(asset.AssetType), name))
			if asset.CreatedAt.After(task.lastAt) {
				task.lastAt = asset.CreatedAt
			}
		}
		if len(recentAssets) < 15 {
			recentAssets = append(recentAssets, kpiAnalysisAsset{
				Time:     formatLocalDateTime(asset.CreatedAt),
				TaskNo:   asset.TaskNo,
				Uploader: uploader,
				Type:     assetTypeLabel(asset.AssetType),
				FileName: firstNonEmpty(asset.OriginalName, asset.FileName),
			})
		}
	}

	return kpiAnalysisEvidence{
		Period: kpiAnalysisPeriod{
			From: from.Format("2006-01-02"),
			To:   to.Format("2006-01-02"),
		},
		Metrics:      metrics,
		People:       summarizePeople(people),
		TaskSamples:  summarizeTasks(tasks),
		RecentAssets: recentAssets,
		Evidence: []string{
			fmt.Sprintf("%s 至 %s 任务关键动作 %d 条", from.Format("2006-01-02"), to.Format("2006-01-02"), len(events)),
			fmt.Sprintf("设计相关资产记录 %d 条，其中最终成品图 %d 条", len(assets), metrics.FinalAssets),
			fmt.Sprintf("创建 %d 条、设计提交 %d 条、审核通过 %d 条、审核打回 %d 条", metrics.TaskCreates, metrics.DesignSubmits, metrics.AuditApproves, metrics.AuditRejects),
		},
		GeneratedAt: time.Now().UTC(),
	}
}

func ensurePerson(people map[string]*personAccumulator, name, role, department, team string) *personAccumulator {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未知人员"
	}
	key := strings.ToLower(name)
	item := people[key]
	if item == nil {
		item = &personAccumulator{roles: map[string]struct{}{}}
		item.Name = name
		item.Department = department
		item.Team = team
		people[key] = item
	}
	if item.Department == "" {
		item.Department = department
	}
	if item.Team == "" {
		item.Team = team
	}
	if role != "" {
		item.roles[role] = struct{}{}
		item.Role = joinRoles(item.roles)
	}
	return item
}

func ensureTask(tasks map[int64]*taskAccumulator, event domain.KPIAnalysisEvent) *taskAccumulator {
	item := tasks[event.TaskID]
	if item == nil {
		item = &taskAccumulator{}
		item.TaskID = event.TaskID
		item.TaskNo = event.TaskNo
		item.TaskName = event.ProductName
		item.TaskType = event.TaskType
		item.BusinessLane = event.BusinessLane
		item.CategoryName = event.CategoryName
		tasks[event.TaskID] = item
	}
	return item
}

func ensureTaskFromAsset(tasks map[int64]*taskAccumulator, asset domain.KPIAnalysisAsset) *taskAccumulator {
	item := tasks[asset.TaskID]
	if item == nil {
		item = &taskAccumulator{}
		item.TaskID = asset.TaskID
		item.TaskNo = asset.TaskNo
		item.TaskName = asset.ProductName
		item.TaskType = asset.TaskType
		item.BusinessLane = asset.BusinessLane
		tasks[asset.TaskID] = item
	}
	return item
}

func summarizePeople(people map[string]*personAccumulator) []kpiAnalysisPerson {
	out := make([]kpiAnalysisPerson, 0, len(people))
	for _, item := range people {
		if item.Name == "" || item.Name == "未知人员" {
			continue
		}
		out = append(out, item.kpiAnalysisPerson)
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].TaskCreates + out[i].DesignSubmits + out[i].AuditApproves + out[i].AuditRejects + out[i].FinalAssets
		right := out[j].TaskCreates + out[j].DesignSubmits + out[j].AuditApproves + out[j].AuditRejects + out[j].FinalAssets
		if left != right {
			return left > right
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 25 {
		return out[:25]
	}
	return out
}

func summarizeTasks(tasks map[int64]*taskAccumulator) []kpiAnalysisTaskSample {
	items := make([]*taskAccumulator, 0, len(tasks))
	for _, item := range tasks {
		if len(item.Timeline) == 0 && len(item.Assets) == 0 {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].lastAt.After(items[j].lastAt) })

	out := make([]kpiAnalysisTaskSample, 0, min(len(items), 12))
	for _, item := range items {
		if len(out) >= 12 {
			break
		}
		task := item.kpiAnalysisTaskSample
		if len(task.Timeline) > 5 {
			task.Timeline = task.Timeline[len(task.Timeline)-5:]
		}
		if len(task.Assets) > 3 {
			task.Assets = task.Assets[:3]
		}
		out = append(out, task)
	}
	return out
}

func eventActor(event domain.KPIAnalysisEvent) (string, string, string) {
	payload := rawPayloadMap(event.Payload)
	if event.EventType == "task.assigned" || event.EventType == "task.reassigned" || event.EventType == "task.batch_assigned" {
		name := firstNonEmpty(
			textValue(payload, "to_handler_name"),
			textValue(payload, "assignee_name"),
			textValue(payload, "designer_name"),
			idLabelValue(payload, "to_handler_id"),
			idLabelValue(payload, "assignee_id"),
			idLabelValue(payload, "designer_id"),
			event.OperatorName,
		)
		return name, event.OperatorDepartment, event.OperatorTeam
	}
	name := firstNonEmpty(
		textValue(payload, "actor_real_name"),
		textValue(payload, "operator_real_name"),
		textValue(payload, "creator_real_name"),
		textValue(payload, "designer_real_name"),
		textValue(payload, "auditor_real_name"),
		textValue(payload, "to_handler_name"),
		textValue(payload, "assignee_name"),
		textValue(payload, "designer_name"),
		textValue(payload, "creator_name"),
		textValue(payload, "auditor_name"),
		event.OperatorName,
	)
	if name == "" && event.OperatorID != nil {
		name = fmt.Sprintf("人员#%d", *event.OperatorID)
	}
	return name, event.OperatorDepartment, event.OperatorTeam
}

func rawPayloadMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func textValue(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(payload[key]))
}

func idLabelValue(payload map[string]any, key string) string {
	raw := textValue(payload, key)
	raw = strings.TrimSpace(strings.Trim(raw, `"`))
	if raw == "" || raw == "<nil>" || raw == "0" {
		return ""
	}
	return "人员#" + raw
}

func eventRole(eventType string) string {
	switch eventType {
	case "task.created", "task.batch_items_created":
		return "运营"
	case "task.assigned", "task.reassigned", "task.batch_assigned", "task.design.submitted":
		return "设计"
	case "task.audit.approved", "task.audit.rejected":
		return "审核"
	default:
		return ""
	}
}

func eventLabel(eventType string) string {
	switch eventType {
	case "task.created":
		return "创建任务"
	case "task.batch_items_created":
		return "创建批量子项"
	case "task.assigned", "task.reassigned", "task.batch_assigned":
		return "接收或被指派设计任务"
	case "task.design.submitted":
		return "提交设计成品"
	case "task.audit.approved":
		return "审核通过"
	case "task.audit.rejected":
		return "审核打回"
	default:
		return eventType
	}
}

func assetTypeLabel(assetType string) string {
	switch assetType {
	case "reference":
		return "参考图"
	case "draft":
		return "设计稿"
	case "revised":
		return "返修稿"
	case "final":
		return "最终成品图"
	case "outsource_return":
		return "外包回稿"
	default:
		return assetType
	}
}

func joinRoles(roles map[string]struct{}) string {
	order := []string{"运营", "设计", "审核"}
	var out []string
	for _, role := range order {
		if _, ok := roles[role]; ok {
			out = append(out, role)
		}
	}
	return strings.Join(out, "/")
}

func formatLocalDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}
