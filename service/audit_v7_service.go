package service

import (
	"context"
	"fmt"
	"log"

	"workflow/domain"
	"workflow/repo"
)

type auditV7Service struct {
	taskRepo          repo.TaskRepo
	auditV7Repo       repo.AuditV7Repo
	taskEventRepo     repo.TaskEventRepo
	codeRuleSvc       CodeRuleService
	txRunner          repo.TxRunner
	filingTrigger     auditTaskFilingTrigger
	dataScopeResolver DataScopeResolver
	scopeUserRepo     repo.UserRepo
}

type auditTaskFilingTrigger interface {
	TriggerFiling(ctx context.Context, p TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError)
}

type taskNeedOutsourceUpdater interface {
	UpdateNeedOutsource(ctx context.Context, tx repo.Tx, id int64, needOutsource bool) error
}

type AuditV7ServiceOption func(*auditV7Service)

func WithAuditV7FilingTrigger(trigger auditTaskFilingTrigger) AuditV7ServiceOption {
	return func(s *auditV7Service) {
		s.filingTrigger = trigger
	}
}

func WithAuditV7DataScopeResolver(resolver DataScopeResolver) AuditV7ServiceOption {
	return func(s *auditV7Service) {
		s.dataScopeResolver = resolver
	}
}

func WithAuditV7ScopeUserRepo(userRepo repo.UserRepo) AuditV7ServiceOption {
	return func(s *auditV7Service) {
		s.scopeUserRepo = userRepo
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

func (s *auditV7Service) taskActionAuthorizer() *taskActionAuthorizer {
	return newTaskActionAuthorizer(s.dataScopeResolver, s.scopeUserRepo)
}

func (s *auditV7Service) Claim(ctx context.Context, p ClaimAuditParams) *domain.AppError {
	task, appErr := s.getTask(ctx, p.TaskID)
	if appErr != nil {
		return appErr
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskActionWithAttributes(ctx, TaskActionAuditClaim, task, TaskActionAttributes{
		AuditStage: p.Stage,
	}); appErr != nil {
		return appErr
	}
	if !isClaimableStatus(task.TaskStatus, p.Stage) {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d in status %q cannot be claimed for stage %q; claim requires PendingAuditA (stage A), PendingAuditB (stage B), or PendingOutsourceReview (stage outsource_review)",
				p.TaskID, task.TaskStatus, p.Stage), map[string]interface{}{
				"task_id":            p.TaskID,
				"task_status":        string(task.TaskStatus),
				"stage":              string(p.Stage),
				"current_handler_id": task.CurrentHandlerID,
			})
	}
	if appErr := s.ensureNoPendingHandover(ctx, p.TaskID); appErr != nil {
		return appErr
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.auditV7Repo.CreateRecord(ctx, tx, &domain.AuditRecord{
			TaskID:         p.TaskID,
			Stage:          p.Stage,
			Action:         domain.AuditActionTypeClaim,
			AuditorID:      p.AuditorID,
			IssueTypesJSON: "[]",
		}); err != nil {
			return fmt.Errorf("audit claim record: %w", err)
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, &p.AuditorID); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventAuditClaimed, &p.AuditorID,
			taskTransitionEventPayload(task, task.TaskStatus, task.TaskStatus, task.CurrentHandlerID, &p.AuditorID, map[string]interface{}{
				"auditor_id": p.AuditorID,
				"stage":      string(p.Stage),
			}))
		return err
	})
	if txErr != nil {
		return infraError("claim audit tx", txErr)
	}
	return nil
}

func (s *auditV7Service) Approve(ctx context.Context, p ApproveAuditParams) *domain.AppError {
	task, appErr := s.getTask(ctx, p.TaskID)
	if appErr != nil {
		return appErr
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicyWithAttributes(ctx, TaskActionAuditApprove, task, "", "", TaskActionAttributes{
		AuditStage: p.Stage,
	})
	authz.logDecision(TaskActionAuditApprove, decision)
	if !decision.Allowed {
		return taskActionDecisionAppError(TaskActionAuditApprove, decision)
	}
	if !isClaimableStatus(task.TaskStatus, p.Stage) {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d in status %q cannot be approved for stage %q",
				p.TaskID, task.TaskStatus, p.Stage), nil)
	}
	if !validApproveTransition(task.TaskStatus, p.NextStatus) {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("transition %q -> %q is not a valid approval path",
				task.TaskStatus, p.NextStatus), nil)
	}
	if appErr := s.ensureNoPendingHandover(ctx, p.TaskID); appErr != nil {
		return appErr
	}

	issueJSON := issueTypesToJSON(p.IssueTypes)
	needOutsource := p.NextStatus == domain.TaskStatusPendingOutsource
	var nextHandlerID *int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.auditV7Repo.CreateRecord(ctx, tx, &domain.AuditRecord{
			TaskID:         p.TaskID,
			Stage:          p.Stage,
			Action:         domain.AuditActionTypeApprove,
			AuditorID:      p.AuditorID,
			IssueTypesJSON: issueJSON,
			Comment:        p.Comment,
			NeedOutsource:  needOutsource,
		}); err != nil {
			return fmt.Errorf("audit approve record: %w", err)
		}
		if err := s.taskRepo.UpdateStatus(ctx, tx, p.TaskID, p.NextStatus); err != nil {
			return err
		}
		if needOutsource {
			if updater, ok := s.taskRepo.(taskNeedOutsourceUpdater); ok {
				if err := updater.UpdateNeedOutsource(ctx, tx, p.TaskID, true); err != nil {
					return err
				}
			}
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, nextHandlerID); err != nil {
			return err
		}
		eventExtra := map[string]interface{}{
			"auditor_id":     p.AuditorID,
			"stage":          string(p.Stage),
			"next_status":    string(p.NextStatus),
			"comment":        p.Comment,
			"need_outsource": needOutsource,
		}
		if p.ReplacementAssetID != nil {
			eventExtra["current_asset_id"] = *p.ReplacementAssetID
			eventExtra["replacement_actor_id"] = p.AuditorID
			if p.PreviousAssetID != nil {
				eventExtra["previous_asset_id"] = *p.PreviousAssetID
			}
			if p.ReplacementNote != "" {
				eventExtra["replacement_note"] = p.ReplacementNote
			}
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventAuditApproved, &p.AuditorID,
			taskTransitionEventPayload(task, task.TaskStatus, p.NextStatus, task.CurrentHandlerID, nextHandlerID, eventExtra))
		return err
	})
	if txErr != nil {
		return infraError("approve audit tx", txErr)
	}
	if isFinalDesignAuditApproval(p.NextStatus) && s.filingTrigger != nil {
		_, filingErr := s.filingTrigger.TriggerFiling(ctx, TriggerTaskFilingParams{
			TaskID:     p.TaskID,
			OperatorID: p.AuditorID,
			Remark:     p.Comment,
			Source:     TaskFilingTriggerSourceAuditFinalApproved,
			Force:      false,
		})
		if filingErr != nil {
			log.Printf("audit_final_approval_filing_trigger_failed task_id=%d err=%s", p.TaskID, filingErr.Message)
		}
	}
	return nil
}

func (s *auditV7Service) Reject(ctx context.Context, p RejectAuditParams) *domain.AppError {
	task, appErr := s.getTask(ctx, p.TaskID)
	if appErr != nil {
		return appErr
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicyWithAttributes(ctx, TaskActionAuditReject, task, "", "", TaskActionAttributes{
		AuditStage: p.Stage,
	})
	authz.logDecision(TaskActionAuditReject, decision)
	if !decision.Allowed {
		return taskActionDecisionAppError(TaskActionAuditReject, decision)
	}
	if !isClaimableStatus(task.TaskStatus, p.Stage) {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d in status %q cannot be rejected for stage %q",
				p.TaskID, task.TaskStatus, p.Stage), nil)
	}
	nextStatus, ok := rejectedStatusForStage(p.Stage)
	if !ok {
		return domain.NewAppError(domain.ErrCodeInvalidRequest,
			fmt.Sprintf("no rejection status defined for stage %q", p.Stage), nil)
	}
	if appErr := s.ensureNoPendingHandover(ctx, p.TaskID); appErr != nil {
		return appErr
	}

	issueJSON := issueTypesToJSON(p.IssueTypes)
	nextHandlerID := cloneInt64Ptr(task.DesignerID)
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.auditV7Repo.CreateRecord(ctx, tx, &domain.AuditRecord{
			TaskID:         p.TaskID,
			Stage:          p.Stage,
			Action:         domain.AuditActionTypeReject,
			AuditorID:      p.AuditorID,
			IssueTypesJSON: issueJSON,
			Comment:        p.Comment,
			AffectsLaunch:  p.AffectsLaunch,
		}); err != nil {
			return fmt.Errorf("audit reject record: %w", err)
		}
		if err := s.taskRepo.UpdateStatus(ctx, tx, p.TaskID, nextStatus); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, nextHandlerID); err != nil {
			return err
		}
		rejectExtra := map[string]interface{}{
			"auditor_id":     p.AuditorID,
			"stage":          string(p.Stage),
			"next_status":    string(nextStatus),
			"comment":        p.Comment,
			"affects_launch": p.AffectsLaunch,
			"designer_id":    cloneInt64Ptr(task.DesignerID),
		}
		if p.ReplacementAssetID != nil {
			rejectExtra["current_asset_id"] = *p.ReplacementAssetID
			rejectExtra["replacement_actor_id"] = p.AuditorID
			if p.PreviousAssetID != nil {
				rejectExtra["previous_asset_id"] = *p.PreviousAssetID
			}
			if p.ReplacementNote != "" {
				rejectExtra["replacement_note"] = p.ReplacementNote
			}
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventAuditRejected, &p.AuditorID,
			taskTransitionEventPayload(task, task.TaskStatus, nextStatus, task.CurrentHandlerID, nextHandlerID, rejectExtra))
		return err
	})
	if txErr != nil {
		return infraError("reject audit tx", txErr)
	}
	return nil
}

func (s *auditV7Service) Transfer(ctx context.Context, p TransferAuditParams) *domain.AppError {
	task, appErr := s.getTask(ctx, p.TaskID)
	if appErr != nil {
		return appErr
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicyWithAttributes(ctx, TaskActionAuditTransfer, task, "", "", TaskActionAttributes{
		AuditStage: p.Stage,
	})
	authz.logDecision(TaskActionAuditTransfer, decision)
	if !decision.Allowed {
		return taskActionDecisionAppError(TaskActionAuditTransfer, decision)
	}
	stage, ok := activeAuditStageFromStatus(task.TaskStatus)
	if !ok {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d in status %q cannot be transferred",
				p.TaskID, task.TaskStatus), nil)
	}
	if stage != p.Stage {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d is in audit stage %q, not %q",
				p.TaskID, stage, p.Stage), nil)
	}
	if appErr := s.ensureNoPendingHandover(ctx, p.TaskID); appErr != nil {
		return appErr
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.auditV7Repo.CreateRecord(ctx, tx, &domain.AuditRecord{
			TaskID:         p.TaskID,
			Stage:          p.Stage,
			Action:         domain.AuditActionTypeTransfer,
			AuditorID:      p.FromAuditorID,
			IssueTypesJSON: "[]",
			Comment:        p.Comment,
		}); err != nil {
			return fmt.Errorf("audit transfer record: %w", err)
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, &p.ToAuditorID); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventAuditTransferred, &p.FromAuditorID,
			taskTransitionEventPayload(task, task.TaskStatus, task.TaskStatus, task.CurrentHandlerID, &p.ToAuditorID, map[string]interface{}{
				"from_auditor_id": p.FromAuditorID,
				"to_auditor_id":   p.ToAuditorID,
				"stage":           string(p.Stage),
				"comment":         p.Comment,
			}))
		return err
	})
	if txErr != nil {
		return infraError("transfer audit tx", txErr)
	}
	return nil
}

func (s *auditV7Service) Handover(ctx context.Context, p HandoverAuditParams) (*domain.AuditHandover, *domain.AppError) {
	task, appErr := s.getTask(ctx, p.TaskID)
	if appErr != nil {
		return nil, appErr
	}
	stage, ok := activeAuditStageFromStatus(task.TaskStatus)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d in status %q cannot be handed over",
				p.TaskID, task.TaskStatus), nil)
	}
	authz := s.taskActionAuthorizer()
	decision := authz.EvaluateTaskActionPolicyWithAttributes(ctx, TaskActionAuditHandover, task, "", "", TaskActionAttributes{
		AuditStage: stage,
	})
	authz.logDecision(TaskActionAuditHandover, decision)
	if !decision.Allowed {
		return nil, taskActionDecisionAppError(TaskActionAuditHandover, decision)
	}
	if appErr := s.ensureNoPendingHandover(ctx, p.TaskID); appErr != nil {
		return nil, appErr
	}
	if p.FromAuditorID == p.ToAuditorID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "to_auditor_id must be different from from_auditor_id", nil)
	}

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
		return nil, infraError("handover tx", txErr)
	}

	handover.ID = newID
	return handover, nil
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
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionAuditTakeover, task); appErr != nil {
		return appErr
	}
	stage, ok := activeAuditStageFromStatus(task.TaskStatus)
	if !ok {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d in status %q cannot take over audit",
				handover.TaskID, task.TaskStatus), nil)
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.auditV7Repo.UpdateHandoverStatus(ctx, tx, handoverID, domain.HandoverStatusTakenOver); err != nil {
			return err
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
		_, err := s.taskEventRepo.Append(ctx, tx, handover.TaskID, domain.TaskEventAuditTakenOver, &auditorID,
			taskTransitionEventPayload(task, task.TaskStatus, task.TaskStatus, task.CurrentHandlerID, &auditorID, map[string]interface{}{
				"handover_id": handoverID,
				"auditor_id":  auditorID,
				"stage":       string(stage),
			}))
		return err
	})
	if txErr != nil {
		return infraError("takeover tx", txErr)
	}
	return nil
}

func (s *auditV7Service) ListHandovers(ctx context.Context, taskID int64) ([]*domain.AuditHandover, *domain.AppError) {
	if _, appErr := s.getTask(ctx, taskID); appErr != nil {
		return nil, appErr
	}

	handovers, err := s.auditV7Repo.ListHandoversByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list handovers", err)
	}
	return handovers, nil
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

func isClaimableStatus(status domain.TaskStatus, stage domain.AuditRecordStage) bool {
	currentStage, ok := activeAuditStageFromStatus(status)
	return ok && currentStage == stage
}

func activeAuditStageFromStatus(status domain.TaskStatus) (domain.AuditRecordStage, bool) {
	switch status {
	case domain.TaskStatusPendingAuditA:
		return domain.AuditRecordStageA, true
	case domain.TaskStatusPendingAuditB:
		return domain.AuditRecordStageB, true
	case domain.TaskStatusPendingOutsourceReview:
		return domain.AuditRecordStageOutsourceReview, true
	default:
		return "", false
	}
}

func rejectedStatusForStage(stage domain.AuditRecordStage) (domain.TaskStatus, bool) {
	switch stage {
	case domain.AuditRecordStageA:
		return domain.TaskStatusRejectedByAuditA, true
	case domain.AuditRecordStageB:
		return domain.TaskStatusRejectedByAuditB, true
	}
	return "", false
}

func validApproveTransition(current, next domain.TaskStatus) bool {
	switch current {
	case domain.TaskStatusPendingAuditA:
		return next == domain.TaskStatusPendingAuditB ||
			next == domain.TaskStatusPendingWarehouseReceive ||
			next == domain.TaskStatusPendingOutsource
	case domain.TaskStatusPendingAuditB:
		return next == domain.TaskStatusPendingWarehouseReceive
	case domain.TaskStatusPendingOutsourceReview:
		return next == domain.TaskStatusPendingWarehouseReceive
	}
	return false
}

func issueTypesToJSON(types []string) string {
	if len(types) == 0 {
		return "[]"
	}
	out := `[`
	for i, t := range types {
		if i > 0 {
			out += ","
		}
		out += `"` + t + `"`
	}
	out += `]`
	return out
}
