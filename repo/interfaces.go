package repo

import (
	"context"
	"time"

	"workflow/domain"
)

// Tx is a marker for a live database transaction.
// It MUST be passed to Append so that event_logs are written
// in the same transaction as the state-changing operation (spec §8.2).
type Tx interface{ IsTx() }

// TxRunner executes fn inside a single database transaction.
//   - If fn returns nil  → the transaction is committed.
//   - If fn returns error → the transaction is rolled back and the error is returned.
//
// Service layer uses this to group multiple repo operations atomically without
// importing mysqlrepo (which would violate layer separation).
type TxRunner interface {
	RunInTx(ctx context.Context, fn func(tx Tx) error) error
}

// SKURepo handles skus table access.
type SKURepo interface {
	GetByID(ctx context.Context, id int64) (*domain.SKU, error)
	GetBySKUCode(ctx context.Context, skuCode string) (*domain.SKU, error)
	List(ctx context.Context, filter SKUListFilter) ([]*domain.SKU, error)
	Create(ctx context.Context, tx Tx, sku *domain.SKU) (int64, error)
	UpdateWorkflowStatus(ctx context.Context, tx Tx, id int64, status domain.WorkflowStatus) error
	SetCurrentVersion(ctx context.Context, tx Tx, skuID, verID int64) error
	// CASWorkflowStatus is an optimistic, atomic status update (spec §8.2 CAS gate).
	// It executes: UPDATE skus SET workflow_status=next WHERE id=id AND workflow_status=expected
	// Returns updated=true if exactly one row was changed; updated=false means a concurrent
	// request already moved the status (CAS miss — the caller MUST surface this as a 409).
	CASWorkflowStatus(ctx context.Context, tx Tx, id int64, expected, next domain.WorkflowStatus) (updated bool, err error)
}

// AssetVersionRepo handles asset_versions (append-only — content columns are never updated).
type AssetVersionRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.AssetVersion, error)
	GetCurrentForSKU(ctx context.Context, skuID int64) (*domain.AssetVersion, error)
	Create(ctx context.Context, tx Tx, ver *domain.AssetVersion) (int64, error)
	UpdateHashState(ctx context.Context, id int64, state domain.HashState) error
	UpdateExistsState(ctx context.Context, id int64, state domain.ExistsState) error
	MarkStable(ctx context.Context, id int64) error
}

// AuditRepo handles audit_actions with idempotent insert support.
type AuditRepo interface {
	// InsertIdempotent inserts or returns the existing row for the same action_id.
	// created=false means a duplicate was found and the existing row is returned.
	InsertIdempotent(ctx context.Context, tx Tx, action *domain.AuditAction) (result *domain.AuditAction, created bool, err error)
	GetByActionID(ctx context.Context, actionID string) (*domain.AuditAction, error)
}

// JobRepo handles distribution_jobs and job_attempts.
type JobRepo interface {
	// CreateBatch creates jobs with IGNORE on UNIQUE(idempotent_key) – safe to call multiple times.
	CreateBatch(ctx context.Context, tx Tx, jobs []*domain.DistributionJob) error
	GetByID(ctx context.Context, id int64) (*domain.DistributionJob, error)
	ListBySKUID(ctx context.Context, skuID int64) ([]*domain.DistributionJob, error)
	// PullPending selects a Pending job FOR UPDATE and creates a new attempt+lease atomically.
	PullPending(ctx context.Context, agentID string, leaseDuration time.Duration) (*domain.DistributionJob, *domain.JobAttempt, error)
	UpdateStatus(ctx context.Context, tx Tx, jobID int64, status domain.JobStatus) error
	UpdateVerifyStatus(ctx context.Context, jobID int64, status domain.VerifyStatus) error
	SetCurrentAttempt(ctx context.Context, tx Tx, jobID int64, attemptID string) error
	GetAttemptByID(ctx context.Context, attemptID string) (*domain.JobAttempt, error)
	RenewLease(ctx context.Context, attemptID string, newExpiry time.Time) error
	MarkAttemptAcked(ctx context.Context, tx Tx, attemptID string) error
	// FindExpiredLeases returns Running jobs whose lease has expired (for LeaseReaper).
	FindExpiredLeases(ctx context.Context) ([]*domain.DistributionJob, error)
	MarkStale(ctx context.Context, tx Tx, jobID int64) error
	// FindRetryable returns Fail/Stale jobs eligible for retry (for RetryScheduler).
	FindRetryable(ctx context.Context) ([]*domain.DistributionJob, error)
	IncrementRetry(ctx context.Context, tx Tx, jobID int64, nextRetryAt time.Time) error
}

// EventRepo is the authoritative event log.
// Append MUST be called inside the same transaction as the state-changing operation (spec §8.2).
type EventRepo interface {
	Append(ctx context.Context, tx Tx, skuID int64, eventType string, payload interface{}) (*domain.EventLog, error)
	ListSince(ctx context.Context, skuID, sinceSequence int64) ([]*domain.EventLog, error)
	GetLatestSequence(ctx context.Context, skuID int64) (int64, error)
}

// IncidentRepo handles incidents table.
type IncidentRepo interface {
	Create(ctx context.Context, tx Tx, incident *domain.Incident) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.Incident, error)
	List(ctx context.Context, filter IncidentListFilter) ([]*domain.Incident, error)
	UpdateStatus(ctx context.Context, tx Tx, id int64, status domain.IncidentStatus) error
	Assign(ctx context.Context, id, assigneeID int64) error
	Resolve(ctx context.Context, id, resolverID int64) error
	Close(ctx context.Context, id, closerID int64, reason string) error // Admin only; reason required
}

// PolicyRepo handles system_policies table.
type PolicyRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.SystemPolicy, error)
	GetByKey(ctx context.Context, key string) (*domain.SystemPolicy, error)
	ListAll(ctx context.Context) ([]*domain.SystemPolicy, error)
	Upsert(ctx context.Context, policy *domain.SystemPolicy) error
}

// SKUListFilter for paginated SKU queries.
type SKUListFilter struct {
	WorkflowStatus *domain.WorkflowStatus
	Page           int
	PageSize       int
}

// IncidentListFilter for paginated incident queries.
type IncidentListFilter struct {
	Status   *domain.IncidentStatus
	SKUID    *int64
	Page     int
	PageSize int
}

// ── V7 Repos ──────────────────────────────────────────────────────────────────

// ProductRepo handles the products table (ERP master data, V7 §4.1).
type ProductRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.Product, error)
	GetByERPProductID(ctx context.Context, erpProductID string) (*domain.Product, error)
	Search(ctx context.Context, filter ProductSearchFilter) ([]*domain.Product, int64, error)
	UpsertBatch(ctx context.Context, tx Tx, products []*domain.Product) (int64, error)
}

type CategoryRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.Category, error)
	GetByCode(ctx context.Context, code string) (*domain.Category, error)
	List(ctx context.Context, filter CategoryListFilter) ([]*domain.Category, int64, error)
	Search(ctx context.Context, filter CategorySearchFilter) ([]*domain.Category, error)
	Create(ctx context.Context, tx Tx, category *domain.Category) (int64, error)
	Update(ctx context.Context, tx Tx, category *domain.Category) error
}

type CategoryERPMappingRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.CategoryERPMapping, error)
	List(ctx context.Context, filter CategoryERPMappingListFilter) ([]*domain.CategoryERPMapping, int64, error)
	Search(ctx context.Context, filter CategoryERPMappingSearchFilter) ([]*domain.CategoryERPMapping, error)
	ListActiveByCategory(ctx context.Context, categoryID *int64, categoryCode string) ([]*domain.CategoryERPMapping, error)
	ListActiveBySearchEntry(ctx context.Context, searchEntryCode string) ([]*domain.CategoryERPMapping, error)
	Create(ctx context.Context, tx Tx, mapping *domain.CategoryERPMapping) (int64, error)
	Update(ctx context.Context, tx Tx, mapping *domain.CategoryERPMapping) error
}

type CostRuleRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.CostRule, error)
	List(ctx context.Context, filter CostRuleListFilter) ([]*domain.CostRule, int64, error)
	ListActiveByCategory(ctx context.Context, categoryID *int64, categoryCode string, asOf time.Time) ([]*domain.CostRule, error)
	Create(ctx context.Context, tx Tx, rule *domain.CostRule) (int64, error)
	Update(ctx context.Context, tx Tx, rule *domain.CostRule) error
}

type TaskCostOverrideEventRepo interface {
	Append(ctx context.Context, tx Tx, event *domain.TaskCostOverrideAuditEvent) (*domain.TaskCostOverrideAuditEvent, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskCostOverrideAuditEvent, error)
	GetByEventID(ctx context.Context, eventID string) (*domain.TaskCostOverrideAuditEvent, error)
}

type TaskCostOverrideReviewRepo interface {
	Upsert(ctx context.Context, tx Tx, record *domain.TaskCostOverrideReviewRecord) (*domain.TaskCostOverrideReviewRecord, error)
	GetByEventID(ctx context.Context, eventID string) (*domain.TaskCostOverrideReviewRecord, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskCostOverrideReviewRecord, error)
}

type TaskCostFinanceFlagRepo interface {
	Upsert(ctx context.Context, tx Tx, flag *domain.TaskCostFinanceFlag) (*domain.TaskCostFinanceFlag, error)
	GetByEventID(ctx context.Context, eventID string) (*domain.TaskCostFinanceFlag, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskCostFinanceFlag, error)
}

// ERPSyncRunRepo stores ERP sync execution history.
type ERPSyncRunRepo interface {
	Create(ctx context.Context, tx Tx, run *domain.ERPSyncRun) (int64, error)
	GetLatest(ctx context.Context) (*domain.ERPSyncRun, error)
}

// TaskRepo handles tasks and task_details tables (V7 §9).
type TaskRepo interface {
	Create(ctx context.Context, tx Tx, task *domain.Task, detail *domain.TaskDetail) (int64, error)
	CreateSKUItems(ctx context.Context, tx Tx, items []*domain.TaskSKUItem) error
	GetByID(ctx context.Context, id int64) (*domain.Task, error)
	GetDetailByTaskID(ctx context.Context, taskID int64) (*domain.TaskDetail, error)
	GetSKUItemBySKUCode(ctx context.Context, skuCode string) (*domain.TaskSKUItem, error)
	ListSKUItemsByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskSKUItem, error)
	List(ctx context.Context, filter TaskListFilter) ([]*domain.TaskListItem, int64, error)
	ListBoardCandidates(ctx context.Context, filter TaskBoardCandidateFilter) ([]*domain.TaskListItem, error)
	UpdateDetailBusinessInfo(ctx context.Context, tx Tx, detail *domain.TaskDetail) error
	UpdateProductBinding(ctx context.Context, tx Tx, task *domain.Task) error
	// UpdateStatus performs a direct status update inside a transaction.
	// The service layer is responsible for validating the transition before calling this.
	UpdateStatus(ctx context.Context, tx Tx, id int64, status domain.TaskStatus) error
	UpdateDesigner(ctx context.Context, tx Tx, id int64, designerID *int64) error
	// UpdateHandler sets the current_handler_id (nil clears it).
	UpdateHandler(ctx context.Context, tx Tx, id int64, handlerID *int64) error
	UpdateCustomizationState(ctx context.Context, tx Tx, id int64, lastOperatorID *int64, rejectReason, rejectCategory string) error
}

// CodeRuleRepo handles code_rules and code_rule_sequences tables (V7 §5).
type CodeRuleRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.CodeRule, error)
	GetEnabledByType(ctx context.Context, ruleType domain.CodeRuleType) (*domain.CodeRule, error)
	ListAll(ctx context.Context) ([]*domain.CodeRule, error)
	// NextSeq atomically increments and returns the next sequence number for a rule.
	// MUST be called inside an active transaction.
	NextSeq(ctx context.Context, tx Tx, ruleID int64) (int64, error)
}

// RuleTemplateRepo handles rule_templates table (v0.5).
type RuleTemplateRepo interface {
	GetByType(ctx context.Context, templateType domain.RuleTemplateType) (*domain.RuleTemplate, error)
	ListAll(ctx context.Context) ([]*domain.RuleTemplate, error)
	Upsert(ctx context.Context, templateType domain.RuleTemplateType, configJSON string) (*domain.RuleTemplate, error)
}

// ProductCodeSequenceRepo allocates category-short-code-scoped product-code sequence ranges.
// MUST be called inside an active transaction.
type ProductCodeSequenceRepo interface {
	AllocateRange(ctx context.Context, tx Tx, prefix, categoryShortCode string, count int) (start int64, err error)
}

// ProductSearchFilter for product keyword/category search.
type ProductSearchFilter struct {
	Keyword      string
	Category     string
	MappingRules []*domain.CategoryERPMapping
	Page         int
	PageSize     int
}

type CategoryListFilter struct {
	Keyword      string
	CategoryType *domain.CategoryType
	ParentID     *int64
	Level        *int
	IsActive     *bool
	Source       string
	Page         int
	PageSize     int
}

type CategorySearchFilter struct {
	Keyword      string
	CategoryType *domain.CategoryType
	IsActive     *bool
	Limit        int
}

type CategoryERPMappingListFilter struct {
	Keyword         string
	CategoryID      *int64
	CategoryCode    string
	SearchEntryCode string
	ERPMatchType    *domain.CategoryERPMatchType
	IsActive        *bool
	IsPrimary       *bool
	Source          string
	Page            int
	PageSize        int
}

type CategoryERPMappingSearchFilter struct {
	Keyword         string
	CategoryCode    string
	SearchEntryCode string
	ERPMatchType    *domain.CategoryERPMatchType
	IsActive        *bool
	Limit           int
}

type CostRuleListFilter struct {
	CategoryID    *int64
	CategoryCode  string
	ProductFamily string
	RuleType      *domain.CostRuleType
	IsActive      *bool
	Page          int
	PageSize      int
}

// TaskListFilter for paginated task queries.
type TaskListFilter struct {
	domain.TaskQueryFilterDefinition
	CreatorID                   *int64
	DesignerID                  *int64
	NeedOutsource               *bool
	Overdue                     *bool
	Keyword                     string
	ScopeViewAll                bool
	ScopeDepartmentCodes        []string
	ScopeTeamCodes              []string
	ScopeManagedDepartmentCodes []string
	ScopeManagedTeamCodes       []string
	ScopeUserIDs                []int64
	ScopeStageVisibilities      []ScopeStageVisibility
	Page                        int
	PageSize                    int
}

type ScopeStageVisibility struct {
	Statuses []domain.TaskStatus
	Lane     *domain.WorkflowLane
}

// TaskBoardCandidateFilter narrows one board-wide candidate scan with
// shared global filters plus the union of selected preset queue predicates.
type TaskBoardCandidateFilter struct {
	TaskListFilter
	CandidateFilters []domain.TaskQueryFilterDefinition
}

type WorkbenchPreferenceScope struct {
	ActorID       int64
	ActorRolesKey string
	AuthMode      domain.AuthMode
}

type UserRepo interface {
	Count(ctx context.Context) (int64, error)
	CountByRole(ctx context.Context, role domain.Role) (int64, error)
	CountByDepartment(ctx context.Context, department string) (int64, error)
	CountByTeam(ctx context.Context, team string) (int64, error)
	Create(ctx context.Context, tx Tx, user *domain.User) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByMobile(ctx context.Context, mobile string) (*domain.User, error)
	GetByJstUID(ctx context.Context, jstUID int64) (*domain.User, error)
	List(ctx context.Context, filter UserListFilter) ([]*domain.User, int64, error)
	// ListActiveByRole returns every user with status=active that carries the
	// given role, with NO pagination, NO department filter, NO team filter,
	// and NO keyword filter. It is intentionally narrow and is used by the
	// assignment-candidate-pool service path (e.g. ListAssignableDesigners).
	// It MUST NOT be used for management-scoped user listing.
	ListActiveByRole(ctx context.Context, role domain.Role) ([]*domain.User, error)
	ListConfigManagedAdmins(ctx context.Context) ([]*domain.User, error)
	Update(ctx context.Context, tx Tx, user *domain.User) error
	UpdateJstFields(ctx context.Context, tx Tx, userID int64, displayName, status, department, team string, managedDepartments, managedTeams []string, jstRawSnapshot string, jstUID *int64, lastLoginAt *time.Time) error
	UpdatePassword(ctx context.Context, tx Tx, userID int64, passwordHash string, updatedAt time.Time) error
	UpdateLastLogin(ctx context.Context, tx Tx, userID int64, at time.Time) error
	ReplaceRoles(ctx context.Context, tx Tx, userID int64, roles []domain.Role) error
	ListRoles(ctx context.Context, userID int64) ([]domain.Role, error)
}

type OrgRepo interface {
	ListDepartments(ctx context.Context, includeDisabled bool) ([]*domain.OrgDepartment, error)
	ListTeams(ctx context.Context, includeDisabled bool) ([]*domain.OrgTeam, error)
	GetDepartmentByID(ctx context.Context, id int64) (*domain.OrgDepartment, error)
	GetDepartmentByName(ctx context.Context, name string) (*domain.OrgDepartment, error)
	GetTeamByID(ctx context.Context, id int64) (*domain.OrgTeam, error)
	GetTeamByName(ctx context.Context, name string) (*domain.OrgTeam, error)
	CreateDepartment(ctx context.Context, tx Tx, department *domain.OrgDepartment) (int64, error)
	UpdateDepartment(ctx context.Context, tx Tx, department *domain.OrgDepartment) error
	CreateTeam(ctx context.Context, tx Tx, team *domain.OrgTeam) (int64, error)
	UpdateTeam(ctx context.Context, tx Tx, team *domain.OrgTeam) error
}

type UserSessionRepo interface {
	Create(ctx context.Context, tx Tx, session *domain.UserSession) (*domain.UserSession, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*domain.UserSession, error)
	Touch(ctx context.Context, sessionID string, at time.Time) error
}

type PermissionLogRepo interface {
	Create(ctx context.Context, entry *domain.PermissionLog) error
	List(ctx context.Context, filter PermissionLogListFilter) ([]*domain.PermissionLog, int64, error)
}

type ServerLogRepo interface {
	Create(ctx context.Context, log *domain.ServerLog) (int64, error)
	List(ctx context.Context, filter ServerLogListFilter) ([]*domain.ServerLog, int64, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

type ServerLogListFilter struct {
	Level    string
	Keyword  string
	Since    *time.Time
	Until    *time.Time
	Page     int
	PageSize int
}

type UserListFilter struct {
	Keyword    string
	Status     *domain.UserStatus
	Role       *domain.Role
	Department *domain.Department
	Team       string
	Page       int
	PageSize   int
}

type PermissionLogListFilter struct {
	ActorID        *int64
	ActorUsername  string
	ActionType     string
	TargetUserID   *int64
	TargetUsername string
	Granted        *bool
	Method         string
	RoutePath      string
	Page           int
	PageSize       int
}

// WorkbenchPreferenceRepo stores lightweight saved workbench preferences keyed by placeholder actor scope.
type WorkbenchPreferenceRepo interface {
	GetByActorScope(ctx context.Context, scope WorkbenchPreferenceScope) (*domain.WorkbenchPreferenceRecord, error)
	UpsertByActorScope(ctx context.Context, record *domain.WorkbenchPreferenceRecord) error
}

type ExportJobRepo interface {
	Create(ctx context.Context, tx Tx, job *domain.ExportJob) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.ExportJob, error)
	List(ctx context.Context, filter ExportJobListFilter) ([]*domain.ExportJob, int64, error)
	UpdateLifecycle(ctx context.Context, tx Tx, update ExportJobLifecycleUpdate) error
}

type ExportJobAttemptRepo interface {
	Create(ctx context.Context, tx Tx, attempt *domain.ExportJobAttempt) (*domain.ExportJobAttempt, error)
	GetLatestByExportJobID(ctx context.Context, exportJobID int64) (*domain.ExportJobAttempt, error)
	ListByExportJobID(ctx context.Context, exportJobID int64) ([]*domain.ExportJobAttempt, error)
	Update(ctx context.Context, tx Tx, update ExportJobAttemptUpdate) error
	SummariesByExportJobIDs(ctx context.Context, exportJobIDs []int64) (map[int64]ExportJobAttemptAggregate, error)
}

type ExportJobDispatchRepo interface {
	Create(ctx context.Context, tx Tx, dispatch *domain.ExportJobDispatch) (*domain.ExportJobDispatch, error)
	GetByDispatchID(ctx context.Context, dispatchID string) (*domain.ExportJobDispatch, error)
	GetLatestByExportJobID(ctx context.Context, exportJobID int64) (*domain.ExportJobDispatch, error)
	ListByExportJobID(ctx context.Context, exportJobID int64) ([]*domain.ExportJobDispatch, error)
	Update(ctx context.Context, tx Tx, update ExportJobDispatchUpdate) error
	SummariesByExportJobIDs(ctx context.Context, exportJobIDs []int64) (map[int64]ExportJobDispatchAggregate, error)
}

type ExportJobEventRepo interface {
	Append(ctx context.Context, tx Tx, event *domain.ExportJobEvent) (*domain.ExportJobEvent, error)
	ListByExportJobID(ctx context.Context, exportJobID int64) ([]*domain.ExportJobEvent, error)
	ListRecent(ctx context.Context, filter ExportJobEventListFilter) ([]*domain.ExportJobEvent, int64, error)
	SummariesByExportJobIDs(ctx context.Context, exportJobIDs []int64) (map[int64]ExportJobEventAggregate, error)
	LatestSummariesByExportJobIDsAndTypes(ctx context.Context, exportJobIDs []int64, eventTypes []string) (map[int64]*domain.ExportJobEventSummary, error)
}

type ExportJobListFilter struct {
	Status          *domain.ExportJobStatus
	SourceQueryType *domain.ExportSourceQueryType
	RequestedByID   *int64
	Page            int
	PageSize        int
}

type ExportJobLifecycleUpdate struct {
	ExportJobID    int64
	Status         domain.ExportJobStatus
	LatestStatusAt time.Time
	FinishedAt     *time.Time
	ResultRef      *domain.ExportResultRef
	Remark         string
}

type ExportJobEventAggregate struct {
	EventCount  int64
	LatestEvent *domain.ExportJobEventSummary
}

type ExportJobEventListFilter struct {
	EventType   string
	ExportJobID *int64
	Page        int
	PageSize    int
}

type ExportJobAttemptUpdate struct {
	AttemptID    string
	Status       domain.ExportJobAttemptStatus
	FinishedAt   *time.Time
	ErrorMessage string
	AdapterNote  string
}

type ExportJobDispatchUpdate struct {
	DispatchID   string
	Status       domain.ExportJobDispatchStatus
	ReceivedAt   *time.Time
	FinishedAt   *time.Time
	ExpiresAt    *time.Time
	StatusReason string
	AdapterNote  string
}

type ExportJobAttemptAggregate struct {
	AttemptCount  int64
	LatestAttempt *domain.ExportJobAttempt
}

type ExportJobDispatchAggregate struct {
	DispatchCount  int64
	LatestDispatch *domain.ExportJobDispatch
}

type UploadRequestRepo interface {
	Create(ctx context.Context, tx Tx, request *domain.UploadRequest) (*domain.UploadRequest, error)
	GetByRequestID(ctx context.Context, requestID string) (*domain.UploadRequest, error)
	List(ctx context.Context, filter UploadRequestListFilter) ([]*domain.UploadRequest, int64, error)
	UpdateLifecycle(ctx context.Context, tx Tx, update UploadRequestLifecycleUpdate) error
	UpdateBinding(ctx context.Context, tx Tx, requestID string, boundAssetID *int64, boundRefID string, status domain.UploadRequestStatus, remark string) error
	UpdateSession(ctx context.Context, tx Tx, update UploadRequestSessionUpdate) error
}

type UploadRequestListFilter struct {
	OwnerType     *domain.AssetOwnerType
	OwnerID       *int64
	TaskAssetType *domain.TaskAssetType
	Status        *domain.UploadRequestStatus
	Page          int
	PageSize      int
}

type UploadRequestLifecycleUpdate struct {
	RequestID string
	Status    domain.UploadRequestStatus
	Remark    string
}

type UploadRequestSessionUpdate struct {
	RequestID      string
	AssetID        *int64
	SessionStatus  domain.DesignAssetSessionStatus
	RemoteUploadID string
	RemoteFileID   *string
	CreatedBy      *int64
	ExpiresAt      *time.Time
	LastSyncedAt   *time.Time
	Remark         string
}

type DesignAssetListFilter struct {
	TaskID        *int64
	SourceAssetID *int64
	AssetType     *domain.TaskAssetType
	ScopeSKUCode  string
}

type AssetStorageRefRepo interface {
	Create(ctx context.Context, tx Tx, ref *domain.AssetStorageRef) (*domain.AssetStorageRef, error)
	GetByRefID(ctx context.Context, refID string) (*domain.AssetStorageRef, error)
	UpdateStatus(ctx context.Context, tx Tx, refID string, status domain.AssetStorageRefStatus) error
}

type IntegrationExecutionRepo interface {
	Create(ctx context.Context, tx Tx, execution *domain.IntegrationExecution) (*domain.IntegrationExecution, error)
	GetByExecutionID(ctx context.Context, executionID string) (*domain.IntegrationExecution, error)
	GetLatestByCallLogID(ctx context.Context, callLogID int64) (*domain.IntegrationExecution, error)
	ListByCallLogID(ctx context.Context, callLogID int64) ([]*domain.IntegrationExecution, error)
	Update(ctx context.Context, tx Tx, update IntegrationExecutionUpdate) error
	SummariesByCallLogIDs(ctx context.Context, callLogIDs []int64) (map[int64]IntegrationExecutionAggregate, error)
}

type IntegrationExecutionUpdate struct {
	ExecutionID    string
	Status         domain.IntegrationExecutionStatus
	LatestStatusAt time.Time
	FinishedAt     *time.Time
	ErrorMessage   string
	AdapterNote    string
	Retryable      bool
}

type IntegrationExecutionAggregate struct {
	ExecutionCount        int64
	LatestExecution       *domain.IntegrationExecution
	RetryCount            int64
	ReplayCount           int64
	LatestRetryExecution  *domain.IntegrationExecution
	LatestReplayExecution *domain.IntegrationExecution
}

type IntegrationCallLogRepo interface {
	Create(ctx context.Context, tx Tx, log *domain.IntegrationCallLog) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.IntegrationCallLog, error)
	List(ctx context.Context, filter IntegrationCallLogListFilter) ([]*domain.IntegrationCallLog, int64, error)
	Update(ctx context.Context, tx Tx, update IntegrationCallLogUpdate) error
}

type IntegrationCallLogListFilter struct {
	ConnectorKey *domain.IntegrationConnectorKey
	Status       *domain.IntegrationCallStatus
	ResourceType string
	ResourceID   *int64
	Page         int
	PageSize     int
}

type IntegrationCallLogUpdate struct {
	CallLogID       int64
	Status          domain.IntegrationCallStatus
	LatestStatusAt  time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	ResponsePayload []byte
	ErrorMessage    string
	Remark          string
}

// ── V7 Step-02 Repos ──────────────────────────────────────────────────────────

// AuditRecordListFilter filters audit records for GET /v1/audit-logs.
type AuditRecordListFilter struct {
	TaskNo   string // contains match on tasks.task_no
	Auditor  string // contains match on user display_name
	Action   string // exact match on action
	StartAt  string // YYYY-MM-DD, records with created_at >= start 00:00:00
	EndAt    string // YYYY-MM-DD, records with created_at <= end 23:59:59
	Page     int
	PageSize int
}

// AuditV7Repo handles audit_records and audit_handovers tables (V7 §11).
type AuditV7Repo interface {
	CreateRecord(ctx context.Context, tx Tx, record *domain.AuditRecord) (int64, error)
	ListRecordsByTaskID(ctx context.Context, taskID int64) ([]*domain.AuditRecord, error)
	ListRecords(ctx context.Context, filter AuditRecordListFilter) ([]*domain.AuditRecord, error)
	CreateHandover(ctx context.Context, tx Tx, handover *domain.AuditHandover) (int64, error)
	GetHandoverByID(ctx context.Context, id int64) (*domain.AuditHandover, error)
	ListHandoversByTaskID(ctx context.Context, taskID int64) ([]*domain.AuditHandover, error)
	UpdateHandoverStatus(ctx context.Context, tx Tx, id int64, status domain.HandoverStatus) error
}

// OutsourceRepo handles outsource_orders table (V7 §6.2).
type OutsourceRepo interface {
	Create(ctx context.Context, tx Tx, order *domain.OutsourceOrder) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.OutsourceOrder, error)
	List(ctx context.Context, filter OutsourceListFilter) ([]*domain.OutsourceOrder, int64, error)
	Update(ctx context.Context, tx Tx, order *domain.OutsourceOrder) error
}

type CustomizationJobRepo interface {
	Create(ctx context.Context, tx Tx, job *domain.CustomizationJob) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.CustomizationJob, error)
	GetLatestByTaskID(ctx context.Context, taskID int64) (*domain.CustomizationJob, error)
	List(ctx context.Context, filter CustomizationJobListFilter) ([]*domain.CustomizationJob, int64, error)
	Update(ctx context.Context, tx Tx, job *domain.CustomizationJob) error
}

type CustomizationPricingRuleRepo interface {
	GetActiveByLevelAndEmploymentType(ctx context.Context, levelCode string, employmentType domain.EmploymentType) (*domain.CustomizationPricingRule, error)
}

// TaskAssetRepo handles task_assets table (V7 Step-04).
type TaskAssetRepo interface {
	Create(ctx context.Context, tx Tx, asset *domain.TaskAsset) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.TaskAsset, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskAsset, error)
	ListByAssetID(ctx context.Context, assetID int64) ([]*domain.TaskAsset, error)
	NextVersionNo(ctx context.Context, tx Tx, taskID int64) (int, error)
	NextAssetVersionNo(ctx context.Context, tx Tx, assetID int64) (int, error)
}

type DesignAssetRepo interface {
	Create(ctx context.Context, tx Tx, asset *domain.DesignAsset) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.DesignAsset, error)
	List(ctx context.Context, filter DesignAssetListFilter) ([]*domain.DesignAsset, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.DesignAsset, error)
	NextAssetNo(ctx context.Context, tx Tx, taskID int64) (string, error)
	UpdateCurrentVersionID(ctx context.Context, tx Tx, id int64, currentVersionID *int64) error
}

// TaskEventRepo handles task_event_logs and task_event_sequences tables.
// Append MUST be called inside the same transaction as the state-changing operation.
type TaskEventRepo interface {
	Append(ctx context.Context, tx Tx, taskID int64, eventType string, operatorID *int64, payload interface{}) (*domain.TaskEvent, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskEvent, error)
	ListRecent(ctx context.Context, filter TaskEventListFilter) ([]*domain.TaskEvent, int64, error)
}

type TaskEventListFilter struct {
	EventType string
	TaskID    *int64
	Page      int
	PageSize  int
}

// OutsourceListFilter for paginated outsource order queries.
type OutsourceListFilter struct {
	TaskID   *int64
	Status   *domain.OutsourceStatus
	Vendor   string
	Page     int
	PageSize int
}

type CustomizationJobListFilter struct {
	TaskID     *int64
	Status     *domain.CustomizationJobStatus
	OperatorID *int64
	Page       int
	PageSize   int
}

// WarehouseRepo handles warehouse_receipts table (V7 Step-03).
type WarehouseRepo interface {
	Create(ctx context.Context, tx Tx, receipt *domain.WarehouseReceipt) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.WarehouseReceipt, error)
	GetByTaskID(ctx context.Context, taskID int64) (*domain.WarehouseReceipt, error)
	List(ctx context.Context, filter WarehouseListFilter) ([]*domain.WarehouseReceipt, int64, error)
	Update(ctx context.Context, tx Tx, receipt *domain.WarehouseReceipt) error
}

// WarehouseListFilter for paginated warehouse receipt queries.
type WarehouseListFilter struct {
	TaskID       *int64
	Status       *domain.WarehouseReceiptStatus
	WorkflowLane *domain.WorkflowLane
	ReceiverID   *int64
	Page         int
	PageSize     int
}
