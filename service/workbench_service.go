package service

import (
	"context"
	"sort"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

const defaultWorkbenchPageSize = 20

var (
	supportedWorkbenchPageSizes = []int{10, 20, 50, 100}
	supportedWorkbenchSorts     = []domain.WorkbenchSortKey{domain.WorkbenchSortUpdatedAtDesc}
)

type WorkbenchService interface {
	GetPreferences(ctx context.Context) (*domain.WorkbenchPreferencesEnvelope, *domain.AppError)
	PatchPreferences(ctx context.Context, patch domain.WorkbenchPreferencesPatch) (*domain.WorkbenchPreferencesEnvelope, *domain.AppError)
}

type workbenchService struct {
	preferenceRepo repo.WorkbenchPreferenceRepo
}

func NewWorkbenchService(preferenceRepo repo.WorkbenchPreferenceRepo) WorkbenchService {
	return &workbenchService{preferenceRepo: preferenceRepo}
}

func (s *workbenchService) GetPreferences(ctx context.Context) (*domain.WorkbenchPreferencesEnvelope, *domain.AppError) {
	actor, scope, appErr := resolveUserScopedWorkbenchActorScope(ctx)
	if appErr != nil {
		return nil, appErr
	}

	record, err := s.preferenceRepo.GetByActorScope(ctx, scope)
	if err != nil {
		return nil, infraError("get workbench preferences", err)
	}

	preferences := effectiveWorkbenchPreferences(domain.WorkbenchPreferences{})
	if record != nil {
		preferences = effectiveWorkbenchPreferences(record.Preferences)
	}

	return &domain.WorkbenchPreferencesEnvelope{
		Actor:           actor,
		Preferences:     preferences,
		WorkbenchConfig: buildWorkbenchConfig(),
	}, nil
}

func (s *workbenchService) PatchPreferences(ctx context.Context, patch domain.WorkbenchPreferencesPatch) (*domain.WorkbenchPreferencesEnvelope, *domain.AppError) {
	actor, scope, appErr := resolveUserScopedWorkbenchActorScope(ctx)
	if appErr != nil {
		return nil, appErr
	}

	record, err := s.preferenceRepo.GetByActorScope(ctx, scope)
	if err != nil {
		return nil, infraError("get workbench preferences", err)
	}

	current := domain.WorkbenchPreferences{}
	if record != nil {
		current = record.Preferences
	}
	updated := mergeWorkbenchPreferences(current, patch)
	if appErr := validateWorkbenchPreferences(updated); appErr != nil {
		return nil, appErr
	}

	if err := s.preferenceRepo.UpsertByActorScope(ctx, &domain.WorkbenchPreferenceRecord{
		ActorID:       scope.ActorID,
		ActorRolesKey: scope.ActorRolesKey,
		AuthMode:      scope.AuthMode,
		Preferences:   updated,
	}); err != nil {
		return nil, infraError("save workbench preferences", err)
	}

	return &domain.WorkbenchPreferencesEnvelope{
		Actor:           actor,
		Preferences:     effectiveWorkbenchPreferences(updated),
		WorkbenchConfig: buildWorkbenchConfig(),
	}, nil
}

func mergeWorkbenchPreferences(current domain.WorkbenchPreferences, patch domain.WorkbenchPreferencesPatch) domain.WorkbenchPreferences {
	if patch.DefaultQueueKey != nil {
		current.DefaultQueueKey = strings.TrimSpace(*patch.DefaultQueueKey)
	}
	if patch.PinnedQueueKeys != nil {
		current.PinnedQueueKeys = normalizeWorkbenchQueueKeys(*patch.PinnedQueueKeys)
	}
	if patch.DefaultFilters != nil {
		current.DefaultFilters = *patch.DefaultFilters
	}
	if patch.DefaultPageSize != nil {
		current.DefaultPageSize = *patch.DefaultPageSize
	}
	if patch.DefaultSort != nil {
		current.DefaultSort = domain.WorkbenchSortKey(strings.TrimSpace(string(*patch.DefaultSort)))
	}
	return current
}

func validateWorkbenchPreferences(preferences domain.WorkbenchPreferences) *domain.AppError {
	knownQueueKeys := knownWorkbenchQueueKeys()
	if preferences.DefaultQueueKey != "" {
		if _, ok := knownQueueKeys[preferences.DefaultQueueKey]; !ok {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "default_queue_key is not a supported preset queue", nil)
		}
	}
	for _, queueKey := range preferences.PinnedQueueKeys {
		if _, ok := knownQueueKeys[queueKey]; !ok {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "pinned_queue_keys contains an unsupported preset queue", nil)
		}
	}
	if preferences.DefaultPageSize != 0 && !isSupportedWorkbenchPageSize(preferences.DefaultPageSize) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "default_page_size must be one of 10/20/50/100 or 0 to clear", nil)
	}
	if !preferences.DefaultSort.Valid() {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "default_sort must be updated_at_desc or empty", nil)
	}
	if _, appErr := taskQueryTemplateToTaskFilter(preferences.DefaultFilters); appErr != nil {
		return appErr
	}
	return nil
}

func effectiveWorkbenchPreferences(preferences domain.WorkbenchPreferences) domain.WorkbenchPreferences {
	out := preferences
	if out.PinnedQueueKeys == nil {
		out.PinnedQueueKeys = []string{}
	}
	if out.DefaultPageSize == 0 {
		out.DefaultPageSize = defaultWorkbenchPageSize
	}
	if out.DefaultSort == "" {
		out.DefaultSort = domain.WorkbenchSortUpdatedAtDesc
	}
	return out
}

func buildWorkbenchConfig() domain.WorkbenchConfig {
	presets := taskBoardPresetDefinitions()
	queues := make([]domain.WorkbenchQueueConfig, 0, len(presets))
	for _, preset := range presets {
		queryTemplate := buildTaskQueryTemplate(TaskFilter{
			TaskQueryFilterDefinition: preset.filters,
		})
		queues = append(queues, domain.WorkbenchQueueConfig{
			QueueKey:                     preset.key,
			QueueName:                    preset.name,
			QueueDescription:             preset.description,
			BoardView:                    preset.boardView,
			TaskBoardQueueOwnershipHints: preset.hints,
			Filters:                      preset.filters,
			NormalizedFilters:            preset.filters,
			QueryTemplate:                queryTemplate,
		})
	}

	return domain.WorkbenchConfig{
		FiltersSchema:      taskBoardFiltersSchema(),
		SupportedSorts:     append([]domain.WorkbenchSortKey(nil), supportedWorkbenchSorts...),
		SupportedPageSizes: append([]int(nil), supportedWorkbenchPageSizes...),
		Queues:             queues,
	}
}

func resolveWorkbenchActorScope(ctx context.Context) (domain.RequestActor, repo.WorkbenchPreferenceScope) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok {
		actor = domain.RequestActor{
			Source:   domain.RequestActorSourceAnonymous,
			AuthMode: domain.AuthModeDebugHeaderRoleEnforced,
		}
	}
	if actor.Source == domain.RequestActorSourceSystemFallback && actor.ID <= 0 {
		actor.ID = 1
	}
	if actor.Source == "" {
		actor.Source = domain.RequestActorSourceAnonymous
	}
	if actor.AuthMode == "" {
		actor.AuthMode = domain.AuthModeDebugHeaderRoleEnforced
	}
	actor.Roles = domain.NormalizeRoleValues(actor.Roles)

	return actor, buildWorkbenchPreferenceScope(actor)
}

func resolveUserScopedWorkbenchActorScope(ctx context.Context) (domain.RequestActor, repo.WorkbenchPreferenceScope, *domain.AppError) {
	actor, scope := resolveWorkbenchActorScope(ctx)
	if domain.IsSessionBackedRequestActor(actor) {
		return actor, scope, nil
	}
	return actor, repo.WorkbenchPreferenceScope{}, domain.NewAppError(
		domain.ErrCodeUnauthorized,
		"workbench preferences require a session-backed actor",
		nil,
	)
}

func buildWorkbenchPreferenceScope(actor domain.RequestActor) repo.WorkbenchPreferenceScope {
	return repo.WorkbenchPreferenceScope{
		ActorID:       actor.ID,
		ActorRolesKey: workbenchRolesKey(actor.Roles),
		AuthMode:      actor.AuthMode,
	}
}

func workbenchRolesKey(roles []domain.Role) string {
	if len(roles) == 0 {
		return ""
	}
	items := make([]string, 0, len(roles))
	for _, role := range roles {
		if role == "" {
			continue
		}
		items = append(items, string(role))
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func knownWorkbenchQueueKeys() map[string]struct{} {
	keys := make(map[string]struct{}, len(taskBoardPresetDefinitions()))
	for _, preset := range taskBoardPresetDefinitions() {
		keys[preset.key] = struct{}{}
	}
	return keys
}

func normalizeWorkbenchQueueKeys(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func isSupportedWorkbenchPageSize(value int) bool {
	for _, candidate := range supportedWorkbenchPageSizes {
		if candidate == value {
			return true
		}
	}
	return false
}

func taskQueryTemplateToTaskFilter(template domain.TaskQueryTemplate) (TaskFilter, *domain.AppError) {
	filter := TaskFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Statuses:                     taskStatusesFromTemplate(template.Status),
			TaskTypes:                    taskTypesFromTemplate(template.TaskType),
			SourceModes:                  taskSourceModesFromTemplate(template.SourceMode),
			WorkflowLanes:                workflowLanesFromTemplate(template.WorkflowLane),
			MainStatuses:                 taskMainStatusesFromTemplate(template.MainStatus),
			SubStatusCodes:               taskSubStatusCodesFromTemplate(template.SubStatusCode),
			CoordinationStatuses:         coordinationStatusesFromTemplate(template.CoordinationStatus),
			OwnerDepartments:             splitTemplateValues(template.OwnerDepartment),
			OwnerOrgTeams:                splitTemplateValues(template.OwnerOrgTeam),
			WarehouseBlockingReasonCodes: workflowReasonCodesFromTemplate(template.WarehouseBlockingReasonCode),
			WarehousePrepareReady:        template.WarehousePrepareReady,
			WarehouseReceiveReady:        template.WarehouseReceiveReady,
		},
		Keyword:       strings.TrimSpace(template.Keyword),
		CreatorID:     template.CreatorID,
		DesignerID:    template.DesignerID,
		NeedOutsource: template.NeedOutsource,
		Overdue:       template.Overdue,
	}

	if raw := strings.TrimSpace(template.SubStatusScope); raw != "" {
		scope := domain.TaskSubStatusScope(raw)
		filter.SubStatusScope = &scope
	}

	return normalizeTaskFilter(filter)
}

func taskStatusesFromTemplate(raw string) []domain.TaskStatus {
	values := splitTemplateValues(raw)
	out := make([]domain.TaskStatus, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskStatus(value))
	}
	return out
}

func taskTypesFromTemplate(raw string) []domain.TaskType {
	values := splitTemplateValues(raw)
	out := make([]domain.TaskType, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskType(value))
	}
	return out
}

func taskSourceModesFromTemplate(raw string) []domain.TaskSourceMode {
	values := splitTemplateValues(raw)
	out := make([]domain.TaskSourceMode, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskSourceMode(value))
	}
	return out
}

func taskMainStatusesFromTemplate(raw string) []domain.TaskMainStatus {
	values := splitTemplateValues(raw)
	out := make([]domain.TaskMainStatus, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskMainStatus(value))
	}
	return out
}

func workflowLanesFromTemplate(raw string) []domain.WorkflowLane {
	values := splitTemplateValues(raw)
	out := make([]domain.WorkflowLane, 0, len(values))
	for _, value := range values {
		out = append(out, domain.WorkflowLane(value))
	}
	return out
}

func taskSubStatusCodesFromTemplate(raw string) []domain.TaskSubStatusCode {
	values := splitTemplateValues(raw)
	out := make([]domain.TaskSubStatusCode, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskSubStatusCode(value))
	}
	return out
}

func coordinationStatusesFromTemplate(raw string) []domain.ProcurementCoordinationStatus {
	values := splitTemplateValues(raw)
	out := make([]domain.ProcurementCoordinationStatus, 0, len(values))
	for _, value := range values {
		out = append(out, domain.ProcurementCoordinationStatus(value))
	}
	return out
}

func workflowReasonCodesFromTemplate(raw string) []domain.WorkflowReasonCode {
	values := splitTemplateValues(raw)
	out := make([]domain.WorkflowReasonCode, 0, len(values))
	for _, value := range values {
		out = append(out, domain.WorkflowReasonCode(value))
	}
	return out
}

func splitTemplateValues(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
