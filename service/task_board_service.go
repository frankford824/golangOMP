package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type TaskBoardService interface {
	GetOperationalOverview(ctx context.Context) (*domain.TaskOperationalOverview, *domain.AppError)
	GetDesignDepartmentDashboard(ctx context.Context, filter domain.DesignDepartmentDashboardFilter) (*domain.DesignDepartmentDashboard, *domain.AppError)
}

func (s *taskBoardService) GetDesignDepartmentDashboard(ctx context.Context, filter domain.DesignDepartmentDashboardFilter) (*domain.DesignDepartmentDashboard, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || actor.DepartmentID == nil || *actor.DepartmentID <= 0 || actor.EffectiveAccess == nil ||
		!actor.EffectiveAccess.Has(domain.PermissionReportView) || !domain.ActorHasAnyRole(actor, []domain.Role{domain.RoleDeptAdmin}) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "design department dashboard requires department administrator report access", map[string]interface{}{
			"deny_code": "design_department_dashboard_access_denied",
		})
	}
	if filter.StartAt.IsZero() || filter.EndAt.IsZero() || !filter.StartAt.Before(filter.EndAt) || filter.EndAt.Sub(filter.StartAt) > 367*24*time.Hour {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "dashboard date range must be between 1 and 366 days", nil)
	}
	if !filter.DateBasis.Valid() || !filter.Granularity.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid dashboard date basis or granularity", nil)
	}
	filter.DepartmentID = *actor.DepartmentID
	members, err := s.dashboardRepo.ListDesignDepartmentMembers(ctx, filter.DepartmentID)
	if err != nil {
		return nil, infraError("list design department dashboard members", err)
	}
	facts, err := s.dashboardRepo.ListDesignDepartmentTaskFacts(ctx, filter)
	if err != nil {
		return nil, infraError("list design department dashboard facts", err)
	}
	return aggregateDesignDepartmentDashboard(actor.Department, filter, members, facts, s.nowFn().UTC()), nil
}

func aggregateDesignDepartmentDashboard(departmentName string, filter domain.DesignDepartmentDashboardFilter, members []domain.DesignDepartmentMember, facts []domain.DesignDepartmentTaskFact, now time.Time) *domain.DesignDepartmentDashboard {
	result := &domain.DesignDepartmentDashboard{
		GeneratedAt: now, TimeZone: "Asia/Shanghai", DepartmentID: filter.DepartmentID,
		DepartmentName: strings.TrimSpace(departmentName), PeriodStart: filter.StartAt, PeriodEnd: filter.EndAt,
		DateBasis: filter.DateBasis, Granularity: filter.Granularity,
		Trend: []domain.DesignDepartmentDashboardTrendPoint{}, People: []domain.DesignDepartmentPersonMetric{},
		TaskTypes: []domain.DesignDepartmentDashboardBreakdown{}, Statuses: []domain.DesignDepartmentDashboardBreakdown{},
		Priorities: []domain.DesignDepartmentDashboardBreakdown{}, BusinessLanes: []domain.DesignDepartmentDashboardBreakdown{},
		FileTypes: []domain.DesignDepartmentDashboardBreakdown{}, RecentTasks: []domain.DesignDepartmentTaskRow{},
		Definitions: domain.DesignDepartmentDashboardDefinitions{
			TaskCount:       "筛选范围内分配给本部门设计人员的去重任务数。",
			DesignFileCount: "有效且上传完成的源文件与成品文件数量之和，不含参考图、自动预览图和缩略图。",
			FirstPassRate:   "已完成任务中没有审核打回事件的任务占比。",
			Turnaround:      "任务创建到准确结单事件的小时数；无准确事件时仅已完成任务回退到更新时间。",
			OnTimeRate:      "有截止时间且已完成的任务中，完成时间不晚于截止时间的占比。",
		},
	}
	if result.DepartmentName == "" {
		result.DepartmentName = "当前部门"
	}
	people := make(map[int64]*domain.DesignDepartmentPersonMetric, len(members))
	teamOptions := map[int64]string{}
	for _, member := range members {
		person := &domain.DesignDepartmentPersonMetric{UserID: member.UserID, DisplayName: member.DisplayName, Username: member.Username, TeamID: member.TeamID, TeamName: member.TeamName, Status: member.Status, TaskTypes: []domain.DesignDepartmentDashboardBreakdown{}}
		people[member.UserID] = person
		if member.TeamID != nil && *member.TeamID > 0 {
			teamOptions[*member.TeamID] = member.TeamName
		}
		result.Options.Members = append(result.Options.Members, member)
		if member.Status == "active" {
			result.Summary.MemberCount++
		}
	}
	typeCounts := map[string]*domain.DesignDepartmentDashboardBreakdown{}
	statusCounts := map[string]*domain.DesignDepartmentDashboardBreakdown{}
	priorityCounts := map[string]*domain.DesignDepartmentDashboardBreakdown{}
	laneCounts := map[string]*domain.DesignDepartmentDashboardBreakdown{}
	personTypeCounts := map[int64]map[string]*domain.DesignDepartmentDashboardBreakdown{}
	trend := map[string]*domain.DesignDepartmentDashboardTrendPoint{}
	turnaround := make([]float64, 0)
	deadlineSamples, onTime := int64(0), int64(0)
	completedFirstPass := int64(0)
	contributing := map[int64]struct{}{}
	nowLocal := now.In(designDashboardLocation())
	dueSoon := nowLocal.Add(72 * time.Hour)
	for _, fact := range facts {
		result.Summary.TaskCount++
		files := fact.SourceFileCount + fact.FinalFileCount
		result.Summary.SourceFileCount += fact.SourceFileCount
		result.Summary.FinalFileCount += fact.FinalFileCount
		result.Summary.DesignFileCount += files
		result.Summary.SKUCount += fact.SKUCount
		result.Summary.ResourceUnitCount += fact.ResourceUnitCount
		result.Summary.DesignSubmissionCount += fact.DesignSubmissionCount
		result.Summary.AuditRejectEventCount += fact.AuditRejectCount
		if fact.AuditRejectCount > 0 {
			result.Summary.RejectedTaskCount++
		}
		active := designDashboardTaskActive(fact.TaskStatus)
		if active {
			result.Summary.ActiveTaskCount++
		}
		if active && fact.DeadlineAt != nil && fact.DeadlineAt.Before(now) {
			result.Summary.OverdueTaskCount++
		}
		if active && fact.DeadlineAt != nil && !fact.DeadlineAt.Before(now) && fact.DeadlineAt.Before(dueSoon.UTC()) {
			result.Summary.DueSoonTaskCount++
		}
		completed := fact.CompletedAt != nil
		if completed {
			result.Summary.CompletedTaskCount++
			if fact.AuditRejectCount == 0 {
				completedFirstPass++
			}
			if fact.CompletedAt.After(fact.CreatedAt) {
				turnaround = append(turnaround, fact.CompletedAt.Sub(fact.CreatedAt).Hours())
			}
			if fact.DeadlineAt != nil {
				deadlineSamples++
				if !fact.CompletedAt.After(*fact.DeadlineAt) {
					onTime++
				}
			}
		}
		contributing[fact.DesignerID] = struct{}{}
		person := people[fact.DesignerID]
		if person == nil {
			person = &domain.DesignDepartmentPersonMetric{UserID: fact.DesignerID, DisplayName: fact.DesignerName, TeamID: fact.TeamID, TeamName: fact.TeamName, Status: fact.DesignerStatus, TaskTypes: []domain.DesignDepartmentDashboardBreakdown{}}
			people[fact.DesignerID] = person
		}
		person.TaskCount++
		person.DesignFileCount += files
		person.SourceFileCount += fact.SourceFileCount
		person.FinalFileCount += fact.FinalFileCount
		person.SKUCount += fact.SKUCount
		person.DesignSubmissionCount += fact.DesignSubmissionCount
		person.AuditRejectEventCount += fact.AuditRejectCount
		if active {
			person.ActiveTaskCount++
		}
		if active && fact.DeadlineAt != nil && fact.DeadlineAt.Before(now) {
			person.OverdueTaskCount++
		}
		if fact.AuditRejectCount > 0 {
			person.RejectedTaskCount++
		}
		if completed {
			person.CompletedTaskCount++
		}
		addDesignBreakdown(typeCounts, string(fact.TaskType), designTaskTypeLabel(fact.TaskType), files, fact.SKUCount)
		addDesignBreakdown(statusCounts, string(fact.TaskStatus), designTaskStatusLabel(fact.TaskStatus), files, fact.SKUCount)
		addDesignBreakdown(priorityCounts, string(fact.Priority), designPriorityLabel(fact.Priority), files, fact.SKUCount)
		laneKey := string(fact.BusinessLane)
		if laneKey == "" {
			laneKey = "normal"
		}
		addDesignBreakdown(laneCounts, laneKey, designBusinessLaneLabel(laneKey), files, fact.SKUCount)
		if personTypeCounts[fact.DesignerID] == nil {
			personTypeCounts[fact.DesignerID] = map[string]*domain.DesignDepartmentDashboardBreakdown{}
		}
		addDesignBreakdown(personTypeCounts[fact.DesignerID], string(fact.TaskType), designTaskTypeLabel(fact.TaskType), files, fact.SKUCount)
		period := designDashboardPeriodKey(fact.PrimaryAt, filter.Granularity)
		point := trend[period]
		if point == nil {
			point = &domain.DesignDepartmentDashboardTrendPoint{Period: period}
			trend[period] = point
		}
		point.TaskCount++
		point.DesignFileCount += files
		point.SKUCount += fact.SKUCount
		if completed {
			point.CompletedCount++
		}
		row := domain.DesignDepartmentTaskRow{TaskID: fact.TaskID, TaskNo: fact.TaskNo, ProductName: fact.ProductName, DesignerID: fact.DesignerID, DesignerName: fact.DesignerName, TaskType: fact.TaskType, TaskStatus: fact.TaskStatus, Priority: fact.Priority, CreatedAt: fact.CreatedAt, CompletedAt: fact.CompletedAt, DeadlineAt: fact.DeadlineAt, SourceFileCount: fact.SourceFileCount, FinalFileCount: fact.FinalFileCount, SKUCount: fact.SKUCount, AuditRejectCount: fact.AuditRejectCount, Overdue: active && fact.DeadlineAt != nil && fact.DeadlineAt.Before(now)}
		if completed && fact.CompletedAt.After(fact.CreatedAt) {
			hours := fact.CompletedAt.Sub(fact.CreatedAt).Hours()
			row.TurnaroundHours = &hours
		}
		if len(result.RecentTasks) < 200 {
			result.RecentTasks = append(result.RecentTasks, row)
		}
	}
	result.Summary.ContributingMemberCount = int64(len(contributing))
	if result.Summary.TaskCount > 0 {
		result.Summary.CompletionRate = percent(result.Summary.CompletedTaskCount, result.Summary.TaskCount)
		result.Summary.AverageFilesPerTask = roundMetric(float64(result.Summary.DesignFileCount) / float64(result.Summary.TaskCount))
	}
	if result.Summary.CompletedTaskCount > 0 {
		result.Summary.FirstPassRate = percent(completedFirstPass, result.Summary.CompletedTaskCount)
	}
	result.Summary.DeadlineSampleCount = deadlineSamples
	if deadlineSamples > 0 {
		result.Summary.OnTimeRate = percent(onTime, deadlineSamples)
	}
	if len(turnaround) > 0 {
		sort.Float64s(turnaround)
		result.Summary.TurnaroundSampleCount = int64(len(turnaround))
		result.Summary.AverageTurnaroundHours = roundMetric(averageFloat64(turnaround))
		result.Summary.MedianTurnaroundHours = roundMetric(percentileFloat64(turnaround, .5))
		result.Summary.P90TurnaroundHours = roundMetric(percentileFloat64(turnaround, .9))
	}
	for _, person := range people {
		if person.TaskCount > 0 {
			person.CompletionRate = percent(person.CompletedTaskCount, person.TaskCount)
			person.WorkloadShare = percent(person.TaskCount, result.Summary.TaskCount)
		}
		completedCount := int64(0)
		firstPass := int64(0)
		onTimeCount := int64(0)
		deadlineCount := int64(0)
		hours := []float64{}
		for _, fact := range facts {
			if fact.DesignerID != person.UserID || fact.CompletedAt == nil {
				continue
			}
			completedCount++
			if fact.AuditRejectCount == 0 {
				firstPass++
			}
			if fact.CompletedAt.After(fact.CreatedAt) {
				hours = append(hours, fact.CompletedAt.Sub(fact.CreatedAt).Hours())
			}
			if fact.DeadlineAt != nil {
				deadlineCount++
				if !fact.CompletedAt.After(*fact.DeadlineAt) {
					onTimeCount++
				}
			}
		}
		if completedCount > 0 {
			person.FirstPassRate = percent(firstPass, completedCount)
		}
		if deadlineCount > 0 {
			person.OnTimeRate = percent(onTimeCount, deadlineCount)
		}
		if len(hours) > 0 {
			person.AverageTurnaroundHours = roundMetric(averageFloat64(hours))
		}
		person.TaskTypes = sortedDesignBreakdowns(personTypeCounts[person.UserID], person.TaskCount)
		result.People = append(result.People, *person)
	}
	sort.Slice(result.People, func(i, j int) bool {
		if result.People[i].TaskCount == result.People[j].TaskCount {
			return result.People[i].DisplayName < result.People[j].DisplayName
		}
		return result.People[i].TaskCount > result.People[j].TaskCount
	})
	for _, point := range trend {
		result.Trend = append(result.Trend, *point)
	}
	sort.Slice(result.Trend, func(i, j int) bool { return result.Trend[i].Period < result.Trend[j].Period })
	result.TaskTypes = sortedDesignBreakdowns(typeCounts, result.Summary.TaskCount)
	result.Statuses = sortedDesignBreakdowns(statusCounts, result.Summary.TaskCount)
	result.Priorities = sortedDesignBreakdowns(priorityCounts, result.Summary.TaskCount)
	result.BusinessLanes = sortedDesignBreakdowns(laneCounts, result.Summary.TaskCount)
	result.FileTypes = []domain.DesignDepartmentDashboardBreakdown{{Key: "source", Label: "设计源文件", TaskCount: result.Summary.SourceFileCount}, {Key: "delivery", Label: "最终成品文件", TaskCount: result.Summary.FinalFileCount}, {Key: "sku", Label: "SKU 数量", TaskCount: result.Summary.SKUCount}, {Key: "resource_unit", Label: "资源单元", TaskCount: result.Summary.ResourceUnitCount}}
	result.Options.TaskTypes = []string{string(domain.TaskTypeNewProductDevelopment), string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskTypeRetouchTask), string(domain.TaskTypeSKUPlanning)}
	result.Options.Statuses = []string{string(domain.TaskStatusPendingAssign), string(domain.TaskStatusAssigned), string(domain.TaskStatusInProgress), string(domain.TaskStatusPendingAudit), string(domain.TaskStatusCompleted), string(domain.TaskStatusBlocked), string(domain.TaskStatusCancelled), string(domain.TaskStatusArchived)}
	result.Options.Priorities = []string{string(domain.TaskPriorityCritical), string(domain.TaskPriorityHigh), string(domain.TaskPriorityDrawing), string(domain.TaskPriorityNormal), string(domain.TaskPriorityLow)}
	result.Options.DateBases = []string{"created", "completed", "deadline"}
	result.Options.Granularities = []string{"day", "week", "month"}
	for id, label := range teamOptions {
		result.Options.Teams = append(result.Options.Teams, domain.DesignDepartmentOption{ID: fmt.Sprintf("%d", id), Label: label})
	}
	sort.Slice(result.Options.Teams, func(i, j int) bool { return result.Options.Teams[i].Label < result.Options.Teams[j].Label })
	return result
}

func designDashboardTaskActive(status domain.TaskStatus) bool {
	return status != domain.TaskStatusCompleted && status != domain.TaskStatusArchived && status != domain.TaskStatusCancelled
}
func addDesignBreakdown(items map[string]*domain.DesignDepartmentDashboardBreakdown, key, label string, files, skus int64) {
	if key == "" {
		key = "unknown"
	}
	item := items[key]
	if item == nil {
		item = &domain.DesignDepartmentDashboardBreakdown{Key: key, Label: label}
		items[key] = item
	}
	item.TaskCount++
	item.DesignFileCount += files
	item.SKUCount += skus
}
func sortedDesignBreakdowns(items map[string]*domain.DesignDepartmentDashboardBreakdown, total int64) []domain.DesignDepartmentDashboardBreakdown {
	out := []domain.DesignDepartmentDashboardBreakdown{}
	for _, item := range items {
		copyItem := *item
		copyItem.Share = percent(copyItem.TaskCount, total)
		out = append(out, copyItem)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskCount == out[j].TaskCount {
			return out[i].Label < out[j].Label
		}
		return out[i].TaskCount > out[j].TaskCount
	})
	return out
}
func percent(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return roundMetric(float64(value) * 100 / float64(total))
}
func roundMetric(value float64) float64 { return math.Round(value*10) / 10 }
func averageFloat64(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	if len(values) == 0 {
		return 0
	}
	return total / float64(len(values))
}
func percentileFloat64(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	if len(sortedValues) == 1 {
		return sortedValues[0]
	}
	pos := p * float64(len(sortedValues)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sortedValues[lo]
	}
	return sortedValues[lo] + (sortedValues[hi]-sortedValues[lo])*(pos-float64(lo))
}
func designDashboardPeriodKey(value time.Time, granularity domain.DesignDashboardGranularity) string {
	local := value.In(designDashboardLocation())
	switch granularity {
	case domain.DesignDashboardGranularityMonth:
		return local.Format("2006-01")
	case domain.DesignDashboardGranularityWeek:
		year, week := local.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	default:
		return local.Format("2006-01-02")
	}
}
func designDashboardLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		return location
	}
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}
func designTaskTypeLabel(value domain.TaskType) string {
	switch value {
	case domain.TaskTypeNewProductDevelopment:
		return "新品开发"
	case domain.TaskTypeOriginalProductDevelopment:
		return "原品开发"
	case domain.TaskTypeRetouchTask:
		return "修图任务"
	case domain.TaskTypeSKUPlanning:
		return "SKU规划"
	default:
		return string(value)
	}
}
func designTaskStatusLabel(value domain.TaskStatus) string {
	switch value {
	case domain.TaskStatusCompleted:
		return "已完成"
	case domain.TaskStatusPendingAudit:
		return "待审核"
	case domain.TaskStatusInProgress:
		return "处理中"
	case domain.TaskStatusCancelled:
		return "已取消"
	case domain.TaskStatusBlocked:
		return "已阻塞"
	case domain.TaskStatusAssigned:
		return "已指派"
	case domain.TaskStatusPendingAssign:
		return "待指派"
	default:
		return string(value)
	}
}
func designPriorityLabel(value domain.TaskPriority) string {
	switch value {
	case domain.TaskPriorityCritical:
		return "紧急"
	case domain.TaskPriorityHigh:
		return "高"
	case domain.TaskPriorityDrawing:
		return "画图优先"
	case domain.TaskPriorityNormal:
		return "普通"
	case domain.TaskPriorityLow:
		return "低"
	default:
		return firstNonEmptyString(string(value), "未设置")
	}
}
func designBusinessLaneLabel(value string) string {
	if value == string(domain.TaskBusinessLaneCustomization) {
		return "定制"
	}
	return "常规"
}

type taskBoardService struct {
	dashboardRepo repo.TaskOperationalDashboardRepo
	nowFn         func() time.Time
}

func NewTaskBoardService(dashboardRepo repo.TaskOperationalDashboardRepo) TaskBoardService {
	return &taskBoardService{dashboardRepo: dashboardRepo, nowFn: time.Now}
}

func (s *taskBoardService) GetOperationalOverview(ctx context.Context) (*domain.TaskOperationalOverview, *domain.AppError) {
	if s.dashboardRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "task operational dashboard repository is not configured", nil)
	}
	overview, err := s.dashboardRepo.GetTaskOperationalOverview(ctx, s.nowFn().UTC())
	if err != nil {
		return nil, infraError("get task operational overview", err)
	}
	return overview, nil
}
