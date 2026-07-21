package repo

import (
	"context"
	"encoding/json"
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
	ListIIDs(ctx context.Context, filter ProductIIDListFilter) ([]*domain.ERPIIDOption, int64, error)
	UpsertBatch(ctx context.Context, tx Tx, products []*domain.Product) (int64, error)
}

type ProductManagementListFilter struct {
	Keyword         string
	DisplayScope    string
	ImageSource     string
	SyncStatus      string
	BaseSyncStatus  string
	ImageSyncStatus string
	CostStatus      string
	IssueScope      string
	CreatorID       *int64
	Page            int
	PageSize        int
}

type CostRuleBindingListFilter struct {
	Keyword   string
	RuleGroup string
	IsActive  *bool
	Page      int
	PageSize  int
}

type UnboundCostRuleCandidateFilter struct {
	Keyword  string
	Limit    int
	Page     int
	PageSize int
}

type CostRecalculationRunFilter struct {
	Status    string
	Mode      string
	CreatedBy *int64
	Page      int
	PageSize  int
}

type CostRecalculationRunItemFilter struct {
	RunID    int64
	Status   string
	Page     int
	PageSize int
}

type ProductManagementImagePatch struct {
	ImageSource         domain.ProductManagementImageSource
	ImageSelectionMode  domain.ProductManagementImageSelectionMode
	ImageAssetID        *int64
	ImageAssetVersionID *int64
	ImageFilename       string
	ImageMimeType       string
	ImageMissingReason  string
	ImageSyncSource     domain.ProductManagementImageSource
	ImageSyncStatus     domain.ProductManagementERPSyncStatus
}

type ProductManagementSyncPatch struct {
	Status            domain.ProductManagementERPSyncStatus
	BaseStatus        domain.ProductManagementERPSyncStatus
	ImageStatus       domain.ProductManagementERPSyncStatus
	LastERPCheckedAt  *time.Time
	LastERPSyncedAt   *time.Time
	LastBaseSyncedAt  *time.Time
	LastImageSyncedAt *time.Time
	SyncCooldownUntil *time.Time
	LastSyncError     string
	BaseSyncError     string
	ImageSyncError    string
}

type ProductManagementRepo interface {
	RefreshReadModel(ctx context.Context) error
	List(ctx context.Context, filter ProductManagementListFilter) ([]*domain.ProductManagementRecord, int64, error)
	CostDashboard(ctx context.Context) (*domain.ProductCostDashboardResponse, error)
	GetByID(ctx context.Context, id int64) (*domain.ProductManagementRecord, error)
	GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, error)
	ClaimQueuedSyncRecords(ctx context.Context, limit int, claimToken string, now time.Time) ([]*domain.ProductManagementRecord, error)
	QueuePendingBaseSyncByTaskID(ctx context.Context, tx Tx, taskID int64, now time.Time, cooldownUntil time.Time) (int64, error)
	UpdateImage(ctx context.Context, tx Tx, id int64, patch ProductManagementImagePatch) error
	UpdateSyncStatus(ctx context.Context, tx Tx, id int64, patch ProductManagementSyncPatch) error
	UpdateBaseSyncStatus(ctx context.Context, tx Tx, id int64, patch ProductManagementSyncPatch) error
	UpdateImageSyncStatus(ctx context.Context, tx Tx, id int64, patch ProductManagementSyncPatch) error
	MarkBaseSyncProjectionSynced(ctx context.Context, tx Tx, taskID int64, taskSKUItemID *int64, now time.Time) error
}

type CostRuleBindingRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.CostRuleBinding, error)
	GetActiveByNormalizedIID(ctx context.Context, normalizedIID string) (*domain.CostRuleBinding, error)
	List(ctx context.Context, filter CostRuleBindingListFilter) ([]*domain.CostRuleBinding, int64, error)
	Create(ctx context.Context, tx Tx, binding *domain.CostRuleBinding) (int64, error)
	Update(ctx context.Context, tx Tx, binding *domain.CostRuleBinding) error
	Patch(ctx context.Context, tx Tx, patch domain.CostRuleBindingPatch) error
	RuleGroupExists(ctx context.Context, ruleGroup string) (bool, error)
	ListUnboundCandidates(ctx context.Context, filter UnboundCostRuleCandidateFilter) ([]*domain.UnboundCostRuleCandidate, int64, error)
}

type CostRecalculationRunRepo interface {
	CreateRun(ctx context.Context, tx Tx, run *domain.CostRecalculationRun) (int64, error)
	GetRun(ctx context.Context, id int64) (*domain.CostRecalculationRun, error)
	ListRuns(ctx context.Context, filter CostRecalculationRunFilter) ([]*domain.CostRecalculationRun, int64, error)
	UpdateRun(ctx context.Context, tx Tx, run *domain.CostRecalculationRun) error
	MarkRunApplying(ctx context.Context, tx Tx, runID int64) (bool, error)
	DeleteRunItems(ctx context.Context, tx Tx, runID int64) error
	InsertRunItems(ctx context.Context, tx Tx, items []*domain.CostRecalculationRunItem) error
	ListRunItems(ctx context.Context, filter CostRecalculationRunItemFilter) ([]*domain.CostRecalculationRunItem, int64, error)
	ListRunItemsForUpdate(ctx context.Context, tx Tx, runID int64) ([]*domain.CostRecalculationRunItem, error)
	UpdateRunItem(ctx context.Context, tx Tx, item *domain.CostRecalculationRunItem) error
	HasOpenRunForRecord(ctx context.Context, tx Tx, excludingRunID int64, productManagementRecordID int64) (bool, error)
	MarkERPQueuedItemsForRun(ctx context.Context, tx Tx, runID int64, recordIDs []int64) (int64, error)
	MarkERPResultForProductManagementRecord(ctx context.Context, tx Tx, productManagementRecordID int64, status domain.CostRecalculationRunItemStatus, message string) error
}

type SKUComboRepo interface {
	UpsertComboRecord(ctx context.Context, tx Tx, record *domain.OMPSKUComboRecord) error
	UpsertComboRelation(ctx context.Context, tx Tx, relation *domain.OMPSKUComboRelation) error
	DeleteStaleComboRelations(ctx context.Context, tx Tx, comboSKUCode string, source string, currentChildSKUs []string) error
	ListRelationsByChildSKUs(ctx context.Context, childSKUs []string) ([]*domain.OMPSKUComboRelationWithRecord, error)
	GetLatestSyncState(ctx context.Context) (*domain.OMPSKUComboSyncState, error)
	EnsureNextSyncWindow(ctx context.Context, now time.Time, windowSize time.Duration) (*domain.OMPSKUComboSyncState, error)
	ClaimSyncState(ctx context.Context, tx Tx, id int64, now time.Time) (bool, error)
	MarkSyncStateSuccess(ctx context.Context, tx Tx, id int64, nextPage int, processed int, finished bool, now time.Time) error
	MarkSyncStateFailed(ctx context.Context, tx Tx, id int64, message string, nextRetryAt time.Time) error
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

type SKUTraceRepo interface {
	UpsertSKURecord(ctx context.Context, tx Tx, record *domain.OMPSKURecord) error
	AppendCostSnapshot(ctx context.Context, tx Tx, snapshot *domain.OMPSKUCostSnapshot) (int64, error)
	AppendERPTraceLog(ctx context.Context, tx Tx, log *domain.OMPSKUERPTraceLog) (int64, error)
	UpsertComboRelation(ctx context.Context, tx Tx, relation *domain.OMPSKUComboRelation) error
}

type ExternalAssetRepo interface {
	Search(ctx context.Context, query domain.ExternalAssetSearchQuery) ([]*domain.ExternalAssetRecord, int64, error)
	Upsert(ctx context.Context, item domain.ExternalAssetUpsert) (*domain.ExternalAssetRecord, error)
	GetByID(ctx context.Context, id int64) (*domain.ExternalAssetRecord, error)
	CreateSyncRun(ctx context.Context, run *domain.ExternalAssetSyncRun) (int64, error)
	FinishSyncRun(ctx context.Context, id int64, status string, scannedCount, upsertedCount int, errorMessage string) error
	MarkOriginPathMissing(ctx context.Context, provider, mountPath, originPath string) error
	MarkMountMissingBefore(ctx context.Context, mountPath string, scannedBefore time.Time) error
	MarkOriginPrefixesMissingBefore(ctx context.Context, prefixes []ExternalAssetOriginPrefix, scannedBefore time.Time) error
	UpdateDirectURL(ctx context.Context, id int64, rawURL string, expiresAt *time.Time, status string) error
	MarkOSSPreparePending(ctx context.Context, id int64) error
	MarkOSSPendingByOriginPrefixes(ctx context.Context, prefixes []ExternalAssetOriginPrefix) (int64, error)
	MarkPreviewPreparePending(ctx context.Context, id int64) error
	MarkPreviewPendingByOriginPrefixes(ctx context.Context, prefixes []ExternalAssetOriginPrefix) (int64, error)
	ListDirectURLRefreshCandidates(ctx context.Context, mountPaths []string, limit int, staleBefore time.Time) ([]*domain.ExternalAssetRecord, error)
	ListPendingOSS(ctx context.Context, mountPaths []string, limit int) ([]*domain.ExternalAssetRecord, error)
	ListPendingOSSPrioritized(ctx context.Context, prefixes []ExternalAssetOriginPrefix, mountPaths []string, limit int) ([]*domain.ExternalAssetRecord, error)
	ClaimPendingOSSPrioritized(ctx context.Context, prefixes []ExternalAssetOriginPrefix, mountPaths []string, limit int, leaseExpiredBefore time.Time) ([]*domain.ExternalAssetRecord, error)
	ListPendingPreview(ctx context.Context, mountPaths []string, limit int) ([]*domain.ExternalAssetRecord, error)
	MarkOSSReady(ctx context.Context, id int64, objectKey string) error
	MarkClaimedOSSReady(ctx context.Context, id int64, objectKey, claimToken string) (bool, error)
	MarkClaimedOSSFailed(ctx context.Context, id int64, claimToken, message string) (bool, error)
	MarkPreviewReady(ctx context.Context, id int64, previewKey string) error
	MarkPrepareFailed(ctx context.Context, id int64, target, message string) error
}

type ExternalAssetOriginPrefix struct {
	MountPath  string
	OriginPath string
}

type AssetWorkbenchProfileFilter struct {
	Keyword    string
	WorkerType string
	JobGrade   string
	Status     string
	UserID     int64
	Page       int
	PageSize   int
}

type AssetWorkbenchPriceMatrixFilter struct {
	WorkerType      string
	JobGrade        string
	DifficultyClass string
	Enabled         *bool
	Page            int
	PageSize        int
}

type AssetWorkbenchDeductionRuleFilter struct {
	WorkerType      string
	JobGrade        string
	DifficultyClass string
	Enabled         *bool
	Page            int
	PageSize        int
}

type AssetWorkbenchWelfareRuleFilter struct {
	WorkerType string
	JobGrade   string
	RuleType   string
	Enabled    *bool
	Page       int
	PageSize   int
}

type AssetWorkbenchPromoCouponFilter struct {
	WorkerType      string
	JobGrade        string
	DifficultyClass string
	Enabled         *bool
	Page            int
	PageSize        int
}

type AssetWorkbenchDifficultyClassFilter struct {
	Enabled *bool
}

type AssetWorkbenchGroupFilter struct {
	Keyword  string
	Enabled  *bool
	Page     int
	PageSize int
}

type AssetWorkbenchTemplateFilter struct {
	Keyword         string
	Category        string
	DifficultyClass string
	WorkerType      string
	Enabled         *bool
	Page            int
	PageSize        int
}

type AssetWorkbenchTemplateAssignmentFilter struct {
	TemplateID *int64
	TargetType string
	TargetID   *int64
	Enabled    *bool
	Page       int
	PageSize   int
}

type AssetWorkbenchMemberFilter struct {
	UserID   int64
	Keyword  string
	Identity string
	Status   string
	Scope    string
	Page     int
	PageSize int
}

type AssetWorkbenchAccessOpenParams struct {
	UserID       int64
	Status       string
	IdentityType string
	Source       string
	OpenedBy     int64
}

type AssetWorkbenchMergeRewriteCounts struct {
	Submissions            int64 `json:"submissions"`
	SubmissionItems        int64 `json:"submission_items"`
	UploadSessions         int64 `json:"upload_sessions"`
	SubmissionFiles        int64 `json:"submission_files"`
	ErrorRecords           int64 `json:"error_records"`
	SettlementSupplements  int64 `json:"settlement_supplements"`
	SettlementItems        int64 `json:"settlement_items"`
	SettlementItemsDeduped int64 `json:"settlement_items_deduped"`
	GroupMembers           int64 `json:"group_members"`
	TemplateAssignments    int64 `json:"template_assignments"`
	SavedViews             int64 `json:"saved_views"`
	GradePeriods           int64 `json:"grade_periods"`
	SupplementPermissions  int64 `json:"supplement_permissions"`
}

type AssetWorkbenchSubmissionFilter struct {
	SubmitterUserID  *int64
	PayeeUserID      *int64
	BusinessMonth    string
	Status           string
	SettlementStatus string
	OrderBy          string
	OrderDir         string
	Page             int
	PageSize         int
}

type AssetWorkbenchOverviewSearchFilter struct {
	Keyword     string
	Creator     string
	OwnerUserID *int64
	Submissions bool
	Items       bool
	Files       bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Page        int
	PageSize    int
}

type AssetWorkbenchDriveFilter struct {
	OwnerUserID       *int64
	UploadDirectoryID *int64
	Unassigned        bool
	OrderNo           string
	Keyword           string
	OwnerKeyword      string
	CreatedFrom       *time.Time
	CreatedTo         *time.Time
	SortBy            string
	SortDir           string
	Page              int
	PageSize          int
}

type AssetWorkbenchPreviewClaim struct {
	WorkerID string
	Now      time.Time
	LeaseTTL time.Duration
	Limit    int
}

type AssetWorkbenchSettlementBatchFilter struct {
	BusinessMonth string
	Status        string
	Page          int
	PageSize      int
}

type AssetWorkbenchSettlementSupplementFilter struct {
	PayeeUserID        *int64
	BusinessMonth      string
	OrderNo            string
	Status             string
	SupplementDate     string
	SupplementDateFrom string
	SupplementDateTo   string
	SortBy             string
	SortDir            string
	Page               int
	PageSize           int
}

type AssetWorkbenchSupplementPermissionFilter struct {
	PayeeUserID   *int64
	BusinessMonth string
	Enabled       *bool
	Page          int
	PageSize      int
}

type AssetWorkbenchEventFilter struct {
	EventType  string
	EntityType string
	EntityID   *int64
	ActorID    *int64
	Page       int
	PageSize   int
}

type AssetWorkbenchSavedViewFilter struct {
	UserID   int64
	ViewType string
}

type AssetWorkbenchUploadDirectoryFilter struct {
	Enabled *bool
}

type AssetWorkbenchClientMaterialFilter struct {
	Enabled *bool
}

type AssetWorkbenchBatchJobFilter struct {
	JobType     string
	Status      string
	RequestedBy *int64
	Page        int
	PageSize    int
}

type AssetWorkbenchRepo interface {
	GetProfileByUserID(ctx context.Context, userID int64) (*domain.AssetWorkbenchProfile, error)
	ListProfiles(ctx context.Context, filter AssetWorkbenchProfileFilter) ([]*domain.AssetWorkbenchProfile, int64, error)
	ListMembers(ctx context.Context, filter AssetWorkbenchMemberFilter) ([]*domain.AssetWorkbenchMember, int64, error)
	SearchPeople(ctx context.Context, filter AssetWorkbenchMemberFilter) ([]*domain.AssetWorkbenchMember, int64, error)
	UpsertProfile(ctx context.Context, tx Tx, profile *domain.AssetWorkbenchProfile) (*domain.AssetWorkbenchProfile, error)
	AppendGradePeriod(ctx context.Context, tx Tx, period *domain.AssetWorkbenchGradePeriod) (*domain.AssetWorkbenchGradePeriod, error)
	GetMembership(ctx context.Context, appCode string, userID int64) (*domain.AppMembership, error)
	LockMembership(ctx context.Context, tx Tx, appCode string, userID int64) (*domain.AppMembership, error)
	UpsertMembership(ctx context.Context, tx Tx, membership *domain.AppMembership) (*domain.AppMembership, error)
	RequestMembership(ctx context.Context, tx Tx, appCode string, userID int64, identityType string) (*domain.AppMembership, error)
	OpenMembership(ctx context.Context, tx Tx, params AssetWorkbenchAccessOpenParams) (*domain.AppMembership, error)
	DisableMembership(ctx context.Context, tx Tx, appCode string, userID int64, disabledBy int64, reason string, lastRoles []domain.Role) (*domain.AppMembership, error)
	MarkMembershipMerged(ctx context.Context, tx Tx, appCode string, sourceUserID int64) error
	CreateAppIdentityEvent(ctx context.Context, tx Tx, event *domain.AppIdentityEvent) (*domain.AppIdentityEvent, error)
	CreateAccountLink(ctx context.Context, tx Tx, link *domain.AssetWorkbenchAccountLink) (*domain.AssetWorkbenchAccountLink, error)
	GetAccountLinkBySource(ctx context.Context, sourceUserID int64) (*domain.AssetWorkbenchAccountLink, error)
	GetAccountLinkByCanonical(ctx context.Context, canonicalUserID int64) (*domain.AssetWorkbenchAccountLink, error)

	ListDifficultyClasses(ctx context.Context, filter AssetWorkbenchDifficultyClassFilter) ([]*domain.AssetWorkbenchDifficultyClass, error)
	GetDifficultyClass(ctx context.Context, code string) (*domain.AssetWorkbenchDifficultyClass, error)
	CreateDifficultyClass(ctx context.Context, tx Tx, item *domain.AssetWorkbenchDifficultyClass) (*domain.AssetWorkbenchDifficultyClass, error)
	UpdateDifficultyClass(ctx context.Context, tx Tx, item *domain.AssetWorkbenchDifficultyClass) (*domain.AssetWorkbenchDifficultyClass, error)

	ListPriceMatrix(ctx context.Context, filter AssetWorkbenchPriceMatrixFilter) ([]*domain.AssetWorkbenchPriceMatrix, int64, error)
	GetPriceMatrixForUpdate(ctx context.Context, tx Tx, id int64) (*domain.AssetWorkbenchPriceMatrix, error)
	LockPriceMatrixDimension(ctx context.Context, tx Tx, workerType, jobGrade, difficultyClass string) ([]*domain.AssetWorkbenchPriceMatrix, error)
	CreatePriceMatrix(ctx context.Context, tx Tx, item *domain.AssetWorkbenchPriceMatrix) (*domain.AssetWorkbenchPriceMatrix, error)
	SetPriceMatrixEnabled(ctx context.Context, tx Tx, id int64, enabled bool) (*domain.AssetWorkbenchPriceMatrix, error)
	SetPriceMatrixEffectiveTo(ctx context.Context, tx Tx, id int64, effectiveTo *time.Time) (*domain.AssetWorkbenchPriceMatrix, error)
	FindActivePrice(ctx context.Context, workerType, jobGrade, difficultyClass string, asOf time.Time) (*domain.AssetWorkbenchPriceMatrix, error)

	ListDeductionRules(ctx context.Context, filter AssetWorkbenchDeductionRuleFilter) ([]*domain.AssetWorkbenchDeductionRule, int64, error)
	GetDeductionRuleForUpdate(ctx context.Context, tx Tx, id int64) (*domain.AssetWorkbenchDeductionRule, error)
	LockDeductionRuleDimension(ctx context.Context, tx Tx, workerType, jobGrade, difficultyClass string) ([]*domain.AssetWorkbenchDeductionRule, error)
	CreateDeductionRule(ctx context.Context, tx Tx, item *domain.AssetWorkbenchDeductionRule) (*domain.AssetWorkbenchDeductionRule, error)
	SetDeductionRuleEnabled(ctx context.Context, tx Tx, id int64, enabled bool) (*domain.AssetWorkbenchDeductionRule, error)

	ListWelfareRules(ctx context.Context, filter AssetWorkbenchWelfareRuleFilter) ([]*domain.AssetWorkbenchWelfareRule, int64, error)
	GetWelfareRuleForUpdate(ctx context.Context, tx Tx, id int64) (*domain.AssetWorkbenchWelfareRule, error)
	CreateWelfareRule(ctx context.Context, tx Tx, item *domain.AssetWorkbenchWelfareRule) (*domain.AssetWorkbenchWelfareRule, error)
	SetWelfareRuleEnabled(ctx context.Context, tx Tx, id int64, enabled bool) (*domain.AssetWorkbenchWelfareRule, error)
	FindActiveWelfareRules(ctx context.Context, workerType, jobGrade string, asOf time.Time) ([]*domain.AssetWorkbenchWelfareRule, error)

	ListPromoCoupons(ctx context.Context, filter AssetWorkbenchPromoCouponFilter) ([]*domain.AssetWorkbenchPromoCoupon, int64, error)
	GetPromoCouponForUpdate(ctx context.Context, tx Tx, id int64) (*domain.AssetWorkbenchPromoCoupon, error)
	CreatePromoCoupon(ctx context.Context, tx Tx, item *domain.AssetWorkbenchPromoCoupon) (*domain.AssetWorkbenchPromoCoupon, error)
	SetPromoCouponEnabled(ctx context.Context, tx Tx, id int64, enabled bool) (*domain.AssetWorkbenchPromoCoupon, error)
	ListActivePromoCoupons(ctx context.Context, workerType, jobGrade, difficultyClass string, asOf time.Time) ([]*domain.AssetWorkbenchPromoCoupon, error)

	ListGroups(ctx context.Context, filter AssetWorkbenchGroupFilter) ([]*domain.AssetWorkbenchGroup, int64, error)
	CreateGroup(ctx context.Context, tx Tx, group *domain.AssetWorkbenchGroup) (*domain.AssetWorkbenchGroup, error)
	UpdateGroup(ctx context.Context, tx Tx, group *domain.AssetWorkbenchGroup) (*domain.AssetWorkbenchGroup, error)
	SetGroupEnabled(ctx context.Context, tx Tx, groupID int64, enabled bool) (*domain.AssetWorkbenchGroup, error)
	AddGroupMembers(ctx context.Context, tx Tx, groupID int64, userIDs []int64) error
	RemoveGroupMembers(ctx context.Context, tx Tx, groupID int64, userIDs []int64) error
	ListGroupMembers(ctx context.Context, groupID int64) ([]*domain.AssetWorkbenchGroupMember, error)

	ListTemplates(ctx context.Context, filter AssetWorkbenchTemplateFilter) ([]*domain.AssetWorkbenchTemplate, int64, error)
	GetTemplate(ctx context.Context, templateID int64) (*domain.AssetWorkbenchTemplate, error)
	CreateTemplate(ctx context.Context, tx Tx, template *domain.AssetWorkbenchTemplate) (*domain.AssetWorkbenchTemplate, error)
	UpdateTemplate(ctx context.Context, tx Tx, template *domain.AssetWorkbenchTemplate) (*domain.AssetWorkbenchTemplate, error)
	SetTemplateEnabled(ctx context.Context, tx Tx, templateID int64, enabled bool) (*domain.AssetWorkbenchTemplate, error)
	ListTemplatesForUser(ctx context.Context, userID int64) ([]*domain.AssetWorkbenchTemplate, error)
	IsTemplateAssignedToUser(ctx context.Context, userID, templateID int64) (bool, error)

	ListTemplateAssignments(ctx context.Context, filter AssetWorkbenchTemplateAssignmentFilter) ([]*domain.AssetWorkbenchTemplateAssignment, int64, error)
	CreateTemplateAssignment(ctx context.Context, tx Tx, assignment *domain.AssetWorkbenchTemplateAssignment) (*domain.AssetWorkbenchTemplateAssignment, error)
	SetTemplateAssignmentEnabled(ctx context.Context, tx Tx, assignmentID int64, enabled bool) (*domain.AssetWorkbenchTemplateAssignment, error)

	ListUploadDirectories(ctx context.Context, filter AssetWorkbenchUploadDirectoryFilter) ([]*domain.AssetWorkbenchUploadDirectory, error)
	GetUploadDirectory(ctx context.Context, directoryID int64) (*domain.AssetWorkbenchUploadDirectory, error)
	CreateUploadDirectory(ctx context.Context, tx Tx, directory *domain.AssetWorkbenchUploadDirectory) (*domain.AssetWorkbenchUploadDirectory, error)
	UpdateUploadDirectory(ctx context.Context, tx Tx, directory *domain.AssetWorkbenchUploadDirectory) (*domain.AssetWorkbenchUploadDirectory, error)

	ListClientMaterials(ctx context.Context, filter AssetWorkbenchClientMaterialFilter) ([]*domain.AssetWorkbenchClientMaterial, error)
	GetClientMaterial(ctx context.Context, materialID int64) (*domain.AssetWorkbenchClientMaterial, error)
	CreateClientMaterial(ctx context.Context, tx Tx, material *domain.AssetWorkbenchClientMaterial) (*domain.AssetWorkbenchClientMaterial, error)
	UpdateClientMaterial(ctx context.Context, tx Tx, material *domain.AssetWorkbenchClientMaterial) (*domain.AssetWorkbenchClientMaterial, error)
	DeleteClientMaterial(ctx context.Context, tx Tx, materialID int64) error
	CreateBatchJob(ctx context.Context, tx Tx, job *domain.AssetWorkbenchBatchJob) (*domain.AssetWorkbenchBatchJob, error)
	GetBatchJob(ctx context.Context, jobID string) (*domain.AssetWorkbenchBatchJob, error)
	ListBatchJobs(ctx context.Context, filter AssetWorkbenchBatchJobFilter) ([]*domain.AssetWorkbenchBatchJob, int64, error)
	ClaimQueuedBatchJobs(ctx context.Context, tx Tx, workerID string, limit int, leaseUntil time.Time) ([]*domain.AssetWorkbenchBatchJob, error)
	MarkBatchJobRunning(ctx context.Context, tx Tx, jobID string, startedAt time.Time) error
	UpdateBatchJobProgress(ctx context.Context, tx Tx, job *domain.AssetWorkbenchBatchJob) error
	CompleteBatchJob(ctx context.Context, tx Tx, job *domain.AssetWorkbenchBatchJob) error

	CreateUploadSession(ctx context.Context, tx Tx, session *domain.AssetWorkbenchUploadSession) (*domain.AssetWorkbenchUploadSession, error)
	GetUploadSession(ctx context.Context, sessionID string) (*domain.AssetWorkbenchUploadSession, error)
	GetUploadSessionForUpdate(ctx context.Context, tx Tx, sessionID string) (*domain.AssetWorkbenchUploadSession, error)
	UpdateUploadSessionStatus(ctx context.Context, tx Tx, sessionID, status string, uploadedAt *time.Time, cancelledAt *time.Time, submittedItemID *int64) error
	ListExpiredUploadSessions(ctx context.Context, now time.Time, limit int) ([]*domain.AssetWorkbenchUploadSession, error)

	CreateSubmission(ctx context.Context, tx Tx, submission *domain.AssetWorkbenchSubmission) (*domain.AssetWorkbenchSubmission, error)
	GetSubmission(ctx context.Context, submissionID int64) (*domain.AssetWorkbenchSubmission, error)
	GetSubmissionForUpdate(ctx context.Context, tx Tx, submissionID int64) (*domain.AssetWorkbenchSubmission, error)
	VoidSubmission(ctx context.Context, tx Tx, submissionID int64, actorID int64, reason string, at time.Time) (*domain.AssetWorkbenchSubmission, error)
	CreateSubmissionItem(ctx context.Context, tx Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error)
	GetSubmissionItem(ctx context.Context, itemID int64) (*domain.AssetWorkbenchSubmissionItem, error)
	UpdateSubmissionItemEditableFields(ctx context.Context, tx Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error)
	UpdateSubmissionItemQCStatus(ctx context.Context, tx Tx, itemID int64, qcStatus string) (*domain.AssetWorkbenchSubmissionItem, error)
	VoidSubmissionItem(ctx context.Context, tx Tx, itemID int64, actorID int64, reason string, at time.Time) (*domain.AssetWorkbenchSubmissionItem, error)
	UpdateSubmissionItemPricing(ctx context.Context, tx Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error)
	CreateSubmissionFile(ctx context.Context, tx Tx, file *domain.AssetWorkbenchSubmissionFile) (*domain.AssetWorkbenchSubmissionFile, error)
	RefreshSubmissionTotals(ctx context.Context, tx Tx, submissionID int64) error
	SearchOverviewRows(ctx context.Context, filter AssetWorkbenchOverviewSearchFilter) ([]*domain.AssetWorkbenchOverviewRow, int64, error)
	DriveListDirectories(ctx context.Context, filter AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveDirectory, error)
	DriveListOrders(ctx context.Context, filter AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveOrder, error)
	DriveListFiles(ctx context.Context, filter AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error)
	DriveSearchFiles(ctx context.Context, filter AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error)
	DriveLocateFile(ctx context.Context, filter AssetWorkbenchDriveFilter, fileID int64) (*domain.AssetWorkbenchDriveFile, error)
	ListSubmissions(ctx context.Context, filter AssetWorkbenchSubmissionFilter) ([]*domain.AssetWorkbenchSubmission, int64, error)
	ListSubmissionItems(ctx context.Context, submissionID int64) ([]*domain.AssetWorkbenchSubmissionItem, error)
	ListSubmissionItemsByMonth(ctx context.Context, businessMonth string) ([]*domain.AssetWorkbenchSubmissionItem, error)
	ListPendingGradeSubmissionItemsForPayee(ctx context.Context, tx Tx, payeeUserID int64, limit int) ([]*domain.AssetWorkbenchSubmissionItem, error)
	ListSubmissionFiles(ctx context.Context, submissionItemID int64) ([]*domain.AssetWorkbenchSubmissionFile, error)
	ListSubmissionFilesForUpdate(ctx context.Context, tx Tx, submissionItemID int64) ([]*domain.AssetWorkbenchSubmissionFile, error)
	GetSubmissionFile(ctx context.Context, fileID int64) (*domain.AssetWorkbenchSubmissionFile, error)
	ListSubmissionFilesByIDs(ctx context.Context, fileIDs []int64) ([]*domain.AssetWorkbenchSubmissionFile, error)
	UpdateSubmissionFileDisplayName(ctx context.Context, tx Tx, fileID int64, displayName string) (*domain.AssetWorkbenchSubmissionFile, error)
	UpdateSubmissionFileLocation(ctx context.Context, tx Tx, file *domain.AssetWorkbenchSubmissionFile) (*domain.AssetWorkbenchSubmissionFile, error)
	DeleteSubmissionFile(ctx context.Context, tx Tx, fileID int64, actorID int64, reason string, at time.Time) error

	ClaimPendingPreviewFiles(ctx context.Context, claim AssetWorkbenchPreviewClaim) ([]*domain.AssetWorkbenchSubmissionFile, error)
	MarkPreviewReady(ctx context.Context, tx Tx, fileID int64, previewKey string) error
	MarkPreviewFailed(ctx context.Context, tx Tx, fileID int64, attempts int, message string, nextRetryAt *time.Time) error

	CreateErrorImportBatch(ctx context.Context, tx Tx, batch *domain.AssetWorkbenchErrorImportBatch) (*domain.AssetWorkbenchErrorImportBatch, error)
	CreateErrorRecord(ctx context.Context, tx Tx, record *domain.AssetWorkbenchErrorRecord) (*domain.AssetWorkbenchErrorRecord, error)
	ListErrorRecordsByMonth(ctx context.Context, businessMonth string) ([]*domain.AssetWorkbenchErrorRecord, error)
	FindActiveDeductionRule(ctx context.Context, workerType, jobGrade, difficultyClass string, asOf time.Time) (*domain.AssetWorkbenchDeductionRule, error)

	LockSettleableItems(ctx context.Context, tx Tx, businessMonth string) ([]*domain.AssetWorkbenchSubmissionItem, error)
	LockSettleableSupplements(ctx context.Context, tx Tx, businessMonth string) ([]*domain.AssetWorkbenchSettlementSupplement, error)
	CreateSettlementBatch(ctx context.Context, tx Tx, batch *domain.AssetWorkbenchSettlementBatch) (*domain.AssetWorkbenchSettlementBatch, error)
	CreateSettlementItem(ctx context.Context, tx Tx, item *domain.AssetWorkbenchSettlementItem) (*domain.AssetWorkbenchSettlementItem, error)
	AttachItemsToSettlementBatch(ctx context.Context, tx Tx, batchID int64, itemIDs []int64) error
	AttachSupplementsToSettlementBatch(ctx context.Context, tx Tx, batchID int64, supplementIDs []int64) error
	ConfirmSettlementBatch(ctx context.Context, tx Tx, batchID int64, actorID int64, at time.Time) error
	FreezeSettlementPayouts(ctx context.Context, tx Tx, batchID int64, at time.Time, snapshots map[int64]json.RawMessage) error
	CancelGeneratedSettlementBatch(ctx context.Context, tx Tx, batchID int64, actorID int64, reason string, at time.Time) error
	LockSettlementBatch(ctx context.Context, tx Tx, batchID int64) (*domain.AssetWorkbenchSettlementBatch, error)
	GetSettlementBatch(ctx context.Context, batchID int64) (*domain.AssetWorkbenchSettlementBatch, error)
	ListSettlementBatches(ctx context.Context, filter AssetWorkbenchSettlementBatchFilter) ([]*domain.AssetWorkbenchSettlementBatch, int64, error)
	ListSettlementItemsByBatch(ctx context.Context, batchID int64) ([]*domain.AssetWorkbenchSettlementItem, error)
	ListConfirmedSettlementItemsByPayee(ctx context.Context, payeeUserID int64) ([]*domain.AssetWorkbenchSettlementItem, error)
	HasConfirmedSettlementForPayeeMonth(ctx context.Context, payeeUserID int64, businessMonth string) (bool, error)
	ListConfirmedSettlementMonthsByPayee(ctx context.Context, payeeUserID int64) ([]string, error)
	CreateSettlementAdjustment(ctx context.Context, tx Tx, item *domain.AssetWorkbenchSettlementAdjustment) (*domain.AssetWorkbenchSettlementAdjustment, error)
	ApplySettlementBatchAdjustment(ctx context.Context, tx Tx, batchID int64, signedAmount float64) error
	ListSettlementSupplements(ctx context.Context, filter AssetWorkbenchSettlementSupplementFilter) ([]*domain.AssetWorkbenchSettlementSupplement, int64, error)
	CreateSettlementSupplement(ctx context.Context, tx Tx, item *domain.AssetWorkbenchSettlementSupplement) (*domain.AssetWorkbenchSettlementSupplement, error)
	GetSettlementSupplementForUpdate(ctx context.Context, tx Tx, id int64) (*domain.AssetWorkbenchSettlementSupplement, error)
	VoidSettlementSupplement(ctx context.Context, tx Tx, id int64) (*domain.AssetWorkbenchSettlementSupplement, error)
	GetSupplementPermission(ctx context.Context, payeeUserID int64, businessMonth string) (*domain.AssetWorkbenchSupplementPermission, error)
	GetSupplementPermissionForUpdate(ctx context.Context, tx Tx, payeeUserID int64, businessMonth string) (*domain.AssetWorkbenchSupplementPermission, error)
	ListSupplementPermissions(ctx context.Context, filter AssetWorkbenchSupplementPermissionFilter) ([]*domain.AssetWorkbenchSupplementPermission, int64, error)
	UpsertSupplementPermission(ctx context.Context, tx Tx, item *domain.AssetWorkbenchSupplementPermission) (*domain.AssetWorkbenchSupplementPermission, error)

	AppendEvent(ctx context.Context, tx Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error)
	ListEvents(ctx context.Context, filter AssetWorkbenchEventFilter) ([]*domain.AssetWorkbenchEvent, int64, error)
	ListSavedViews(ctx context.Context, filter AssetWorkbenchSavedViewFilter) ([]*domain.AssetWorkbenchSavedView, error)
	UpsertSavedView(ctx context.Context, tx Tx, view *domain.AssetWorkbenchSavedView) (*domain.AssetWorkbenchSavedView, error)
	DeleteSavedView(ctx context.Context, tx Tx, userID, viewID int64) error
	MergeProfiles(ctx context.Context, tx Tx, sourceUserID, canonicalUserID int64, fieldChoices map[string]string, actorID int64) error
	CountAccountMergeImpact(ctx context.Context, sourceUserID, canonicalUserID int64) (AssetWorkbenchMergeRewriteCounts, error)
	RewriteAccountOwnership(ctx context.Context, tx Tx, sourceUserID, canonicalUserID int64) (AssetWorkbenchMergeRewriteCounts, error)
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
	UpdatePriority(ctx context.Context, tx Tx, id int64, priority domain.TaskPriority) error
	UpdateProductBinding(ctx context.Context, tx Tx, task *domain.Task) error
	// UpdateStatus performs a direct status update inside a transaction.
	// The service layer is responsible for validating the transition before calling this.
	UpdateStatus(ctx context.Context, tx Tx, id int64, status domain.TaskStatus) error
	UpdateDesigner(ctx context.Context, tx Tx, id int64, designerID *int64) error
	// UpdateHandler sets the current_handler_id (nil clears it).
	UpdateHandler(ctx context.Context, tx Tx, id int64, handlerID *int64) error
	UpdateCustomizationState(ctx context.Context, tx Tx, id int64, lastOperatorID *int64, rejectReason, rejectCategory string) error
}

type TaskCreateRequestRepo interface {
	Reserve(ctx context.Context, actorID int64, clientCreateID, payloadHash, requestPayloadJSON string, expiresAt time.Time) (*domain.TaskCreateRequest, string, error)
	FindRecentActiveByActorPayloadHash(ctx context.Context, actorID int64, payloadHash string, since time.Time) (*domain.TaskCreateRequest, error)
	MarkSucceeded(ctx context.Context, tx Tx, actorID int64, clientCreateID, payloadHash string, taskID int64) error
	MarkFailed(ctx context.Context, actorID int64, clientCreateID, payloadHash, errorMessage string) error
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

type ProductIIDListFilter struct {
	Q        string
	Page     int
	PageSize int
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
	MineActorID                 *int64
	CurrentHandlerID            *int64
	DesignerID                  *int64
	DesignerEmpty               *bool
	NeedOutsource               *bool
	Overdue                     *bool
	CreatedFrom                 *time.Time
	CreatedTo                   *time.Time
	Keyword                     string
	ExcludePendingAuditHandover bool
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
	GetByEmployeeNo(ctx context.Context, employeeNo int) (*domain.User, error)
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
	// GetTeamByDepartmentAndName resolves one team by its department-scoped
	// unique name (backed by uq_org_teams_department_name). Returns nil when
	// no team matches.
	GetTeamByDepartmentAndName(ctx context.Context, departmentID int64, name string) (*domain.OrgTeam, error)
	CreateDepartment(ctx context.Context, tx Tx, department *domain.OrgDepartment) (int64, error)
	UpdateDepartment(ctx context.Context, tx Tx, department *domain.OrgDepartment) error
	CreateTeam(ctx context.Context, tx Tx, team *domain.OrgTeam) (int64, error)
	UpdateTeam(ctx context.Context, tx Tx, team *domain.OrgTeam) error
	// DeleteDepartment / DeleteTeam are hard deletes used by org-master
	// governance for retired rows that have no remaining member references.
	DeleteDepartment(ctx context.Context, tx Tx, id int64) error
	DeleteTeam(ctx context.Context, tx Tx, id int64) error
	DeleteTeamsByDepartment(ctx context.Context, tx Tx, departmentID int64) error
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

type TaskReferenceAssetBindingRepo interface {
	Create(ctx context.Context, tx Tx, binding *domain.TaskReferenceAssetBinding) (*domain.TaskReferenceAssetBinding, error)
	GetByTaskAndRefID(ctx context.Context, taskID int64, refID string) (*domain.TaskReferenceAssetBinding, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskReferenceAssetBinding, error)
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

type KPIAnalysisRepo interface {
	ListTaskEvents(ctx context.Context, filter KPIAnalysisFilter) ([]domain.KPIAnalysisEvent, error)
	ListTaskAssets(ctx context.Context, filter KPIAnalysisFilter) ([]domain.KPIAnalysisAsset, error)
}

type KPIAnalysisFilter struct {
	From  time.Time
	To    time.Time
	Limit int
}

type BusinessTrendRepo interface {
	ListRecentTaskTexts(ctx context.Context, filter BusinessTrendFilter) ([]domain.BusinessTrendTaskText, error)
}

type BusinessTrendFilter struct {
	From           time.Time
	To             time.Time
	Limit          int
	BatchItemLimit int
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
