package service

import "workflow/domain"

type TaskAction string

const (
	TaskActionCreate                       TaskAction = "create"
	TaskActionReadDetail                   TaskAction = "read_detail"
	TaskActionUpdateBusinessInfo           TaskAction = "update_business_info"
	TaskActionAssign                       TaskAction = "assign"
	TaskActionReassign                     TaskAction = "reassign"
	TaskActionSubmitDesign                 TaskAction = "submit_design"
	TaskActionAssetUploadSessionCreate     TaskAction = "asset_upload_session_create"
	TaskActionAssetUploadSessionComplete   TaskAction = "asset_upload_session_complete"
	TaskActionAssetUploadSessionCancel     TaskAction = "asset_upload_session_cancel"
	TaskActionAuditClaim                   TaskAction = "audit_claim"
	TaskActionAuditApprove                 TaskAction = "audit_approve"
	TaskActionAuditReject                  TaskAction = "audit_reject"
	TaskActionAuditTransfer                TaskAction = "audit_transfer"
	TaskActionAuditHandover                TaskAction = "audit_handover"
	TaskActionAuditTakeover                TaskAction = "audit_takeover"
	TaskActionAuditAClaim                  TaskAction = "audit_a_claim"
	TaskActionAuditAApprove                TaskAction = "audit_a_approve"
	TaskActionAuditAReject                 TaskAction = "audit_a_reject"
	TaskActionAuditATransfer               TaskAction = "audit_a_transfer"
	TaskActionAuditAHandover               TaskAction = "audit_a_handover"
	TaskActionAuditATakeover               TaskAction = "audit_a_takeover"
	TaskActionAuditBClaim                  TaskAction = "audit_b_claim"
	TaskActionAuditBApprove                TaskAction = "audit_b_approve"
	TaskActionAuditBReject                 TaskAction = "audit_b_reject"
	TaskActionAuditBTransfer               TaskAction = "audit_b_transfer"
	TaskActionAuditBHandover               TaskAction = "audit_b_handover"
	TaskActionAuditBTakeover               TaskAction = "audit_b_takeover"
	TaskActionAuditOutsourceReviewClaim    TaskAction = "audit_outsource_review_claim"
	TaskActionAuditOutsourceReviewApprove  TaskAction = "audit_outsource_review_approve"
	TaskActionAuditOutsourceReviewReject   TaskAction = "audit_outsource_review_reject"
	TaskActionAuditOutsourceReviewTransfer TaskAction = "audit_outsource_review_transfer"
	TaskActionAuditOutsourceReviewHandover TaskAction = "audit_outsource_review_handover"
	TaskActionAuditOutsourceReviewTakeover TaskAction = "audit_outsource_review_takeover"
	TaskActionWarehousePrepare             TaskAction = "warehouse_prepare"
	TaskActionWarehouseReceive             TaskAction = "warehouse_receive"
	TaskActionWarehouseReject              TaskAction = "warehouse_reject"
	TaskActionWarehouseComplete            TaskAction = "warehouse_complete"
	TaskActionCustomizationReview          TaskAction = "customization_review"
	TaskActionCustomizationEffectPreview   TaskAction = "customization_effect_preview"
	TaskActionCustomizationEffectReview    TaskAction = "customization_effect_review"
	TaskActionCustomizationTransfer        TaskAction = "customization_transfer"
	TaskActionClose                        TaskAction = "close"
	TaskActionUpdateProcurement            TaskAction = "update_procurement"
	TaskActionAdvanceProcurement           TaskAction = "advance_procurement"
)

type taskActionHandlerPolicy string

const (
	taskActionHandlerPolicyNone                     taskActionHandlerPolicy = ""
	taskActionHandlerPolicyRequireCurrentHandler    taskActionHandlerPolicy = "require_current_handler"
	taskActionHandlerPolicyUnassignedOrCurrentActor taskActionHandlerPolicy = "unassigned_or_current_actor"
)

type taskActionRule struct {
	Action            TaskAction
	RequiredRoles     []domain.Role
	AllowedStatuses   []domain.TaskStatus
	AllowedScopes     []TaskActionScopeSource
	PreferHandlerDeny bool
	HandlerPolicy     taskActionHandlerPolicy
	UseReadVisibility bool
	StatusDenyCode    string
	StatusGateMessage string
	RoleGateMessage   string
	ScopeGateMessage  string
	MatchedRule       string
}

func taskActionRuleFor(action TaskAction) taskActionRule {
	managerRoles := []domain.Role{
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleRoleAdmin,
		domain.RoleHRAdmin,
		domain.RoleDeptAdmin,
		domain.RoleTeamLead,
		domain.RoleDesignDirector,
	}
	switch action {
	case TaskActionCreate:
		return taskActionRule{
			Action:           action,
			RequiredRoles:    append([]domain.Role{domain.RoleOps}, managerRoles...),
			AllowedScopes:    []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeDepartment, TaskActionScopeTeam},
			RoleGateMessage:  "task create requires an operation or management role",
			ScopeGateMessage: "task create is outside the actor organization scope",
			MatchedRule:      "role_plus_owner_scope",
		}
	case TaskActionReadDetail:
		return taskActionRule{
			Action: action,
			RequiredRoles: []domain.Role{
				domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse,
				domain.RoleOutsource, domain.RoleCustomizationReviewer, domain.RoleCustomizationOperator,
				domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleOrgAdmin,
				domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector,
			},
			UseReadVisibility: true,
			RoleGateMessage:   "task detail read requires a task-facing role",
			ScopeGateMessage:  "task detail is outside the current data scope",
			MatchedRule:       "role_plus_read_visibility",
		}
	case TaskActionAssign:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleOps}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAssign},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeCreator},
			StatusDenyCode:    "task_status_not_actionable",
			StatusGateMessage: "task action is not allowed in the current status",
			RoleGateMessage:   "task assign requires an operation or management role",
			ScopeGateMessage:  "task assign is outside the actor organization scope",
			MatchedRule:       "role_plus_assignment_scope",
		}
	case TaskActionReassign:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleOps}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusInProgress},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeCreator, TaskActionScopeRequester},
			StatusDenyCode:    "task_not_reassignable",
			StatusGateMessage: "task cannot be reassigned in the current status",
			RoleGateMessage:   "task reassign requires an eligible operation or management role",
			ScopeGateMessage:  "task reassign is outside the actor organization scope",
			MatchedRule:       "ops_or_manager_reassignment_scope",
		}
	case TaskActionSubmitDesign:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleDesigner, domain.RoleOps}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusInProgress, domain.TaskStatusRejectedByAuditA, domain.TaskStatusRejectedByAuditB},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeHandler, TaskActionScopeDesigner},
			PreferHandlerDeny: true,
			RoleGateMessage:   "design submission requires a design, operation, or management role",
			ScopeGateMessage:  "task design action is outside the actor organization scope",
			MatchedRule:       "role_plus_design_scope_or_handler",
		}
	case TaskActionAssetUploadSessionCreate, TaskActionAssetUploadSessionComplete, TaskActionAssetUploadSessionCancel:
		return taskActionRule{
			Action:        action,
			RequiredRoles: append([]domain.Role{domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps}, managerRoles...),
			AllowedStatuses: []domain.TaskStatus{
				domain.TaskStatusPendingAssign,
				domain.TaskStatusInProgress,
				domain.TaskStatusRejectedByAuditA,
				domain.TaskStatusRejectedByAuditB,
				domain.TaskStatusPendingCustomizationReview,
				domain.TaskStatusPendingCustomizationProduction,
				domain.TaskStatusPendingEffectReview,
				domain.TaskStatusPendingEffectRevision,
				domain.TaskStatusPendingProductionTransfer,
				domain.TaskStatusRejectedByWarehouse,
			},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeDesigner, TaskActionScopeStage},
			PreferHandlerDeny: true,
			RoleGateMessage:   "asset upload requires a design, customization, operation, or management role",
			ScopeGateMessage:  "task asset upload is outside the actor organization scope",
			MatchedRule:       "role_plus_asset_upload_scope_or_handler",
		}
	case TaskActionAuditClaim:
		return taskActionRule{
			Action:           action,
			RequiredRoles:    append([]domain.Role{domain.RoleAuditA, domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:  []domain.TaskStatus{domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB, domain.TaskStatusPendingOutsourceReview},
			AllowedScopes:    []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam},
			RoleGateMessage:  "audit claim requires an audit or management role",
			ScopeGateMessage: "audit claim is outside the actor organization scope",
			MatchedRule:      "role_plus_audit_scope",
		}
	case TaskActionAuditAClaim:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditA},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			HandlerPolicy:     taskActionHandlerPolicyUnassignedOrCurrentActor,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "audit A claim requires PendingAuditA",
			RoleGateMessage:   "audit A claim requires the Audit_A role or a management role",
			ScopeGateMessage:  "audit A claim is outside the actor organization scope",
			PreferHandlerDeny: true,
			MatchedRule:       "audit_a_claim_scope",
		}
	case TaskActionAuditBClaim:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditB},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			HandlerPolicy:     taskActionHandlerPolicyUnassignedOrCurrentActor,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "audit B claim requires PendingAuditB",
			RoleGateMessage:   "audit B claim requires the Audit_B role or a management role",
			ScopeGateMessage:  "audit B claim is outside the actor organization scope",
			PreferHandlerDeny: true,
			MatchedRule:       "audit_b_claim_scope",
		}
	case TaskActionAuditOutsourceReviewClaim:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA, domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingOutsourceReview},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			HandlerPolicy:     taskActionHandlerPolicyUnassignedOrCurrentActor,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "outsource review claim requires PendingOutsourceReview",
			RoleGateMessage:   "outsource review claim requires an audit or management role",
			ScopeGateMessage:  "outsource review claim is outside the actor organization scope",
			PreferHandlerDeny: true,
			MatchedRule:       "audit_outsource_review_claim_scope",
		}
	case TaskActionAuditApprove, TaskActionAuditReject, TaskActionAuditTransfer, TaskActionAuditHandover:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA, domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB, domain.TaskStatusPendingOutsourceReview},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler},
			PreferHandlerDeny: true,
			RoleGateMessage:   "audit action requires an audit or management role",
			ScopeGateMessage:  "audit action is outside the actor organization scope",
			MatchedRule:       "role_plus_audit_scope_or_handler",
		}
	case TaskActionAuditAApprove, TaskActionAuditAReject:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditA},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			HandlerPolicy:     taskActionHandlerPolicyRequireCurrentHandler,
			PreferHandlerDeny: true,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "audit A action requires PendingAuditA",
			RoleGateMessage:   "audit A action requires the Audit_A role or a management role",
			ScopeGateMessage:  "audit A action is outside the actor organization scope",
			MatchedRule:       "audit_a_scope_or_handler",
		}
	case TaskActionAuditATransfer, TaskActionAuditAHandover:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditA},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler},
			HandlerPolicy:     taskActionHandlerPolicyRequireCurrentHandler,
			PreferHandlerDeny: true,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "audit A action requires PendingAuditA",
			RoleGateMessage:   "audit A action requires the Audit_A role or a management role",
			ScopeGateMessage:  "audit A action is outside the actor organization scope",
			MatchedRule:       "audit_a_scope_or_handler",
		}
	case TaskActionAuditBApprove, TaskActionAuditBReject:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditB},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			HandlerPolicy:     taskActionHandlerPolicyRequireCurrentHandler,
			PreferHandlerDeny: true,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "audit B action requires PendingAuditB",
			RoleGateMessage:   "audit B action requires the Audit_B role or a management role",
			ScopeGateMessage:  "audit B action is outside the actor organization scope",
			MatchedRule:       "audit_b_scope_or_handler",
		}
	case TaskActionAuditBTransfer, TaskActionAuditBHandover:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditB},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler},
			HandlerPolicy:     taskActionHandlerPolicyRequireCurrentHandler,
			PreferHandlerDeny: true,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "audit B action requires PendingAuditB",
			RoleGateMessage:   "audit B action requires the Audit_B role or a management role",
			ScopeGateMessage:  "audit B action is outside the actor organization scope",
			MatchedRule:       "audit_b_scope_or_handler",
		}
	case TaskActionAuditOutsourceReviewApprove, TaskActionAuditOutsourceReviewReject:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA, domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingOutsourceReview},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			HandlerPolicy:     taskActionHandlerPolicyRequireCurrentHandler,
			PreferHandlerDeny: true,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "outsource review action requires PendingOutsourceReview",
			RoleGateMessage:   "outsource review action requires an audit or management role",
			ScopeGateMessage:  "outsource review action is outside the actor organization scope",
			MatchedRule:       "audit_outsource_review_scope_or_handler",
		}
	case TaskActionAuditOutsourceReviewTransfer, TaskActionAuditOutsourceReviewHandover:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA, domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingOutsourceReview},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler},
			HandlerPolicy:     taskActionHandlerPolicyRequireCurrentHandler,
			PreferHandlerDeny: true,
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "outsource review action requires PendingOutsourceReview",
			RoleGateMessage:   "outsource review action requires an audit or management role",
			ScopeGateMessage:  "outsource review action is outside the actor organization scope",
			MatchedRule:       "audit_outsource_review_scope_or_handler",
		}
	case TaskActionAuditTakeover:
		return taskActionRule{
			Action:           action,
			RequiredRoles:    append([]domain.Role{domain.RoleAuditA, domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:  []domain.TaskStatus{domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB, domain.TaskStatusPendingOutsourceReview},
			AllowedScopes:    []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam},
			RoleGateMessage:  "audit takeover requires an audit or management role",
			ScopeGateMessage: "audit takeover is outside the actor organization scope",
			MatchedRule:      "role_plus_audit_takeover_scope",
		}
	case TaskActionAuditATakeover:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditA},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam},
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "audit A takeover requires PendingAuditA",
			RoleGateMessage:   "audit A takeover requires the Audit_A role or a management role",
			ScopeGateMessage:  "audit A takeover is outside the actor organization scope",
			MatchedRule:       "audit_a_takeover_scope",
		}
	case TaskActionAuditBTakeover:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingAuditB},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam},
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "audit B takeover requires PendingAuditB",
			RoleGateMessage:   "audit B takeover requires the Audit_B role or a management role",
			ScopeGateMessage:  "audit B takeover is outside the actor organization scope",
			MatchedRule:       "audit_b_takeover_scope",
		}
	case TaskActionAuditOutsourceReviewTakeover:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleAuditA, domain.RoleAuditB}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingOutsourceReview},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam},
			StatusDenyCode:    "audit_stage_mismatch",
			StatusGateMessage: "outsource review takeover requires PendingOutsourceReview",
			RoleGateMessage:   "outsource review takeover requires an audit or management role",
			ScopeGateMessage:  "outsource review takeover is outside the actor organization scope",
			MatchedRule:       "audit_outsource_review_takeover_scope",
		}
	case TaskActionWarehousePrepare:
		return taskActionRule{
			Action:           action,
			RequiredRoles:    append([]domain.Role{domain.RoleWarehouse, domain.RoleOps}, managerRoles...),
			AllowedScopes:    []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam},
			RoleGateMessage:  "warehouse action requires a warehouse, operation, or management role",
			ScopeGateMessage: "warehouse action is outside the actor organization scope",
			MatchedRule:      "role_plus_warehouse_scope",
		}
	case TaskActionWarehouseReceive:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleWarehouse, domain.RoleOps}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingWarehouseQC},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			HandlerPolicy:     taskActionHandlerPolicyUnassignedOrCurrentActor,
			PreferHandlerDeny: true,
			StatusDenyCode:    "warehouse_stage_mismatch",
			StatusGateMessage: "warehouse receive requires PendingWarehouseReceive",
			RoleGateMessage:   "warehouse action requires a warehouse, operation, or management role",
			ScopeGateMessage:  "warehouse action is outside the actor organization scope",
			MatchedRule:       "warehouse_receive_scope",
		}
	case TaskActionWarehouseReject:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleWarehouse, domain.RoleOps}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingWarehouseQC},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			PreferHandlerDeny: true,
			HandlerPolicy:     taskActionHandlerPolicyRequireCurrentHandler,
			StatusDenyCode:    "warehouse_stage_mismatch",
			StatusGateMessage: "warehouse action requires PendingWarehouseReceive",
			RoleGateMessage:   "warehouse action requires a warehouse, operation, or management role",
			ScopeGateMessage:  "warehouse action is outside the actor organization scope",
			MatchedRule:       "role_plus_warehouse_scope_or_handler",
		}
	case TaskActionWarehouseComplete:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleWarehouse, domain.RoleOps}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingWarehouseQC, domain.TaskStatusPendingProductionTransfer},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			PreferHandlerDeny: true,
			HandlerPolicy:     taskActionHandlerPolicyRequireCurrentHandler,
			StatusDenyCode:    "warehouse_stage_mismatch",
			StatusGateMessage: "warehouse complete requires a received warehouse task",
			RoleGateMessage:   "warehouse action requires a warehouse, operation, or management role",
			ScopeGateMessage:  "warehouse action is outside the actor organization scope",
			MatchedRule:       "role_plus_warehouse_scope_or_handler",
		}
	case TaskActionCustomizationReview:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleCustomizationReviewer}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingCustomizationReview},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeStage},
			StatusDenyCode:    "customization_stage_mismatch",
			StatusGateMessage: "customization review requires PendingCustomizationReview",
			RoleGateMessage:   "customization review requires a customization reviewer or management role",
			ScopeGateMessage:  "customization review is outside the actor organization scope",
			MatchedRule:       "customization_review_scope",
		}
	case TaskActionCustomizationEffectPreview:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleCustomizationOperator, domain.RoleDesigner, domain.RoleOps}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingCustomizationProduction, domain.TaskStatusPendingEffectRevision, domain.TaskStatusRejectedByWarehouse},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			PreferHandlerDeny: true,
			RoleGateMessage:   "customization effect submit requires a production operator or management role",
			ScopeGateMessage:  "customization effect submit is outside the actor organization scope",
			MatchedRule:       "customization_effect_preview_scope",
		}
	case TaskActionCustomizationEffectReview:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleCustomizationReviewer}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingEffectReview},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeStage},
			StatusDenyCode:    "customization_stage_mismatch",
			StatusGateMessage: "customization effect review requires PendingEffectReview",
			RoleGateMessage:   "customization effect review requires a customization reviewer or management role",
			ScopeGateMessage:  "customization effect review is outside the actor organization scope",
			MatchedRule:       "customization_effect_review_scope",
		}
	case TaskActionCustomizationTransfer:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleCustomizationOperator, domain.RoleDesigner, domain.RoleOps}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingProductionTransfer},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeHandler, TaskActionScopeStage},
			PreferHandlerDeny: true,
			StatusDenyCode:    "customization_stage_mismatch",
			StatusGateMessage: "customization transfer requires PendingProductionTransfer",
			RoleGateMessage:   "customization transfer requires a production operator or management role",
			ScopeGateMessage:  "customization transfer is outside the actor organization scope",
			MatchedRule:       "customization_transfer_scope",
		}
	case TaskActionUpdateBusinessInfo, TaskActionUpdateProcurement, TaskActionAdvanceProcurement:
		return taskActionRule{
			Action:           action,
			RequiredRoles:    append([]domain.Role{domain.RoleOps, domain.RoleWarehouse}, managerRoles...),
			AllowedScopes:    []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam, TaskActionScopeCreator, TaskActionScopeHandler},
			RoleGateMessage:  "task maintenance requires an operation, warehouse, or management role",
			ScopeGateMessage: "task maintenance is outside the actor organization scope",
			MatchedRule:      "role_plus_maintenance_scope",
		}
	case TaskActionClose:
		return taskActionRule{
			Action:            action,
			RequiredRoles:     append([]domain.Role{domain.RoleOps, domain.RoleWarehouse}, managerRoles...),
			AllowedStatuses:   []domain.TaskStatus{domain.TaskStatusPendingClose},
			AllowedScopes:     []TaskActionScopeSource{TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam, TaskActionScopeDepartment, TaskActionScopeTeam},
			StatusDenyCode:    "task_not_closable",
			StatusGateMessage: "task close requires PendingClose",
			RoleGateMessage:   "task maintenance requires an operation, warehouse, or management role",
			ScopeGateMessage:  "task maintenance is outside the actor organization scope",
			MatchedRule:       "role_plus_maintenance_scope",
		}
	default:
		return taskActionRule{
			Action:           action,
			RoleGateMessage:  "task action requires an allowed role",
			ScopeGateMessage: "task action is outside the actor organization scope",
			MatchedRule:      "unconfigured_rule",
		}
	}
}
