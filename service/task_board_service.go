package service

import (
	"context"
	"strings"
	"time"

	"workflow/domain"
)

type TaskBoardFilter struct {
	BoardView domain.TaskBoardView
	QueueKey  string
	TaskFilter
	PreviewSize int
}

type TaskBoardService interface {
	GetSummary(ctx context.Context, filter TaskBoardFilter) (*domain.TaskBoardSummary, *domain.AppError)
	GetQueues(ctx context.Context, filter TaskBoardFilter) (*domain.TaskBoardQueuesResponse, *domain.AppError)
}

type taskBoardCandidateLister interface {
	ListBoardCandidates(ctx context.Context, filter TaskFilter, presets []domain.TaskQueryFilterDefinition) ([]*domain.TaskListItem, *domain.AppError)
}

type taskBoardService struct {
	taskSvc taskBoardCandidateLister
	nowFn   func() time.Time
}

func NewTaskBoardService(taskSvc taskBoardCandidateLister) TaskBoardService {
	return &taskBoardService{
		taskSvc: taskSvc,
		nowFn:   time.Now,
	}
}

type taskBoardPresetDefinition struct {
	key         string
	name        string
	description string
	boardView   domain.TaskBoardView
	hints       domain.TaskBoardQueueOwnershipHints
	filters     domain.TaskQueryFilterDefinition
}

type taskBoardQueueState struct {
	preset        taskBoardPresetDefinition
	items         []*domain.TaskListItem
	queryTemplate domain.TaskQueryTemplate
}

func (s *taskBoardService) GetSummary(ctx context.Context, filter TaskBoardFilter) (*domain.TaskBoardSummary, *domain.AppError) {
	boardView, appErr := normalizeBoardView(filter.BoardView)
	if appErr != nil {
		return nil, appErr
	}
	previewSize := filter.PreviewSize
	if previewSize < 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "preview_size must be >= 0", nil)
	}
	if previewSize == 0 {
		previewSize = 3
	}
	if previewSize > 10 {
		previewSize = 10
	}

	presets, appErr := s.resolvePresets(boardView, filter.QueueKey)
	if appErr != nil {
		return nil, appErr
	}

	queueStates, appErr := s.aggregateQueueStates(ctx, filter.TaskFilter, presets)
	if appErr != nil {
		return nil, appErr
	}

	queues := make([]domain.TaskBoardQueueSummary, 0, len(queueStates))
	for _, state := range queueStates {
		sampleTasks := make([]*domain.TaskListItem, 0, minInt(previewSize, len(state.items)))
		if previewSize > 0 {
			limit := minInt(previewSize, len(state.items))
			sampleTasks = append(sampleTasks, state.items[:limit]...)
		}
		for _, task := range sampleTasks {
			domain.HydrateTaskListItemPolicy(task)
		}

		queue := domain.TaskBoardQueueSummary{
			QueueKey:                     state.preset.key,
			QueueName:                    state.preset.name,
			QueueDescription:             state.preset.description,
			BoardView:                    state.preset.boardView,
			TaskBoardQueueOwnershipHints: state.preset.hints,
			Filters:                      state.preset.filters,
			NormalizedFilters:            state.preset.filters,
			QueryTemplate:                state.queryTemplate,
			Count:                        int64(len(state.items)),
			SampleTasks:                  sampleTasks,
		}
		domain.HydrateTaskBoardQueueSummaryPolicy(&queue)
		queues = append(queues, queue)
	}

	summary := &domain.TaskBoardSummary{
		BoardView:     boardView,
		BoardName:     boardViewLabel(boardView),
		GeneratedAt:   s.nowFn().UTC(),
		FiltersSchema: taskBoardFiltersSchema(),
		Queues:        queues,
	}
	domain.HydrateTaskBoardSummaryPolicy(summary)
	return summary, nil
}

func (s *taskBoardService) GetQueues(ctx context.Context, filter TaskBoardFilter) (*domain.TaskBoardQueuesResponse, *domain.AppError) {
	boardView, appErr := normalizeBoardView(filter.BoardView)
	if appErr != nil {
		return nil, appErr
	}
	presets, appErr := s.resolvePresets(boardView, filter.QueueKey)
	if appErr != nil {
		return nil, appErr
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	queueStates, appErr := s.aggregateQueueStates(ctx, filter.TaskFilter, presets)
	if appErr != nil {
		return nil, appErr
	}

	queues := make([]domain.TaskBoardQueue, 0, len(queueStates))
	for _, state := range queueStates {
		pageItems, pagination := paginateTaskListItems(state.items, page, pageSize)
		for _, task := range pageItems {
			domain.HydrateTaskListItemPolicy(task)
		}

		queue := domain.TaskBoardQueue{
			QueueKey:                     state.preset.key,
			QueueName:                    state.preset.name,
			QueueDescription:             state.preset.description,
			BoardView:                    state.preset.boardView,
			TaskBoardQueueOwnershipHints: state.preset.hints,
			Filters:                      state.preset.filters,
			NormalizedFilters:            state.preset.filters,
			QueryTemplate:                state.queryTemplate,
			Count:                        int64(len(state.items)),
			Tasks:                        pageItems,
			Pagination:                   pagination,
		}
		domain.HydrateTaskBoardQueuePolicy(&queue)
		queues = append(queues, queue)
	}

	response := &domain.TaskBoardQueuesResponse{
		BoardView:     boardView,
		BoardName:     boardViewLabel(boardView),
		GeneratedAt:   s.nowFn().UTC(),
		FiltersSchema: taskBoardFiltersSchema(),
		Queues:        queues,
	}
	domain.HydrateTaskBoardQueuesResponsePolicy(response)
	return response, nil
}

func (s *taskBoardService) resolvePresets(boardView domain.TaskBoardView, queueKey string) ([]taskBoardPresetDefinition, *domain.AppError) {
	all := taskBoardPresetDefinitions()
	presets := make([]taskBoardPresetDefinition, 0, len(all))
	for _, preset := range all {
		if boardView != domain.TaskBoardViewAll && preset.boardView != boardView {
			continue
		}
		if queueKey != "" && preset.key != queueKey {
			continue
		}
		presets = append(presets, preset)
	}
	if queueKey != "" && len(presets) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "queue_key is not supported by the selected board_view", nil)
	}
	return presets, nil
}

func (s *taskBoardService) aggregateQueueStates(ctx context.Context, base TaskFilter, presets []taskBoardPresetDefinition) ([]taskBoardQueueState, *domain.AppError) {
	if len(presets) == 0 {
		return []taskBoardQueueState{}, nil
	}

	base, appErr := normalizeTaskFilter(base)
	if appErr != nil {
		return nil, appErr
	}

	baseItems, appErr := s.taskSvc.ListBoardCandidates(ctx, base, boardCandidateFilters(presets))
	if appErr != nil {
		return nil, appErr
	}

	states := make([]taskBoardQueueState, 0, len(presets))
	for _, preset := range presets {
		effective, ok := mergeTaskBoardFilter(base, preset.filters)
		if !ok {
			continue
		}
		queueItems := filterTaskListItems(baseItems, effective)
		states = append(states, taskBoardQueueState{
			preset:        preset,
			items:         queueItems,
			queryTemplate: buildTaskQueryTemplate(effective),
		})
	}
	return states, nil
}

func filterTaskListItems(items []*domain.TaskListItem, filter TaskFilter) []*domain.TaskListItem {
	filtered := make([]*domain.TaskListItem, 0, len(items))
	for _, item := range items {
		if matchesTaskFilter(item, filter) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func normalizeBoardView(view domain.TaskBoardView) (domain.TaskBoardView, *domain.AppError) {
	if view == "" {
		return domain.TaskBoardViewAll, nil
	}
	if !view.Valid() {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "board_view must be all/ops/designer/audit/procurement/warehouse", nil)
	}
	return view, nil
}

func boardCandidateFilters(presets []taskBoardPresetDefinition) []domain.TaskQueryFilterDefinition {
	filters := make([]domain.TaskQueryFilterDefinition, 0, len(presets))
	for _, preset := range presets {
		filters = append(filters, preset.filters)
	}
	return filters
}

func boardViewLabel(view domain.TaskBoardView) string {
	switch view {
	case domain.TaskBoardViewOps:
		return "Operations board"
	case domain.TaskBoardViewDesigner:
		return "Designer board"
	case domain.TaskBoardViewAudit:
		return "Audit board"
	case domain.TaskBoardViewProcurement:
		return "Procurement board"
	case domain.TaskBoardViewWarehouse:
		return "Warehouse board"
	default:
		return "All boards"
	}
}

func taskBoardFiltersSchema() domain.TaskBoardFiltersSchema {
	return domain.TaskBoardFiltersSchema{
		BoardViews: []domain.TaskBoardView{
			domain.TaskBoardViewAll,
			domain.TaskBoardViewOps,
			domain.TaskBoardViewDesigner,
			domain.TaskBoardViewAudit,
			domain.TaskBoardViewProcurement,
			domain.TaskBoardViewWarehouse,
		},
		SupportedGlobalFilters: []string{
			"keyword",
			"task_type",
			"source_mode",
			"workflow_lane",
			"creator_id",
			"designer_id",
			"owner_department",
			"owner_org_team",
			"need_outsource",
			"overdue",
		},
		QueueConditionFields: []string{
			"statuses",
			"task_types",
			"source_modes",
			"workflow_lanes",
			"main_statuses",
			"sub_status_scope",
			"sub_status_codes",
			"coordination_statuses",
			"warehouse_prepare_ready",
			"warehouse_receive_ready",
			"warehouse_blocking_reason_codes",
		},
		TaskListEndpoint: "/v1/tasks",
		TaskListPassthroughFields: []string{
			"status",
			"task_type",
			"source_mode",
			"workflow_lane",
			"main_status",
			"sub_status_scope",
			"sub_status_code",
			"coordination_status",
			"warehouse_prepare_ready",
			"warehouse_receive_ready",
			"warehouse_blocking_reason_code",
			"keyword",
			"creator_id",
			"designer_id",
			"owner_department",
			"owner_org_team",
			"need_outsource",
			"overdue",
		},
	}
}

func taskBoardPresetDefinitions() []taskBoardPresetDefinition {
	procurementScope := domain.TaskSubStatusScopeProcurement
	designScope := domain.TaskSubStatusScopeDesign
	auditScope := domain.TaskSubStatusScopeAudit
	trueValue := true
	return []taskBoardPresetDefinition{
		{
			key:         "ops_pending_material",
			name:        "Ops pending materials",
			description: "Tasks still waiting for operations-side filing or business info completion.",
			boardView:   domain.TaskBoardViewOps,
			hints: domain.TaskBoardQueueOwnershipHints{
				SuggestedRoles:     []domain.Role{domain.RoleOps, domain.RoleAdmin},
				SuggestedActorType: "shared_role_pool",
				DefaultVisibility:  "board_view_default",
				OwnershipHint:      "Advisory only. This queue is typically handled by operations roles completing filing and business info.",
			},
			filters: domain.TaskQueryFilterDefinition{
				MainStatuses: []domain.TaskMainStatus{
					domain.TaskMainStatusCreated,
					domain.TaskMainStatusFiled,
				},
				WarehouseBlockingReasonCodes: []domain.WorkflowReasonCode{
					domain.WorkflowReasonFiledAtMissing,
					domain.WorkflowReasonCategoryMissing,
					domain.WorkflowReasonSpecMissing,
					domain.WorkflowReasonCostPriceMissing,
				},
			},
		},
		{
			key:         "design_pending_submit",
			name:        "Designer pending submit",
			description: "Design tasks currently being worked on or sent back for rework.",
			boardView:   domain.TaskBoardViewDesigner,
			hints: domain.TaskBoardQueueOwnershipHints{
				SuggestedRoles:     []domain.Role{domain.RoleDesigner, domain.RoleOps},
				SuggestedActorType: "assigned_actor",
				DefaultVisibility:  "board_view_default",
				OwnershipHint:      "Advisory only. Usually shown to the assigned designer, with operations able to assist in placeholder flows.",
			},
			filters: domain.TaskQueryFilterDefinition{
				SubStatusScope: &designScope,
				SubStatusCodes: []domain.TaskSubStatusCode{
					domain.TaskSubStatusDesigning,
					domain.TaskSubStatusReworkRequired,
				},
			},
		},
		{
			key:         "audit_pending_review",
			name:        "Audit pending review",
			description: "Tasks actively waiting for audit handling.",
			boardView:   domain.TaskBoardViewAudit,
			hints: domain.TaskBoardQueueOwnershipHints{
				SuggestedRoles:     []domain.Role{domain.RoleAuditA, domain.RoleAuditB, domain.RoleAdmin},
				SuggestedActorType: "shared_role_pool",
				DefaultVisibility:  "board_view_default",
				OwnershipHint:      "Advisory only. This queue suggests likely audit ownership but does not claim or lock tasks.",
			},
			filters: domain.TaskQueryFilterDefinition{
				SubStatusScope: &auditScope,
				SubStatusCodes: []domain.TaskSubStatusCode{
					domain.TaskSubStatusInReview,
				},
			},
		},
		{
			key:         "procurement_pending_followup",
			name:        "Procurement pending follow-up",
			description: "Purchase tasks still being prepared or waiting for inbound arrival.",
			boardView:   domain.TaskBoardViewProcurement,
			hints: domain.TaskBoardQueueOwnershipHints{
				SuggestedRoles:     []domain.Role{domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin},
				SuggestedActorType: "shared_role_pool",
				DefaultVisibility:  "cross_function_default",
				OwnershipHint:      "Advisory only. Procurement-related follow-up is still handled through placeholder operations and warehouse roles in the current phase.",
			},
			filters: domain.TaskQueryFilterDefinition{
				TaskTypes: []domain.TaskType{
					domain.TaskTypePurchaseTask,
				},
				SubStatusScope: &procurementScope,
				SubStatusCodes: []domain.TaskSubStatusCode{
					domain.TaskSubStatusNotStarted,
					domain.TaskSubStatusPreparing,
					domain.TaskSubStatusPendingInbound,
				},
				CoordinationStatuses: []domain.ProcurementCoordinationStatus{
					domain.ProcurementCoordinationStatusPreparing,
					domain.ProcurementCoordinationStatusAwaitingArrival,
				},
			},
		},
		{
			key:         "awaiting_arrival",
			name:        "Awaiting arrival",
			description: "Purchase tasks already in procurement and still waiting for goods to arrive.",
			boardView:   domain.TaskBoardViewProcurement,
			hints: domain.TaskBoardQueueOwnershipHints{
				SuggestedRoles:     []domain.Role{domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin},
				SuggestedActorType: "shared_role_pool",
				DefaultVisibility:  "cross_function_default",
				OwnershipHint:      "Advisory only. This is a coordination hint for purchase follow-up, not a permission or assignment boundary.",
			},
			filters: domain.TaskQueryFilterDefinition{
				TaskTypes: []domain.TaskType{
					domain.TaskTypePurchaseTask,
				},
				CoordinationStatuses: []domain.ProcurementCoordinationStatus{
					domain.ProcurementCoordinationStatusAwaitingArrival,
				},
			},
		},
		{
			key:         "warehouse_pending_prepare",
			name:        "Pending warehouse prepare",
			description: "Purchase tasks that are procurement-complete and ready to be handed over to warehouse.",
			boardView:   domain.TaskBoardViewProcurement,
			hints: domain.TaskBoardQueueOwnershipHints{
				SuggestedRoles:     []domain.Role{domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin},
				SuggestedActorType: "shared_role_pool",
				DefaultVisibility:  "cross_function_default",
				OwnershipHint:      "Advisory only. This queue highlights handoff readiness between operations and warehouse roles.",
			},
			filters: domain.TaskQueryFilterDefinition{
				TaskTypes: []domain.TaskType{
					domain.TaskTypePurchaseTask,
				},
				CoordinationStatuses: []domain.ProcurementCoordinationStatus{
					domain.ProcurementCoordinationStatusReadyForWarehouse,
				},
				WarehousePrepareReady: &trueValue,
			},
		},
		{
			key:         "warehouse_pending_receive",
			name:        "Warehouse pending receive",
			description: "Tasks already handed to warehouse and still waiting for warehouse receive.",
			boardView:   domain.TaskBoardViewWarehouse,
			hints: domain.TaskBoardQueueOwnershipHints{
				SuggestedRoles:     []domain.Role{domain.RoleWarehouse, domain.RoleOps, domain.RoleAdmin},
				SuggestedActorType: "shared_role_pool",
				DefaultVisibility:  "board_view_default",
				OwnershipHint:      "Advisory only. Warehouse roles usually own this queue after handoff, but no enforcement is applied yet.",
			},
			filters: domain.TaskQueryFilterDefinition{
				MainStatuses: []domain.TaskMainStatus{
					domain.TaskMainStatusPendingWarehouseReceive,
				},
				WarehouseReceiveReady: &trueValue,
			},
		},
		{
			key:         "pending_close",
			name:        "Pending close",
			description: "Tasks whose warehouse flow is done and now wait for explicit close.",
			boardView:   domain.TaskBoardViewWarehouse,
			hints: domain.TaskBoardQueueOwnershipHints{
				SuggestedRoles:     []domain.Role{domain.RoleWarehouse, domain.RoleOps, domain.RoleAdmin},
				SuggestedActorType: "shared_role_pool",
				DefaultVisibility:  "board_view_default",
				OwnershipHint:      "Advisory only. This queue signals likely close follow-up responsibility and does not block other placeholder actors.",
			},
			filters: domain.TaskQueryFilterDefinition{
				MainStatuses: []domain.TaskMainStatus{
					domain.TaskMainStatusPendingClose,
				},
			},
		},
	}
}

func mergeTaskBoardFilter(base TaskFilter, preset domain.TaskQueryFilterDefinition) (TaskFilter, bool) {
	merged := base
	var ok bool

	merged.Statuses, ok = intersectComparableSlice(base.Statuses, preset.Statuses)
	if !ok {
		return TaskFilter{}, false
	}
	merged.TaskTypes, ok = intersectComparableSlice(base.TaskTypes, preset.TaskTypes)
	if !ok {
		return TaskFilter{}, false
	}
	merged.SourceModes, ok = intersectComparableSlice(base.SourceModes, preset.SourceModes)
	if !ok {
		return TaskFilter{}, false
	}
	merged.WorkflowLanes, ok = intersectComparableSlice(base.WorkflowLanes, preset.WorkflowLanes)
	if !ok {
		return TaskFilter{}, false
	}
	merged.MainStatuses, ok = intersectComparableSlice(base.MainStatuses, preset.MainStatuses)
	if !ok {
		return TaskFilter{}, false
	}
	merged.SubStatusCodes, ok = intersectComparableSlice(base.SubStatusCodes, preset.SubStatusCodes)
	if !ok {
		return TaskFilter{}, false
	}
	merged.CoordinationStatuses, ok = intersectComparableSlice(base.CoordinationStatuses, preset.CoordinationStatuses)
	if !ok {
		return TaskFilter{}, false
	}
	merged.OwnerDepartments, ok = intersectComparableSlice(base.OwnerDepartments, preset.OwnerDepartments)
	if !ok {
		return TaskFilter{}, false
	}
	merged.OwnerOrgTeams, ok = intersectComparableSlice(base.OwnerOrgTeams, preset.OwnerOrgTeams)
	if !ok {
		return TaskFilter{}, false
	}
	merged.WarehouseBlockingReasonCodes, ok = intersectComparableSlice(base.WarehouseBlockingReasonCodes, preset.WarehouseBlockingReasonCodes)
	if !ok {
		return TaskFilter{}, false
	}
	merged.SubStatusScope, ok = mergeOptionalComparable(base.SubStatusScope, preset.SubStatusScope)
	if !ok {
		return TaskFilter{}, false
	}
	merged.WarehousePrepareReady, ok = mergeOptionalComparable(base.WarehousePrepareReady, preset.WarehousePrepareReady)
	if !ok {
		return TaskFilter{}, false
	}
	merged.WarehouseReceiveReady, ok = mergeOptionalComparable(base.WarehouseReceiveReady, preset.WarehouseReceiveReady)
	if !ok {
		return TaskFilter{}, false
	}

	return merged, true
}

func buildTaskQueryTemplate(filter TaskFilter) domain.TaskQueryTemplate {
	return domain.TaskQueryTemplate{
		Status:                      joinComparableValues(filter.Statuses),
		TaskType:                    joinComparableValues(filter.TaskTypes),
		SourceMode:                  joinComparableValues(filter.SourceModes),
		WorkflowLane:                joinComparableValues(filter.WorkflowLanes),
		MainStatus:                  joinComparableValues(filter.MainStatuses),
		SubStatusCode:               joinComparableValues(filter.SubStatusCodes),
		CoordinationStatus:          joinComparableValues(filter.CoordinationStatuses),
		WarehouseBlockingReasonCode: joinComparableValues(filter.WarehouseBlockingReasonCodes),
		Keyword:                     filter.Keyword,
		CreatorID:                   filter.CreatorID,
		DesignerID:                  filter.DesignerID,
		OwnerDepartment:             joinStrings(filter.OwnerDepartments),
		OwnerOrgTeam:                joinStrings(filter.OwnerOrgTeams),
		NeedOutsource:               filter.NeedOutsource,
		Overdue:                     filter.Overdue,
		WarehousePrepareReady:       filter.WarehousePrepareReady,
		WarehouseReceiveReady:       filter.WarehouseReceiveReady,
		SubStatusScope:              optionalComparableString(filter.SubStatusScope),
	}
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ",")
}

func intersectComparableSlice[T comparable](base []T, preset []T) ([]T, bool) {
	if len(base) == 0 && len(preset) == 0 {
		return nil, true
	}
	if len(base) == 0 {
		return append([]T(nil), preset...), true
	}
	if len(preset) == 0 {
		return append([]T(nil), base...), true
	}
	allowed := make(map[T]struct{}, len(base))
	for _, value := range base {
		allowed[value] = struct{}{}
	}
	out := make([]T, 0, len(preset))
	seen := make(map[T]struct{}, len(preset))
	for _, value := range preset {
		if _, ok := allowed[value]; !ok {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func mergeOptionalComparable[T comparable](base, preset *T) (*T, bool) {
	if base == nil && preset == nil {
		return nil, true
	}
	if base == nil {
		value := *preset
		return &value, true
	}
	if preset == nil {
		value := *base
		return &value, true
	}
	if *base != *preset {
		return nil, false
	}
	value := *base
	return &value, true
}

func joinComparableValues[T ~string](values []T) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, string(value))
	}
	return strings.Join(parts, ",")
}

func optionalComparableString[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
