package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type auditV7Service struct {
	taskRepo         repo.TaskRepo
	auditV7Repo      repo.AuditV7Repo
	taskEventRepo    repo.TaskEventRepo
	codeRuleSvc      CodeRuleService
	txRunner         repo.TxRunner
	scopeUserRepo    repo.UserRepo
	v8AccessResolver auditV8EffectiveAccessResolver
}

type AuditV7ServiceOption func(*auditV7Service)

type auditV8EffectiveAccessResolver interface {
	EffectiveAccess(ctx context.Context, userID int64) (*domain.EffectiveAccess, *domain.AppError)
}

type auditV8TaskLockingRepo interface {
	GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.Task, error)
}

type auditV8HandoverLockingRepo interface {
	GetHandoverByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.AuditHandover, error)
	CASUpdateHandoverStatus(ctx context.Context, tx repo.Tx, id int64, expected, next domain.HandoverStatus) (bool, error)
}

func WithAuditV7ScopeUserRepo(userRepo repo.UserRepo) AuditV7ServiceOption {
	return func(s *auditV7Service) {
		s.scopeUserRepo = userRepo
	}
}

func WithAuditV8EffectiveAccessResolver(resolver auditV8EffectiveAccessResolver) AuditV7ServiceOption {
	return func(s *auditV7Service) {
		s.v8AccessResolver = resolver
	}
}

func NewAuditV7Service(
	taskRepo repo.TaskRepo,
	auditV7Repo repo.AuditV7Repo,
	taskEventRepo repo.TaskEventRepo,
	codeRuleSvc CodeRuleService,
	txRunner repo.TxRunner,
	opts ...AuditV7ServiceOption,
) AuditV7Service {
	svc := &auditV7Service{
		taskRepo:      taskRepo,
		auditV7Repo:   auditV7Repo,
		taskEventRepo: taskEventRepo,
		codeRuleSvc:   codeRuleSvc,
		txRunner:      txRunner,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *auditV7Service) Handover(ctx context.Context, p HandoverAuditParams) (*domain.AuditHandover, *domain.AppError) {
	task, appErr := s.getTask(ctx, p.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	if task.TaskStatus != domain.TaskStatusPendingAudit {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d in status %q cannot be handed over",
				p.TaskID, task.TaskStatus), nil)
	}
	stage := domain.AuditRecordStageUnified
	if appErr := authorizeV8AuditTask(ctx, task, domain.PermissionTaskAuditDecision, p.FromAuditorID); appErr != nil {
		return nil, appErr
	}
	if task.CurrentHandlerID == nil || *task.CurrentHandlerID != p.FromAuditorID {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "only the current audit handler can hand over this task", map[string]interface{}{
			"deny_code": "audit_handover_requires_current_handler",
			"task_id":   task.ID,
			"actor_id":  p.FromAuditorID,
		})
	}
	if appErr := s.authorizeV8AuditTarget(ctx, task, p.ToAuditorID); appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureNoPendingHandover(ctx, p.TaskID); appErr != nil {
		return nil, appErr
	}
	if p.FromAuditorID == p.ToAuditorID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "to_auditor_id must be different from from_auditor_id", nil)
	}
	p.Reason = strings.TrimSpace(p.Reason)
	if p.Reason == "" {
		return nil, domain.ErrReasonRequired
	}
	expectedWorkflowRevision := task.WorkflowRevision

	handoverNo, appErr := s.codeRuleSvc.GenerateCode(ctx, domain.CodeRuleTypeHandoverNo)
	if appErr != nil {
		return nil, appErr
	}

	handover := &domain.AuditHandover{
		HandoverNo:       handoverNo,
		TaskID:           p.TaskID,
		FromAuditorID:    p.FromAuditorID,
		ToAuditorID:      p.ToAuditorID,
		Reason:           p.Reason,
		CurrentJudgement: p.CurrentJudgement,
		RiskRemark:       p.RiskRemark,
		Status:           domain.HandoverStatusPendingTakeover,
	}

	var newID int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		lockingRepo, ok := s.taskRepo.(auditV8TaskLockingRepo)
		if !ok {
			return fmt.Errorf("v8 audit handover task locking is unavailable")
		}
		lockedTask, err := lockingRepo.GetByIDForUpdate(ctx, tx, p.TaskID)
		if err != nil {
			return fmt.Errorf("lock task for v8 audit handover: %w", err)
		}
		if lockedTask == nil || lockedTask.TaskStatus != domain.TaskStatusPendingAudit ||
			lockedTask.WorkflowRevision != expectedWorkflowRevision || lockedTask.CurrentHandlerID == nil ||
			*lockedTask.CurrentHandlerID != p.FromAuditorID {
			return repo.ErrConflict
		}
		id, err := s.auditV7Repo.CreateHandover(ctx, tx, handover)
		if err != nil {
			return fmt.Errorf("create handover: %w", err)
		}
		newID = id

		if _, err := s.auditV7Repo.CreateRecord(ctx, tx, &domain.AuditRecord{
			TaskID:         p.TaskID,
			Stage:          stage,
			Action:         domain.AuditActionTypeHandover,
			AuditorID:      p.FromAuditorID,
			IssueTypesJSON: "[]",
			Comment:        p.Reason,
		}); err != nil {
			return fmt.Errorf("handover audit record: %w", err)
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, nil); err != nil {
			return err
		}
		_, err = s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventAuditHandedOver, &p.FromAuditorID,
			taskTransitionEventPayload(task, task.TaskStatus, task.TaskStatus, task.CurrentHandlerID, nil, map[string]interface{}{
				"handover_id":       newID,
				"handover_no":       handoverNo,
				"from_auditor_id":   p.FromAuditorID,
				"to_auditor_id":     p.ToAuditorID,
				"stage":             string(stage),
				"reason":            p.Reason,
				"current_judgement": p.CurrentJudgement,
				"risk_remark":       p.RiskRemark,
			}))
		return err
	})
	if txErr != nil {
		if errors.Is(txErr, repo.ErrConflict) {
			return nil, domain.NewAppError(domain.ErrCodeConflict, "task audit handler or workflow revision changed; refresh and retry", map[string]interface{}{
				"deny_code": "audit_handover_concurrent_change",
				"task_id":   p.TaskID,
			})
		}
		return nil, infraError("handover tx", txErr)
	}

	handover.ID = newID
	return handover, nil
}

func (s *auditV7Service) ListHandoverCandidates(ctx context.Context, filter AuditHandoverCandidateFilter) (*AuditHandoverCandidateListResponse, *domain.AppError) {
	actor, appErr := auditHandoverActorFromContext(ctx)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := authorizeAuditHandoverBatchActor(actor); appErr != nil {
		return nil, appErr
	}
	return s.listHandoverCandidatesForActor(ctx, actor, filter)
}

func (s *auditV7Service) BatchHandover(ctx context.Context, p BatchAuditHandoverParams) (*BatchAuditHandoverResponse, *domain.AppError) {
	actor, appErr := auditHandoverActorFromContext(ctx)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := authorizeAuditHandoverBatchActor(actor); appErr != nil {
		return nil, appErr
	}

	p.Mode = BatchAuditHandoverMode(strings.TrimSpace(string(p.Mode)))
	if p.Mode == "" && len(p.TaskIDs) > 0 {
		p.Mode = BatchAuditHandoverModeExplicit
	}
	if p.Mode != BatchAuditHandoverModeExplicit && p.Mode != BatchAuditHandoverModeAllMatching {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "mode must be explicit or all_matching", nil)
	}
	if p.ToAuditorID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "to_auditor_id is required", nil)
	}
	if p.ToAuditorID == actor.ID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "to_auditor_id must be different from current auditor", nil)
	}
	p.Reason = strings.TrimSpace(p.Reason)
	if p.Reason == "" {
		return nil, domain.ErrReasonRequired
	}
	p.CurrentJudgement = strings.TrimSpace(p.CurrentJudgement)
	p.RiskRemark = strings.TrimSpace(p.RiskRemark)

	targets, appErr := s.resolveBatchHandoverTargets(ctx, actor, p)
	if appErr != nil {
		return nil, appErr
	}
	if len(targets) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "no audit handover candidates match the request", nil)
	}

	resp := &BatchAuditHandoverResponse{
		Results: make([]BatchAuditHandoverResultItem, 0, len(targets)),
	}
	for _, target := range targets {
		item := BatchAuditHandoverResultItem{
			TaskID: target.TaskID,
			TaskNo: target.TaskNo,
			Status: "failed",
		}
		if target.TaskNo == "" {
			task, appErr := s.getTask(ctx, target.TaskID)
			if appErr != nil {
				item.Message = auditBatchErrorMessage(appErr)
				resp.FailureCount++
				resp.Results = append(resp.Results, item)
				continue
			}
			item.TaskNo = task.TaskNo
		}

		handover, appErr := s.Handover(ctx, HandoverAuditParams{
			TaskID:           target.TaskID,
			FromAuditorID:    actor.ID,
			ToAuditorID:      p.ToAuditorID,
			Reason:           p.Reason,
			CurrentJudgement: p.CurrentJudgement,
			RiskRemark:       p.RiskRemark,
		})
		if appErr != nil {
			item.Message = auditBatchErrorMessage(appErr)
			resp.FailureCount++
			resp.Results = append(resp.Results, item)
			continue
		}
		item.Status = "success"
		item.Message = "created"
		if handover != nil {
			item.HandoverID = &handover.ID
		}
		resp.SuccessCount++
		resp.Results = append(resp.Results, item)
	}
	return resp, nil
}

type batchAuditHandoverTarget struct {
	TaskID int64
	TaskNo string
}

func (s *auditV7Service) resolveBatchHandoverTargets(ctx context.Context, actor domain.RequestActor, p BatchAuditHandoverParams) ([]batchAuditHandoverTarget, *domain.AppError) {
	switch p.Mode {
	case BatchAuditHandoverModeExplicit:
		taskIDs := dedupePositiveTaskIDs(p.TaskIDs)
		if len(taskIDs) == 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_ids is required for explicit mode", nil)
		}
		if len(taskIDs) > AuditHandoverBatchDefaultLimit {
			return nil, auditHandoverBatchLimitExceeded(int64(len(taskIDs)))
		}
		targets := make([]batchAuditHandoverTarget, 0, len(taskIDs))
		for _, taskID := range taskIDs {
			targets = append(targets, batchAuditHandoverTarget{TaskID: taskID})
		}
		return targets, nil
	case BatchAuditHandoverModeAllMatching:
		filter := p.Filters
		filter.Page = 1
		filter.PageSize = 100
		first, appErr := s.listHandoverCandidatesForActor(ctx, actor, filter)
		if appErr != nil {
			return nil, appErr
		}
		if first.EligibleCount > int64(AuditHandoverBatchDefaultLimit) {
			return nil, auditHandoverBatchLimitExceeded(first.EligibleCount)
		}
		targets := candidateItemsToBatchTargets(first.Items)
		for page := 2; int64(len(targets)) < first.EligibleCount; page++ {
			filter.Page = page
			next, appErr := s.listHandoverCandidatesForActor(ctx, actor, filter)
			if appErr != nil {
				return nil, appErr
			}
			if len(next.Items) == 0 {
				break
			}
			targets = append(targets, candidateItemsToBatchTargets(next.Items)...)
		}
		return targets, nil
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "mode must be explicit or all_matching", nil)
	}
}

func (s *auditV7Service) listHandoverCandidatesForActor(ctx context.Context, actor domain.RequestActor, filter AuditHandoverCandidateFilter) (*AuditHandoverCandidateListResponse, *domain.AppError) {
	normalized, appErr := normalizeAuditHandoverCandidateFilter(filter)
	if appErr != nil {
		return nil, appErr
	}
	statuses := []domain.TaskStatus{domain.TaskStatusPendingAudit}
	ownerOrgTeams := []string{}
	if normalized.OwnerOrgTeam != "" {
		ownerOrgTeams = []string{normalized.OwnerOrgTeam}
	}
	access := domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionTaskAuditDecision)
	handlerID := actor.ID
	items, total, err := s.taskRepo.List(ctx, repo.TaskListFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Statuses:      statuses,
			OwnerOrgTeams: ownerOrgTeams,
		},
		CurrentHandlerID:            &handlerID,
		Keyword:                     normalized.Keyword,
		ExcludePendingAuditHandover: true,
		ScopeViewAll:                access.Global,
		ScopeUserIDs:                v8ScopeUserIDs(access),
		ScopeDepartmentIDs:          access.DepartmentIDs,
		ScopeTeamIDs:                access.TeamIDs,
		Page:                        normalized.Page,
		PageSize:                    normalized.PageSize,
	})
	if err != nil {
		return nil, infraError("list audit handover candidates", err)
	}
	items = hydrateTaskListItems(items)
	responseItems := make([]AuditHandoverCandidateItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		responseItems = append(responseItems, AuditHandoverCandidateItem{
			TaskID:             item.ID,
			TaskNo:             item.TaskNo,
			SKUCode:            item.SKUCode,
			PrimarySKUCode:     item.PrimarySKUCode,
			ProductName:        item.ProductNameSnapshot,
			TaskStatus:         item.TaskStatus,
			OwnerOrgTeam:       item.OwnerOrgTeam,
			CurrentHandlerID:   item.CurrentHandlerID,
			CurrentHandlerName: item.CurrentHandlerName,
			UpdatedAt:          item.UpdatedAt,
		})
	}
	return &AuditHandoverCandidateListResponse{
		Items:         responseItems,
		Pagination:    buildPaginationMeta(normalized.Page, normalized.PageSize, total),
		EligibleCount: total,
		SelectedLimit: AuditHandoverBatchDefaultLimit,
	}, nil
}

func normalizeAuditHandoverCandidateFilter(filter AuditHandoverCandidateFilter) (AuditHandoverCandidateFilter, *domain.AppError) {
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	filter.OwnerOrgTeam = strings.TrimSpace(filter.OwnerOrgTeam)
	filter.Status = domain.TaskStatus(strings.TrimSpace(string(filter.Status)))
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	switch filter.Status {
	case "", domain.TaskStatusPendingAudit:
		return filter, nil
	default:
		return AuditHandoverCandidateFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "status must be PendingAudit", nil)
	}
}

func auditHandoverActorFromContext(ctx context.Context) (domain.RequestActor, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return domain.RequestActor{}, domain.ErrUnauthorized
	}
	actor.Roles = domain.NormalizeRoleValues(actor.Roles)
	return actor, nil
}

func authorizeAuditHandoverBatchActor(actor domain.RequestActor) *domain.AppError {
	if domain.ActorHasPermission(actor, domain.PermissionTaskAuditDecision) && actor.EffectiveAccess != nil {
		return nil
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "task.audit.decision is required", map[string]interface{}{
		"action":      string(TaskActionAuditHandover),
		"deny_code":   "missing_required_capability",
		"deny_reason": "missing_required_capability",
		"actor_id":    actor.ID,
	})
}

func dedupePositiveTaskIDs(taskIDs []int64) []int64 {
	if len(taskIDs) == 0 {
		return nil
	}
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID <= 0 {
			continue
		}
		if _, exists := seen[taskID]; exists {
			continue
		}
		seen[taskID] = struct{}{}
		out = append(out, taskID)
	}
	return out
}

func candidateItemsToBatchTargets(items []AuditHandoverCandidateItem) []batchAuditHandoverTarget {
	targets := make([]batchAuditHandoverTarget, 0, len(items))
	for _, item := range items {
		targets = append(targets, batchAuditHandoverTarget{
			TaskID: item.TaskID,
			TaskNo: item.TaskNo,
		})
	}
	return targets
}

func auditHandoverBatchLimitExceeded(eligibleCount int64) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "audit handover batch exceeds selected limit", map[string]interface{}{
		"deny_code":      "BATCH_LIMIT_EXCEEDED",
		"eligible_count": eligibleCount,
		"selected_limit": AuditHandoverBatchDefaultLimit,
	})
}

func auditBatchErrorMessage(appErr *domain.AppError) string {
	if appErr == nil {
		return ""
	}
	if appErr.Message != "" {
		return appErr.Message
	}
	return appErr.Code
}

func (s *auditV7Service) Takeover(ctx context.Context, taskID, handoverID, auditorID int64) *domain.AppError {
	handover, err := s.auditV7Repo.GetHandoverByID(ctx, handoverID)
	if err != nil {
		return infraError("get handover", err)
	}
	if handover == nil {
		return domain.ErrNotFound
	}
	if handover.TaskID != taskID {
		return domain.NewAppError(domain.ErrCodeInvalidRequest,
			fmt.Sprintf("handover %d does not belong to task %d", handoverID, taskID), nil)
	}
	if handover.ToAuditorID != auditorID {
		return domain.NewAppError(domain.ErrCodeInvalidRequest,
			fmt.Sprintf("handover %d is assigned to auditor %d, not %d",
				handoverID, handover.ToAuditorID, auditorID), nil)
	}
	if handover.Status != domain.HandoverStatusPendingTakeover {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("handover %d is in status %q, not pending_takeover", handoverID, handover.Status), nil)
	}

	task, appErr := s.getTask(ctx, handover.TaskID)
	if appErr != nil {
		return appErr
	}
	if task.TaskStatus != domain.TaskStatusPendingAudit {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d in status %q cannot take over audit",
				handover.TaskID, task.TaskStatus), nil)
	}
	subject := task.AccessSubject()
	subject.CurrentHandlerID = &auditorID
	if appErr := authorizeV8AuditSubject(ctx, subject, domain.PermissionTaskAuditDecision, auditorID); appErr != nil {
		return appErr
	}
	stage := domain.AuditRecordStageUnified
	expectedWorkflowRevision := task.WorkflowRevision

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		taskLockingRepo, ok := s.taskRepo.(auditV8TaskLockingRepo)
		if !ok {
			return fmt.Errorf("v8 audit takeover task locking is unavailable")
		}
		handoverLockingRepo, ok := s.auditV7Repo.(auditV8HandoverLockingRepo)
		if !ok {
			return fmt.Errorf("v8 audit takeover handover locking is unavailable")
		}
		lockedTask, err := taskLockingRepo.GetByIDForUpdate(ctx, tx, task.ID)
		if err != nil {
			return fmt.Errorf("lock task for v8 audit takeover: %w", err)
		}
		lockedHandover, err := handoverLockingRepo.GetHandoverByIDForUpdate(ctx, tx, handoverID)
		if err != nil {
			return fmt.Errorf("lock handover for v8 audit takeover: %w", err)
		}
		if lockedTask == nil || lockedHandover == nil ||
			lockedTask.TaskStatus != domain.TaskStatusPendingAudit || lockedTask.WorkflowRevision != expectedWorkflowRevision ||
			lockedTask.CurrentHandlerID != nil || lockedHandover.Status != domain.HandoverStatusPendingTakeover ||
			lockedHandover.TaskID != taskID || lockedHandover.ToAuditorID != auditorID {
			return repo.ErrConflict
		}
		updated, err := handoverLockingRepo.CASUpdateHandoverStatus(ctx, tx, handoverID, domain.HandoverStatusPendingTakeover, domain.HandoverStatusTakenOver)
		if err != nil {
			return err
		}
		if !updated {
			return repo.ErrConflict
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, handover.TaskID, &auditorID); err != nil {
			return err
		}
		if _, err := s.auditV7Repo.CreateRecord(ctx, tx, &domain.AuditRecord{
			TaskID:         handover.TaskID,
			Stage:          stage,
			Action:         domain.AuditActionTypeTakeover,
			AuditorID:      auditorID,
			IssueTypesJSON: "[]",
		}); err != nil {
			return fmt.Errorf("takeover audit record: %w", err)
		}
		_, err = s.taskEventRepo.Append(ctx, tx, handover.TaskID, domain.TaskEventAuditTakenOver, &auditorID,
			taskTransitionEventPayload(task, task.TaskStatus, task.TaskStatus, task.CurrentHandlerID, &auditorID, map[string]interface{}{
				"handover_id": handoverID,
				"auditor_id":  auditorID,
				"stage":       string(stage),
			}))
		return err
	})
	if txErr != nil {
		if errors.Is(txErr, repo.ErrConflict) {
			return domain.NewAppError(domain.ErrCodeConflict, "task or audit handover changed concurrently; refresh and retry", map[string]interface{}{
				"deny_code":   "audit_takeover_concurrent_change",
				"task_id":     taskID,
				"handover_id": handoverID,
			})
		}
		return infraError("takeover tx", txErr)
	}
	return nil
}

func (s *auditV7Service) ListHandovers(ctx context.Context, taskID int64) ([]*domain.AuditHandover, *domain.AppError) {
	task, appErr := s.getTask(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	handovers, err := s.auditV7Repo.ListHandoversByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list handovers", err)
	}
	if appErr := s.authorizeListHandovers(ctx, task, handovers); appErr != nil {
		return nil, appErr
	}
	actor, _ := domain.RequestActorFromContext(ctx)
	for _, handover := range handovers {
		if handover == nil {
			continue
		}
		handover.AllowedActions = []string{}
		if task.TaskStatus != domain.TaskStatusPendingAudit || handover.Status != domain.HandoverStatusPendingTakeover || handover.ToAuditorID != actor.ID {
			continue
		}
		prospectiveSubject := task.AccessSubject()
		prospectiveSubject.CurrentHandlerID = &actor.ID
		if domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, prospectiveSubject) {
			handover.AllowedActions = append(handover.AllowedActions, "task.audit.takeover")
		}
	}
	return handovers, nil
}

func (s *auditV7Service) authorizeListHandovers(ctx context.Context, task *domain.Task, handovers []*domain.AuditHandover) *domain.AppError {
	if task == nil || task.TaskStatus != domain.TaskStatusPendingAudit {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "audit handovers are only available while a task is pending audit", nil)
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return domain.ErrUnauthorized
	}
	if domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskView, task.AccessSubject()) || domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, task.AccessSubject()) {
		return nil
	}
	prospectiveSubject := task.AccessSubject()
	prospectiveSubject.CurrentHandlerID = &actor.ID
	for _, handover := range handovers {
		if handover != nil && handover.Status == domain.HandoverStatusPendingTakeover && handover.ToAuditorID == actor.ID &&
			domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, prospectiveSubject) {
			return nil
		}
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "task handovers are outside the effective data scope", map[string]interface{}{
		"deny_code": "audit_handover_list_out_of_scope",
		"task_id":   task.ID,
	})
}

func (s *auditV7Service) getTask(ctx context.Context, taskID int64) (*domain.Task, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, infraError("get task for audit", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	return task, nil
}

func (s *auditV7Service) ensureNoPendingHandover(ctx context.Context, taskID int64) *domain.AppError {
	handovers, err := s.auditV7Repo.ListHandoversByTaskID(ctx, taskID)
	if err != nil {
		return infraError("list handovers for active audit action", err)
	}
	for _, handover := range handovers {
		if handover != nil && handover.Status == domain.HandoverStatusPendingTakeover {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task has a pending audit handover and must be taken over before continuing", map[string]interface{}{
				"handover_id": handover.ID,
				"handover_no": handover.HandoverNo,
			})
		}
	}
	return nil
}

func activeAuditStageFromStatus(status domain.TaskStatus) (domain.AuditRecordStage, bool) {
	if status == domain.TaskStatusPendingAudit {
		return domain.AuditRecordStageUnified, true
	}
	return "", false
}

func authorizeV8AuditTask(ctx context.Context, task *domain.Task, permission domain.PermissionCode, actorID int64) *domain.AppError {
	if task == nil {
		return domain.ErrNotFound
	}
	return authorizeV8AuditSubject(ctx, task.AccessSubject(), permission, actorID)
}

func authorizeV8AuditSubject(ctx context.Context, subject domain.TaskAccessSubject, permission domain.PermissionCode, actorID int64) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return domain.ErrUnauthorized
	}
	if actor.ID != actorID || !domain.EffectiveAccessAllowsTask(actor, permission, subject) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "task audit action is outside the effective data scope", map[string]interface{}{
			"deny_code": "task_audit_out_of_scope",
			"task_id":   subject.TaskID,
			"actor_id":  actor.ID,
		})
	}
	return nil
}

func v8ScopeUserIDs(access domain.ResourceGroupAccessFilter) []int64 {
	if access.Self && access.ActorID > 0 {
		return []int64{access.ActorID}
	}
	return nil
}

func (s *auditV7Service) authorizeV8AuditTarget(ctx context.Context, task *domain.Task, targetID int64) *domain.AppError {
	if targetID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "to_auditor_id is required", nil)
	}
	if s.v8AccessResolver == nil || s.scopeUserRepo == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "v8 audit target authorization is unavailable", nil)
	}
	user, err := s.scopeUserRepo.GetByID(ctx, targetID)
	if err != nil {
		return infraError("get v8 audit target", err)
	}
	if user == nil || user.Status != domain.UserStatusActive {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "target auditor must be an active user", map[string]interface{}{"to_auditor_id": targetID})
	}
	effective, appErr := s.v8AccessResolver.EffectiveAccess(ctx, targetID)
	if appErr != nil {
		return appErr
	}
	target := domain.RequestActor{
		ID:              targetID,
		DepartmentID:    user.DepartmentID,
		TeamID:          user.TeamID,
		EffectiveAccess: effective,
	}
	if effective != nil {
		target.Permissions = effective.Permissions
	}
	prospectiveSubject := task.AccessSubject()
	prospectiveSubject.CurrentHandlerID = &targetID
	if !domain.EffectiveAccessAllowsTask(target, domain.PermissionTaskAuditDecision, prospectiveSubject) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "target auditor is outside the effective task audit scope", map[string]interface{}{
			"deny_code":     "target_auditor_out_of_scope",
			"task_id":       task.ID,
			"to_auditor_id": targetID,
		})
	}
	return nil
}
