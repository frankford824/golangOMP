package domain

// WorkflowStatus represents SKU workflow states (spec §6.1).
// All state values MUST use these constants — no magic strings.
type WorkflowStatus string

const (
	WorkflowDraft                 WorkflowStatus = "Draft"
	WorkflowSubmitted             WorkflowStatus = "Submitted"
	WorkflowAuditAPending         WorkflowStatus = "AuditA_Pending"
	WorkflowAuditAApproved        WorkflowStatus = "AuditA_Approved"
	WorkflowAuditARejected        WorkflowStatus = "AuditA_Rejected"
	WorkflowAuditBPending         WorkflowStatus = "AuditB_Pending"
	WorkflowAuditBApproved        WorkflowStatus = "AuditB_Approved"
	WorkflowAuditBRejected        WorkflowStatus = "AuditB_Rejected"
	WorkflowApprovedPendingVerify WorkflowStatus = "Approved_PendingVerify" // MUST NOT produce Running jobs
	WorkflowApproved              WorkflowStatus = "Approved"
	WorkflowDistributionRunning   WorkflowStatus = "Distribution_Running"
	WorkflowCompleted             WorkflowStatus = "Completed"
	WorkflowBlocked               WorkflowStatus = "Blocked"
	WorkflowCancelled             WorkflowStatus = "Cancelled"
)

// HashState for AssetVersion (spec §6.2).
type HashState string

const (
	HashStatePartial  HashState = "Partial"
	HashStateReady    HashState = "Ready"
	HashStateMismatch HashState = "Mismatch"
)

// AuditStatus for AssetVersion (spec §6.2).
type AuditStatus string

const (
	AuditStatusUnreviewed AuditStatus = "Unreviewed"
	AuditStatusApproved   AuditStatus = "Approved"
	AuditStatusRejected   AuditStatus = "Rejected"
	AuditStatusSuperseded AuditStatus = "Superseded"
)

// ExistsState for AssetVersion (spec §6.2).
// exists_state=Missing MUST block audit and set workflow=Blocked.
type ExistsState string

const (
	ExistsStateExists  ExistsState = "Exists"
	ExistsStateMissing ExistsState = "Missing"
)

// JobStatus for DistributionJob (spec §6.3).
type JobStatus string

const (
	JobStatusPendingVerify   JobStatus = "PendingVerify" // only valid during Approved_PendingVerify
	JobStatusPending         JobStatus = "Pending"
	JobStatusRunning         JobStatus = "Running"
	JobStatusDone            JobStatus = "Done"
	JobStatusFail            JobStatus = "Fail"
	JobStatusStale           JobStatus = "Stale"
	JobStatusExceededRetries JobStatus = "ExceededRetries" // terminal
	JobStatusCancelled       JobStatus = "Cancelled"       // terminal
)

// VerifyStatus for DistributionJob (spec §6.4).
type VerifyStatus string

const (
	VerifyStatusNotRequested VerifyStatus = "NotRequested"
	VerifyStatusVerifying    VerifyStatus = "Verifying"
	VerifyStatusVerified     VerifyStatus = "Verified"
	VerifyStatusVerifyFailed VerifyStatus = "VerifyFailed"
)

// IncidentStatus for Incident (spec §6.5).
type IncidentStatus string

const (
	IncidentStatusOpen       IncidentStatus = "Open"
	IncidentStatusInProgress IncidentStatus = "InProgress"
	IncidentStatusResolved   IncidentStatus = "Resolved"
	IncidentStatusClosed     IncidentStatus = "Closed" // Admin only; requires reason
)

// AuditStage identifies which audit gate an action belongs to.
type AuditStage string

const (
	AuditStageA AuditStage = "A"
	AuditStageB AuditStage = "B"
)

// AuditDecision is the outcome of an audit action.
type AuditDecision string

const (
	AuditDecisionApprove AuditDecision = "Approve"
	AuditDecisionReject  AuditDecision = "Reject"
)

// EvidenceLevel as per spec §11.2.
type EvidenceLevel int

const (
	EvidenceLevelL1 EvidenceLevel = 1 // file_id + size (default policy minimum)
	EvidenceLevelL2 EvidenceLevel = 2 // cloud_path + size
	EvidenceLevelL3 EvidenceLevel = 3 // share_url (display only — not used for judgment)
)

// Role for RBAC (spec §3.1).
type Role string

// --- Official product roles (v1.0) ---
const (
	RoleMember                Role = "Member"
	RoleSuperAdmin            Role = "SuperAdmin"
	RoleHRAdmin               Role = "HRAdmin"
	RoleDeptAdmin             Role = "DepartmentAdmin"
	RoleTeamLead              Role = "TeamLead"
	RoleOps                   Role = "Ops"
	RoleDesigner              Role = "Designer"
	RoleCustomizationOperator Role = "CustomizationOperator"
	RoleAuditA                Role = "Audit_A"
	RoleAuditB                Role = "Audit_B"
	RoleCustomizationReviewer Role = "CustomizationReviewer"
	RoleWarehouse             Role = "Warehouse"
)

// --- Compatibility-only roles: do NOT use in new logic ---
const (
	RoleAdmin          Role = "Admin"          // compatibility: treated as SuperAdmin equivalent
	RoleOrgAdmin       Role = "OrgAdmin"       // compatibility: limited org-scope management
	RoleRoleAdmin      Role = "RoleAdmin"      // compatibility: role assignment only
	RoleDesignDirector Role = "DesignDirector" // compatibility: design department scope
	RoleDesignReviewer Role = "DesignReviewer" // compatibility: design review scope
	RoleOutsource      Role = "Outsource"      // compatibility: outsource workflow
	RoleERP            Role = "ERP"            // compatibility: ERP integration
)

// Permission points (spec §3.2).
// Dangerous actions (marked) MUST include a non-empty reason and MUST write to audit_log + event_logs.
const (
	PermSKUCreate       = "sku.create"
	PermAssetUpload     = "asset.upload"
	PermAuditSubmit     = "audit.submit"
	PermJobRetry        = "job.retry"
	PermJobReassign     = "job.reassign"
	PermJobForceDone    = "job.force_done" // DANGEROUS — requires reason
	PermIncidentAssign  = "incident.assign"
	PermIncidentResolve = "incident.resolve"
	PermAssetDelete     = "asset.delete"    // DANGEROUS — requires reason
	PermPolicyOverride  = "policy.override" // MOST DANGEROUS — requires reason
)
