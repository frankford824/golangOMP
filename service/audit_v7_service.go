package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	auditLanePinnedNormalAuditorName        = "马雨琪"
	auditLanePinnedCustomizationAuditorName = "章鹏鹏"
)

type auditLaneAccessSubject struct {
	UserID int64
	Label  string
}

type auditLaneProfile struct {
	UserID      int64
	Username    string
	DisplayName string
	Allowed     map[domain.TaskBusinessLane]struct{}
}

type auditV7Service struct {
	taskRepo          repo.TaskRepo
	auditV7Repo       repo.AuditV7Repo
	taskEventRepo     repo.TaskEventRepo
	codeRuleSvc       CodeRuleService
	txRunner          repo.TxRunner
	filingTrigger     auditTaskFilingTrigger
	dataScopeResolver DataScopeResolver
	scopeUserRepo     repo.UserRepo
	assetFlowRepo     AuditAssetFlowRepo
	experienceSvc     ExperienceService
}

type auditTaskFilingTrigger interface {
	TriggerFiling(ctx context.Context, p TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError)
}

type taskNeedOutsourceUpdater interface {
	UpdateNeedOutsource(ctx context.Context, tx repo.Tx, id int64, needOutsource bool) error
}

type AuditAssetFlowRepo interface {
	MarkCurrentDeliveryVersionsApprovedForTask(ctx context.Context, tx repo.Tx, taskID, actorID int64, approvedAt time.Time) (int64, error)
	MarkCurrentDeliveryVersionsRejectedForTask(ctx context.Context, tx repo.Tx, taskID, actorID int64, rejectedAt time.Time) (int64, error)
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

func WithAuditV7AssetFlowRepo(assetFlowRepo AuditAssetFlowRepo) AuditV7ServiceOption {
	return func(s *auditV7Service) {
		s.assetFlowRepo = assetFlowRepo
	}
}

func WithAuditV7ExperienceService(experienceSvc ExperienceService) AuditV7ServiceOption {
	return func(s *auditV7Service) {
		s.experienceSvc = experienceSvc
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
	if appErr := s.ensureAuditLanePolicy(ctx, task, string(TaskActionAuditClaim), []auditLaneAccessSubject{
		{UserID: p.AuditorID, Label: "auditor_id"},
	}); appErr != nil {
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
	fromStatus := task.TaskStatus
	if appErr := s.ensureAuditLanePolicy(ctx, task, string(TaskActionAuditApprove), []auditLaneAccessSubject{
		{UserID: p.AuditorID, Label: "auditor_id"},
	}); appErr != nil {
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
		if p.NextStatus == domain.TaskStatusPendingWarehouseReceive && s.assetFlowRepo != nil {
			if _, err := s.assetFlowRepo.MarkCurrentDeliveryVersionsApprovedForTask(ctx, tx, p.TaskID, p.AuditorID, time.Now().UTC()); err != nil {
				return err
			}
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
	s.enqueueAuditExperienceEvent(ctx, task, auditExperienceEventParams{
		AuditorID:  p.AuditorID,
		Stage:      p.Stage,
		Action:     domain.AuditActionTypeApprove,
		FromStatus: fromStatus,
		ToStatus:   p.NextStatus,
		Outcome:    "approved",
		Comment:    p.Comment,
		IssueTypes: p.IssueTypes,
		Extra: map[string]interface{}{
			"need_outsource": needOutsource,
		},
	})
	return nil
}

func (s *auditV7Service) Reject(ctx context.Context, p RejectAuditParams) *domain.AppError {
	task, appErr := s.getTask(ctx, p.TaskID)
	if appErr != nil {
		return appErr
	}
	fromStatus := task.TaskStatus
	if appErr := s.ensureAuditLanePolicy(ctx, task, string(TaskActionAuditReject), []auditLaneAccessSubject{
		{UserID: p.AuditorID, Label: "auditor_id"},
	}); appErr != nil {
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
		if s.assetFlowRepo != nil {
			if _, err := s.assetFlowRepo.MarkCurrentDeliveryVersionsRejectedForTask(ctx, tx, p.TaskID, p.AuditorID, time.Now().UTC()); err != nil {
				return err
			}
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
	s.enqueueAuditExperienceEvent(ctx, task, auditExperienceEventParams{
		AuditorID:     p.AuditorID,
		Stage:         p.Stage,
		Action:        domain.AuditActionTypeReject,
		FromStatus:    fromStatus,
		ToStatus:      nextStatus,
		Outcome:       "rejected",
		Comment:       p.Comment,
		IssueTypes:    p.IssueTypes,
		AffectsLaunch: p.AffectsLaunch,
	})
	return nil
}

type auditExperienceEventParams struct {
	AuditorID     int64
	Stage         domain.AuditRecordStage
	Action        domain.AuditActionType
	FromStatus    domain.TaskStatus
	ToStatus      domain.TaskStatus
	Outcome       string
	Comment       string
	IssueTypes    []string
	AffectsLaunch bool
	Extra         map[string]interface{}
}

func (s *auditV7Service) enqueueAuditExperienceEvent(ctx context.Context, task *domain.Task, p auditExperienceEventParams) {
	if s == nil || s.experienceSvc == nil || task == nil {
		return
	}
	occurredAt := time.Now().UTC()
	payload := map[string]interface{}{
		"stage":              string(p.Stage),
		"audit_action":       string(p.Action),
		"from_task_status":   string(p.FromStatus),
		"to_task_status":     string(p.ToStatus),
		"reason_note":        trimMax(strings.TrimSpace(p.Comment), experienceReasonNoteMaxLength),
		"affects_launch":     p.AffectsLaunch,
		"current_handler_id": cloneInt64Ptr(task.CurrentHandlerID),
		"designer_id":        cloneInt64Ptr(task.DesignerID),
	}
	if len(p.IssueTypes) > 0 {
		payload["reason_codes"] = append([]string(nil), p.IssueTypes...)
	}
	for key, value := range p.Extra {
		payload[key] = value
	}
	taskID := task.ID
	event := &domain.ExperienceOutboxEvent{
		EventKey:           trimMax(fmt.Sprintf("task_audit:%d:%s:%s:%s:%d", task.ID, p.Stage, p.Action, p.ToStatus, occurredAt.UnixNano()), 191),
		SourceType:         "task_audit",
		SourceID:           auditExperienceSourceID(task),
		TaskID:             &taskID,
		Action:             auditExperienceAction(p.Action),
		Outcome:            p.Outcome,
		EventTime:          occurredAt,
		ActorSnapshot:      auditExperienceJSON(map[string]interface{}{"actor_id": p.AuditorID, "actor_type": "user", "surface": "task_audit"}),
		BusinessSnapshot:   auditExperienceJSON(auditExperienceTaskSnapshot(task, p.FromStatus, p.ToStatus)),
		Payload:            auditExperienceJSON(payload),
		DataClassification: "business_fact",
		GroundTruthStatus:  "observed",
	}
	if appErr := s.experienceSvc.EnqueueEvent(ctx, event); appErr != nil {
		log.Printf("audit_experience_enqueue_failed task_id=%d action=%s err=%s", task.ID, p.Action, appErr.Message)
	}
}

func auditExperienceSourceID(task *domain.Task) string {
	if task == nil {
		return ""
	}
	if value := strings.TrimSpace(task.TaskNo); value != "" {
		return trimMax(value, 128)
	}
	return fmt.Sprintf("task-%d", task.ID)
}

func auditExperienceAction(action domain.AuditActionType) string {
	switch action {
	case domain.AuditActionTypeApprove:
		return "audit_approved"
	case domain.AuditActionTypeReject:
		return "audit_rejected"
	default:
		return trimMax("audit_"+strings.TrimSpace(string(action)), 96)
	}
}

func auditExperienceTaskSnapshot(task *domain.Task, fromStatus, toStatus domain.TaskStatus) map[string]interface{} {
	snapshot := map[string]interface{}{}
	if task == nil {
		return snapshot
	}
	snapshot["task_id"] = task.ID
	snapshot["task_no"] = task.TaskNo
	snapshot["task_type"] = string(task.TaskType)
	snapshot["source_mode"] = string(task.SourceMode)
	snapshot["business_lane"] = string(domain.NormalizeTaskBusinessLane(task.BusinessLane, task.CustomizationRequired))
	snapshot["workflow_lane"] = string(task.WorkflowLane())
	snapshot["from_task_status"] = string(fromStatus)
	snapshot["to_task_status"] = string(toStatus)
	snapshot["sku_code"] = task.SKUCode
	snapshot["primary_sku_code"] = task.PrimarySKUCode
	snapshot["product_id"] = cloneInt64Ptr(task.ProductID)
	snapshot["product_name_snapshot"] = task.ProductNameSnapshot
	snapshot["owner_team"] = task.OwnerTeam
	snapshot["owner_department"] = task.OwnerDepartment
	snapshot["owner_org_team"] = task.OwnerOrgTeam
	snapshot["priority"] = string(task.Priority)
	snapshot["need_outsource"] = task.NeedOutsource
	snapshot["is_outsource"] = task.IsOutsource
	snapshot["is_batch_task"] = task.IsBatchTask
	snapshot["batch_item_count"] = task.BatchItemCount
	return snapshot
}

func auditExperienceJSON(value interface{}) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(raw)
}

func (s *auditV7Service) Transfer(ctx context.Context, p TransferAuditParams) *domain.AppError {
	task, appErr := s.getTask(ctx, p.TaskID)
	if appErr != nil {
		return appErr
	}
	if appErr := s.ensureAuditLanePolicy(ctx, task, string(TaskActionAuditTransfer), []auditLaneAccessSubject{
		{UserID: p.FromAuditorID, Label: "from_auditor_id"},
		{UserID: p.ToAuditorID, Label: "to_auditor_id"},
	}); appErr != nil {
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
	if appErr := s.ensureAuditLanePolicy(ctx, task, string(TaskActionAuditHandover), []auditLaneAccessSubject{
		{UserID: p.FromAuditorID, Label: "from_auditor_id"},
		{UserID: p.ToAuditorID, Label: "to_auditor_id"},
	}); appErr != nil {
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
	if appErr := s.ensureAuditLanePolicy(ctx, task, string(TaskActionAuditTakeover), []auditLaneAccessSubject{
		{UserID: handover.FromAuditorID, Label: "handover_from_auditor_id"},
		{UserID: handover.ToAuditorID, Label: "handover_to_auditor_id"},
		{UserID: auditorID, Label: "auditor_id"},
	}); appErr != nil {
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

func (s *auditV7Service) ensureAuditLanePolicy(
	ctx context.Context,
	task *domain.Task,
	action string,
	subjects []auditLaneAccessSubject,
) *domain.AppError {
	lane := domain.NormalizeTaskBusinessLane(task.BusinessLane, task.CustomizationRequired)
	if appErr := s.ensureRequestActorLanePolicy(ctx, lane, action); appErr != nil {
		return appErr
	}
	if s.scopeUserRepo == nil {
		return nil
	}
	seen := map[int64]struct{}{}
	for _, subject := range subjects {
		if subject.UserID <= 0 {
			continue
		}
		if _, exists := seen[subject.UserID]; exists {
			continue
		}
		seen[subject.UserID] = struct{}{}
		profile, appErr := s.resolveAuditLaneProfileByUserID(ctx, subject.UserID)
		if appErr != nil {
			return appErr
		}
		if !profile.allows(lane) {
			return s.auditLaneDeniedError(action, lane, subject, profile)
		}
	}
	return nil
}

func (s *auditV7Service) ensureRequestActorLanePolicy(ctx context.Context, lane domain.TaskBusinessLane, action string) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return nil
	}
	profile, appErr := s.resolveAuditLaneProfileForRequestActor(ctx, actor)
	if appErr != nil {
		return appErr
	}
	if profile == nil || !profile.allows(lane) {
		subject := auditLaneAccessSubject{UserID: actor.ID, Label: "request_actor"}
		return s.auditLaneDeniedError(action, lane, subject, profile)
	}
	return nil
}

func (s *auditV7Service) resolveAuditLaneProfileForRequestActor(ctx context.Context, actor domain.RequestActor) (*auditLaneProfile, *domain.AppError) {
	if actor.ID <= 0 {
		return nil, nil
	}
	if s.scopeUserRepo != nil {
		user, err := s.scopeUserRepo.GetByID(ctx, actor.ID)
		if err != nil {
			return nil, infraError("load request actor for audit lane", err)
		}
		if user != nil {
			if len(user.Roles) == 0 {
				user.Roles = append([]domain.Role(nil), actor.Roles...)
			}
			return buildAuditLaneProfileFromUser(user), nil
		}
	}
	pseudoUser := &domain.User{
		ID:          actor.ID,
		Username:    strings.TrimSpace(actor.Username),
		DisplayName: strings.TrimSpace(actor.Username),
		Department:  domain.Department(strings.TrimSpace(actor.Department)),
		Team:        strings.TrimSpace(actor.Team),
		Roles:       append([]domain.Role(nil), actor.Roles...),
	}
	return buildAuditLaneProfileFromUser(pseudoUser), nil
}

func (s *auditV7Service) resolveAuditLaneProfileByUserID(ctx context.Context, userID int64) (*auditLaneProfile, *domain.AppError) {
	if s.scopeUserRepo == nil {
		return nil, nil
	}
	user, err := s.scopeUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, infraError("load audit subject", err)
	}
	if user == nil {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "audit lane policy: auditor not found", map[string]interface{}{
			"deny_code": "audit_lane_subject_not_found",
			"user_id":   userID,
		})
	}
	return buildAuditLaneProfileFromUser(user), nil
}

func (s *auditV7Service) auditLaneDeniedError(
	action string,
	lane domain.TaskBusinessLane,
	subject auditLaneAccessSubject,
	profile *auditLaneProfile,
) *domain.AppError {
	details := map[string]interface{}{
		"deny_code":     "audit_lane_forbidden",
		"action":        action,
		"business_lane": string(lane),
		"subject":       subject.Label,
		"user_id":       subject.UserID,
	}
	if profile != nil {
		details["username"] = profile.Username
		details["display_name"] = profile.DisplayName
		details["allowed_lanes"] = profile.allowedLaneValues()
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "audit action is outside the auditor business lane", details)
}

func buildAuditLaneProfileFromUser(user *domain.User) *auditLaneProfile {
	if user == nil {
		return nil
	}
	profile := &auditLaneProfile{
		UserID:      user.ID,
		Username:    strings.TrimSpace(user.Username),
		DisplayName: strings.TrimSpace(user.DisplayName),
		Allowed:     map[domain.TaskBusinessLane]struct{}{},
	}
	if profile.DisplayName == auditLanePinnedNormalAuditorName || profile.Username == auditLanePinnedNormalAuditorName {
		profile.Allow(domain.TaskBusinessLaneNormal)
		return profile
	}
	if profile.DisplayName == auditLanePinnedCustomizationAuditorName || profile.Username == auditLanePinnedCustomizationAuditorName {
		profile.Allow(domain.TaskBusinessLaneCustomization)
		return profile
	}
	team := strings.TrimSpace(user.Team)
	switch {
	case strings.Contains(team, "定制"):
		profile.Allow(domain.TaskBusinessLaneCustomization)
	case strings.Contains(team, "普通"), strings.Contains(team, "常规"), strings.Contains(team, "设计审核"):
		profile.Allow(domain.TaskBusinessLaneNormal)
	}
	if hasAnyRoleValue(user.Roles, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer) {
		profile.Allow(domain.TaskBusinessLaneCustomization)
	}
	if hasAnyRoleValue(user.Roles, domain.RoleAuditA, domain.RoleAuditB, domain.RoleDesignReviewer) {
		profile.Allow(domain.TaskBusinessLaneNormal)
	}
	if hasAnyRoleValue(user.Roles,
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleRoleAdmin,
		domain.RoleHRAdmin,
		domain.RoleOrgAdmin,
		domain.RoleDeptAdmin,
		domain.RoleTeamLead,
		domain.RoleDesignDirector,
	) && len(profile.Allowed) == 0 {
		return profile
	}
	return profile
}

func (p *auditLaneProfile) Allow(lane domain.TaskBusinessLane) {
	if p == nil || !lane.Valid() {
		return
	}
	if p.Allowed == nil {
		p.Allowed = map[domain.TaskBusinessLane]struct{}{}
	}
	p.Allowed[lane] = struct{}{}
}

func (p *auditLaneProfile) allows(lane domain.TaskBusinessLane) bool {
	if p == nil || !lane.Valid() {
		return false
	}
	_, ok := p.Allowed[lane]
	return ok
}

func (p *auditLaneProfile) allowedLaneValues() []string {
	if p == nil || len(p.Allowed) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(p.Allowed))
	for lane := range p.Allowed {
		out = append(out, string(lane))
	}
	return out
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
