package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"workflow/cmd/tools/internal/v1migrate"
)

const (
	workflowGroupsSnapshotVersion = 9
	workflowGroupsToolVersion     = "workflow-groups-migrate/v8.9"
	workflowGroupsSchemaVersion   = "124-126"
	previousSnapshotVersion       = 8
	previousToolVersion           = "workflow-groups-migrate/v8.8"
	olderSnapshotVersion          = 7
	olderToolVersion              = "workflow-groups-migrate/v8.7"
	historicalSnapshotVersion     = 6
	historicalToolVersion         = "workflow-groups-migrate/v8.6"
	legacySnapshotVersion         = 4
	legacyToolVersion             = "workflow-groups-migrate/v8.4"
)

var workflowGroupsAutoIncrementTables = []string{
	"task_assets",
	"task_asset_groups",
	"task_asset_group_revisions",
	"task_asset_group_revision_items",
	"task_asset_group_revision_references",
	"task_planning_sku_revisions",
}

type options struct {
	DSN                   string
	DryRun                bool
	Apply                 bool
	Rollback              bool
	SnapshotDir           string
	BatchSize             int
	MappingFile           string
	ReportFile            string
	ConfirmDB             string
	TargetEnvironment     string
	ProductionMarker      string
	ApprovedCommit        string
	ProductionRecoveryRun string
	ProductionRelease     string
}

type recoveryEvidenceTarget struct {
	Environment string
	RunID       string
	Release     string
}

type mappingFile struct {
	Version              int                        `json:"version,omitempty"`
	Resources            []resourceMapping          `json:"resources"`
	Planning             []planningMapping          `json:"planning_tasks"`
	TaskDecisions        []taskStateDecisionMapping `json:"task_state_decisions,omitempty"`
	AssetRecoveries      []assetRecoveryMapping     `json:"asset_recoveries,omitempty"`
	OrganizationMappings []organizationMapping      `json:"organization_mappings,omitempty"`
	AccessDecisions      []accessDecisionMapping    `json:"access_decisions,omitempty"`
}

type resourceMapping struct {
	TaskID              int64                     `json:"task_id"`
	ScopeKind           string                    `json:"scope_kind"`
	ScopeRefID          int64                     `json:"scope_ref_id"`
	Mode                string                    `json:"mode"`
	SourceAssetID       *int64                    `json:"source_task_asset_id"`
	FinalAssetIDs       []int64                   `json:"final_task_asset_ids"`
	ReferenceIDs        []int64                   `json:"reference_file_ref_ids"`
	CreatedBy           int64                     `json:"created_by"`
	TargetStatus        string                    `json:"target_status"`
	Reason              string                    `json:"reason"`
	History             []resourceRevisionMapping `json:"history,omitempty"`
	WorkingRevisionNo   *int                      `json:"working_revision_no,omitempty"`
	FinalizedRevisionNo *int                      `json:"finalized_revision_no,omitempty"`
	V2Declared          bool                      `json:"-"`
}

type planningMapping struct {
	TaskID             int64                 `json:"task_id"`
	TargetTaskStatus   string                `json:"target_task_status"`
	CodeRuleRevisionID int64                 `json:"code_rule_revision_id"`
	CreatedBy          int64                 `json:"created_by"`
	Confidence         string                `json:"confidence"`
	ReviewPolicyIDs    []string              `json:"review_policy_ids"`
	Blockers           []string              `json:"blockers,omitempty"`
	ConfirmedBy        int64                 `json:"confirmed_by"`
	ConfirmedAt        time.Time             `json:"confirmed_at"`
	ConfirmationNote   string                `json:"confirmation_note"`
	ManifestRowHash    string                `json:"manifest_row_hash,omitempty"`
	Items              []planningItemMapping `json:"items"`
}

type planningItemMapping struct {
	TaskSKUItemID   int64   `json:"task_sku_item_id"`
	DescriptionSpec string  `json:"description_spec"`
	Quantity        int64   `json:"quantity"`
	TargetPrice     *string `json:"target_price"`
	Note            string  `json:"note"`
	ReferenceURL    string  `json:"reference_url"`
	ERPProductIID   string  `json:"erp_product_i_id"`
	ERPProductName  string  `json:"erp_product_name"`
	ImageStorageRef string  `json:"image_storage_ref_id"`
}

type taskSnapshot struct {
	ID               int64     `json:"id"`
	TaskType         string    `json:"task_type"`
	TaskStatus       string    `json:"task_status"`
	WorkflowRevision int64     `json:"workflow_revision"`
	CurrentHandlerID *int64    `json:"current_handler_id"`
	UpdatedAt        time.Time `json:"updated_at"`
	EventIDs         []string  `json:"event_ids"`
	ModuleEventIDs   []int64   `json:"module_event_ids,omitempty"`
}

type taskModuleSnapshot struct {
	ID               int64           `json:"id"`
	TaskID           int64           `json:"task_id"`
	ModuleKey        string          `json:"module_key"`
	State            string          `json:"state"`
	PoolTeamCode     *string         `json:"pool_team_code,omitempty"`
	ClaimedBy        *int64          `json:"claimed_by,omitempty"`
	ClaimedTeamCode  *string         `json:"claimed_team_code,omitempty"`
	ClaimedAt        *time.Time      `json:"claimed_at,omitempty"`
	ActorOrgSnapshot json.RawMessage `json:"actor_org_snapshot,omitempty"`
	EnteredAt        time.Time       `json:"entered_at"`
	TerminalAt       *time.Time      `json:"terminal_at,omitempty"`
	Data             json.RawMessage `json:"data"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type searchDocumentSnapshot struct {
	GroupID             int64     `json:"group_id"`
	TaskID              int64     `json:"task_id"`
	FinalizedRevisionID *int64    `json:"finalized_revision_id,omitempty"`
	InternalText        string    `json:"internal_text"`
	FinalText           string    `json:"final_text"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type taskSearchDocumentSnapshot struct {
	TaskID              int64      `json:"task_id"`
	TaskNo              string     `json:"task_no"`
	ProductNameSnapshot string     `json:"product_name_snapshot"`
	SKUCode             string     `json:"sku_code"`
	PrimarySKUCode      string     `json:"primary_sku_code"`
	ProductIID          string     `json:"product_i_id"`
	TaskType            string     `json:"task_type"`
	TaskStatus          string     `json:"task_status"`
	Priority            string     `json:"priority"`
	OwnerDepartment     string     `json:"owner_department"`
	OwnerTeam           string     `json:"owner_team"`
	OwnerOrgTeam        string     `json:"owner_org_team"`
	CreatorID           *int64     `json:"creator_id,omitempty"`
	CreatorName         string     `json:"creator_name"`
	RequesterID         *int64     `json:"requester_id,omitempty"`
	RequesterName       string     `json:"requester_name"`
	DesignerID          *int64     `json:"designer_id,omitempty"`
	DesignerName        string     `json:"designer_name"`
	CurrentHandlerID    *int64     `json:"current_handler_id,omitempty"`
	CurrentHandlerName  string     `json:"current_handler_name"`
	CreatedAt           *time.Time `json:"created_at,omitempty"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
	DeadlineAt          *time.Time `json:"deadline_at,omitempty"`
	AssetText           *string    `json:"asset_text,omitempty"`
	SearchText          string     `json:"search_text"`
}

type resourceGroupSnapshot struct {
	ID                  int64                      `json:"id"`
	TaskID              int64                      `json:"task_id"`
	WorkingRevisionID   *int64                     `json:"working_revision_id,omitempty"`
	FinalizedRevisionID *int64                     `json:"finalized_revision_id,omitempty"`
	LockVersion         int64                      `json:"lock_version"`
	MigrationIncomplete bool                       `json:"migration_incomplete"`
	MigrationIssue      string                     `json:"migration_issue"`
	UpdatedAt           time.Time                  `json:"updated_at"`
	RevisionIDs         []int64                    `json:"revision_ids"`
	Revisions           []resourceRevisionSnapshot `json:"revisions,omitempty"`
}

type resourceRevisionSnapshot struct {
	ID            int64                               `json:"id"`
	GroupID       int64                               `json:"group_id"`
	RevisionNo    int                                 `json:"revision_no"`
	Status        string                              `json:"status"`
	Mode          string                              `json:"mode"`
	SourceAssetID *int64                              `json:"source_task_asset_id,omitempty"`
	SourceStage   string                              `json:"source_stage"`
	CreatedBy     int64                               `json:"created_by"`
	Reason        string                              `json:"reason"`
	SubmittedAt   *time.Time                          `json:"submitted_at,omitempty"`
	FinalizedAt   *time.Time                          `json:"finalized_at,omitempty"`
	CreatedAt     time.Time                           `json:"created_at"`
	Items         []resourceRevisionItemSnapshot      `json:"items"`
	References    []resourceRevisionReferenceSnapshot `json:"references"`
}

type resourceRevisionItemSnapshot struct {
	ID          int64     `json:"id"`
	RevisionID  int64     `json:"revision_id"`
	TaskAssetID int64     `json:"task_asset_id"`
	SortOrder   int       `json:"sort_order"`
	ItemName    string    `json:"item_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type resourceRevisionReferenceSnapshot struct {
	ID                 int64     `json:"id"`
	RevisionID         int64     `json:"revision_id"`
	ReferenceFileRefID int64     `json:"reference_file_ref_id"`
	FormalTaskAssetID  *int64    `json:"formal_task_asset_id,omitempty"`
	SortOrder          int       `json:"sort_order"`
	RefIDSnapshot      string    `json:"ref_id_snapshot"`
	FileNameSnapshot   string    `json:"file_name_snapshot"`
	ScopeSnapshot      string    `json:"scope_snapshot"`
	CreatedAt          time.Time `json:"created_at"`
}

type assetBindingSnapshot struct {
	ID                         int64      `json:"id"`
	TaskID                     int64      `json:"task_id"`
	BindingState               string     `json:"binding_state"`
	BoundGroupID               *int64     `json:"bound_group_id,omitempty"`
	BoundRole                  *string    `json:"bound_role,omitempty"`
	StagedTaskSKUItemID        *int64     `json:"staged_task_sku_item_id,omitempty"`
	StagedRetouchRequirementID *int64     `json:"staged_retouch_requirement_id,omitempty"`
	StagedRole                 *string    `json:"staged_role,omitempty"`
	StagedBy                   *int64     `json:"staged_by,omitempty"`
	UploadSessionID            *string    `json:"upload_session_id,omitempty"`
	StagedExpiresAt            *time.Time `json:"staged_expires_at,omitempty"`
	AccessRevokedAt            *time.Time `json:"access_revoked_at,omitempty"`
	AccessRevokedReason        string     `json:"access_revoked_reason"`
	ObjectDeletedAt            *time.Time `json:"object_deleted_at,omitempty"`
	AssetType                  string     `json:"asset_type,omitempty"`
	ScopeSKUCode               *string    `json:"scope_sku_code"`
	RetouchRequirementID       *int64     `json:"retouch_requirement_id,omitempty"`
	FlowReviewStatus           string     `json:"flow_review_status"`
	ApprovedAt                 *time.Time `json:"approved_at,omitempty"`
	ApprovedBy                 *int64     `json:"approved_by,omitempty"`
	MimeType                   string     `json:"mime_type,omitempty"`
	WholeHash                  string     `json:"whole_hash,omitempty"`
	DeletedAt                  *time.Time `json:"deleted_at,omitempty"`
	CleanedAt                  *time.Time `json:"cleaned_at,omitempty"`
}

type skuOriginSnapshot struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	Origin    *string   `json:"sku_origin"`
	UpdatedAt time.Time `json:"updated_at"`
}

type planningDetailSnapshot struct {
	TaskSKUItemID     int64     `json:"task_sku_item_id"`
	CurrentRevisionID *int64    `json:"current_revision_id"`
	LockVersion       int64     `json:"lock_version"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type planningImageSnapshot struct {
	RevisionID   int64  `json:"revision_id"`
	StorageRefID string `json:"storage_ref_id"`
}

type planningStateSnapshot struct {
	TaskID             int64                    `json:"task_id"`
	SettingsExists     bool                     `json:"settings_exists"`
	ERPSyncMode        string                   `json:"erp_sync_mode"`
	CodeRuleRevisionID int64                    `json:"code_rule_revision_id"`
	ClientCreateID     string                   `json:"client_create_id"`
	CreatedBy          int64                    `json:"created_by"`
	Details            []planningDetailSnapshot `json:"details"`
	RevisionIDs        []int64                  `json:"revision_ids"`
	ImageRevisionIDs   []int64                  `json:"image_revision_ids"`
	Images             []planningImageSnapshot  `json:"images"`
}

type planningCreatedSnapshot struct {
	TaskID           int64   `json:"task_id"`
	SettingsCreated  bool    `json:"settings_created"`
	DetailIDs        []int64 `json:"detail_ids"`
	RevisionIDs      []int64 `json:"revision_ids"`
	ImageRevisionIDs []int64 `json:"image_revision_ids"`
}

type organizationStateSnapshot struct {
	SubjectType      string    `json:"subject_type"`
	SubjectID        int64     `json:"subject_id"`
	LegacyDepartment string    `json:"legacy_department"`
	LegacyTeam       string    `json:"legacy_team"`
	DepartmentID     *int64    `json:"department_id"`
	TeamID           *int64    `json:"team_id"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type accessStateSnapshot struct {
	UserID      int64                      `json:"user_id"`
	Assignments []accessAssignmentEvidence `json:"assignments"`
}

type assetStorageRefStatusSnapshot struct {
	RefID   string `json:"ref_id"`
	AssetID *int64 `json:"asset_id,omitempty"`
	Status  string `json:"status"`
}

type autoIncrementSnapshot struct {
	Table     string `json:"table"`
	NextValue int64  `json:"next_value"`
}

type snapshot struct {
	Version                       int                             `json:"version"`
	ToolVersion                   string                          `json:"tool_version"`
	SchemaVersion                 string                          `json:"schema_version"`
	Database                      string                          `json:"database"`
	MappingSHA256                 string                          `json:"mapping_sha256"`
	IntegritySHA256               string                          `json:"integrity_sha256"`
	ApplyState                    string                          `json:"apply_state"`
	CreatedAt                     time.Time                       `json:"created_at"`
	AppliedAt                     *time.Time                      `json:"applied_at"`
	Tasks                         []taskSnapshot                  `json:"tasks_before"`
	AfterTasks                    []taskSnapshot                  `json:"tasks_after"`
	TaskModulesBefore             []taskModuleSnapshot            `json:"task_modules_before"`
	TaskModulesAfter              []taskModuleSnapshot            `json:"task_modules_after"`
	SearchDocumentsBefore         []searchDocumentSnapshot        `json:"search_documents_before"`
	SearchDocumentsAfter          []searchDocumentSnapshot        `json:"search_documents_after"`
	TaskSearchDocumentsBefore     []taskSearchDocumentSnapshot    `json:"task_search_documents_before"`
	TaskSearchDocumentsAfter      []taskSearchDocumentSnapshot    `json:"task_search_documents_after"`
	ResourceGroups                []resourceGroupSnapshot         `json:"resource_groups_before"`
	AfterResourceGroups           []resourceGroupSnapshot         `json:"resource_groups_after"`
	AssetBindings                 []assetBindingSnapshot          `json:"asset_bindings_before"`
	AfterAssetBindings            []assetBindingSnapshot          `json:"asset_bindings_after"`
	SKUOrigins                    []skuOriginSnapshot             `json:"sku_origins_before"`
	AfterSKUOrigins               []skuOriginSnapshot             `json:"sku_origins_after"`
	PlanningBefore                []planningStateSnapshot         `json:"planning_before"`
	PlanningAfter                 []planningStateSnapshot         `json:"planning_after"`
	PlanningCreated               []planningCreatedSnapshot       `json:"planning_created"`
	OrganizationBefore            []organizationStateSnapshot     `json:"organization_before,omitempty"`
	OrganizationAfter             []organizationStateSnapshot     `json:"organization_after,omitempty"`
	AccessBefore                  []accessStateSnapshot           `json:"access_before,omitempty"`
	AccessAfter                   []accessStateSnapshot           `json:"access_after,omitempty"`
	StorageRefsBefore             []assetStorageRefStatusSnapshot `json:"asset_storage_refs_before,omitempty"`
	StorageRefsAfter              []assetStorageRefStatusSnapshot `json:"asset_storage_refs_after,omitempty"`
	AutoIncrementsBefore          []autoIncrementSnapshot         `json:"auto_increments_before"`
	AutoIncrementsAfter           []autoIncrementSnapshot         `json:"auto_increments_after"`
	AutoIncrementRecoveryCeilings []autoIncrementSnapshot         `json:"auto_increment_recovery_ceilings"`
	InsertedGroupIDs              []int64                         `json:"inserted_group_ids"`
	AppliedRevisionIDs            []int64                         `json:"applied_revision_ids"`
	InsertedAliasIDs              []int64                         `json:"inserted_alias_asset_ids,omitempty"`
}

type report struct {
	Mode                   string                   `json:"mode"`
	Database               string                   `json:"database"`
	GeneratedAt            time.Time                `json:"generated_at"`
	MappingFileSHA256      string                   `json:"mapping_file_sha256"`
	MappingCanonicalSHA256 string                   `json:"mapping_canonical_sha256"`
	Counts                 map[string]int64         `json:"counts"`
	StateCounts            map[string]int64         `json:"state_counts"`
	ManualTaskIDs          []int64                  `json:"manual_task_ids"`
	ManualTaskIssues       []taskMigrationIssue     `json:"manual_task_issues"`
	ManualResourceGroupIDs []int64                  `json:"manual_resource_group_ids"`
	ManualAccessUserIDs    []int64                  `json:"manual_access_user_ids"`
	ManualAccessIssues     []accessMigrationIssue   `json:"manual_access_issues"`
	ManualOrgIssues        []orgMigrationIssue      `json:"manual_org_issues"`
	ManualResourceIssues   []resourceMigrationIssue `json:"manual_resource_issues"`
	MappingResourceCount   int                      `json:"mapping_resource_count"`
	MappingPlanningCount   int                      `json:"mapping_planning_count"`
	MappingCandidateIssues []mappingCandidateIssue  `json:"mapping_candidate_issues,omitempty"`
	Warnings               []string                 `json:"warnings"`
}

type mappingCandidateIssue struct {
	TaskID     int64    `json:"task_id"`
	ScopeKind  string   `json:"scope_kind"`
	ScopeRefID int64    `json:"scope_ref_id"`
	RevisionNo int      `json:"revision_no"`
	Confidence string   `json:"confidence"`
	Reason     string   `json:"reason"`
	Blockers   []string `json:"blockers,omitempty"`
}

type orgMigrationIssue struct {
	SubjectType string `json:"subject_type"`
	SubjectID   int64  `json:"subject_id"`
	Reason      string `json:"reason"`
}

type accessMigrationIssue struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

type taskMigrationIssue struct {
	TaskID int64  `json:"task_id"`
	Reason string `json:"reason"`
}

type resourceMigrationIssue struct {
	GroupID int64  `json:"group_id"`
	TaskID  int64  `json:"task_id"`
	Reason  string `json:"reason"`
}

type cutoverBlockers struct {
	Org       []orgMigrationIssue
	Access    []accessMigrationIssue
	Tasks     []taskMigrationIssue
	Resources []resourceMigrationIssue
}

func (b cutoverBlockers) Empty() bool {
	return len(b.Org) == 0 && len(b.Access) == 0 && len(b.Tasks) == 0 && len(b.Resources) == 0
}

func main() {
	var o options
	flag.StringVar(&o.DSN, "dsn", os.Getenv("MYSQL_DSN"), "MySQL DSN")
	flag.BoolVar(&o.DryRun, "dry-run", true, "read-only analysis and report (default)")
	flag.BoolVar(&o.Apply, "apply", false, "apply deterministic state and explicit mapping changes")
	flag.BoolVar(&o.Rollback, "rollback", false, "rollback this tool's apply using its snapshot manifest")
	flag.StringVar(&o.SnapshotDir, "snapshot-dir", "", "required for apply/rollback; external DB/object snapshots remain operator-owned")
	flag.IntVar(&o.BatchSize, "batch-size", 500, "read/report batch size")
	flag.StringVar(&o.MappingFile, "mapping-file", "", "JSON file containing only manually confirmed resource/planning mappings")
	flag.StringVar(&o.ReportFile, "report-file", "", "optional JSON report path")
	flag.StringVar(&o.ConfirmDB, "confirm-database", "", "required for writes and must exactly match SELECT DATABASE()")
	flag.StringVar(&o.TargetEnvironment, "target-environment", "clone_b", "clone_b or production")
	flag.StringVar(&o.ProductionMarker, "production-marker", "", "exact production cutover marker file")
	flag.StringVar(&o.ApprovedCommit, "approved-commit", "", "exact production-approved Git commit")
	flag.StringVar(&o.ProductionRecoveryRun, "production-recovery-run-id", "", "exact production asset recovery run id")
	flag.StringVar(&o.ProductionRelease, "production-release", "", "exact production release name")
	flag.Parse()

	if err := run(context.Background(), o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(v1migrate.ExitCode(err))
	}
}

func run(ctx context.Context, o options) error {
	if err := validateOptions(o); err != nil {
		return err
	}
	db, err := v1migrate.OpenDB(o.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	// information_schema.TABLES is cached by MySQL unless the session expiry is
	// disabled. This tool snapshots and restores AUTO_INCREMENT exactly, so all
	// queries and transactions must share the configured session.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, "SET SESSION information_schema_stats_expiry = 0"); err != nil {
		return fmt.Errorf("configure exact information_schema metadata: %w", err)
	}

	database, err := currentDatabase(ctx, db)
	if err != nil {
		return err
	}
	if (o.Apply || o.Rollback) && o.ConfirmDB != database {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "write guard: --confirm-database=%q does not match %q", o.ConfirmDB, database)
	}
	if o.TargetEnvironment == "production" && isCloneBDatabaseName(database) {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "production target cannot use Clone B database %q", database)
	}
	recoveryTarget, err := recoveryEvidenceTargetFromOptions(o)
	if err != nil {
		return err
	}

	mapping, err := readMapping(o.MappingFile)
	if err != nil {
		return err
	}
	mappingFileSHA256, err := exactFileDigest(o.MappingFile)
	if err != nil {
		return fmt.Errorf("hash mapping file: %w", err)
	}
	mappingCanonicalSHA256, err := mappingDigest(mapping)
	if err != nil {
		return fmt.Errorf("hash canonical mapping: %w", err)
	}
	if o.Apply || o.Rollback {
		if err := validateMapping(mapping); err != nil {
			return err
		}
	} else if err := validateCandidateMapping(mapping); err != nil {
		return err
	}
	if o.Apply {
		if err := validateFormalApplyMapping(mapping); err != nil {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "%v", err)
		}
	}
	if !o.Rollback {
		if err := validatePrematerializedAssetRecoveries(ctx, db, mapping.AssetRecoveries, recoveryTarget); err != nil {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "%s recovery preflight failed: %v", recoveryTarget.Environment, err)
		}
		if err := validatePlanningImages(ctx, db, mapping.Planning, false); err != nil {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "planning image preflight failed: %v", err)
		}
	}

	switch {
	case o.Rollback:
		return rollback(ctx, db, database, o, mapping)
	case o.Apply:
		if err := apply(ctx, db, database, o, mapping); err != nil {
			return err
		}
	}

	r, err := buildReport(ctx, db, database, mapping, mappingFileSHA256, mappingCanonicalSHA256)
	if err != nil {
		return err
	}
	if o.Apply {
		r.Mode = "apply"
	} else {
		r.Mode = "dry-run"
	}
	if err := writeJSONReport(r, o.ReportFile); err != nil {
		return err
	}
	if len(r.MappingCandidateIssues) > 0 {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "dry-run completed with %d proposed/hard-blocked mapping revisions; inspect mapping_candidate_issues", len(r.MappingCandidateIssues))
	}
	return nil
}

func validateOptions(o options) error {
	if o.BatchSize < 1 || o.BatchSize > 10000 {
		return fmt.Errorf("--batch-size must be between 1 and 10000")
	}
	if o.Apply && o.Rollback {
		return fmt.Errorf("--apply and --rollback are mutually exclusive")
	}
	if (o.Apply || o.Rollback) && strings.TrimSpace(o.SnapshotDir) == "" {
		return fmt.Errorf("--snapshot-dir is required for apply/rollback")
	}
	if (o.Apply || o.Rollback) && strings.TrimSpace(o.MappingFile) == "" {
		return fmt.Errorf("--mapping-file is required for apply/rollback so the journal stays bound to the reviewed mapping")
	}
	if _, err := recoveryEvidenceTargetFromOptions(o); err != nil {
		return err
	}
	return nil
}

func recoveryEvidenceTargetFromOptions(o options) (recoveryEvidenceTarget, error) {
	switch o.TargetEnvironment {
	case "", "clone_b":
		return recoveryEvidenceTarget{Environment: "clone_b"}, nil
	case "production":
		if err := validateWorkflowProductionMarker(o.ProductionMarker, o.ApprovedCommit); err != nil {
			return recoveryEvidenceTarget{}, err
		}
		if !recoveryRunIDPattern.MatchString(o.ProductionRecoveryRun) {
			return recoveryEvidenceTarget{}, errors.New("--production-recovery-run-id is invalid")
		}
		if !recoveryRunIDPattern.MatchString(o.ProductionRelease) ||
			!strings.HasPrefix(o.ProductionRelease, "v") {
			return recoveryEvidenceTarget{}, errors.New("--production-release is invalid")
		}
		return recoveryEvidenceTarget{
			Environment: "production",
			RunID:       o.ProductionRecoveryRun,
			Release:     o.ProductionRelease,
		}, nil
	default:
		return recoveryEvidenceTarget{}, errors.New("--target-environment must be clone_b or production")
	}
}

func recoveryEvidenceTargetOrClone(targets []recoveryEvidenceTarget) recoveryEvidenceTarget {
	if len(targets) == 1 {
		return targets[0]
	}
	return recoveryEvidenceTarget{Environment: "clone_b"}
}

func validateWorkflowProductionMarker(path, approvedCommit string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("--production-marker is required for production")
	}
	if len(approvedCommit) != 40 {
		return errors.New("--approved-commit must be an exact 40-character Git SHA")
	}
	if _, err := hex.DecodeString(approvedCommit); err != nil {
		return errors.New("--approved-commit must be an exact 40-character Git SHA")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve production marker: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return fmt.Errorf("read production marker: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Size() <= 0 || info.Size() > 1024 {
		return errors.New("production marker must be a small regular non-symlink file")
	}
	raw, err := os.ReadFile(absolute)
	if err != nil {
		return fmt.Errorf("read production marker: %w", err)
	}
	if string(raw) != "APPROVED_COMMIT="+approvedCommit+"\n" {
		return errors.New("production marker does not exactly approve the requested commit")
	}
	return nil
}

func isCloneBDatabaseName(database string) bool {
	value := strings.ToLower(strings.TrimSpace(database))
	return strings.HasPrefix(value, "ab_") && strings.HasSuffix(value, "_b")
}

func readMapping(path string) (mappingFile, error) {
	if strings.TrimSpace(path) == "" {
		return mappingFile{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return mappingFile{}, err
	}
	var m mappingFile
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return mappingFile{}, fmt.Errorf("decode mapping file: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return mappingFile{}, fmt.Errorf("decode mapping file: trailing JSON values are not allowed")
	}
	if mappingVersion(m) == workflowGroupsMappingV2 {
		for i := range m.Resources {
			m.Resources[i].V2Declared = true
		}
	}
	return m, nil
}

func validateMapping(m mappingFile) error {
	return validateMappingMode(m, false)
}

func validateCandidateMapping(m mappingFile) error {
	return validateMappingMode(m, true)
}

func validateMappingMode(m mappingFile, allowCandidateConfidence bool) error {
	if mappingVersion(m) != workflowGroupsMappingV1 && mappingVersion(m) != workflowGroupsMappingV2 {
		return fmt.Errorf("mapping version must be 1 or 2")
	}
	if err := validateTaskStateDecisions(m, allowCandidateConfidence); err != nil {
		return err
	}
	if err := validateAssetRecoveries(m, allowCandidateConfidence); err != nil {
		return err
	}
	if err := validateOrganizationMappings(m, allowCandidateConfidence); err != nil {
		return err
	}
	if err := validateAccessDecisions(m, allowCandidateConfidence); err != nil {
		return err
	}
	seenResource := map[string]struct{}{}
	resourceTasks := map[int64]struct{}{}
	retouchVisualTask2533Scopes := map[int64]struct{}{}
	for i, r := range m.Resources {
		if mappingVersion(m) == workflowGroupsMappingV2 {
			r.V2Declared = true
			var err error
			if allowCandidateConfidence {
				err = validateCandidateResourceMappingV2(i, r)
			} else {
				err = validateResourceMappingV2(i, r)
			}
			if err != nil {
				return err
			}
			m.Resources[i].V2Declared = true
			key := fmt.Sprintf("%d/%s/%d", r.TaskID, r.ScopeKind, r.ScopeRefID)
			if _, ok := seenResource[key]; ok {
				return fmt.Errorf("resources[%d]: duplicate scope %s", i, key)
			}
			seenResource[key] = struct{}{}
			resourceTasks[r.TaskID] = struct{}{}
			for _, revision := range r.History {
				if containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyRetouchVisualScopeTask2533) {
					retouchVisualTask2533Scopes[r.ScopeRefID] = struct{}{}
				}
			}
			continue
		}
		if r.TaskID <= 0 || r.CreatedBy <= 0 {
			return fmt.Errorf("resources[%d]: task_id and created_by are required", i)
		}
		if r.ScopeKind != "task" && r.ScopeKind != "sku" && r.ScopeKind != "retouch_requirement" {
			return fmt.Errorf("resources[%d]: invalid scope_kind", i)
		}
		if (r.ScopeKind == "task" && r.ScopeRefID != 0) || (r.ScopeKind != "task" && r.ScopeRefID <= 0) {
			return fmt.Errorf("resources[%d]: invalid scope_ref_id", i)
		}
		if r.TargetStatus == "shell" {
			if r.Mode != "" || r.SourceAssetID != nil || len(r.FinalAssetIDs) != 0 || len(r.ReferenceIDs) != 0 {
				return fmt.Errorf("resources[%d]: shell mappings cannot contain resource files", i)
			}
		} else if (r.Mode == "single" && len(r.FinalAssetIDs) != 1) || (r.Mode == "set" && len(r.FinalAssetIDs) < 2) {
			return fmt.Errorf("resources[%d]: single requires one final and set requires at least two", i)
		}
		if r.TargetStatus != "shell" && r.Mode != "single" && r.Mode != "set" {
			return fmt.Errorf("resources[%d]: invalid mode", i)
		}
		if r.TargetStatus != "shell" && r.TargetStatus != "draft" && r.TargetStatus != "submitted" && r.TargetStatus != "finalized" {
			return fmt.Errorf("resources[%d]: target_status must be shell, draft, submitted, or finalized", i)
		}
		seenFinal := map[int64]struct{}{}
		for _, assetID := range r.FinalAssetIDs {
			if assetID <= 0 {
				return fmt.Errorf("resources[%d]: final asset ids must be positive", i)
			}
			if _, duplicate := seenFinal[assetID]; duplicate {
				return fmt.Errorf("resources[%d]: final asset ids must be unique and ordered", i)
			}
			seenFinal[assetID] = struct{}{}
			if r.SourceAssetID != nil && assetID == *r.SourceAssetID {
				return fmt.Errorf("resources[%d]: source asset cannot also be a final asset", i)
			}
		}
		key := fmt.Sprintf("%d/%s/%d", r.TaskID, r.ScopeKind, r.ScopeRefID)
		if _, ok := seenResource[key]; ok {
			return fmt.Errorf("resources[%d]: duplicate scope %s", i, key)
		}
		seenResource[key] = struct{}{}
		resourceTasks[r.TaskID] = struct{}{}
	}
	if len(retouchVisualTask2533Scopes) != 0 {
		if len(retouchVisualTask2533Scopes) != len(legacyRetouchVisualTask2533) {
			return fmt.Errorf("%s requires all five exact task 2533 requirement scopes", reviewPolicyLegacyRetouchVisualScopeTask2533)
		}
		for scopeID := range legacyRetouchVisualTask2533 {
			if _, exists := retouchVisualTask2533Scopes[scopeID]; !exists {
				return fmt.Errorf("%s is missing task 2533 requirement %d", reviewPolicyLegacyRetouchVisualScopeTask2533, scopeID)
			}
		}
	}
	seenPlanning := map[int64]struct{}{}
	for i, p := range m.Planning {
		if p.TaskID <= 0 {
			return fmt.Errorf("planning_tasks[%d]: task_id is required", i)
		}
		if mappingVersion(m) == workflowGroupsMappingV2 {
			path := fmt.Sprintf("planning_tasks[%d]", i)
			if err := validateReviewPolicyIDs(path, p.ReviewPolicyIDs); err != nil {
				return err
			}
			for _, requiredPolicy := range []string{
				reviewPolicyLegacyPurchaseToSKUPlanning,
				reviewPolicyFrozenSKUPlanningRuleRevision9,
			} {
				if !containsString(p.ReviewPolicyIDs, requiredPolicy) {
					return fmt.Errorf("%s: review_policy_ids must include %s", path, requiredPolicy)
				}
			}
			if p.CodeRuleRevisionID != 9 {
				return fmt.Errorf("%s: %s requires code_rule_revision_id=9", path, reviewPolicyFrozenSKUPlanningRuleRevision9)
			}
			tombstonePolicy := containsString(p.ReviewPolicyIDs, reviewPolicyLegacyIncompleteUATPlanningTombstone)
			if tombstonePolicy {
				if !isIncompleteUATPlanningTombstone(p) {
					return fmt.Errorf("%s: %s is restricted to task_id=497, target_task_status=Cancelled, rule revision 9, exact SKU item 380, and zero inferred planning fields", path, reviewPolicyLegacyIncompleteUATPlanningTombstone)
				}
			}
			expectedHash, err := planningManifestRowHash(p)
			if err != nil {
				return fmt.Errorf("%s: compute manifest_row_hash: %w", path, err)
			}
			if !sha256Pattern.MatchString(p.ManifestRowHash) || p.ManifestRowHash != expectedHash {
				return fmt.Errorf("%s: manifest_row_hash does not match canonical planning content", path)
			}
			hardCandidate := allowCandidateConfidence && p.Confidence == "hard_blocked"
			if !hardCandidate && !validPlanningTargetStatus(p.TargetTaskStatus) {
				return fmt.Errorf("planning_tasks[%d]: target_task_status is required and must be a current task status", i)
			}
			switch p.Confidence {
			case "confirmed_auto":
				if len(p.Blockers) != 0 {
					return fmt.Errorf("planning_tasks[%d]: confirmed mapping cannot retain candidate blockers", i)
				}
				if len(p.Items) == 0 || p.CodeRuleRevisionID <= 0 || p.CreatedBy <= 0 || p.ConfirmedBy <= 0 || p.ConfirmedAt.IsZero() || strings.TrimSpace(p.ConfirmationNote) == "" {
					return fmt.Errorf("planning_tasks[%d]: confirmed mapping requires rule, creator and complete confirmation metadata", i)
				}
			case "proposed_review", "hard_blocked":
				if !allowCandidateConfidence {
					return fmt.Errorf("planning_tasks[%d]: confidence=%s cannot be applied", i, p.Confidence)
				}
			default:
				return fmt.Errorf("planning_tasks[%d]: confidence must be confirmed_auto, proposed_review, or hard_blocked", i)
			}
		} else if p.CodeRuleRevisionID <= 0 || p.CreatedBy <= 0 {
			return fmt.Errorf("planning_tasks[%d]: rule and creator are required", i)
		}
		if _, ok := seenPlanning[p.TaskID]; ok {
			return fmt.Errorf("planning_tasks[%d]: duplicate task_id", i)
		}
		if _, hasResources := resourceTasks[p.TaskID]; hasResources {
			return fmt.Errorf("planning_tasks[%d]: task %d cannot migrate both planning data and design resources", i, p.TaskID)
		}
		seenPlanning[p.TaskID] = struct{}{}
		for j, item := range p.Items {
			if item.TaskSKUItemID <= 0 || (!isIncompleteUATPlanningTombstone(p) && !allowCandidateConfidence && (strings.TrimSpace(item.DescriptionSpec) == "" || item.Quantity <= 0)) {
				return fmt.Errorf("planning_tasks[%d].items[%d]: item id, description and positive quantity are required", i, j)
			}
		}
	}
	return nil
}

func validPlanningTargetStatus(status string) bool {
	switch status {
	case "Draft", "PendingAssign", "Assigned", "InProgress", "PendingAudit", "Completed", "Archived", "Blocked", "Cancelled":
		return true
	default:
		return false
	}
}

func currentDatabase(ctx context.Context, db *sql.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

func buildReport(ctx context.Context, db *sql.DB, database string, m mappingFile, mappingFileSHA256, mappingCanonicalSHA256 string) (report, error) {
	r := report{
		Database: database, GeneratedAt: time.Now().UTC(),
		MappingFileSHA256: mappingFileSHA256, MappingCanonicalSHA256: mappingCanonicalSHA256,
		Counts: map[string]int64{}, StateCounts: map[string]int64{},
		MappingResourceCount: len(m.Resources), MappingPlanningCount: len(m.Planning),
	}
	queries := map[string]string{
		"tasks":                           `SELECT COUNT(*) FROM tasks`,
		"tasks_without_stable_department": `SELECT COUNT(*) FROM tasks WHERE owner_department_id IS NULL`,
		"users_without_stable_department": `SELECT COUNT(*) FROM users WHERE department_id IS NULL`,
		"tasks_without_resource_groups":   `SELECT COUNT(*) FROM tasks t LEFT JOIN task_asset_groups g ON g.task_id=t.id WHERE g.id IS NULL AND t.task_type <> 'sku_planning'`,
		"legacy_planning_candidates":      `SELECT COUNT(*) FROM tasks WHERE task_type='purchase_task'`,
		"migration_incomplete_groups":     `SELECT COUNT(*) FROM task_asset_groups WHERE migration_incomplete=1`,
	}
	for name, query := range queries {
		var count int64
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return r, fmt.Errorf("report %s: %w", name, err)
		}
		r.Counts[name] = count
	}
	rows, err := db.QueryContext(ctx, `SELECT task_status, COUNT(*) FROM tasks GROUP BY task_status ORDER BY task_status`)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			rows.Close()
			return r, err
		}
		r.StateCounts[status] = n
	}
	rows.Close()
	blockers, err := queryCutoverBlockers(ctx, db, m)
	if err != nil {
		return r, err
	}
	r.ManualOrgIssues = blockers.Org
	r.ManualAccessIssues = blockers.Access
	r.ManualTaskIssues = blockers.Tasks
	r.ManualResourceIssues = blockers.Resources
	r.MappingCandidateIssues = collectMappingCandidateIssues(m)
	for _, item := range blockers.Tasks {
		r.ManualTaskIDs = append(r.ManualTaskIDs, item.TaskID)
	}
	seenAccessUsers := map[int64]struct{}{}
	for _, item := range blockers.Access {
		if _, exists := seenAccessUsers[item.UserID]; !exists {
			seenAccessUsers[item.UserID] = struct{}{}
			r.ManualAccessUserIDs = append(r.ManualAccessUserIDs, item.UserID)
		}
	}
	for _, item := range blockers.Resources {
		r.ManualResourceGroupIDs = append(r.ManualResourceGroupIDs, item.GroupID)
	}
	if len(r.ManualTaskIDs) > 0 {
		r.Warnings = append(r.Warnings, "manual_task_ids require explicit mapping; the tool never infers missing business facts")
	}
	if len(r.ManualAccessUserIDs) > 0 {
		r.Warnings = append(r.Warnings, "manual_access_user_ids remain blockers until an administrator confirms an exact preserve-existing or no-new-grant decision")
	}
	if len(r.ManualOrgIssues) > 0 {
		r.Warnings = append(r.Warnings, "manual_org_issues block apply; stable organization ids are never inferred from ambiguous names")
	}
	if len(r.ManualResourceGroupIDs) > 0 {
		r.Warnings = append(r.Warnings, "manual_resource_group_ids lack a confirmed ordered mapping; apply is blocked until every migration marker is resolved")
	}
	if len(r.MappingCandidateIssues) > 0 {
		r.Warnings = append(r.Warnings, "mapping candidate contains proposed_review/hard_blocked revisions; dry-run reports all candidates but apply remains blocked")
	}
	return r, nil
}

func collectMappingCandidateIssues(m mappingFile) []mappingCandidateIssue {
	var issues []mappingCandidateIssue
	for _, resource := range m.Resources {
		for _, revision := range resource.History {
			if revision.Confidence != "proposed_review" && revision.Confidence != "hard_blocked" {
				continue
			}
			issues = append(issues, mappingCandidateIssue{
				TaskID: resource.TaskID, ScopeKind: resource.ScopeKind, ScopeRefID: resource.ScopeRefID,
				RevisionNo: revision.RevisionNo, Confidence: revision.Confidence, Reason: revision.Reason,
				Blockers: append([]string(nil), revision.Blockers...),
			})
		}
	}
	for _, planning := range m.Planning {
		if planning.Confidence != "proposed_review" && planning.Confidence != "hard_blocked" {
			continue
		}
		issues = append(issues, mappingCandidateIssue{
			TaskID: planning.TaskID, ScopeKind: "planning", Confidence: planning.Confidence,
			Reason:   "planning rule, creator, fields, and target status require explicit review",
			Blockers: append([]string(nil), planning.Blockers...),
		})
	}
	for _, decision := range m.TaskDecisions {
		if decision.Confidence != "proposed_review" && decision.Confidence != "hard_blocked" {
			continue
		}
		issues = append(issues, mappingCandidateIssue{
			TaskID: decision.TaskID, ScopeKind: "task_state_decision",
			Confidence: decision.Confidence,
			Reason:     fmt.Sprintf("task state transition %s -> %s requires explicit review", decision.FromStatus, decision.TargetStatus),
			Blockers:   append([]string(nil), decision.Blockers...),
		})
	}
	for _, recovery := range m.AssetRecoveries {
		if recovery.Confidence != "proposed_review" && recovery.Confidence != "hard_blocked" {
			continue
		}
		reason := fmt.Sprintf(
			"task asset %d recovery identity requires isolated Clone B pre-materialization and explicit review",
			recovery.MissingTaskAssetID,
		)
		if recovery.Strategy == "historical_unavailable_tombstone_v1" {
			reason = fmt.Sprintf(
				"task asset %d is proven historically unavailable; API/UI and object-integrity gates must expose the tombstone without claiming original bytes exist",
				recovery.MissingTaskAssetID,
			)
		}
		issues = append(issues, mappingCandidateIssue{
			TaskID: recovery.TaskID, ScopeKind: "asset_recovery",
			ScopeRefID: recovery.MissingTaskAssetID,
			Confidence: recovery.Confidence,
			Reason:     reason,
			Blockers:   append([]string(nil), recovery.Blockers...),
		})
	}
	return issues
}

func queryCutoverBlockers(ctx context.Context, q snapshotQueryer, m mappingFile) (cutoverBlockers, error) {
	m = normalizeMapping(m)
	var blockers cutoverBlockers
	var err error
	rawOrgIssues, err := queryOrgMigrationIssues(ctx, q)
	if err != nil {
		return blockers, err
	}
	blockers.Org, err = resolveOrganizationMigrationIssues(ctx, q, rawOrgIssues, m.OrganizationMappings)
	if err != nil {
		return blockers, err
	}
	rows, err := q.QueryContext(ctx, `
		SELECT ur.user_id,ur.role
		FROM user_roles ur
		JOIN users u ON u.id=ur.user_id AND u.status='active'
		WHERE ur.role NOT IN ('Member','SuperAdmin','Admin','RoleAdmin','Ops','Designer','CustomizationOperator','Audit_A','Audit_B','DesignReviewer','CustomizationReviewer','AssetSubmitter','AssetManager','AssetTemplateAdmin','AssetSettlement','HRAdmin','DepartmentAdmin','TeamLead','DesignDirector','ERP')
		   OR (ur.role IN ('DepartmentAdmin','DesignDirector') AND u.department_id IS NULL)
		   OR (ur.role='TeamLead' AND u.team_id IS NULL)
		ORDER BY ur.user_id,ur.role`)
	if err != nil {
		return blockers, err
	}
	rawAccessIssues := []accessMigrationIssue{}
	for rows.Next() {
		var item accessMigrationIssue
		if err := rows.Scan(&item.UserID, &item.Role); err != nil {
			rows.Close()
			return blockers, err
		}
		switch item.Role {
		case "DepartmentAdmin", "DesignDirector":
			item.Reason = "role migration requires a stable department id"
		case "TeamLead":
			item.Reason = "role migration requires a stable team id"
		default:
			item.Reason = "active legacy role requires an explicit administrator-reviewed assignment before auth cutover"
		}
		rawAccessIssues = append(rawAccessIssues, item)
	}
	if err := rows.Close(); err != nil {
		return blockers, err
	}
	blockers.Access, err = resolveAccessMigrationIssues(ctx, q, rawAccessIssues, m.AccessDecisions)
	if err != nil {
		return blockers, err
	}
	mappedPlanning := map[int64]planningMapping{}
	duplicatePlanningTasks := map[int64]bool{}
	for _, item := range m.Planning {
		confirmed := mappingVersion(m) != workflowGroupsMappingV2 || item.Confidence == "confirmed_auto"
		if confirmed && item.TaskID > 0 && item.CodeRuleRevisionID > 0 && item.CreatedBy > 0 && len(item.Items) > 0 {
			if _, exists := mappedPlanning[item.TaskID]; exists {
				duplicatePlanningTasks[item.TaskID] = true
			}
			mappedPlanning[item.TaskID] = item
		}
	}
	reviewedTaskDecisions := map[int64]taskStateDecisionMapping{}
	for _, decision := range m.TaskDecisions {
		if mappingVersion(m) == workflowGroupsMappingV2 && decision.Confidence != "confirmed_auto" {
			continue
		}
		reviewedTaskDecisions[decision.TaskID] = decision
		if issue := validateTaskStateDecisionPreflight(ctx, q, decision, m.Resources); issue != nil {
			blockers.Tasks = append(blockers.Tasks, *issue)
		}
	}
	rows, err = q.QueryContext(ctx, `SELECT id,task_type,task_status FROM tasks WHERE task_type='purchase_task' OR task_status='RejectedByWarehouse' ORDER BY id`)
	if err != nil {
		return blockers, err
	}
	type taskCandidate struct {
		ID     int64
		Type   string
		Status string
	}
	taskCandidates := []taskCandidate{}
	for rows.Next() {
		var taskID int64
		var taskType, taskStatus string
		if err := rows.Scan(&taskID, &taskType, &taskStatus); err != nil {
			rows.Close()
			return blockers, err
		}
		taskCandidates = append(taskCandidates, taskCandidate{ID: taskID, Type: taskType, Status: taskStatus})
	}
	if err := rows.Close(); err != nil {
		return blockers, err
	}
	taskTypeByID := map[int64]string{}
	taskStatusByID := map[int64]string{}
	for _, candidate := range taskCandidates {
		taskTypeByID[candidate.ID] = candidate.Type
		taskStatusByID[candidate.ID] = candidate.Status
	}
	for _, candidate := range taskCandidates {
		taskID := candidate.ID
		if candidate.Status == "RejectedByWarehouse" {
			if _, reviewed := reviewedTaskDecisions[taskID]; !reviewed {
				blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: "warehouse rejection has no reviewed v8 state decision"})
			}
			continue
		}
		if candidate.Type == "purchase_task" {
			_, mapped := mappedPlanning[taskID]
			if !mapped {
				blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: "legacy purchase task has no complete administrator-confirmed planning SKU mapping"})
			}
		}
	}
	planningTaskIDs := make([]int64, 0, len(mappedPlanning))
	for taskID := range mappedPlanning {
		planningTaskIDs = append(planningTaskIDs, taskID)
	}
	sort.Slice(planningTaskIDs, func(i, j int) bool { return planningTaskIDs[i] < planningTaskIDs[j] })
	for _, taskID := range planningTaskIDs {
		mapping := mappedPlanning[taskID]
		taskType, known := taskTypeByID[taskID]
		taskStatus := taskStatusByID[taskID]
		if !known {
			if err := q.QueryRowContext(ctx, `SELECT task_type,task_status FROM tasks WHERE id=?`, taskID).Scan(&taskType, &taskStatus); errors.Is(err, sql.ErrNoRows) {
				blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: "planning mapping references a missing task"})
				continue
			} else if err != nil {
				return blockers, err
			}
		}
		if taskType != "purchase_task" && taskType != "sku_planning" {
			blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: fmt.Sprintf("planning mapping targets task_type=%s, expected purchase_task or an idempotent sku_planning rerun", taskType)})
			continue
		}
		if (taskStatus == "Cancelled" || taskStatus == "Archived") && mapping.TargetTaskStatus != taskStatus {
			blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: fmt.Sprintf("terminal planning status %s must be preserved, mapping requests %s", taskStatus, mapping.TargetTaskStatus)})
			continue
		}
		databaseItemIDs, err := queryInt64IDs(ctx, q, `SELECT id FROM task_sku_items WHERE task_id=? ORDER BY id`, taskID)
		if err != nil {
			return blockers, err
		}
		mappingItemIDs := make([]int64, 0, len(mapping.Items))
		seenItemIDs := map[int64]struct{}{}
		duplicate := duplicatePlanningTasks[taskID]
		for _, item := range mapping.Items {
			if _, exists := seenItemIDs[item.TaskSKUItemID]; exists {
				duplicate = true
			}
			seenItemIDs[item.TaskSKUItemID] = struct{}{}
			mappingItemIDs = append(mappingItemIDs, item.TaskSKUItemID)
		}
		sort.Slice(mappingItemIDs, func(i, j int) bool { return mappingItemIDs[i] < mappingItemIDs[j] })
		if duplicate || !equalInt64Slices(databaseItemIDs, mappingItemIDs) {
			blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{
				TaskID: taskID,
				Reason: fmt.Sprintf(
					"planning mapping SKU ids must exactly match task_sku_items (database=%v mapping=%v duplicate=%v)",
					databaseItemIDs, mappingItemIDs, duplicate,
				),
			})
			continue
		}
		if taskType == "sku_planning" {
			if err := verifyPlanningMappingQuery(ctx, q, mapping); err != nil {
				blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{
					TaskID: taskID,
					Reason: fmt.Sprintf("existing sku_planning task does not match the reviewed idempotent mapping: %v", err),
				})
			}
		}
	}
	mappedGroups := map[string]struct{}{}
	for _, item := range m.Resources {
		mappedGroups[fmt.Sprintf("%d/%s/%d", item.TaskID, item.ScopeKind, item.ScopeRefID)] = struct{}{}
	}
	rows, err = q.QueryContext(ctx, `SELECT id,task_id,scope_kind,scope_ref_id FROM task_asset_groups WHERE migration_incomplete=1 ORDER BY id`)
	if err != nil {
		return blockers, err
	}
	for rows.Next() {
		var id, taskID, scopeRefID int64
		var scopeKind string
		if err := rows.Scan(&id, &taskID, &scopeKind, &scopeRefID); err != nil {
			rows.Close()
			return blockers, err
		}
		if _, mapped := mappedGroups[fmt.Sprintf("%d/%s/%d", taskID, scopeKind, scopeRefID)]; !mapped {
			blockers.Resources = append(blockers.Resources, resourceMigrationIssue{
				GroupID: id,
				TaskID:  taskID,
				Reason:  "migration-incomplete resource group has no administrator-confirmed ordered mapping",
			})
		}
	}
	if err := rows.Close(); err != nil {
		return blockers, err
	}
	for _, mapping := range m.Resources {
		var issue *resourceMigrationIssue
		var err error
		if decision, reviewed := reviewedTaskDecisions[mapping.TaskID]; reviewed && mapping.isV2() {
			issue, err = validateResourceMappingV2PreflightForStatus(ctx, q, mapping, decision.TargetStatus)
		} else {
			issue, err = validateResourceMappingPreflight(ctx, q, mapping)
		}
		if err != nil {
			return blockers, err
		}
		if issue != nil {
			blockers.Resources = append(blockers.Resources, *issue)
		}
	}
	return blockers, nil
}

func validateResourceMappingPreflight(ctx context.Context, q snapshotQueryer, m resourceMapping) (*resourceMigrationIssue, error) {
	if m.isV2() {
		return validateResourceMappingV2Preflight(ctx, q, m)
	}
	issue := func(groupID int64, format string, args ...interface{}) *resourceMigrationIssue {
		return &resourceMigrationIssue{GroupID: groupID, TaskID: m.TaskID, Reason: fmt.Sprintf(format, args...)}
	}
	var taskType, taskStatus string
	if err := q.QueryRowContext(ctx, `SELECT task_type,task_status FROM tasks WHERE id=?`, m.TaskID).Scan(&taskType, &taskStatus); errors.Is(err, sql.ErrNoRows) {
		return issue(0, "resource mapping references a missing task"), nil
	} else if err != nil {
		return nil, err
	}
	var creatorCount int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=?`, m.CreatedBy).Scan(&creatorCount); err != nil {
		return nil, err
	}
	if creatorCount != 1 {
		return issue(0, "resource mapping creator %d does not exist", m.CreatedBy), nil
	}
	if m.ScopeKind == "sku" || m.ScopeKind == "retouch_requirement" {
		table := "task_sku_items"
		if m.ScopeKind == "retouch_requirement" {
			table = "task_retouch_requirements"
		}
		var scopeCount int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE id=? AND task_id=?`, m.ScopeRefID, m.TaskID).Scan(&scopeCount); err != nil {
			return nil, err
		}
		if scopeCount != 1 {
			return issue(0, "%s scope %d does not belong to task", m.ScopeKind, m.ScopeRefID), nil
		}
	}
	var groupID int64
	err := q.QueryRowContext(ctx, `SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=?`, m.TaskID, m.ScopeKind, m.ScopeRefID).Scan(&groupID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		var revisionCount int
		var incomplete bool
		if err := q.QueryRowContext(ctx, `SELECT COUNT(r.id),migration_incomplete FROM task_asset_group_revisions r RIGHT JOIN task_asset_groups g ON g.id=r.group_id WHERE g.id=? GROUP BY g.id,migration_incomplete`, groupID).Scan(&revisionCount, &incomplete); err != nil {
			return nil, err
		}
		if revisionCount > 0 {
			if incomplete {
				return issue(groupID, "resource group has revisions but remains migration_incomplete"), nil
			}
			if err := verifyResourceMappingQuery(ctx, q, groupID, m); err != nil {
				return issue(groupID, "existing resource revision differs from reviewed mapping: %v", err), nil
			}
			return nil, nil
		}
	}
	effectiveStatus := migratedTaskStatus(taskStatus)
	if m.TargetStatus == "shell" {
		if effectiveStatus == "PendingAudit" || effectiveStatus == "Completed" {
			return issue(groupID, "task status %s cannot be migrated as an empty shell", effectiveStatus), nil
		}
		return nil, nil
	}
	if taskType != "retouch_task" && m.SourceAssetID == nil {
		return issue(groupID, "design/customization resource mapping has no confirmed source file"), nil
	}
	if effectiveStatus == "Completed" && m.TargetStatus != "finalized" {
		return issue(groupID, "completed task requires a finalized resource mapping"), nil
	}
	if effectiveStatus == "PendingAudit" && m.TargetStatus != "submitted" {
		return issue(groupID, "pending-audit task requires a submitted resource mapping"), nil
	}
	assetIDs := append([]int64(nil), m.FinalAssetIDs...)
	if m.SourceAssetID != nil {
		assetIDs = append(assetIDs, *m.SourceAssetID)
	}
	for _, assetID := range assetIDs {
		var ownerTaskID int64
		if err := q.QueryRowContext(ctx, `SELECT task_id FROM task_assets WHERE id=?`, assetID).Scan(&ownerTaskID); errors.Is(err, sql.ErrNoRows) {
			return issue(groupID, "task asset %d is missing", assetID), nil
		} else if err != nil {
			return nil, err
		}
		if ownerTaskID != m.TaskID {
			return issue(groupID, "task asset %d belongs to task %d", assetID, ownerTaskID), nil
		}
	}
	for _, referenceID := range m.ReferenceIDs {
		var ownerTaskID int64
		var skuItemID, retouchRequirementID sql.NullInt64
		if err := q.QueryRowContext(ctx, `SELECT task_id,sku_item_id,retouch_requirement_id FROM reference_file_refs WHERE id=?`, referenceID).Scan(&ownerTaskID, &skuItemID, &retouchRequirementID); errors.Is(err, sql.ErrNoRows) {
			return issue(groupID, "reference file %d is missing", referenceID), nil
		} else if err != nil {
			return nil, err
		}
		if ownerTaskID != m.TaskID || !referenceMatchesResourceScope(m.ScopeKind, m.ScopeRefID, skuItemID, retouchRequirementID) {
			return issue(groupID, "reference file %d does not belong to the mapped task scope", referenceID), nil
		}
	}
	return nil, nil
}

func referenceMatchesResourceScope(scopeKind string, scopeRefID int64, skuItemID, retouchRequirementID sql.NullInt64) bool {
	// Task-level references may be inherited by one concrete group. Once a
	// reference declares a SKU or retouch requirement, it must never cross into
	// another discriminator. A row declaring both is invalid for every group.
	if skuItemID.Valid && retouchRequirementID.Valid {
		return false
	}
	switch scopeKind {
	case "task":
		return !skuItemID.Valid && !retouchRequirementID.Valid
	case "sku":
		return !retouchRequirementID.Valid && (!skuItemID.Valid || skuItemID.Int64 == scopeRefID)
	case "retouch_requirement":
		return !skuItemID.Valid && (!retouchRequirementID.Valid || retouchRequirementID.Int64 == scopeRefID)
	default:
		return false
	}
}

func migratedTaskStatus(status string) string {
	switch status {
	case "PendingAuditA", "PendingAuditB", "PendingCustomizationReview", "PendingOutsourceReview", "PendingEffectReview":
		return "PendingAudit"
	case "RejectedByAuditA", "RejectedByAuditB", "PendingCustomizationProduction", "PendingEffectRevision", "PendingOutsource", "Outsourcing":
		return "InProgress"
	case "PendingWarehouseQC", "PendingWarehouseReceive", "PendingProductionTransfer", "PendingClose":
		return "Completed"
	default:
		return status
	}
}

func requireNoCutoverBlockers(blockers cutoverBlockers) error {
	if blockers.Empty() {
		return nil
	}
	return v1migrate.NewHardAbort(
		v1migrate.ExitCodeHardAbort,
		"cutover preflight blocked: org=%d access=%d tasks=%d resource_groups=%d; run --dry-run and resolve every listed ID before enabling auth/workflow v8",
		len(blockers.Org), len(blockers.Access), len(blockers.Tasks), len(blockers.Resources),
	)
}

func queryOrgMigrationIssues(ctx context.Context, q snapshotQueryer) ([]orgMigrationIssue, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT subject_type,subject_id,reason FROM (
		  SELECT 'user' AS subject_type,u.id AS subject_id,
		    CASE
		      WHEN NULLIF(TRIM(u.department),'') IS NOT NULL AND u.department_id IS NULL THEN 'department name missing or ambiguous'
		      WHEN NULLIF(TRIM(u.team),'') IS NOT NULL AND u.team_id IS NULL THEN 'team name missing or ambiguous within stable department'
		      WHEN u.team_id IS NOT NULL AND (u.department_id IS NULL OR NOT EXISTS (SELECT 1 FROM org_teams ot WHERE ot.id=u.team_id AND ot.department_id=u.department_id)) THEN 'department/team stable ids are inconsistent'
		    END AS reason
		  FROM users u
		  WHERE (NULLIF(TRIM(u.department),'') IS NOT NULL AND u.department_id IS NULL)
		     OR (NULLIF(TRIM(u.team),'') IS NOT NULL AND u.team_id IS NULL)
		     OR (u.team_id IS NOT NULL AND (u.department_id IS NULL OR NOT EXISTS (SELECT 1 FROM org_teams ot WHERE ot.id=u.team_id AND ot.department_id=u.department_id)))
		  UNION ALL
		  SELECT 'task',t.id,
		    CASE
		      WHEN NULLIF(TRIM(t.owner_department),'') IS NOT NULL AND t.owner_department_id IS NULL THEN 'owner department name missing or ambiguous'
		      WHEN COALESCE(NULLIF(TRIM(t.owner_org_team),''),NULLIF(TRIM(t.owner_team),'')) IS NOT NULL AND t.owner_team_id IS NULL THEN 'owner team name missing, ambiguous, or conflicting'
		      WHEN t.owner_team_id IS NOT NULL AND (t.owner_department_id IS NULL OR NOT EXISTS (SELECT 1 FROM org_teams ot WHERE ot.id=t.owner_team_id AND ot.department_id=t.owner_department_id)) THEN 'owner department/team stable ids are inconsistent'
		    END
		  FROM tasks t
		  WHERE (NULLIF(TRIM(t.owner_department),'') IS NOT NULL AND t.owner_department_id IS NULL)
		     OR (COALESCE(NULLIF(TRIM(t.owner_org_team),''),NULLIF(TRIM(t.owner_team),'')) IS NOT NULL AND t.owner_team_id IS NULL)
		     OR (t.owner_team_id IS NOT NULL AND (t.owner_department_id IS NULL OR NOT EXISTS (SELECT 1 FROM org_teams ot WHERE ot.id=t.owner_team_id AND ot.department_id=t.owner_department_id)))
		) issues ORDER BY subject_type,subject_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := []orgMigrationIssue{}
	for rows.Next() {
		var item orgMigrationIssue
		if err := rows.Scan(&item.SubjectType, &item.SubjectID, &item.Reason); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}
	return issues, rows.Err()
}

func resolveOrganizationMigrationIssues(ctx context.Context, q snapshotQueryer, raw []orgMigrationIssue, mappings []organizationMapping) ([]orgMigrationIssue, error) {
	bySubject := make(map[string]organizationMapping, len(mappings))
	for _, item := range mappings {
		bySubject[fmt.Sprintf("%s/%d", item.SubjectType, item.SubjectID)] = item
	}
	issues := make([]orgMigrationIssue, 0, len(raw))
	rawSubjects := map[string]struct{}{}
	for _, issue := range raw {
		subjectKey := fmt.Sprintf("%s/%d", issue.SubjectType, issue.SubjectID)
		rawSubjects[subjectKey] = struct{}{}
		item, ok := bySubject[subjectKey]
		if !ok {
			issues = append(issues, issue)
			continue
		}
		if item.Confidence != "confirmed_auto" {
			issue.Reason = fmt.Sprintf("organization mapping is %s: %s", item.Confidence, strings.Join(item.Blockers, "; "))
			issues = append(issues, issue)
			continue
		}
		if err := validateOrganizationMappingPreflight(ctx, q, item); err != nil {
			issue.Reason = err.Error()
			issues = append(issues, issue)
		}
	}
	for _, item := range mappings {
		if item.Confidence != "confirmed_auto" {
			continue
		}
		if _, wasRaw := rawSubjects[fmt.Sprintf("%s/%d", item.SubjectType, item.SubjectID)]; wasRaw {
			continue
		}
		if err := validateOrganizationMappingPreflight(ctx, q, item); err != nil {
			issues = append(issues, orgMigrationIssue{
				SubjectType: item.SubjectType,
				SubjectID:   item.SubjectID,
				Reason:      err.Error(),
			})
		}
	}
	return issues, nil
}

func validateOrganizationMappingPreflight(ctx context.Context, q snapshotQueryer, item organizationMapping) error {
	current, err := loadOrganizationState(ctx, q, item.SubjectType, item.SubjectID)
	if err != nil {
		return fmt.Errorf("organization mapping subject lookup failed: %w", err)
	}
	if current.LegacyDepartment != item.LegacyDepartment || current.LegacyTeam != item.LegacyTeam {
		return fmt.Errorf("organization display evidence drifted")
	}
	beforeMatches := int64PointerEqual(current.DepartmentID, item.FromDepartmentID) &&
		int64PointerEqual(current.TeamID, item.FromTeamID)
	afterMatches := int64PointerValue(current.DepartmentID) == item.TargetDepartmentID &&
		int64PointerValue(current.TeamID) == item.TargetTeamID
	if !beforeMatches && !afterMatches {
		return fmt.Errorf("organization stable ids differ from both reviewed before and target state")
	}
	var exists int
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM org_teams ot
		JOIN org_departments od ON od.id=ot.department_id
		WHERE od.id=? AND ot.id=?`, item.TargetDepartmentID, item.TargetTeamID).Scan(&exists); err != nil {
		return fmt.Errorf("organization target lookup failed: %w", err)
	}
	if exists != 1 {
		return fmt.Errorf("organization target department/team pair does not exist")
	}
	return nil
}

func resolveAccessMigrationIssues(ctx context.Context, q snapshotQueryer, raw []accessMigrationIssue, decisions []accessDecisionMapping) ([]accessMigrationIssue, error) {
	byRole := make(map[string]accessDecisionMapping, len(decisions))
	for _, item := range decisions {
		byRole[fmt.Sprintf("%d/%s", item.UserID, item.LegacyRole)] = item
	}
	issues := make([]accessMigrationIssue, 0, len(raw))
	rawRoles := map[string]struct{}{}
	for _, issue := range raw {
		roleKey := fmt.Sprintf("%d/%s", issue.UserID, issue.Role)
		rawRoles[roleKey] = struct{}{}
		item, ok := byRole[roleKey]
		if !ok {
			issues = append(issues, issue)
			continue
		}
		if item.Confidence != "confirmed_auto" {
			issue.Reason = fmt.Sprintf("access decision is %s: %s", item.Confidence, strings.Join(item.Blockers, "; "))
			issues = append(issues, issue)
			continue
		}
		if err := validateAccessDecisionPreflight(ctx, q, item); err != nil {
			issue.Reason = err.Error()
			issues = append(issues, issue)
		}
	}
	for _, item := range decisions {
		if item.Confidence != "confirmed_auto" {
			continue
		}
		if _, wasRaw := rawRoles[fmt.Sprintf("%d/%s", item.UserID, item.LegacyRole)]; wasRaw {
			continue
		}
		if err := validateAccessDecisionPreflight(ctx, q, item); err != nil {
			issues = append(issues, accessMigrationIssue{
				UserID: item.UserID,
				Role:   item.LegacyRole,
				Reason: err.Error(),
			})
		}
	}
	return issues, nil
}

func validateAccessDecisionPreflight(ctx context.Context, q snapshotQueryer, item accessDecisionMapping) error {
	var activeCount int
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM users u
		JOIN user_roles ur ON ur.user_id=u.id
		WHERE u.id=? AND u.status='active' AND ur.role=?`, item.UserID, item.LegacyRole).Scan(&activeCount); err != nil {
		return fmt.Errorf("access legacy role lookup failed: %w", err)
	}
	if activeCount != 1 {
		return fmt.Errorf("reviewed active legacy role no longer exists exactly once")
	}
	actual, err := loadAccessAssignmentEvidence(ctx, q, item.UserID)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, item.RequiredExistingAssignments) {
		return fmt.Errorf("explicit access assignment evidence drifted")
	}
	return nil
}

func loadAccessAssignmentEvidence(ctx context.Context, q snapshotQueryer, userID int64) ([]accessAssignmentEvidence, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT r.code,a.scope_mode,a.source_type,a.source_ref_id
		FROM auth_user_role_assignments a
		JOIN auth_roles r ON r.id=a.role_id
		WHERE a.user_id=?
		ORDER BY r.code,a.scope_mode,a.source_type,a.source_ref_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("load access assignment evidence: %w", err)
	}
	defer rows.Close()
	items := []accessAssignmentEvidence{}
	for rows.Next() {
		var item accessAssignmentEvidence
		if err := rows.Scan(&item.RoleCode, &item.ScopeMode, &item.SourceType, &item.SourceRefID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func int64PointerEqual(left, right *int64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func int64PointerValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func loadOrganizationState(ctx context.Context, q snapshotQueryer, subjectType string, subjectID int64) (organizationStateSnapshot, error) {
	item := organizationStateSnapshot{SubjectType: subjectType, SubjectID: subjectID}
	var departmentID, teamID sql.NullInt64
	var err error
	switch subjectType {
	case "task":
		err = q.QueryRowContext(ctx, `
			SELECT COALESCE(owner_department,''),
			       COALESCE(NULLIF(TRIM(owner_org_team),''),NULLIF(TRIM(owner_team),''),''),
			       owner_department_id,owner_team_id,updated_at
			FROM tasks WHERE id=?`, subjectID).
			Scan(&item.LegacyDepartment, &item.LegacyTeam, &departmentID, &teamID, &item.UpdatedAt)
	case "user":
		err = q.QueryRowContext(ctx, `
			SELECT COALESCE(department,''),COALESCE(team,''),department_id,team_id,updated_at
			FROM users WHERE id=?`, subjectID).
			Scan(&item.LegacyDepartment, &item.LegacyTeam, &departmentID, &teamID, &item.UpdatedAt)
	default:
		return item, fmt.Errorf("unsupported organization subject_type %q", subjectType)
	}
	if err != nil {
		return item, err
	}
	item.DepartmentID = nullInt64Pointer(departmentID)
	item.TeamID = nullInt64Pointer(teamID)
	return item, nil
}

func captureOrganizationStates(ctx context.Context, q snapshotQueryer, mappings []organizationMapping) ([]organizationStateSnapshot, error) {
	if len(mappings) == 0 {
		return nil, nil
	}
	items := make([]organizationStateSnapshot, 0, len(mappings))
	for _, mapping := range mappings {
		item, err := loadOrganizationState(ctx, q, mapping.SubjectType, mapping.SubjectID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].SubjectType == items[j].SubjectType {
			return items[i].SubjectID < items[j].SubjectID
		}
		return items[i].SubjectType < items[j].SubjectType
	})
	return items, nil
}

func captureAccessStates(ctx context.Context, q snapshotQueryer, decisions []accessDecisionMapping) ([]accessStateSnapshot, error) {
	if len(decisions) == 0 {
		return nil, nil
	}
	userIDs := make([]int64, 0, len(decisions))
	for _, decision := range decisions {
		userIDs = append(userIDs, decision.UserID)
	}
	items := make([]accessStateSnapshot, 0, len(userIDs))
	for _, userID := range uniqueSortedInt64(userIDs) {
		assignments, err := loadAccessAssignmentEvidence(ctx, q, userID)
		if err != nil {
			return nil, err
		}
		items = append(items, accessStateSnapshot{UserID: userID, Assignments: assignments})
	}
	return items, nil
}

func queryIDs(ctx context.Context, db *sql.DB, query string) ([]int64, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type snapshotQueryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func loadAutoIncrementState(ctx context.Context, q snapshotQueryer, table string) (autoIncrementSnapshot, error) {
	if !isWorkflowGroupsAutoIncrementTable(table) {
		return autoIncrementSnapshot{}, fmt.Errorf("unsupported auto-increment table %q", table)
	}
	var metadataExpiry, increment, offset int64
	var next sql.NullInt64
	if err := q.QueryRowContext(ctx, `
		SELECT @@SESSION.information_schema_stats_expiry,
		       @@SESSION.auto_increment_increment,
		       @@SESSION.auto_increment_offset,
		       AUTO_INCREMENT
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, table).Scan(&metadataExpiry, &increment, &offset, &next); err != nil {
		return autoIncrementSnapshot{}, err
	}
	if metadataExpiry != 0 {
		return autoIncrementSnapshot{}, fmt.Errorf("information_schema_stats_expiry=%d; exact auto-increment metadata requires 0", metadataExpiry)
	}
	if increment != 1 || offset != 1 {
		return autoIncrementSnapshot{}, fmt.Errorf("auto-increment session requires increment=1 and offset=1, got increment=%d offset=%d", increment, offset)
	}
	if !next.Valid || next.Int64 <= 0 {
		return autoIncrementSnapshot{}, fmt.Errorf("auto-increment table %s has invalid next value", table)
	}
	return autoIncrementSnapshot{Table: table, NextValue: next.Int64}, nil
}

func captureAutoIncrementStates(ctx context.Context, q snapshotQueryer) ([]autoIncrementSnapshot, error) {
	states := make([]autoIncrementSnapshot, 0, len(workflowGroupsAutoIncrementTables))
	for _, table := range workflowGroupsAutoIncrementTables {
		state, err := loadAutoIncrementState(ctx, q, table)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func isWorkflowGroupsAutoIncrementTable(table string) bool {
	for _, allowed := range workflowGroupsAutoIncrementTables {
		if table == allowed {
			return true
		}
	}
	return false
}

func validateAutoIncrementStates(states []autoIncrementSnapshot) error {
	if len(states) != len(workflowGroupsAutoIncrementTables) {
		return fmt.Errorf("auto-increment snapshot table count is %d, expected %d", len(states), len(workflowGroupsAutoIncrementTables))
	}
	for index, expected := range workflowGroupsAutoIncrementTables {
		state := states[index]
		if state.Table != expected {
			return fmt.Errorf("auto-increment snapshot table %d is %q, expected %q", index, state.Table, expected)
		}
		if state.NextValue <= 0 {
			return fmt.Errorf("auto-increment snapshot table %s has invalid next value %d", state.Table, state.NextValue)
		}
	}
	return nil
}

func autoIncrementStatesMatch(ctx context.Context, q snapshotQueryer, expected []autoIncrementSnapshot) (bool, error) {
	if err := validateAutoIncrementStates(expected); err != nil {
		return false, err
	}
	actual, err := captureAutoIncrementStates(ctx, q)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(actual, expected), nil
}

func needsPreCommitAutoIncrementRecovery(applyState string, beforeRowsMatch, beforeCountersMatch bool) bool {
	return (applyState == "prepared" || applyState == "commit_pending") &&
		beforeRowsMatch &&
		!beforeCountersMatch
}

func autoIncrementRecoveryCeilings(before []autoIncrementSnapshot, m mappingFile) ([]autoIncrementSnapshot, error) {
	if err := validateAutoIncrementStates(before); err != nil {
		return nil, err
	}
	increments := map[string]int64{}
	for _, resource := range m.Resources {
		increments["task_asset_groups"]++
		if resource.isV2() {
			for _, revision := range resource.History {
				increments["task_asset_group_revisions"]++
				increments["task_asset_group_revision_items"] += int64(len(revision.FinalAssetIDs))
				increments["task_asset_group_revision_references"] += int64(len(revision.ReferenceIDs))
				if revision.SourceAliasFrom != nil {
					increments["task_assets"]++
				}
			}
			continue
		}
		if resource.TargetStatus != "shell" {
			increments["task_asset_group_revisions"]++
			increments["task_asset_group_revision_items"] += int64(len(resource.FinalAssetIDs))
			increments["task_asset_group_revision_references"] += int64(len(resource.ReferenceIDs))
		}
	}
	for _, planning := range m.Planning {
		increments["task_planning_sku_revisions"] += int64(len(planning.Items))
	}
	ceilings := make([]autoIncrementSnapshot, 0, len(before))
	for _, state := range before {
		increment := increments[state.Table]
		if increment < 0 || state.NextValue > int64(^uint64(0)>>1)-increment {
			return nil, fmt.Errorf("auto-increment recovery ceiling overflow for %s", state.Table)
		}
		ceilings = append(ceilings, autoIncrementSnapshot{
			Table:     state.Table,
			NextValue: state.NextValue + increment,
		})
	}
	return ceilings, nil
}

func restoreAutoIncrementStatesWithinRecoveryCeilings(
	ctx context.Context,
	db *sql.DB,
	before []autoIncrementSnapshot,
	ceilings []autoIncrementSnapshot,
) error {
	if err := validateAutoIncrementStates(before); err != nil {
		return err
	}
	if err := validateAutoIncrementStates(ceilings); err != nil {
		return err
	}
	for index, target := range before {
		ceiling := ceilings[index]
		if target.Table != ceiling.Table || ceiling.NextValue < target.NextValue {
			return fmt.Errorf("invalid auto-increment recovery ceiling for %s", target.Table)
		}
		current, err := loadAutoIncrementState(ctx, db, target.Table)
		if err != nil {
			return err
		}
		if current.NextValue == target.NextValue {
			continue
		}
		if current.NextValue < target.NextValue || current.NextValue > ceiling.NextValue {
			return v1migrate.NewHardAbort(
				v1migrate.ExitCodeHardAbort,
				"pre-commit auto-increment recovery refused for %s: current=%d before=%d ceiling=%d",
				target.Table,
				current.NextValue,
				target.NextValue,
				ceiling.NextValue,
			)
		}
		var maxID int64
		query := fmt.Sprintf("SELECT COALESCE(MAX(id),0) FROM `%s`", target.Table)
		if err := db.QueryRowContext(ctx, query).Scan(&maxID); err != nil {
			return err
		}
		if maxID >= target.NextValue {
			return v1migrate.NewHardAbort(
				v1migrate.ExitCodeHardAbort,
				"pre-commit auto-increment recovery refused for %s: max id %d reaches target next value %d",
				target.Table,
				maxID,
				target.NextValue,
			)
		}
		statement := fmt.Sprintf("ALTER TABLE `%s` AUTO_INCREMENT = %d", target.Table, target.NextValue)
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	matches, err := autoIncrementStatesMatch(ctx, db, before)
	if err != nil {
		return err
	}
	if !matches {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "pre-commit auto-increment recovery verification failed")
	}
	return nil
}

func restoreAutoIncrementStates(
	ctx context.Context,
	db *sql.DB,
	before []autoIncrementSnapshot,
	after []autoIncrementSnapshot,
) error {
	if err := validateAutoIncrementStates(before); err != nil {
		return err
	}
	if err := validateAutoIncrementStates(after); err != nil {
		return err
	}
	for index, target := range before {
		recordedAfter := after[index]
		if target.Table != recordedAfter.Table {
			return fmt.Errorf("auto-increment before/after table order drifted")
		}
		current, err := loadAutoIncrementState(ctx, db, target.Table)
		if err != nil {
			return err
		}
		if current.NextValue == target.NextValue {
			continue
		}
		if current.NextValue != recordedAfter.NextValue {
			return v1migrate.NewHardAbort(
				v1migrate.ExitCodeHardAbort,
				"auto-increment rollback refused for %s: current=%d before=%d after=%d",
				target.Table,
				current.NextValue,
				target.NextValue,
				recordedAfter.NextValue,
			)
		}
		var maxID int64
		query := fmt.Sprintf("SELECT COALESCE(MAX(id),0) FROM `%s`", target.Table)
		if err := db.QueryRowContext(ctx, query).Scan(&maxID); err != nil {
			return err
		}
		if maxID >= target.NextValue {
			return v1migrate.NewHardAbort(
				v1migrate.ExitCodeHardAbort,
				"auto-increment rollback refused for %s: max id %d reaches target next value %d",
				target.Table,
				maxID,
				target.NextValue,
			)
		}
		statement := fmt.Sprintf("ALTER TABLE `%s` AUTO_INCREMENT = %d", target.Table, target.NextValue)
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
		restored, err := loadAutoIncrementState(ctx, db, target.Table)
		if err != nil {
			return err
		}
		if restored.NextValue != target.NextValue {
			return v1migrate.NewHardAbort(
				v1migrate.ExitCodeHardAbort,
				"auto-increment rollback verification failed for %s: got %d expected %d",
				target.Table,
				restored.NextValue,
				target.NextValue,
			)
		}
	}
	return nil
}

func queryInt64IDs(ctx context.Context, q snapshotQueryer, query string, args ...interface{}) ([]int64, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func queryStringIDs(ctx context.Context, q snapshotQueryer, query string, args ...interface{}) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func planningTaskIDs(m mappingFile) []int64 {
	ids := make([]int64, 0, len(m.Planning))
	for _, item := range m.Planning {
		ids = append(ids, item.TaskID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func uniqueSortedInt64(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func lockInt64Rows(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func lockCutoverTargetsLegacy(ctx context.Context, tx *sql.Tx, m mappingFile) error {
	taskIDs, err := lockInt64Rows(ctx, tx, `
		SELECT id FROM tasks
		WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingCustomizationReview','PendingCustomizationProduction','PendingEffectReview','PendingEffectRevision','PendingProductionTransfer','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview')
		   OR task_type='purchase_task'
		ORDER BY id FOR UPDATE`)
	if err != nil {
		return err
	}
	for _, item := range m.Resources {
		taskIDs = append(taskIDs, item.TaskID)
	}
	for _, item := range m.Planning {
		taskIDs = append(taskIDs, item.TaskID)
	}
	for _, item := range m.TaskDecisions {
		taskIDs = append(taskIDs, item.TaskID)
	}
	for _, item := range m.AssetRecoveries {
		taskIDs = append(taskIDs, item.TaskID)
	}
	taskIDs = uniqueSortedInt64(taskIDs)
	for _, taskID := range taskIDs {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM tasks WHERE id=? FOR UPDATE`, taskID); err != nil {
			return err
		}
	}
	for _, taskID := range taskIDs {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_sku_items WHERE task_id=? ORDER BY id FOR UPDATE`, taskID); err != nil {
			return err
		}
	}
	if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE migration_incomplete=1 ORDER BY id FOR UPDATE`); err != nil {
		return err
	}
	resources := append([]resourceMapping(nil), m.Resources...)
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].TaskID != resources[j].TaskID {
			return resources[i].TaskID < resources[j].TaskID
		}
		if resources[i].ScopeKind != resources[j].ScopeKind {
			return resources[i].ScopeKind < resources[j].ScopeKind
		}
		return resources[i].ScopeRefID < resources[j].ScopeRefID
	})
	groupIDs := []int64{}
	for _, taskID := range taskIDs {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE task_id=? ORDER BY id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, ids...)
	}
	for _, item := range resources {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=? FOR UPDATE`, item.TaskID, item.ScopeKind, item.ScopeRefID)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, ids...)
	}
	groupIDs = uniqueSortedInt64(groupIDs)
	for _, groupID := range groupIDs {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_group_revisions WHERE group_id=? ORDER BY id FOR UPDATE`, groupID); err != nil {
			return err
		}
	}
	assetIDs := []int64{}
	for _, taskID := range taskIDs {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_assets WHERE task_id=? ORDER BY id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		assetIDs = append(assetIDs, ids...)
	}
	for _, item := range resources {
		assetIDs = append(assetIDs, item.mappedAssetIDs()...)
	}
	for _, assetID := range uniqueSortedInt64(assetIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_assets WHERE id=? FOR UPDATE`, assetID); err != nil {
			return err
		}
	}
	for _, recovery := range m.AssetRecoveries {
		if recovery.Strategy == "verified_oss_recovery_v1" {
			continue
		}
		if _, err := queryStringIDs(ctx, tx, `SELECT ref_id FROM asset_storage_refs WHERE ref_id=? FOR UPDATE`, recovery.OriginalStorageRefID); err != nil {
			return err
		}
	}
	referenceIDs := []int64{}
	for _, item := range resources {
		referenceIDs = append(referenceIDs, item.mappedReferenceIDs()...)
	}
	for _, referenceID := range uniqueSortedInt64(referenceIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM reference_file_refs WHERE id=? FOR UPDATE`, referenceID); err != nil {
			return err
		}
	}
	for _, taskID := range planningTaskIDs(m) {
		queries := []string{
			`SELECT task_id FROM task_planning_settings WHERE task_id=? FOR UPDATE`,
			`SELECT d.task_sku_item_id FROM task_planning_sku_details d JOIN task_sku_items si ON si.id=d.task_sku_item_id WHERE si.task_id=? ORDER BY d.task_sku_item_id FOR UPDATE`,
			`SELECT r.id FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY r.id FOR UPDATE`,
			`SELECT i.revision_id FROM task_planning_sku_revision_images i JOIN task_planning_sku_revisions r ON r.id=i.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY i.revision_id FOR UPDATE`,
		}
		for _, query := range queries {
			if _, err := lockInt64Rows(ctx, tx, query, taskID); err != nil {
				return err
			}
		}
	}
	for _, taskID := range taskIDs {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM task_event_logs WHERE task_id=? ORDER BY sequence,id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
		rows, err = tx.QueryContext(ctx, `
			SELECT e.id FROM task_module_events e
			JOIN task_modules m ON m.id=e.task_module_id
			WHERE m.task_id=? ORDER BY e.id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func captureSKUOrigins(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]skuOriginSnapshot, error) {
	items := []skuOriginSnapshot{}
	for _, taskID := range taskIDs {
		rows, err := q.QueryContext(ctx, `SELECT id,task_id,sku_origin,updated_at FROM task_sku_items WHERE task_id=? ORDER BY id`, taskID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item skuOriginSnapshot
			var origin sql.NullString
			if err := rows.Scan(&item.ID, &item.TaskID, &origin, &item.UpdatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			if origin.Valid {
				value := origin.String
				item.Origin = &value
			}
			items = append(items, item)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func capturePlanningStates(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]planningStateSnapshot, error) {
	states := make([]planningStateSnapshot, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		state := planningStateSnapshot{TaskID: taskID, Details: []planningDetailSnapshot{}, RevisionIDs: []int64{}, ImageRevisionIDs: []int64{}, Images: []planningImageSnapshot{}}
		err := q.QueryRowContext(ctx, `SELECT erp_sync_mode,code_rule_revision_id,client_create_id,created_by FROM task_planning_settings WHERE task_id=?`, taskID).
			Scan(&state.ERPSyncMode, &state.CodeRuleRevisionID, &state.ClientCreateID, &state.CreatedBy)
		if errors.Is(err, sql.ErrNoRows) {
			state.SettingsExists = false
		} else if err != nil {
			return nil, err
		} else {
			state.SettingsExists = true
		}
		rows, err := q.QueryContext(ctx, `
			SELECT d.task_sku_item_id,d.current_revision_id,d.lock_version,d.updated_at
			FROM task_planning_sku_details d
			JOIN task_sku_items si ON si.id=d.task_sku_item_id
			WHERE si.task_id=? ORDER BY d.task_sku_item_id`, taskID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var detail planningDetailSnapshot
			var revisionID sql.NullInt64
			if err := rows.Scan(&detail.TaskSKUItemID, &revisionID, &detail.LockVersion, &detail.UpdatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			if revisionID.Valid {
				value := revisionID.Int64
				detail.CurrentRevisionID = &value
			}
			state.Details = append(state.Details, detail)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		state.RevisionIDs, err = queryInt64IDs(ctx, q, `SELECT r.id FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY r.id`, taskID)
		if err != nil {
			return nil, err
		}
		rows, err = q.QueryContext(ctx, `SELECT i.revision_id,i.storage_ref_id FROM task_planning_sku_revision_images i JOIN task_planning_sku_revisions r ON r.id=i.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY i.revision_id`, taskID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var image planningImageSnapshot
			if err := rows.Scan(&image.RevisionID, &image.StorageRefID); err != nil {
				rows.Close()
				return nil, err
			}
			state.ImageRevisionIDs = append(state.ImageRevisionIDs, image.RevisionID)
			state.Images = append(state.Images, image)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func loadTaskSnapshot(ctx context.Context, q snapshotQueryer, id int64) (taskSnapshot, error) {
	var item taskSnapshot
	var handlerID sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT id,task_type,task_status,workflow_revision,current_handler_id,updated_at FROM tasks WHERE id=?`, id).
		Scan(&item.ID, &item.TaskType, &item.TaskStatus, &item.WorkflowRevision, &handlerID, &item.UpdatedAt); err != nil {
		return item, err
	}
	if handlerID.Valid {
		value := handlerID.Int64
		item.CurrentHandlerID = &value
	}
	var err error
	item.EventIDs, err = queryStringIDs(ctx, q, `SELECT id FROM task_event_logs WHERE task_id=? ORDER BY sequence,id`, id)
	if err != nil {
		return item, err
	}
	item.ModuleEventIDs, err = queryInt64IDs(ctx, q, `
		SELECT e.id
		FROM task_module_events e
		JOIN task_modules tm ON tm.id=e.task_module_id
		WHERE tm.task_id=?
		ORDER BY e.id`, id)
	return item, err
}

func loadResourceGroupSnapshot(ctx context.Context, q snapshotQueryer, id int64) (resourceGroupSnapshot, error) {
	var item resourceGroupSnapshot
	var workingID, finalizedID sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT id,task_id,working_revision_id,finalized_revision_id,lock_version,migration_incomplete,migration_issue,updated_at FROM task_asset_groups WHERE id=?`, id).
		Scan(&item.ID, &item.TaskID, &workingID, &finalizedID, &item.LockVersion, &item.MigrationIncomplete, &item.MigrationIssue, &item.UpdatedAt); err != nil {
		return item, err
	}
	if workingID.Valid {
		value := workingID.Int64
		item.WorkingRevisionID = &value
	}
	if finalizedID.Valid {
		value := finalizedID.Int64
		item.FinalizedRevisionID = &value
	}
	var err error
	item.RevisionIDs, err = queryInt64IDs(ctx, q, `SELECT id FROM task_asset_group_revisions WHERE group_id=? ORDER BY id`, id)
	if err != nil {
		return item, err
	}
	item.Revisions, err = loadResourceRevisionGraph(ctx, q, id)
	return item, err
}

func captureResourceGroupsForTasksLegacy(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]resourceGroupSnapshot, error) {
	items := []resourceGroupSnapshot{}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		groupIDs, err := queryInt64IDs(ctx, q, `SELECT id FROM task_asset_groups WHERE task_id=? ORDER BY id`, taskID)
		if err != nil {
			return nil, err
		}
		for _, groupID := range groupIDs {
			item, err := loadResourceGroupSnapshot(ctx, q, groupID)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func loadAssetBindingSnapshot(ctx context.Context, q snapshotQueryer, id int64) (assetBindingSnapshot, error) {
	var item assetBindingSnapshot
	var boundGroupID, stagedSKUItemID, stagedRetouchID, stagedBy sql.NullInt64
	var boundRole, stagedRole, uploadSessionID, scopeSKUCode sql.NullString
	var retouchRequirementID sql.NullInt64
	var stagedExpiresAt, revokedAt, objectDeletedAt, approvedAt, deletedAt, cleanedAt sql.NullTime
	var approvedBy sql.NullInt64
	if err := q.QueryRowContext(ctx, `
		SELECT id,task_id,binding_state,bound_group_id,bound_role,
		       staged_task_sku_item_id,staged_retouch_requirement_id,staged_role,staged_by,upload_session_id,staged_expires_at,
		       access_revoked_at,access_revoked_reason,object_deleted_at,
		       asset_type,scope_sku_code,retouch_requirement_id,flow_review_status,approved_at,approved_by,
		       COALESCE(mime_type,''),COALESCE(whole_hash,''),deleted_at,cleaned_at
		FROM task_assets WHERE id=?`, id).
		Scan(&item.ID, &item.TaskID, &item.BindingState, &boundGroupID, &boundRole,
			&stagedSKUItemID, &stagedRetouchID, &stagedRole, &stagedBy, &uploadSessionID, &stagedExpiresAt,
			&revokedAt, &item.AccessRevokedReason, &objectDeletedAt,
			&item.AssetType, &scopeSKUCode, &retouchRequirementID, &item.FlowReviewStatus, &approvedAt, &approvedBy,
			&item.MimeType, &item.WholeHash, &deletedAt, &cleanedAt); err != nil {
		return item, err
	}
	item.BoundGroupID = nullInt64Pointer(boundGroupID)
	item.BoundRole = nullStringPointer(boundRole)
	item.StagedTaskSKUItemID = nullInt64Pointer(stagedSKUItemID)
	item.StagedRetouchRequirementID = nullInt64Pointer(stagedRetouchID)
	item.StagedRole = nullStringPointer(stagedRole)
	item.StagedBy = nullInt64Pointer(stagedBy)
	item.UploadSessionID = nullStringPointer(uploadSessionID)
	item.StagedExpiresAt = nullTimePointer(stagedExpiresAt)
	item.AccessRevokedAt = nullTimePointer(revokedAt)
	item.ObjectDeletedAt = nullTimePointer(objectDeletedAt)
	item.ScopeSKUCode = nullStringPointer(scopeSKUCode)
	item.RetouchRequirementID = nullInt64Pointer(retouchRequirementID)
	item.ApprovedAt = nullTimePointer(approvedAt)
	item.ApprovedBy = nullInt64Pointer(approvedBy)
	item.DeletedAt = nullTimePointer(deletedAt)
	item.CleanedAt = nullTimePointer(cleanedAt)
	return item, nil
}

func captureAssetBindingsForTasksLegacy(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]assetBindingSnapshot, error) {
	items := []assetBindingSnapshot{}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		assetIDs, err := queryInt64IDs(ctx, q, `SELECT id FROM task_assets WHERE task_id=? ORDER BY id`, taskID)
		if err != nil {
			return nil, err
		}
		for _, assetID := range assetIDs {
			item, err := loadAssetBindingSnapshot(ctx, q, assetID)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func loadAssetStorageRefStatusSnapshot(ctx context.Context, q snapshotQueryer, refID string) (assetStorageRefStatusSnapshot, error) {
	var item assetStorageRefStatusSnapshot
	var assetID sql.NullInt64
	if err := q.QueryRowContext(ctx, `
		SELECT ref_id,asset_id,status
		FROM asset_storage_refs
		WHERE ref_id=?`, refID).Scan(&item.RefID, &assetID, &item.Status); err != nil {
		return item, err
	}
	item.AssetID = nullInt64Pointer(assetID)
	return item, nil
}

func captureAssetStorageRefStates(ctx context.Context, q snapshotQueryer, recoveries []assetRecoveryMapping) ([]assetStorageRefStatusSnapshot, error) {
	if len(recoveries) == 0 {
		return nil, nil
	}
	byRefID := make(map[string]assetStorageRefStatusSnapshot, len(recoveries))
	for _, recovery := range recoveries {
		if recovery.Strategy == "verified_oss_recovery_v1" {
			continue
		}
		refID := strings.TrimSpace(recovery.OriginalStorageRefID)
		if refID == "" {
			return nil, fmt.Errorf("asset recovery %d has no original storage ref id", recovery.MissingTaskAssetID)
		}
		if _, exists := byRefID[refID]; exists {
			continue
		}
		item, err := loadAssetStorageRefStatusSnapshot(ctx, q, refID)
		if err != nil {
			return nil, fmt.Errorf("load asset storage ref %s: %w", refID, err)
		}
		byRefID[refID] = item
	}
	items := make([]assetStorageRefStatusSnapshot, 0, len(byRefID))
	for _, item := range byRefID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RefID < items[j].RefID })
	return items, nil
}

func populateAfterSnapshot(ctx context.Context, tx *sql.Tx, s *snapshot, m mappingFile) error {
	taskIDs := make([]int64, 0, len(s.Tasks))
	for _, task := range s.Tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	var err error
	s.AfterTasks, err = captureTaskSnapshotsBulk(ctx, tx, taskIDs)
	if err != nil {
		return err
	}
	s.TaskModulesAfter, err = captureTaskModulesForTasks(ctx, tx, taskIDs)
	if err != nil {
		return err
	}
	s.SearchDocumentsAfter, err = captureSearchDocumentsForTasks(ctx, tx, taskIDs)
	if err != nil {
		return err
	}
	s.TaskSearchDocumentsAfter, err = captureTaskSearchDocumentsForTasks(ctx, tx, taskIDs)
	if err != nil {
		return err
	}
	s.AfterResourceGroups, err = captureResourceGroupsForTasks(ctx, tx, taskIDs)
	if err != nil {
		return err
	}
	s.AfterAssetBindings, err = captureAssetBindingsForTasks(ctx, tx, taskIDs)
	if err != nil {
		return err
	}
	s.AfterSKUOrigins, err = captureSKUOrigins(ctx, tx, planningTaskIDs(m))
	if err != nil {
		return err
	}
	s.PlanningAfter, err = capturePlanningStates(ctx, tx, planningTaskIDs(m))
	if err != nil {
		return err
	}
	s.OrganizationAfter, err = captureOrganizationStates(ctx, tx, m.OrganizationMappings)
	if err != nil {
		return err
	}
	s.AccessAfter, err = captureAccessStates(ctx, tx, m.AccessDecisions)
	if err != nil {
		return err
	}
	s.StorageRefsAfter, err = captureAssetStorageRefStates(ctx, tx, m.AssetRecoveries)
	if err != nil {
		return err
	}
	s.AutoIncrementsAfter, err = captureAutoIncrementStates(ctx, tx)
	if err != nil {
		return err
	}
	s.PlanningCreated = diffPlanningCreated(s.PlanningBefore, s.PlanningAfter)
	now := time.Now().UTC()
	s.AppliedAt = &now
	return nil
}

func diffPlanningCreated(before, after []planningStateSnapshot) []planningCreatedSnapshot {
	beforeByTask := map[int64]planningStateSnapshot{}
	for _, item := range before {
		beforeByTask[item.TaskID] = item
	}
	created := make([]planningCreatedSnapshot, 0, len(after))
	for _, item := range after {
		previous := beforeByTask[item.TaskID]
		detailBefore := make([]int64, 0, len(previous.Details))
		for _, detail := range previous.Details {
			detailBefore = append(detailBefore, detail.TaskSKUItemID)
		}
		entry := planningCreatedSnapshot{
			TaskID: item.TaskID, SettingsCreated: !previous.SettingsExists && item.SettingsExists,
			DetailIDs:        differenceInt64(planningDetailIDs(item.Details), detailBefore),
			RevisionIDs:      differenceInt64(item.RevisionIDs, previous.RevisionIDs),
			ImageRevisionIDs: differenceInt64(item.ImageRevisionIDs, previous.ImageRevisionIDs),
		}
		if entry.SettingsCreated || len(entry.DetailIDs) > 0 || len(entry.RevisionIDs) > 0 || len(entry.ImageRevisionIDs) > 0 {
			created = append(created, entry)
		}
	}
	return created
}

func planningDetailIDs(items []planningDetailSnapshot) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TaskSKUItemID)
	}
	return ids
}

func differenceInt64(after, before []int64) []int64 {
	seen := make(map[int64]struct{}, len(before))
	for _, id := range before {
		seen[id] = struct{}{}
	}
	result := []int64{}
	for _, id := range after {
		if _, exists := seen[id]; !exists {
			result = append(result, id)
		}
	}
	return result
}

type applyPhaseLogger struct {
	startedAt time.Time
	phaseAt   time.Time
}

func newApplyPhaseLogger() *applyPhaseLogger {
	now := time.Now()
	return &applyPhaseLogger{startedAt: now, phaseAt: now}
}

func (l *applyPhaseLogger) mark(phase string) {
	now := time.Now()
	payload := map[string]interface{}{
		"event":                 "workflow_groups_migrate_phase",
		"phase":                 phase,
		"phase_elapsed_seconds": now.Sub(l.phaseAt).Seconds(),
		"total_elapsed_seconds": now.Sub(l.startedAt).Seconds(),
	}
	encoded, err := json.Marshal(payload)
	if err == nil {
		_, _ = fmt.Fprintln(os.Stderr, string(encoded))
	}
	l.phaseAt = now
}

func apply(ctx context.Context, db *sql.DB, database string, o options, m mappingFile) error {
	phases := newApplyPhaseLogger()
	recoveryTarget, err := recoveryEvidenceTargetFromOptions(o)
	if err != nil {
		return err
	}
	m = normalizeMapping(m)
	if err := validateMapping(m); err != nil {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "invalid reviewed mapping: %v", err)
	}
	phases.mark("mapping_validated")
	blockers, err := queryCutoverBlockers(ctx, db, m)
	if err != nil {
		return err
	}
	if err := requireNoCutoverBlockers(blockers); err != nil {
		return err
	}
	phases.mark("unlocked_preflight_passed")
	if err := os.MkdirAll(o.SnapshotDir, 0o750); err != nil {
		return err
	}
	mappingSHA256, err := mappingDigest(m)
	if err != nil {
		return err
	}
	path := filepath.Join(o.SnapshotDir, "workflow-groups-snapshot.json")
	if _, err := os.Stat(path); err == nil {
		retry, err := recoverExistingApply(ctx, db, database, path, m)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockCutoverTargets(ctx, tx, m); err != nil {
		return err
	}
	phases.mark("cutover_targets_locked")
	if err := lockPreflightRows(ctx, tx); err != nil {
		return err
	}
	phases.mark("preflight_rows_locked")
	blockers, err = queryCutoverBlockers(ctx, tx, m)
	if err != nil {
		return err
	}
	if err := requireNoCutoverBlockers(blockers); err != nil {
		return err
	}
	phases.mark("authoritative_preflight_passed")
	snap, err := captureSnapshot(ctx, tx, database, m)
	if err != nil {
		return err
	}
	snap.MappingSHA256 = mappingSHA256
	snap.ApplyState = "prepared"
	phases.mark("before_snapshot_captured")
	if err := writeSnapshot(path, snap); err != nil {
		return err
	}
	phases.mark("prepared_journal_written")
	if err := applyOrganizationMappings(ctx, tx, m.OrganizationMappings); err != nil {
		return err
	}
	phases.mark("organization_mappings_applied")
	if err := migrateStates(ctx, tx, m); err != nil {
		return err
	}
	phases.mark("task_states_migrated")
	for _, item := range m.Resources {
		if item.isV2() {
			id, revisionIDs, aliasIDs, inserted, applied, err := applyResourceV2(ctx, tx, item)
			if err != nil {
				return err
			}
			if inserted {
				snap.InsertedGroupIDs = append(snap.InsertedGroupIDs, id)
			}
			if applied {
				snap.AppliedRevisionIDs = append(snap.AppliedRevisionIDs, revisionIDs...)
			}
			snap.InsertedAliasIDs = append(snap.InsertedAliasIDs, aliasIDs...)
			continue
		}
		id, revisionID, inserted, applied, err := applyResource(ctx, tx, item)
		if err != nil {
			return err
		}
		if inserted {
			snap.InsertedGroupIDs = append(snap.InsertedGroupIDs, id)
		}
		if applied {
			snap.AppliedRevisionIDs = append(snap.AppliedRevisionIDs, revisionID)
		}
	}
	phases.mark("resource_histories_applied")
	for _, item := range m.Planning {
		inserted, err := applyPlanning(ctx, tx, item)
		if err != nil {
			return err
		}
		_ = inserted
	}
	phases.mark("planning_applied")
	taskIDs := make([]int64, 0, len(snap.Tasks))
	for _, task := range snap.Tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	if err := normalizeCompletedTaskModules(ctx, tx, taskIDs); err != nil {
		return err
	}
	phases.mark("completed_task_modules_normalized")
	if err := applyAssetRecoveries(ctx, tx, m.AssetRecoveries, recoveryTarget); err != nil {
		return err
	}
	phases.mark("asset_recoveries_applied")
	if err := validateCutoverState(ctx, tx, m, recoveryTarget); err != nil {
		return err
	}
	phases.mark("cutover_state_validated")
	if err := populateAfterSnapshot(ctx, tx, &snap, m); err != nil {
		return err
	}
	phases.mark("after_snapshot_captured")
	snap.ApplyState = "commit_pending"
	if err := writeSnapshot(path, snap); err != nil {
		return err
	}
	phases.mark("commit_pending_journal_written")
	if err := tx.Commit(); err != nil {
		return err
	}
	phases.mark("database_committed")
	snap.ApplyState = "applied"
	if err := writeSnapshot(path, snap); err != nil {
		return fmt.Errorf("database apply committed but final manifest marker failed; preserve commit_pending manifest and recover before continuing: %w", err)
	}
	phases.mark("applied_journal_written")
	return nil
}

func lockPreflightRows(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		`SELECT id FROM tasks ORDER BY id FOR UPDATE`,
		`SELECT id FROM org_departments ORDER BY id FOR UPDATE`,
		`SELECT id FROM org_teams ORDER BY id FOR UPDATE`,
		`SELECT id FROM users ORDER BY id FOR UPDATE`,
	}
	for _, query := range queries {
		if _, err := lockInt64Rows(ctx, tx, query); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT user_id,role FROM user_roles ORDER BY user_id,role FOR UPDATE`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var userID int64
		var role string
		if err := rows.Scan(&userID, &role); err != nil {
			rows.Close()
			return err
		}
	}
	return rows.Close()
}

func applyOrganizationMappings(ctx context.Context, tx *sql.Tx, mappings []organizationMapping) error {
	for _, item := range mappings {
		var result sql.Result
		var err error
		switch item.SubjectType {
		case "task":
			result, err = tx.ExecContext(ctx, `
				UPDATE tasks
				SET owner_department_id=?,owner_team_id=?
				WHERE id=?
				  AND owner_department_id <=> ?
				  AND owner_team_id <=> ?`,
				item.TargetDepartmentID, item.TargetTeamID, item.SubjectID,
				nullableInt64Pointer(item.FromDepartmentID), nullableInt64Pointer(item.FromTeamID))
		case "user":
			result, err = tx.ExecContext(ctx, `
				UPDATE users
				SET department_id=?,team_id=?
				WHERE id=?
				  AND department_id <=> ?
				  AND team_id <=> ?`,
				item.TargetDepartmentID, item.TargetTeamID, item.SubjectID,
				nullableInt64Pointer(item.FromDepartmentID), nullableInt64Pointer(item.FromTeamID))
		default:
			return fmt.Errorf("unsupported organization subject_type %q", item.SubjectType)
		}
		if err != nil {
			return fmt.Errorf("apply organization mapping %s/%d: %w", item.SubjectType, item.SubjectID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			current, loadErr := loadOrganizationState(ctx, tx, item.SubjectType, item.SubjectID)
			if loadErr != nil {
				return loadErr
			}
			if int64PointerValue(current.DepartmentID) != item.TargetDepartmentID ||
				int64PointerValue(current.TeamID) != item.TargetTeamID {
				return fmt.Errorf("organization mapping %s/%d did not update exactly one reviewed before-state row", item.SubjectType, item.SubjectID)
			}
		}
	}
	return nil
}

const currentPointerAssetReferencesSQL = `
		WITH RECURSIVE asset_lineage AS (
		  SELECT seed.id,seed.storage_ref_id
		  FROM task_assets seed
		  WHERE seed.id=?
		  UNION DISTINCT
		  SELECT seed.id,seed.storage_ref_id
		  FROM task_assets seed
		  LEFT JOIN task_asset_groups seed_group ON seed_group.id=seed.bound_group_id
		  WHERE seed.source_module_key='migration'
		    AND seed.bound_role='source'
		    AND seed.remark=CONCAT('v8-source-alias:group=',seed_group.id,':origin=',?)
		  UNION DISTINCT
		  SELECT ta.id,ta.storage_ref_id
		  FROM task_assets ta
		  JOIN asset_lineage parent ON ta.source_asset_version_id=parent.id
		),
		current_revisions AS (
		  SELECT task_id,working_revision_id AS revision_id
		  FROM task_asset_groups
		  WHERE working_revision_id IS NOT NULL
		  UNION DISTINCT
		  SELECT task_id,finalized_revision_id AS revision_id
		  FROM task_asset_groups
		  WHERE finalized_revision_id IS NOT NULL
		),
		referenced_revisions AS (
		  SELECT current.revision_id
		  FROM current_revisions current
		  JOIN task_asset_group_revisions r ON r.id=current.revision_id
		  JOIN asset_lineage lineage ON lineage.id=r.source_task_asset_id
		  UNION DISTINCT
		  SELECT current.revision_id
		  FROM current_revisions current
		  JOIN task_asset_group_revision_items i ON i.revision_id=current.revision_id
		  JOIN asset_lineage lineage ON lineage.id=i.task_asset_id
		  UNION DISTINCT
		  SELECT current.revision_id
		  FROM current_revisions current
		  JOIN task_asset_group_revision_references rr ON rr.revision_id=current.revision_id
		  JOIN asset_lineage lineage ON lineage.id=rr.formal_task_asset_id
		  UNION DISTINCT
		  SELECT current.revision_id
		  FROM current_revisions current
		  JOIN task_asset_group_revision_references rr ON rr.revision_id=current.revision_id
		  JOIN reference_file_refs rfr ON rfr.id=rr.reference_file_ref_id
		  JOIN asset_storage_refs live_ref ON live_ref.ref_id=rfr.ref_id
		  JOIN asset_lineage lineage ON lineage.id=live_ref.asset_id
		  UNION DISTINCT
		  SELECT current.revision_id
		  FROM current_revisions current
		  JOIN task_asset_group_revision_references rr ON rr.revision_id=current.revision_id
		  JOIN reference_file_refs rfr ON rfr.id=rr.reference_file_ref_id
		  JOIN asset_storage_refs live_ref
		    ON live_ref.ref_id=rfr.ref_id
		   AND live_ref.owner_type='task_asset'
		  JOIN asset_lineage lineage ON lineage.id=live_ref.owner_id
		  UNION DISTINCT
		  SELECT current.revision_id
		  FROM current_revisions current
		  JOIN task_asset_group_revision_references rr ON rr.revision_id=current.revision_id
		  JOIN asset_storage_refs frozen_ref ON frozen_ref.ref_id=rr.ref_id_snapshot
		  JOIN asset_lineage lineage ON lineage.id=frozen_ref.asset_id
		  UNION DISTINCT
		  SELECT current.revision_id
		  FROM current_revisions current
		  JOIN task_asset_group_revision_references rr ON rr.revision_id=current.revision_id
		  JOIN asset_storage_refs frozen_ref
		    ON frozen_ref.ref_id=rr.ref_id_snapshot
		   AND frozen_ref.owner_type='task_asset'
		  JOIN asset_lineage lineage ON lineage.id=frozen_ref.owner_id
		  UNION DISTINCT
		  SELECT current.revision_id
		  FROM current_revisions current
		  JOIN task_asset_group_revision_references rr ON rr.revision_id=current.revision_id
		  JOIN reference_file_refs rfr ON rfr.id=rr.reference_file_ref_id
		  JOIN task_reference_asset_bindings binding
		    ON binding.task_id=current.task_id
		   AND binding.ref_id COLLATE utf8mb4_0900_ai_ci=rfr.ref_id
		  JOIN asset_lineage lineage ON lineage.id=binding.task_asset_id
		)
		SELECT COUNT(*) FROM referenced_revisions`

func countCurrentPointerAssetReferences(ctx context.Context, q snapshotQueryer, taskAssetID int64) (int, error) {
	var count int
	err := q.QueryRowContext(ctx, currentPointerAssetReferencesSQL, taskAssetID, taskAssetID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count current pointer references for task asset %d: %w", taskAssetID, err)
	}
	return count, nil
}

func validateHistoricalUnavailableRecoveryEvidence(
	ctx context.Context,
	q snapshotQueryer,
	recovery assetRecoveryMapping,
	expectedStorageStatus string,
) error {
	expected, known := frozenAssetRecoveryEvidenceByMissingID[recovery.MissingTaskAssetID]
	if !known || recovery.MissingTaskAssetID != 12323 {
		return fmt.Errorf("task asset %d is outside the frozen historical-unavailable evidence set", recovery.MissingTaskAssetID)
	}
	expectedRowHash, err := assetRecoveryManifestRowHash(recovery)
	if err != nil {
		return fmt.Errorf("compute task asset %d recovery manifest row hash: %w", recovery.MissingTaskAssetID, err)
	}
	if recovery.ManifestRowHash != expectedRowHash ||
		recovery.ObjectProbeResult != expected.ObjectProbeResult ||
		recovery.ObjectProbeInputSHA256 != expected.ObjectProbeInputSHA256 ||
		recovery.ObjectProbeEvidenceHash != expected.ObjectProbeEvidenceHash ||
		recovery.ObjectProbeObjectKeySHA256 != expected.ObjectProbeObjectKeySHA256 ||
		recovery.ObjectProbeReadOnlyGETs != expected.ObjectProbeReadOnlyGETs {
		return fmt.Errorf("task asset %d historical-unavailable mapping is not bound to the frozen object-absence evidence", recovery.MissingTaskAssetID)
	}

	type lineageRow struct {
		id             int64
		taskID         int64
		rootAssetID    int64
		fileSize       int64
		storageRefID   string
		supersededByID sql.NullInt64
		uploadStatus   string
		deletedAt      sql.NullTime
		cleanedAt      sql.NullTime
		objectDeleted  sql.NullTime
	}
	rows, err := q.QueryContext(ctx, `
		SELECT id,task_id,asset_id,file_size,COALESCE(storage_ref_id,''),
		       superseded_by_version_id,COALESCE(upload_status,''),
		       deleted_at,cleaned_at,object_deleted_at
		FROM task_assets
		WHERE id IN (12323,14510,14514)
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("load historical-unavailable lineage: %w", err)
	}
	defer rows.Close()
	var lineage []lineageRow
	for rows.Next() {
		var item lineageRow
		if err := rows.Scan(
			&item.id, &item.taskID, &item.rootAssetID, &item.fileSize, &item.storageRefID,
			&item.supersededByID, &item.uploadStatus,
			&item.deletedAt, &item.cleanedAt, &item.objectDeleted,
		); err != nil {
			return fmt.Errorf("scan historical-unavailable lineage: %w", err)
		}
		lineage = append(lineage, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	expectedLineage := []struct {
		id           int64
		fileSize     int64
		storageRefID string
		supersededBy int64
		hasSuccessor bool
	}{
		{id: 12323, fileSize: 17755216, storageRefID: recovery.OriginalStorageRefID, supersededBy: 14510, hasSuccessor: true},
		{id: 14510, fileSize: 17595421, storageRefID: "58aebabe-355c-4d24-814a-d6dca306b73d", supersededBy: 14514, hasSuccessor: true},
		{id: 14514, fileSize: 11275123, storageRefID: "6e6cd051-f261-424d-8b55-49dd6868be9a"},
	}
	if len(lineage) != len(expectedLineage) {
		return fmt.Errorf("task asset 12323 lineage requires exactly 12323->14510->14514; got %d rows", len(lineage))
	}
	for index, wanted := range expectedLineage {
		got := lineage[index]
		if got.id != wanted.id ||
			got.taskID != recovery.TaskID ||
			got.rootAssetID != expected.RootAssetID ||
			got.fileSize != wanted.fileSize ||
			got.storageRefID != wanted.storageRefID ||
			got.supersededByID.Valid != wanted.hasSuccessor ||
			(wanted.hasSuccessor && got.supersededByID.Int64 != wanted.supersededBy) ||
			got.uploadStatus != "uploaded" ||
			got.deletedAt.Valid ||
			got.cleanedAt.Valid ||
			got.objectDeleted.Valid {
			return fmt.Errorf("task asset 12323 lineage row %d no longer matches frozen 12323->14510->14514 evidence", wanted.id)
		}
	}
	var rootRowCount int
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM task_assets
		WHERE task_id=? AND asset_id=?
		  AND NOT (
		    asset_type='source'
		    AND source_module_key='migration'
		    AND remark LIKE 'v8-source-alias:%'
		  )`,
		recovery.TaskID, expected.RootAssetID,
	).Scan(&rootRowCount); err != nil {
		return err
	}
	if rootRowCount != len(expectedLineage) {
		return fmt.Errorf("task asset 12323 root %d contains %d non-alias rows; frozen delivery lineage requires exactly 3", expected.RootAssetID, rootRowCount)
	}
	aliasRows, err := q.QueryContext(ctx, `
		SELECT id,bound_group_id,COALESCE(bound_role,''),COALESCE(binding_state,''),remark
		FROM task_assets
		WHERE task_id=? AND asset_id=?
		  AND asset_type='source'
		  AND source_module_key='migration'
		ORDER BY id`,
		recovery.TaskID, expected.RootAssetID,
	)
	if err != nil {
		return fmt.Errorf("load task asset 12323 migration source aliases: %w", err)
	}
	defer aliasRows.Close()
	expectedAliasOrigins := map[int64]bool{12323: false, 14510: false, 14514: false}
	aliasCount := 0
	for aliasRows.Next() {
		var aliasID int64
		var boundGroupID sql.NullInt64
		var boundRole, bindingState, remark string
		if err := aliasRows.Scan(&aliasID, &boundGroupID, &boundRole, &bindingState, &remark); err != nil {
			return fmt.Errorf("scan task asset 12323 migration source alias: %w", err)
		}
		aliasCount++
		matchedOrigin := int64(0)
		if boundGroupID.Valid && boundGroupID.Int64 > 0 && boundRole == "source" && bindingState == "bound" {
			for originID := range expectedAliasOrigins {
				if remark == sourceAliasRemark(boundGroupID.Int64, originID) {
					matchedOrigin = originID
					break
				}
			}
		}
		if matchedOrigin == 0 || expectedAliasOrigins[matchedOrigin] {
			return fmt.Errorf("task asset 12323 migration source alias %d is outside the frozen 12323/14510/14514 alias set", aliasID)
		}
		expectedAliasOrigins[matchedOrigin] = true
	}
	if err := aliasRows.Close(); err != nil {
		return err
	}
	if aliasCount != len(expectedAliasOrigins) {
		return fmt.Errorf("task asset 12323 root %d has %d migration source aliases; frozen mapping requires exactly 3", expected.RootAssetID, aliasCount)
	}

	var storageAssetID sql.NullInt64
	var storageOwnerType, storageRefKey, storageStatus string
	var storageOwnerID, storageFileSize int64
	if err := q.QueryRowContext(ctx, `
		SELECT asset_id,owner_type,owner_id,ref_key,status,file_size
		FROM asset_storage_refs
		WHERE ref_id=?`, recovery.OriginalStorageRefID).
		Scan(&storageAssetID, &storageOwnerType, &storageOwnerID, &storageRefKey, &storageStatus, &storageFileSize); err != nil {
		return fmt.Errorf("load historical-unavailable storage ref %s: %w", recovery.OriginalStorageRefID, err)
	}
	if !storageAssetID.Valid ||
		storageAssetID.Int64 != recovery.MissingTaskAssetID ||
		storageOwnerType != "task_asset" ||
		storageOwnerID != recovery.MissingTaskAssetID ||
		storageRefKey != expected.OriginalStorageRefKey ||
		storageStatus != expectedStorageStatus ||
		storageFileSize != recovery.ExpectedFileSize {
		return fmt.Errorf("task asset 12323 original storage ref identity no longer matches frozen evidence")
	}

	var taskID, fileSize int64
	var storageRefID, uploadStatus string
	var deletedAt, cleanedAt, objectDeletedAt sql.NullTime
	if err := q.QueryRowContext(ctx, `
		SELECT task_id,file_size,COALESCE(storage_ref_id,''),COALESCE(upload_status,''),
		       deleted_at,cleaned_at,object_deleted_at
		FROM task_assets
		WHERE id=?`, recovery.MissingTaskAssetID).
		Scan(&taskID, &fileSize, &storageRefID, &uploadStatus, &deletedAt, &cleanedAt, &objectDeletedAt); err != nil {
		return fmt.Errorf("load historical-unavailable task asset %d: %w", recovery.MissingTaskAssetID, err)
	}
	if taskID != recovery.TaskID ||
		fileSize != recovery.ExpectedFileSize ||
		storageRefID != recovery.OriginalStorageRefID ||
		uploadStatus != "uploaded" ||
		deletedAt.Valid ||
		cleanedAt.Valid ||
		objectDeletedAt.Valid {
		return fmt.Errorf("task asset %d no longer matches the frozen historical-unavailable before state", recovery.MissingTaskAssetID)
	}
	derivatives := []struct {
		assetType string
		wholeHash string
	}{
		{assetType: "preview", wholeHash: recovery.PreviewWholeHash},
		{assetType: "design_thumb", wholeHash: recovery.DesignThumbWholeHash},
	}
	for _, derivative := range derivatives {
		var count int
		if err := q.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM task_assets
			WHERE task_id=?
			  AND source_asset_version_id=?
			  AND asset_type=?
			  AND whole_hash=?
			  AND deleted_at IS NULL
			  AND cleaned_at IS NULL
			  AND object_deleted_at IS NULL`,
			recovery.TaskID, recovery.MissingTaskAssetID, derivative.assetType, derivative.wholeHash).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("task asset %d requires exactly one %s derivative matching frozen lineage/hash; got %d", recovery.MissingTaskAssetID, derivative.assetType, count)
		}
	}
	currentReferences, err := countCurrentPointerAssetReferences(ctx, q, recovery.MissingTaskAssetID)
	if err != nil {
		return err
	}
	if currentReferences != 0 {
		return fmt.Errorf("task asset %d is historical-unavailable but is referenced by %d current working/finalized rows", recovery.MissingTaskAssetID, currentReferences)
	}
	return nil
}

func applyAssetRecoveries(
	ctx context.Context,
	tx *sql.Tx,
	recoveries []assetRecoveryMapping,
	targets ...recoveryEvidenceTarget,
) error {
	target := recoveryEvidenceTargetOrClone(targets)
	for _, recovery := range recoveries {
		if recovery.Strategy == "verified_oss_recovery_v1" {
			if err := validatePrematerializedAssetRecoveryEvidence(ctx, tx, recovery, target); err != nil {
				return err
			}
			continue
		}
		if recovery.Strategy != "historical_unavailable_tombstone_v1" || recovery.MissingTaskAssetID != 12323 {
			return fmt.Errorf("asset recovery %d has an unsupported apply strategy", recovery.MissingTaskAssetID)
		}
		var currentStatus string
		if err := tx.QueryRowContext(ctx, `
			SELECT status
			FROM asset_storage_refs
			WHERE ref_id=?
			FOR UPDATE`, recovery.OriginalStorageRefID).Scan(&currentStatus); err != nil {
			return fmt.Errorf(
				"load asset recovery %d current storage ref status: %w",
				recovery.MissingTaskAssetID, err,
			)
		}
		if currentStatus != "recorded" && currentStatus != "historical_unavailable" {
			return fmt.Errorf(
				"asset recovery %d storage ref has unsupported current status %q",
				recovery.MissingTaskAssetID, currentStatus,
			)
		}
		if err := validateHistoricalUnavailableRecoveryEvidence(ctx, tx, recovery, currentStatus); err != nil {
			return err
		}
		if currentStatus == "historical_unavailable" {
			continue
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE asset_storage_refs
			SET status='historical_unavailable'
			WHERE ref_id=? AND status='recorded'`, recovery.OriginalStorageRefID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("asset recovery %d did not update exactly one recorded storage ref", recovery.MissingTaskAssetID)
		}
	}
	return nil
}

func validatePrematerializedAssetRecoveries(
	ctx context.Context,
	q snapshotQueryer,
	recoveries []assetRecoveryMapping,
	targets ...recoveryEvidenceTarget,
) error {
	target := recoveryEvidenceTargetOrClone(targets)
	for _, recovery := range recoveries {
		if recovery.Confidence != "confirmed_auto" || recovery.Strategy != "verified_oss_recovery_v1" {
			continue
		}
		if err := validatePrematerializedAssetRecoveryEvidence(ctx, q, recovery, target); err != nil {
			return err
		}
	}
	return nil
}

func validatePrematerializedAssetRecoveryEvidence(
	ctx context.Context,
	q snapshotQueryer,
	recovery assetRecoveryMapping,
	targets ...recoveryEvidenceTarget,
) error {
	target := recoveryEvidenceTargetOrClone(targets)
	expected, ok := frozenAssetRecoveryEvidenceByMissingID[recovery.MissingTaskAssetID]
	if !ok || recovery.MissingTaskAssetID == 12323 || recovery.TaskID != 2807 {
		return fmt.Errorf("task asset %d is outside the exact prematerialized recovery allowlist", recovery.MissingTaskAssetID)
	}
	expectedRowHash, err := assetRecoveryManifestRowHash(recovery)
	if err != nil {
		return err
	}
	if recovery.ManifestRowHash != expectedRowHash ||
		recovery.RecoverySourceSHA256 != expected.RecoverySourceSHA256 ||
		recovery.ControlledReadProtocol != "controlled-asset-read-v1" ||
		recovery.ControlledReadEvidenceHash != "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08" {
		return fmt.Errorf("task asset %d mapping is not bound to the frozen controlled-read evidence", recovery.MissingTaskAssetID)
	}

	var expectedObjectKey, expectedStorageAdapter string
	switch target.Environment {
	case "clone_b":
		var guardEnvironment, guardRunID, guardPlanSHA string
		if err := q.QueryRowContext(ctx, `
			SELECT environment,run_id,plan_sha256
			FROM v8_ab_clone_guard
			WHERE singleton_id=1`).Scan(&guardEnvironment, &guardRunID, &guardPlanSHA); err != nil {
			return fmt.Errorf("read Clone B recovery guard: %w", err)
		}
		if guardEnvironment != "clone_b" ||
			!recoveryRunIDPattern.MatchString(guardRunID) ||
			!sha256Pattern.MatchString(guardPlanSHA) {
			return fmt.Errorf("task asset %d recovery guard does not identify a valid Clone B executor run", recovery.MissingTaskAssetID)
		}
		expectedObjectKey = fmt.Sprintf(
			"v8-ab/%s/recovered/task-%d/task-asset-%d/%s.bin",
			guardRunID, recovery.TaskID, recovery.MissingTaskAssetID, recovery.RecoverySourceSHA256,
		)
		expectedStorageAdapter = "local"
	case "production":
		expectedObjectKey = fmt.Sprintf(
			"v8-production/%s/%s/recovered/task-%d/task-asset-%d/%s.bin",
			target.Release, target.RunID, recovery.TaskID, recovery.MissingTaskAssetID,
			recovery.RecoverySourceSHA256,
		)
		expectedStorageAdapter = "oss_upload_service"
	default:
		return fmt.Errorf("task asset %d recovery target environment is invalid", recovery.MissingTaskAssetID)
	}

	var taskID, rootAssetID, fileSize int64
	var uploadRequestID, storageRefID, storageKey, wholeHash, uploadStatus, accessRevokedReason string
	var deletedAt, cleanedAt, objectDeletedAt, accessRevokedAt sql.NullTime
	if err := q.QueryRowContext(ctx, `
		SELECT task_id,asset_id,file_size,upload_request_id,
		       COALESCE(storage_ref_id,''),COALESCE(storage_key,''),COALESCE(whole_hash,''),
		       COALESCE(upload_status,''),COALESCE(access_revoked_reason,''),
		       deleted_at,cleaned_at,object_deleted_at,access_revoked_at
		FROM task_assets
		WHERE id=?`, recovery.MissingTaskAssetID).Scan(
		&taskID, &rootAssetID, &fileSize, &uploadRequestID,
		&storageRefID, &storageKey, &wholeHash, &uploadStatus, &accessRevokedReason,
		&deletedAt, &cleanedAt, &objectDeletedAt, &accessRevokedAt,
	); err != nil {
		return fmt.Errorf("read recovered task asset %d: %w", recovery.MissingTaskAssetID, err)
	}
	if taskID != recovery.TaskID ||
		fileSize != recovery.ExpectedFileSize ||
		storageRefID == "" ||
		storageKey != expectedObjectKey ||
		wholeHash != recovery.RecoverySourceSHA256 ||
		uploadStatus != "uploaded" ||
		accessRevokedReason != "" ||
		deletedAt.Valid || cleanedAt.Valid || objectDeletedAt.Valid || accessRevokedAt.Valid {
		return fmt.Errorf("task asset %d does not match the exact recovered after-state", recovery.MissingTaskAssetID)
	}

	var storageAssetID sql.NullInt64
	var ownerType, storageUploadRequestID, storageAdapter, refType, refKey, checksumHint, storageStatus string
	var ownerID, storageFileSize, isPlaceholder int64
	if err := q.QueryRowContext(ctx, `
		SELECT asset_id,owner_type,owner_id,upload_request_id,storage_adapter,ref_type,
		       ref_key,file_size,is_placeholder,COALESCE(checksum_hint,''),status
		FROM asset_storage_refs
		WHERE ref_id=?`, storageRefID).Scan(
		&storageAssetID, &ownerType, &ownerID, &storageUploadRequestID,
		&storageAdapter, &refType, &refKey, &storageFileSize, &isPlaceholder,
		&checksumHint, &storageStatus,
	); err != nil {
		return fmt.Errorf("read recovered storage ref %s: %w", storageRefID, err)
	}
	if !storageAssetID.Valid || storageAssetID.Int64 != rootAssetID ||
		ownerType != "task_asset" || ownerID != recovery.MissingTaskAssetID ||
		storageUploadRequestID != uploadRequestID ||
		storageAdapter != expectedStorageAdapter || refType != "task_asset_object" ||
		refKey != expectedObjectKey || storageFileSize != recovery.ExpectedFileSize ||
		isPlaceholder != 0 || checksumHint != recovery.RecoverySourceSHA256 ||
		storageStatus != "recorded" {
		return fmt.Errorf("task asset %d target storage ref does not match the exact recovered after-state", recovery.MissingTaskAssetID)
	}

	var requestID string
	var boundRefID, uploadChecksum, requestStatus, sessionStatus string
	var uploadFileSize int64
	if err := q.QueryRowContext(ctx, `
		SELECT request_id,COALESCE(bound_ref_id,''),COALESCE(checksum_hint,''),
		       file_size,status,session_status
		FROM upload_requests
		WHERE request_id=?`, uploadRequestID).Scan(
		&requestID, &boundRefID, &uploadChecksum, &uploadFileSize, &requestStatus, &sessionStatus,
	); err != nil {
		return fmt.Errorf("read recovered upload request %s: %w", uploadRequestID, err)
	}
	if requestID != uploadRequestID ||
		boundRefID != storageRefID ||
		uploadChecksum != recovery.RecoverySourceSHA256 ||
		uploadFileSize != recovery.ExpectedFileSize ||
		requestStatus != "bound" || sessionStatus != "completed" {
		return fmt.Errorf("task asset %d upload request does not match the exact recovered after-state", recovery.MissingTaskAssetID)
	}
	return nil
}

func restoreAssetStorageRefStates(ctx context.Context, tx *sql.Tx, states []assetStorageRefStatusSnapshot) error {
	for _, storageRef := range states {
		result, err := tx.ExecContext(ctx, `UPDATE asset_storage_refs SET status=? WHERE ref_id=?`, storageRef.Status, storageRef.RefID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("rollback storage ref %s did not update exactly one row", storageRef.RefID)
		}
	}
	return nil
}

func recoverExistingApply(ctx context.Context, db *sql.DB, database, path string, m mappingFile) (bool, error) {
	s, err := readSnapshot(path, database, m)
	if err != nil {
		return false, err
	}
	if s.Version != workflowGroupsSnapshotVersion {
		return false, v1migrate.NewHardAbort(
			v1migrate.ExitCodeHardAbort,
			"apply recovery refused: snapshot v%d predates lossless timestamp and nullable-scope recovery; use a fresh snapshot directory",
			s.Version,
		)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := lockRollbackTargets(ctx, tx, s); err != nil {
		return false, err
	}
	beforeMatches, err := snapshotStateMatches(ctx, tx, s, false)
	if err != nil {
		return false, err
	}
	autoBeforeMatches, err := autoIncrementStatesMatch(ctx, tx, s.AutoIncrementsBefore)
	if err != nil {
		return false, err
	}
	afterMatches := false
	autoAfterMatches := false
	if s.AppliedAt != nil && s.AfterTasks != nil {
		afterMatches, err = snapshotStateMatches(ctx, tx, s, true)
		if err != nil {
			return false, err
		}
		autoAfterMatches, err = autoIncrementStatesMatch(ctx, tx, s.AutoIncrementsAfter)
		if err != nil {
			return false, err
		}
	}
	switch {
	case s.ApplyState == "applied" && afterMatches && autoAfterMatches:
		return false, tx.Commit()
	case s.ApplyState == "commit_pending" && afterMatches && autoAfterMatches:
		if err := tx.Commit(); err != nil {
			return false, err
		}
		s.ApplyState = "applied"
		return false, writeSnapshot(path, s)
	case needsPreCommitAutoIncrementRecovery(s.ApplyState, beforeMatches, autoBeforeMatches):
		if err := tx.Commit(); err != nil {
			return false, err
		}
		if err := restoreAutoIncrementStatesWithinRecoveryCeilings(
			ctx,
			db,
			s.AutoIncrementsBefore,
			s.AutoIncrementRecoveryCeilings,
		); err != nil {
			return false, err
		}
		if err := removeSnapshot(path); err != nil {
			return false, err
		}
		return true, nil
	case (s.ApplyState == "prepared" || s.ApplyState == "commit_pending" || s.ApplyState == "rolled_back") && beforeMatches && autoBeforeMatches:
		if err := tx.Commit(); err != nil {
			return false, err
		}
		if err := removeSnapshot(path); err != nil {
			return false, err
		}
		return true, nil
	case s.ApplyState == "rollback_dml_pending" || s.ApplyState == "rollback_autoincrement_pending":
		return false, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "apply recovery refused: rollback state %q must be completed with --rollback", s.ApplyState)
	default:
		return false, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "apply recovery refused: manifest state %q does not match the current database before/after state", s.ApplyState)
	}
}

func captureSnapshot(ctx context.Context, q snapshotQueryer, database string, m mappingFile) (snapshot, error) {
	ids := map[int64]struct{}{}
	for _, r := range m.Resources {
		ids[r.TaskID] = struct{}{}
	}
	for _, p := range m.Planning {
		ids[p.TaskID] = struct{}{}
	}
	for _, decision := range m.TaskDecisions {
		ids[decision.TaskID] = struct{}{}
	}
	for _, recovery := range m.AssetRecoveries {
		ids[recovery.TaskID] = struct{}{}
	}
	rows, err := q.QueryContext(ctx, `
		SELECT id,task_type,task_status,workflow_revision,current_handler_id,updated_at
		FROM tasks
		WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingCustomizationReview','PendingCustomizationProduction','PendingEffectReview','PendingEffectRevision','PendingProductionTransfer','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview')
		   OR task_type='purchase_task'
		   OR (task_status='Completed' AND EXISTS (
		       SELECT 1 FROM task_modules tm
		       WHERE tm.task_id=tasks.id
		         AND tm.state NOT IN ('completed','closed','forcibly_closed','closed_by_admin')
		   ))
		ORDER BY id`)
	if err != nil {
		return snapshot{}, err
	}
	defer rows.Close()
	s := snapshot{
		Version: workflowGroupsSnapshotVersion, ToolVersion: workflowGroupsToolVersion,
		SchemaVersion: workflowGroupsSchemaVersion, Database: database, CreatedAt: time.Now().UTC(),
	}
	for rows.Next() {
		var t taskSnapshot
		var handlerID sql.NullInt64
		if err := rows.Scan(&t.ID, &t.TaskType, &t.TaskStatus, &t.WorkflowRevision, &handlerID, &t.UpdatedAt); err != nil {
			return s, err
		}
		if handlerID.Valid {
			value := handlerID.Int64
			t.CurrentHandlerID = &value
		}
		s.Tasks = append(s.Tasks, t)
		delete(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return s, err
	}
	for id := range ids {
		var t taskSnapshot
		var handlerID sql.NullInt64
		if err := q.QueryRowContext(ctx, `SELECT id, task_type, task_status, workflow_revision, current_handler_id, updated_at FROM tasks WHERE id=?`, id).Scan(&t.ID, &t.TaskType, &t.TaskStatus, &t.WorkflowRevision, &handlerID, &t.UpdatedAt); err != nil {
			return s, err
		}
		if handlerID.Valid {
			value := handlerID.Int64
			t.CurrentHandlerID = &value
		}
		s.Tasks = append(s.Tasks, t)
	}
	sort.Slice(s.Tasks, func(i, j int) bool { return s.Tasks[i].ID < s.Tasks[j].ID })
	taskIDs := make([]int64, 0, len(s.Tasks))
	for _, task := range s.Tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	s.Tasks, err = captureTaskSnapshotsBulk(ctx, q, taskIDs)
	if err != nil {
		return s, err
	}
	s.TaskModulesBefore, err = captureTaskModulesForTasks(ctx, q, taskIDs)
	if err != nil {
		return s, err
	}
	s.SearchDocumentsBefore, err = captureSearchDocumentsForTasks(ctx, q, taskIDs)
	if err != nil {
		return s, err
	}
	s.TaskSearchDocumentsBefore, err = captureTaskSearchDocumentsForTasks(ctx, q, taskIDs)
	if err != nil {
		return s, err
	}
	s.ResourceGroups, err = captureResourceGroupsForTasks(ctx, q, taskIDs)
	if err != nil {
		return s, err
	}
	s.AssetBindings, err = captureAssetBindingsForTasks(ctx, q, taskIDs)
	if err != nil {
		return s, err
	}
	s.SKUOrigins, err = captureSKUOrigins(ctx, q, planningTaskIDs(m))
	if err != nil {
		return s, err
	}
	s.PlanningBefore, err = capturePlanningStates(ctx, q, planningTaskIDs(m))
	if err != nil {
		return s, err
	}
	s.OrganizationBefore, err = captureOrganizationStates(ctx, q, m.OrganizationMappings)
	if err != nil {
		return s, err
	}
	s.AccessBefore, err = captureAccessStates(ctx, q, m.AccessDecisions)
	if err != nil {
		return s, err
	}
	s.StorageRefsBefore, err = captureAssetStorageRefStates(ctx, q, m.AssetRecoveries)
	if err != nil {
		return s, err
	}
	s.AutoIncrementsBefore, err = captureAutoIncrementStates(ctx, q)
	if err != nil {
		return s, err
	}
	s.AutoIncrementRecoveryCeilings, err = autoIncrementRecoveryCeilings(s.AutoIncrementsBefore, m)
	if err != nil {
		return s, err
	}
	return s, nil
}

func migrateStates(ctx context.Context, tx *sql.Tx, m mappingFile) error {
	for _, decision := range m.TaskDecisions {
		result, err := tx.ExecContext(ctx, `
			UPDATE tasks
			SET task_status=?,workflow_revision=workflow_revision+1
			WHERE id=? AND task_status=?`, decision.TargetStatus, decision.TaskID, decision.FromStatus)
		if err != nil {
			return err
		}
		if rows, _ := result.RowsAffected(); rows != 1 {
			var currentStatus string
			if err := tx.QueryRowContext(ctx, `SELECT task_status FROM tasks WHERE id=? FOR UPDATE`, decision.TaskID).Scan(&currentStatus); err != nil {
				return fmt.Errorf("load task %d after reviewed state decision miss: %w", decision.TaskID, err)
			}
			if currentStatus != decision.TargetStatus {
				return fmt.Errorf("task %d no longer matches reviewed state decision %s -> %s", decision.TaskID, decision.FromStatus, decision.TargetStatus)
			}
		}
		if isReviewedRetouchReopenDecision(decision) {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_modules
				SET state='in_progress',terminal_at=NULL,updated_at=?
				WHERE task_id=? AND module_key='retouch'`,
				decision.ConfirmedAt.UTC(), decision.TaskID); err != nil {
				return fmt.Errorf("reopen reviewed retouch module for task %d: %w", decision.TaskID, err)
			}
			rows, err := tx.QueryContext(ctx, `
				SELECT state,terminal_at
				FROM task_modules
				WHERE task_id=? AND module_key='retouch'
				FOR UPDATE`, decision.TaskID)
			if err != nil {
				return fmt.Errorf("verify reviewed retouch module for task %d: %w", decision.TaskID, err)
			}
			moduleCount := 0
			activeCount := 0
			for rows.Next() {
				var state string
				var terminalAt sql.NullTime
				if err := rows.Scan(&state, &terminalAt); err != nil {
					rows.Close()
					return fmt.Errorf("scan reviewed retouch module for task %d: %w", decision.TaskID, err)
				}
				moduleCount++
				if state == "in_progress" && !terminalAt.Valid {
					activeCount++
				}
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return fmt.Errorf("iterate reviewed retouch module rows for task %d: %w", decision.TaskID, err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close reviewed retouch module rows for task %d: %w", decision.TaskID, err)
			}
			if moduleCount != 1 || activeCount != 1 {
				return fmt.Errorf(
					"task %d reviewed retouch reopen requires exactly one active non-terminal retouch module, found modules=%d active=%d",
					decision.TaskID, moduleCount, activeCount,
				)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET task_status=CASE
		WHEN task_status IN ('PendingAuditA','PendingAuditB','PendingCustomizationReview','PendingOutsourceReview','PendingEffectReview') THEN 'PendingAudit'
		WHEN task_status IN ('RejectedByAuditA','RejectedByAuditB','PendingCustomizationProduction','PendingEffectRevision','PendingOutsource','Outsourcing') THEN 'InProgress'
		WHEN task_status IN ('PendingWarehouseQC','PendingWarehouseReceive','PendingProductionTransfer','PendingClose') THEN 'Completed'
		ELSE task_status END,
		workflow_revision=workflow_revision+1
	WHERE task_status IN ('PendingAuditA','PendingAuditB','PendingCustomizationReview','PendingOutsourceReview','PendingEffectReview','RejectedByAuditA','RejectedByAuditB','PendingCustomizationProduction','PendingEffectRevision','PendingOutsource','Outsourcing','PendingWarehouseQC','PendingWarehouseReceive','PendingProductionTransfer','PendingClose')`); err != nil {
		return err
	}
	return nil
}

func normalizeCompletedTaskModules(ctx context.Context, tx *sql.Tx, taskIDs []int64) error {
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		if _, err := tx.ExecContext(ctx, `
			UPDATE task_modules tm
			JOIN tasks t ON t.id=tm.task_id
			SET tm.state='completed',
			    tm.claimed_by=NULL,
			    tm.claimed_team_code=NULL,
			    tm.terminal_at=COALESCE(tm.terminal_at,CURRENT_TIMESTAMP)
			WHERE tm.task_id IN (`+placeholders+`)
			  AND t.task_status='Completed'
			  AND tm.state NOT IN ('completed','closed','forcibly_closed','closed_by_admin')`,
			args...); err != nil {
			return fmt.Errorf("normalize completed task modules: %w", err)
		}
	}
	return nil
}

func applyResource(ctx context.Context, tx *sql.Tx, m resourceMapping) (int64, int64, bool, bool, error) {
	var groupID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=? FOR UPDATE`, m.TaskID, m.ScopeKind, m.ScopeRefID).Scan(&groupID)
	inserted := false
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, false, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		var skuID, retouchID any
		if m.ScopeKind == "sku" {
			skuID = m.ScopeRefID
		}
		if m.ScopeKind == "retouch_requirement" {
			retouchID = m.ScopeRefID
		}
		res, insertErr := tx.ExecContext(ctx, `INSERT INTO task_asset_groups (task_id,scope_kind,task_sku_item_id,retouch_requirement_id,migration_incomplete,migration_issue) VALUES (?,?,?,?,1,'legacy resource revision pending cutover mapping')`, m.TaskID, m.ScopeKind, skuID, retouchID)
		if insertErr != nil {
			return 0, 0, false, false, insertErr
		}
		groupID, _ = res.LastInsertId()
		inserted = true
	}
	var taskType, taskStatus string
	if err := tx.QueryRowContext(ctx, `SELECT task_type, task_status FROM tasks WHERE id=? FOR UPDATE`, m.TaskID).Scan(&taskType, &taskStatus); err != nil {
		return 0, 0, inserted, false, err
	}
	var revisionCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_asset_group_revisions WHERE group_id=?`, groupID).Scan(&revisionCount); err != nil {
		return 0, 0, inserted, false, err
	}
	if revisionCount > 0 {
		var incomplete bool
		if err := tx.QueryRowContext(ctx, `SELECT migration_incomplete FROM task_asset_groups WHERE id=?`, groupID).Scan(&incomplete); err != nil {
			return 0, 0, inserted, false, err
		}
		if incomplete {
			return 0, 0, inserted, false, fmt.Errorf("group %d has revisions but is still migration_incomplete; manual reconciliation required", groupID)
		}
		if err := verifyResourceMappingQuery(ctx, tx, groupID, m); err != nil {
			return 0, 0, inserted, false, err
		}
		return groupID, 0, inserted, false, nil
	}
	status := m.TargetStatus
	if status == "shell" {
		if taskStatus == "PendingAudit" || taskStatus == "Completed" {
			return 0, 0, inserted, false, fmt.Errorf("task %d in %s cannot be migrated as an empty shell", m.TaskID, taskStatus)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE task_asset_groups SET migration_incomplete=0,migration_issue='' WHERE id=?`, groupID); err != nil {
			return 0, 0, inserted, false, err
		}
		return groupID, 0, inserted, false, nil
	}
	if status != "draft" && status != "submitted" && status != "finalized" {
		return 0, 0, inserted, false, fmt.Errorf("invalid target_status %q", status)
	}
	if taskType != "retouch_task" && m.SourceAssetID == nil {
		return 0, 0, inserted, false, fmt.Errorf("task %d scope %s/%d has no confirmed source file", m.TaskID, m.ScopeKind, m.ScopeRefID)
	}
	if taskStatus == "Completed" && status != "finalized" {
		return 0, 0, inserted, false, fmt.Errorf("completed task %d requires finalized resource mappings", m.TaskID)
	}
	if taskStatus == "PendingAudit" && status != "submitted" {
		return 0, 0, inserted, false, fmt.Errorf("pending-audit task %d requires submitted resource mappings", m.TaskID)
	}
	assetIDs := append([]int64(nil), m.FinalAssetIDs...)
	if m.SourceAssetID != nil {
		assetIDs = append(assetIDs, *m.SourceAssetID)
	}
	for _, assetID := range assetIDs {
		var ownerTaskID int64
		if err := tx.QueryRowContext(ctx, `SELECT task_id FROM task_assets WHERE id=? FOR UPDATE`, assetID).Scan(&ownerTaskID); err != nil {
			return 0, 0, inserted, false, fmt.Errorf("task asset %d: %w", assetID, err)
		}
		if ownerTaskID != m.TaskID {
			return 0, 0, inserted, false, fmt.Errorf("task asset %d belongs to task %d, not task %d", assetID, ownerTaskID, m.TaskID)
		}
	}
	for _, referenceID := range m.ReferenceIDs {
		var ownerTaskID int64
		var skuItemID, retouchRequirementID sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT task_id,sku_item_id,retouch_requirement_id FROM reference_file_refs WHERE id=? FOR UPDATE`, referenceID).Scan(&ownerTaskID, &skuItemID, &retouchRequirementID); err != nil {
			return 0, 0, inserted, false, fmt.Errorf("reference file %d: %w", referenceID, err)
		}
		if ownerTaskID != m.TaskID || !referenceMatchesResourceScope(m.ScopeKind, m.ScopeRefID, skuItemID, retouchRequirementID) {
			return 0, 0, inserted, false, fmt.Errorf("reference file %d does not belong to the mapped task scope", referenceID)
		}
	}
	stage := "migration"
	res, err := tx.ExecContext(ctx, `INSERT INTO task_asset_group_revisions (group_id,revision_no,status,mode,source_task_asset_id,source_stage,created_by,reason,submitted_at,finalized_at) VALUES (?,1,?,?,?,?,?,?,IF(? IN ('submitted','finalized'),NOW(),NULL),IF(?='finalized',NOW(),NULL))`, groupID, status, m.Mode, m.SourceAssetID, stage, m.CreatedBy, firstNonEmpty(m.Reason, "confirmed migration mapping"), status, status)
	if err != nil {
		return 0, 0, inserted, false, err
	}
	revisionID, _ := res.LastInsertId()
	for i, assetID := range m.FinalAssetIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO task_asset_group_revision_items (revision_id,task_asset_id,sort_order) VALUES (?,?,?)`, revisionID, assetID, i); err != nil {
			return 0, 0, inserted, false, err
		}
	}
	for i, refID := range m.ReferenceIDs {
		if err = insertMigratedReferenceSnapshot(ctx, tx, revisionID, refID, i); err != nil {
			return 0, 0, inserted, false, err
		}
	}
	pointer := "working_revision_id"
	if status == "finalized" {
		pointer = "finalized_revision_id"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE task_asset_groups SET `+pointer+`=?, lock_version=lock_version+1,migration_incomplete=0,migration_issue='' WHERE id=?`, revisionID, groupID); err != nil {
		return 0, 0, inserted, false, err
	}
	if m.SourceAssetID != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE task_assets SET binding_state='bound',bound_group_id=?,bound_role='source' WHERE id=?`, groupID, *m.SourceAssetID); err != nil {
			return 0, 0, inserted, false, err
		}
	}
	for _, assetID := range m.FinalAssetIDs {
		if _, err = tx.ExecContext(ctx, `UPDATE task_assets SET binding_state='bound',bound_group_id=?,bound_role='final' WHERE id=?`, groupID, assetID); err != nil {
			return 0, 0, inserted, false, err
		}
	}
	return groupID, revisionID, inserted, true, nil
}

func insertMigratedReferenceSnapshot(ctx context.Context, tx *sql.Tx, revisionID, referenceID int64, sortOrder int) error {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO task_asset_group_revision_references
		  (revision_id,reference_file_ref_id,sort_order,ref_id_snapshot,file_name_snapshot,scope_snapshot)
		SELECT ?,rfr.id,?,rfr.ref_id,COALESCE(asr.file_name,''),
		       CASE
		         WHEN rfr.retouch_requirement_id IS NOT NULL THEN CONCAT('retouch_requirement:',rfr.retouch_requirement_id)
		         WHEN rfr.sku_item_id IS NOT NULL THEN CONCAT('sku:',rfr.sku_item_id)
		         ELSE 'task'
		       END
		FROM reference_file_refs rfr
		LEFT JOIN asset_storage_refs asr ON asr.ref_id=rfr.ref_id
		WHERE rfr.id=?`, revisionID, sortOrder, referenceID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return fmt.Errorf("reference file %d disappeared before snapshot insert", referenceID)
	}
	return nil
}

func verifyResourceMappingQuery(ctx context.Context, q snapshotQueryer, groupID int64, m resourceMapping) error {
	if m.TargetStatus == "shell" {
		return fmt.Errorf("group %d already has resource revisions and cannot match a shell mapping", groupID)
	}
	var revisionID sql.NullInt64
	column := "working_revision_id"
	if m.TargetStatus == "finalized" {
		column = "finalized_revision_id"
	}
	if err := q.QueryRowContext(ctx, `SELECT `+column+` FROM task_asset_groups WHERE id=?`, groupID).Scan(&revisionID); err != nil {
		return err
	}
	if !revisionID.Valid {
		return fmt.Errorf("group %d has revisions but no %s pointer", groupID, column)
	}
	var status, mode, sourceStage string
	var sourceID sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT status,mode,source_task_asset_id,source_stage FROM task_asset_group_revisions WHERE id=? AND group_id=?`, revisionID.Int64, groupID).Scan(&status, &mode, &sourceID, &sourceStage); err != nil {
		return err
	}
	if status != m.TargetStatus || mode != m.Mode || nullableInt64Value(sourceID) != pointerInt64Value(m.SourceAssetID) {
		return fmt.Errorf("group %d existing revision does not match requested status/mode/source", groupID)
	}
	rows, err := q.QueryContext(ctx, `SELECT task_asset_id FROM task_asset_group_revision_items WHERE revision_id=? ORDER BY sort_order,id`, revisionID.Int64)
	if err != nil {
		return err
	}
	var finals []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		finals = append(finals, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !equalInt64Slices(finals, m.FinalAssetIDs) {
		return fmt.Errorf("group %d existing final-file order differs from mapping", groupID)
	}
	rows, err = q.QueryContext(ctx, `
		SELECT rr.reference_file_ref_id,rr.ref_id_snapshot,rr.file_name_snapshot,rr.scope_snapshot,
		       rfr.ref_id,COALESCE(asr.file_name,''),
		       CASE
		         WHEN rfr.retouch_requirement_id IS NOT NULL THEN CONCAT('retouch_requirement:',rfr.retouch_requirement_id)
		         WHEN rfr.sku_item_id IS NOT NULL THEN CONCAT('sku:',rfr.sku_item_id)
		         ELSE 'task'
		       END AS expected_scope_snapshot
		FROM task_asset_group_revision_references rr
		JOIN reference_file_refs rfr ON rfr.id=rr.reference_file_ref_id
		LEFT JOIN asset_storage_refs asr ON asr.ref_id=rfr.ref_id
		WHERE rr.revision_id=?
		ORDER BY rr.sort_order,rr.id`, revisionID.Int64)
	if err != nil {
		return err
	}
	var references []int64
	for rows.Next() {
		var id int64
		var refIDSnapshot, fileNameSnapshot, scopeSnapshot string
		var expectedRefID, expectedFileName, expectedScope string
		if err := rows.Scan(&id, &refIDSnapshot, &fileNameSnapshot, &scopeSnapshot, &expectedRefID, &expectedFileName, &expectedScope); err != nil {
			rows.Close()
			return err
		}
		if sourceStage == "migration" && (refIDSnapshot != expectedRefID || fileNameSnapshot != expectedFileName || scopeSnapshot != expectedScope) {
			rows.Close()
			return fmt.Errorf("group %d migrated reference %d has an invalid frozen snapshot", groupID, id)
		}
		references = append(references, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !equalInt64Slices(references, m.ReferenceIDs) {
		return fmt.Errorf("group %d existing references differ from mapping", groupID)
	}
	return nil
}

func nullableInt64Value(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

func pointerInt64Value(value *int64) int64 {
	if value != nil {
		return *value
	}
	return 0
}

func equalInt64Slices(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func applyPlanning(ctx context.Context, tx *sql.Tx, m planningMapping) (bool, error) {
	var taskType, taskStatus string
	if err := tx.QueryRowContext(ctx, `SELECT task_type,task_status FROM tasks WHERE id=? FOR UPDATE`, m.TaskID).Scan(&taskType, &taskStatus); err != nil {
		return false, err
	}
	if taskType != "purchase_task" && taskType != "sku_planning" {
		return false, fmt.Errorf("planning task %d has unsupported task_type=%s", m.TaskID, taskType)
	}
	if err := validatePlanningTargetTransition(taskStatus, m.TargetTaskStatus); err != nil {
		return false, fmt.Errorf("planning task %d: %w", m.TaskID, err)
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_planning_settings WHERE task_id=?`, m.TaskID).Scan(&exists); err != nil {
		return false, err
	}
	if exists > 0 {
		if err := verifyPlanningMappingQuery(ctx, tx, m); err != nil {
			return false, err
		}
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO task_planning_settings (task_id,erp_sync_mode,code_rule_revision_id,client_create_id,created_by) VALUES (?,'none',?,?,?)`, m.TaskID, m.CodeRuleRevisionID, fmt.Sprintf("migration-%d", m.TaskID), m.CreatedBy); err != nil {
		return false, err
	}
	if !isIncompleteUATPlanningTombstone(m) {
		for _, item := range m.Items {
			if _, err := tx.ExecContext(ctx, `INSERT INTO task_planning_sku_details (task_sku_item_id) VALUES (?)`, item.TaskSKUItemID); err != nil {
				return false, err
			}
			res, err := tx.ExecContext(ctx, `INSERT INTO task_planning_sku_revisions (task_sku_item_id,version_no,description_spec,quantity,target_price,currency,note,reference_url,erp_product_i_id,erp_product_name,reason,created_by) VALUES (?,1,?,?,?,'CNY',?,?,?,?,'confirmed legacy planning migration',?)`, item.TaskSKUItemID, item.DescriptionSpec, item.Quantity, item.TargetPrice, item.Note, item.ReferenceURL, item.ERPProductIID, item.ERPProductName, m.CreatedBy)
			if err != nil {
				return false, err
			}
			revisionID, _ := res.LastInsertId()
			if _, err = tx.ExecContext(ctx, `UPDATE task_planning_sku_details SET current_revision_id=? WHERE task_sku_item_id=?`, revisionID, item.TaskSKUItemID); err != nil {
				return false, err
			}
			if item.ImageStorageRef != "" {
				if _, err = tx.ExecContext(ctx, `INSERT INTO task_planning_sku_revision_images (revision_id,storage_ref_id) VALUES (?,?)`, revisionID, item.ImageStorageRef); err != nil {
					return false, err
				}
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET task_type='sku_planning',task_status=?,workflow_revision=workflow_revision+1 WHERE id=?`, m.TargetTaskStatus, m.TaskID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE task_sku_items SET sku_origin='legacy_migration' WHERE task_id=?`, m.TaskID); err != nil {
		return false, err
	}
	return true, nil
}

func validatePlanningTargetTransition(current, target string) error {
	if !validPlanningTargetStatus(target) {
		return fmt.Errorf("target_task_status=%q is not a current task status", target)
	}
	if (current == "Cancelled" || current == "Archived") && target != current {
		return fmt.Errorf("terminal status %s must be preserved", current)
	}
	return nil
}

func verifyPlanningMappingQuery(ctx context.Context, q snapshotQueryer, m planningMapping) error {
	var taskType, taskStatus string
	var ruleRevisionID, createdBy int64
	if err := q.QueryRowContext(ctx, `
		SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by
		FROM task_planning_settings s JOIN tasks t ON t.id=s.task_id
		WHERE s.task_id=?`, m.TaskID).Scan(&taskType, &taskStatus, &ruleRevisionID, &createdBy); err != nil {
		return err
	}
	if taskType != "sku_planning" || taskStatus != m.TargetTaskStatus || ruleRevisionID != m.CodeRuleRevisionID || createdBy != m.CreatedBy {
		return fmt.Errorf("planning task %d settings differ from the confirmed mapping", m.TaskID)
	}
	var itemCount int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_sku_items WHERE task_id=?`, m.TaskID).Scan(&itemCount); err != nil {
		return err
	}
	if itemCount != len(m.Items) {
		return fmt.Errorf("planning task %d has %d SKU rows but mapping has %d", m.TaskID, itemCount, len(m.Items))
	}
	if isIncompleteUATPlanningTombstone(m) {
		var exactItemCount int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_sku_items WHERE task_id=? AND id=?`, m.TaskID, int64(380)).Scan(&exactItemCount); err != nil {
			return err
		}
		if exactItemCount != 1 {
			return fmt.Errorf("planning tombstone task %d must preserve exact SKU item 380", m.TaskID)
		}
		zeroChecks := []struct {
			name  string
			query string
		}{
			{name: "details", query: `SELECT COUNT(*) FROM task_planning_sku_details d JOIN task_sku_items si ON si.id=d.task_sku_item_id WHERE si.task_id=?`},
			{name: "revisions", query: `SELECT COUNT(*) FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=?`},
			{name: "images", query: `SELECT COUNT(*) FROM task_planning_sku_revision_images img JOIN task_planning_sku_revisions r ON r.id=img.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=?`},
		}
		for _, check := range zeroChecks {
			var count int
			if err := q.QueryRowContext(ctx, check.query, m.TaskID).Scan(&count); err != nil {
				return err
			}
			if count != 0 {
				return fmt.Errorf("planning tombstone task %d has %d %s; expected zero", m.TaskID, count, check.name)
			}
		}
		return nil
	}
	for _, item := range m.Items {
		var description string
		var quantity int64
		var targetPrice sql.NullString
		var note, referenceURL, erpIID, erpName string
		var imageRef sql.NullString
		err := q.QueryRowContext(ctx, `
			SELECT r.description_spec,r.quantity,CAST(r.target_price AS CHAR),r.note,r.reference_url,
			       r.erp_product_i_id,r.erp_product_name,img.storage_ref_id
			FROM task_sku_items si
			JOIN task_planning_sku_details d ON d.task_sku_item_id=si.id AND d.current_revision_id IS NOT NULL
			JOIN task_planning_sku_revisions r ON r.id=d.current_revision_id AND r.task_sku_item_id=d.task_sku_item_id
			LEFT JOIN task_planning_sku_revision_images img ON img.revision_id=r.id
			WHERE si.task_id=? AND si.id=?`, m.TaskID, item.TaskSKUItemID).
			Scan(&description, &quantity, &targetPrice, &note, &referenceURL, &erpIID, &erpName, &imageRef)
		if err != nil {
			return fmt.Errorf("planning task %d item %d is incomplete: %w", m.TaskID, item.TaskSKUItemID, err)
		}
		if description != item.DescriptionSpec || quantity != item.Quantity || nullableStringValue(targetPrice) != stringPointerValue(item.TargetPrice) || note != item.Note || referenceURL != item.ReferenceURL || erpIID != item.ERPProductIID || erpName != item.ERPProductName || nullableStringValue(imageRef) != item.ImageStorageRef {
			return fmt.Errorf("planning task %d item %d differs from the confirmed mapping", m.TaskID, item.TaskSKUItemID)
		}
	}
	return nil
}

func validateCutoverState(
	ctx context.Context,
	tx *sql.Tx,
	m mappingFile,
	targets ...recoveryEvidenceTarget,
) error {
	target := recoveryEvidenceTargetOrClone(targets)
	// The complete mapping preflight already ran twice before mutation, with the
	// second pass protected by the cutover locks. Keep this function focused on
	// post-cutover invariants instead of repeating the expensive before-state scan.
	planningTombstoneExclusion := ""
	for _, planning := range m.Planning {
		if planning.Confidence != "confirmed_auto" {
			continue
		}
		if err := verifyPlanningMappingQuery(ctx, tx, planning); err != nil {
			return fmt.Errorf("cutover blocked: verified planning state differs from mapping: %w", err)
		}
		if isIncompleteUATPlanningTombstone(planning) {
			planningTombstoneExclusion = "AND t.id <> 497"
		}
	}
	checks := []struct {
		name  string
		query string
	}{
		{name: "resource groups still migration-incomplete", query: `SELECT COUNT(*) FROM task_asset_groups WHERE migration_incomplete=1`},
		{name: "legacy purchase_task rows remain", query: `SELECT COUNT(*) FROM tasks WHERE task_type='purchase_task'`},
		{name: "retired active task statuses remain", query: `SELECT COUNT(*) FROM tasks WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingCustomizationReview','PendingCustomizationProduction','PendingEffectReview','PendingEffectRevision','PendingProductionTransfer','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview','RejectedByWarehouse')`},
		{name: "completed tasks retain open modules", query: `SELECT COUNT(*) FROM task_modules tm JOIN tasks t ON t.id=tm.task_id WHERE t.task_status='Completed' AND tm.state NOT IN ('completed','closed','forcibly_closed','closed_by_admin')`},
		{name: "planning settings/detail/current revision parity failures", query: fmt.Sprintf(`
			SELECT COUNT(*) FROM tasks t
			WHERE t.task_type='sku_planning' %s AND (
			  NOT EXISTS (SELECT 1 FROM task_planning_settings s WHERE s.task_id=t.id)
			  OR EXISTS (
			    SELECT 1 FROM task_sku_items si
			    LEFT JOIN task_planning_sku_details d ON d.task_sku_item_id=si.id
			    LEFT JOIN task_planning_sku_revisions r ON r.id=d.current_revision_id AND r.task_sku_item_id=d.task_sku_item_id
			    WHERE si.task_id=t.id AND (d.task_sku_item_id IS NULL OR d.current_revision_id IS NULL OR r.id IS NULL)
			  )
			  OR NOT EXISTS (SELECT 1 FROM task_sku_items si WHERE si.task_id=t.id)
			)`, planningTombstoneExclusion)},
		{name: "task resource-group scope set differs from the exact expected set", query: `
			WITH expected_scopes AS (
			  SELECT t.id AS task_id,'retouch_requirement' AS scope_kind,trr.id AS scope_ref_id
			  FROM tasks t JOIN task_retouch_requirements trr ON trr.task_id=t.id
			  WHERE t.task_type='retouch_task'
			  UNION ALL
			  SELECT t.id,'sku',tsi.id
			  FROM tasks t JOIN task_sku_items tsi ON tsi.task_id=t.id
			  WHERE t.task_type NOT IN ('retouch_task','purchase_task','sku_planning')
			  UNION ALL
			  SELECT t.id,'task',0
			  FROM tasks t
			  WHERE t.task_type NOT IN ('retouch_task','purchase_task','sku_planning')
			    AND NOT EXISTS (SELECT 1 FROM task_sku_items tsi WHERE tsi.task_id=t.id)
			), scope_drift AS (
			  SELECT e.task_id,e.scope_kind,e.scope_ref_id
			  FROM expected_scopes e
			  LEFT JOIN task_asset_groups g ON g.task_id=e.task_id AND g.scope_kind=e.scope_kind AND g.scope_ref_id=e.scope_ref_id
			  WHERE g.id IS NULL
			  UNION ALL
			  SELECT g.task_id,g.scope_kind,g.scope_ref_id
			  FROM task_asset_groups g JOIN tasks t ON t.id=g.task_id
			  LEFT JOIN expected_scopes e ON e.task_id=g.task_id AND e.scope_kind=g.scope_kind AND e.scope_ref_id=g.scope_ref_id
			  WHERE t.task_type NOT IN ('purchase_task','sku_planning') AND e.task_id IS NULL
			)
			SELECT COUNT(*) FROM scope_drift`},
		{name: "sku_planning tasks still own design resource groups", query: `SELECT COUNT(*) FROM tasks t JOIN task_asset_groups g ON g.task_id=t.id WHERE t.task_type='sku_planning'`},
	}
	for _, check := range checks {
		var count int64
		if err := tx.QueryRowContext(ctx, check.query).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("cutover blocked: %s (%d)", check.name, count)
		}
	}
	for _, recovery := range m.AssetRecoveries {
		if recovery.Strategy == "verified_oss_recovery_v1" {
			if err := validatePrematerializedAssetRecoveryEvidence(ctx, tx, recovery, target); err != nil {
				return fmt.Errorf("cutover blocked: prematerialized recovery evidence drifted: %w", err)
			}
			continue
		}
		if err := validateHistoricalUnavailableRecoveryEvidence(ctx, tx, recovery, "historical_unavailable"); err != nil {
			return fmt.Errorf("cutover blocked: historical-unavailable recovery evidence drifted: %w", err)
		}
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM asset_storage_refs WHERE ref_id=?`, recovery.OriginalStorageRefID).Scan(&status); err != nil {
			return err
		}
		if status != "historical_unavailable" {
			return fmt.Errorf("cutover blocked: asset recovery %d storage ref status=%q", recovery.MissingTaskAssetID, status)
		}
	}
	return nil
}

func nullableStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func stringPointerValue(value *string) string {
	if value != nil {
		return *value
	}
	return ""
}

func restoreTaskModuleStates(ctx context.Context, tx *sql.Tx, items []taskModuleSnapshot) error {
	for _, item := range items {
		_, err := tx.ExecContext(ctx, `
			UPDATE task_modules
			SET state=?,pool_team_code=?,claimed_by=?,claimed_team_code=?,
			    claimed_at=?,actor_org_snapshot=?,entered_at=?,terminal_at=?,
			    data=?,updated_at=?
			WHERE id=? AND task_id=?`,
			item.State, nullableStringPointer(item.PoolTeamCode),
			nullableInt64Pointer(item.ClaimedBy),
			nullableStringPointer(item.ClaimedTeamCode),
			nullableTimePointer(item.ClaimedAt), []byte(item.ActorOrgSnapshot),
			item.EnteredAt, nullableTimePointer(item.TerminalAt), []byte(item.Data),
			item.UpdatedAt, item.ID, item.TaskID)
		if err != nil {
			return err
		}
	}
	return nil
}

func restoreSearchDocuments(
	ctx context.Context,
	tx *sql.Tx,
	taskIDs []int64,
	items []searchDocumentSnapshot,
) error {
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM task_asset_group_search_documents WHERE task_id IN (`+placeholders+`)`,
			args...); err != nil {
			return err
		}
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_asset_group_search_documents
				(group_id,task_id,finalized_revision_id,internal_text,final_text,updated_at)
			VALUES (?,?,?,?,?,?)`,
			item.GroupID, item.TaskID,
			nullableInt64Pointer(item.FinalizedRevisionID),
			item.InternalText, item.FinalText, item.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}

func restoreTaskSearchDocuments(
	ctx context.Context,
	tx *sql.Tx,
	taskIDs []int64,
	items []taskSearchDocumentSnapshot,
) error {
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM task_search_documents WHERE task_id IN (`+placeholders+`)`,
			args...); err != nil {
			return err
		}
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_search_documents
				(task_id,task_no,product_name_snapshot,sku_code,primary_sku_code,
				 product_i_id,task_type,task_status,priority,owner_department,
				 owner_team,owner_org_team,creator_id,creator_name,requester_id,
				 requester_name,designer_id,designer_name,current_handler_id,
				 current_handler_name,created_at,updated_at,deadline_at,asset_text,
				 search_text)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			item.TaskID, item.TaskNo, item.ProductNameSnapshot, item.SKUCode,
			item.PrimarySKUCode, item.ProductIID, item.TaskType, item.TaskStatus,
			item.Priority, item.OwnerDepartment, item.OwnerTeam, item.OwnerOrgTeam,
			nullableInt64Pointer(item.CreatorID), item.CreatorName,
			nullableInt64Pointer(item.RequesterID), item.RequesterName,
			nullableInt64Pointer(item.DesignerID), item.DesignerName,
			nullableInt64Pointer(item.CurrentHandlerID), item.CurrentHandlerName,
			nullableTimePointer(item.CreatedAt), nullableTimePointer(item.UpdatedAt),
			nullableTimePointer(item.DeadlineAt), nullableStringPointer(item.AssetText),
			item.SearchText); err != nil {
			return err
		}
	}
	return nil
}

func rollback(ctx context.Context, db *sql.DB, database string, o options, m mappingFile) error {
	path := filepath.Join(o.SnapshotDir, "workflow-groups-snapshot.json")
	s, err := readSnapshot(path, database, m)
	if err != nil {
		return err
	}
	if s.Version != workflowGroupsSnapshotVersion {
		return v1migrate.NewHardAbort(
			v1migrate.ExitCodeHardAbort,
			"rollback refused: snapshot v%d predates lossless timestamp and nullable-scope recovery; rerun with the matching historical tool or prepare a reviewed recovery",
			s.Version,
		)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockRollbackTargets(ctx, tx, s); err != nil {
		return err
	}
	beforeMatches, err := snapshotStateMatches(ctx, tx, s, false)
	if err != nil {
		return err
	}
	autoBeforeMatches, err := autoIncrementStatesMatch(ctx, tx, s.AutoIncrementsBefore)
	if err != nil {
		return err
	}
	afterMatches := false
	autoAfterMatches := false
	if s.AppliedAt != nil && s.AfterTasks != nil {
		afterMatches, err = snapshotStateMatches(ctx, tx, s, true)
		if err != nil {
			return err
		}
		autoAfterMatches, err = autoIncrementStatesMatch(ctx, tx, s.AutoIncrementsAfter)
		if err != nil {
			return err
		}
	}
	if s.ApplyState == "rolled_back" {
		if !beforeMatches || !autoBeforeMatches {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback refused: manifest is rolled_back but database no longer matches the before state")
		}
		return tx.Commit()
	}
	if s.ApplyState == "rollback_autoincrement_pending" {
		if !beforeMatches {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback refused: auto-increment recovery requires the exact before row state")
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return completeAutoIncrementRollback(ctx, db, path, &s)
	}
	if s.ApplyState == "rollback_dml_pending" && beforeMatches {
		if err := tx.Commit(); err != nil {
			return err
		}
		s.ApplyState = "rollback_autoincrement_pending"
		if err := writeSnapshot(path, s); err != nil {
			return err
		}
		return completeAutoIncrementRollback(ctx, db, path, &s)
	}
	if !afterMatches {
		if (s.ApplyState == "prepared" || s.ApplyState == "commit_pending") && beforeMatches {
			if err := tx.Commit(); err != nil {
				return err
			}
			if needsPreCommitAutoIncrementRecovery(s.ApplyState, beforeMatches, autoBeforeMatches) {
				if err := restoreAutoIncrementStatesWithinRecoveryCeilings(
					ctx,
					db,
					s.AutoIncrementsBefore,
					s.AutoIncrementRecoveryCeilings,
				); err != nil {
					return err
				}
			}
			s.ApplyState = "rolled_back"
			return writeSnapshot(path, s)
		}
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback refused: current database differs from both the recorded before and apply-after states; preserve forward writes and investigate")
	}
	if !autoAfterMatches {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback refused: auto-increment state differs from the recorded apply-after state")
	}
	if s.ApplyState != "applied" && s.ApplyState != "commit_pending" && s.ApplyState != "rollback_dml_pending" {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback refused: unsupported manifest apply_state %q", s.ApplyState)
	}
	if s.ApplyState != "rollback_dml_pending" {
		s.ApplyState = "rollback_dml_pending"
		if err := writeSnapshot(path, s); err != nil {
			return err
		}
	}
	rollbackTaskIDs := make([]int64, 0, len(s.Tasks))
	for _, task := range s.Tasks {
		rollbackTaskIDs = append(rollbackTaskIDs, task.ID)
	}
	for _, created := range s.PlanningCreated {
		beforeDetails := map[int64]planningDetailSnapshot{}
		for _, state := range s.PlanningBefore {
			if state.TaskID != created.TaskID {
				continue
			}
			for _, detail := range state.Details {
				beforeDetails[detail.TaskSKUItemID] = detail
			}
		}
		for _, detailID := range created.DetailIDs {
			if _, err = tx.ExecContext(ctx, `UPDATE task_planning_sku_details SET current_revision_id=NULL WHERE task_sku_item_id=?`, detailID); err != nil {
				return err
			}
		}
		for detailID, detail := range beforeDetails {
			if _, err = tx.ExecContext(ctx, `UPDATE task_planning_sku_details SET current_revision_id=?,lock_version=?,updated_at=? WHERE task_sku_item_id=?`, nullableInt64Pointer(detail.CurrentRevisionID), detail.LockVersion, detail.UpdatedAt, detailID); err != nil {
				return err
			}
		}
		for _, revisionID := range created.ImageRevisionIDs {
			if _, err = tx.ExecContext(ctx, `DELETE FROM task_planning_sku_revision_images WHERE revision_id=?`, revisionID); err != nil {
				return err
			}
		}
		for _, revisionID := range created.RevisionIDs {
			if _, err = tx.ExecContext(ctx, `DELETE FROM task_planning_sku_revisions WHERE id=?`, revisionID); err != nil {
				return err
			}
		}
		for _, detailID := range created.DetailIDs {
			if _, err = tx.ExecContext(ctx, `DELETE FROM task_planning_sku_details WHERE task_sku_item_id=?`, detailID); err != nil {
				return err
			}
		}
		if created.SettingsCreated {
			if _, err = tx.ExecContext(ctx, `DELETE FROM task_planning_settings WHERE task_id=?`, created.TaskID); err != nil {
				return err
			}
		}
	}
	if err := restoreSearchDocuments(
		ctx,
		tx,
		rollbackTaskIDs,
		s.SearchDocumentsBefore,
	); err != nil {
		return err
	}
	if err := restoreTaskSearchDocuments(
		ctx,
		tx,
		rollbackTaskIDs,
		s.TaskSearchDocumentsBefore,
	); err != nil {
		return err
	}
	for _, origin := range s.SKUOrigins {
		if _, err = tx.ExecContext(ctx, `UPDATE task_sku_items SET sku_origin=?,updated_at=? WHERE id=?`, nullableStringPointer(origin.Origin), origin.UpdatedAt, origin.ID); err != nil {
			return err
		}
	}
	if err := restoreAssetStorageRefStates(ctx, tx, s.StorageRefsBefore); err != nil {
		return err
	}
	for _, asset := range s.AssetBindings {
		if _, err = tx.ExecContext(ctx, `
			UPDATE task_assets
			SET binding_state=?,bound_group_id=?,bound_role=?,
			    staged_task_sku_item_id=?,staged_retouch_requirement_id=?,staged_role=?,staged_by=?,upload_session_id=?,staged_expires_at=?,
			    access_revoked_at=?,access_revoked_reason=?,object_deleted_at=?,scope_sku_code=?,retouch_requirement_id=?,
			    flow_review_status=?,approved_at=?,approved_by=?
			WHERE id=?`,
			asset.BindingState, nullableInt64Pointer(asset.BoundGroupID), nullableStringPointer(asset.BoundRole),
			nullableInt64Pointer(asset.StagedTaskSKUItemID), nullableInt64Pointer(asset.StagedRetouchRequirementID), nullableStringPointer(asset.StagedRole), nullableInt64Pointer(asset.StagedBy), nullableStringPointer(asset.UploadSessionID), nullableTimePointer(asset.StagedExpiresAt),
			nullableTimePointer(asset.AccessRevokedAt), asset.AccessRevokedReason, nullableTimePointer(asset.ObjectDeletedAt), nullableStringPointer(asset.ScopeSKUCode), nullableInt64Pointer(asset.RetouchRequirementID),
			asset.FlowReviewStatus, nullableTimePointer(asset.ApprovedAt), nullableInt64Pointer(asset.ApprovedBy), asset.ID); err != nil {
			return err
		}
	}
	for _, group := range s.ResourceGroups {
		if _, err = tx.ExecContext(ctx, `
			UPDATE task_asset_groups
			SET working_revision_id=?,finalized_revision_id=?,lock_version=?,migration_incomplete=?,migration_issue=?,updated_at=?
			WHERE id=?`, nullableInt64Pointer(group.WorkingRevisionID), nullableInt64Pointer(group.FinalizedRevisionID), group.LockVersion, group.MigrationIncomplete, group.MigrationIssue, group.UpdatedAt, group.ID); err != nil {
			return err
		}
	}
	for _, groupID := range s.InsertedGroupIDs {
		if _, err = tx.ExecContext(ctx, `UPDATE task_asset_groups SET working_revision_id=NULL,finalized_revision_id=NULL WHERE id=?`, groupID); err != nil {
			return err
		}
	}
	for _, revisionID := range s.AppliedRevisionIDs {
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_asset_group_revision_references WHERE revision_id=?`, revisionID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_asset_group_revision_items WHERE revision_id=?`, revisionID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_asset_group_revisions WHERE id=?`, revisionID); err != nil {
			return err
		}
	}
	for _, aliasID := range s.InsertedAliasIDs {
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_assets WHERE id=? AND asset_type='source' AND source_module_key='migration'`, aliasID); err != nil {
			return err
		}
	}
	for _, groupID := range s.InsertedGroupIDs {
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_asset_groups WHERE id=?`, groupID); err != nil {
			return err
		}
	}
	for _, t := range s.Tasks {
		if _, err = tx.ExecContext(ctx, `UPDATE tasks SET task_type=?,task_status=?,workflow_revision=?,current_handler_id=?,updated_at=? WHERE id=?`, t.TaskType, t.TaskStatus, t.WorkflowRevision, nullableInt64Pointer(t.CurrentHandlerID), t.UpdatedAt, t.ID); err != nil {
			return err
		}
	}
	if err := restoreTaskModuleStates(ctx, tx, s.TaskModulesBefore); err != nil {
		return err
	}
	for _, item := range s.OrganizationBefore {
		switch item.SubjectType {
		case "task":
			_, err = tx.ExecContext(ctx, `UPDATE tasks SET owner_department_id=?,owner_team_id=?,updated_at=? WHERE id=?`,
				nullableInt64Pointer(item.DepartmentID), nullableInt64Pointer(item.TeamID), item.UpdatedAt, item.SubjectID)
		case "user":
			_, err = tx.ExecContext(ctx, `UPDATE users SET department_id=?,team_id=?,updated_at=? WHERE id=?`,
				nullableInt64Pointer(item.DepartmentID), nullableInt64Pointer(item.TeamID), item.UpdatedAt, item.SubjectID)
		default:
			err = fmt.Errorf("rollback unsupported organization subject_type %q", item.SubjectType)
		}
		if err != nil {
			return err
		}
	}
	matches, err := snapshotStateMatches(ctx, tx, s, false)
	if err != nil {
		return err
	}
	if !matches {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback verification failed before commit")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.ApplyState = "rollback_autoincrement_pending"
	if err := writeSnapshot(path, s); err != nil {
		return err
	}
	return completeAutoIncrementRollback(ctx, db, path, &s)
}

func completeAutoIncrementRollback(ctx context.Context, db *sql.DB, path string, s *snapshot) error {
	if err := restoreAutoIncrementStates(ctx, db, s.AutoIncrementsBefore, s.AutoIncrementsAfter); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockRollbackTargets(ctx, tx, *s); err != nil {
		return err
	}
	rowsMatch, err := snapshotStateMatches(ctx, tx, *s, false)
	if err != nil {
		return err
	}
	autoIncrementsMatch, err := autoIncrementStatesMatch(ctx, tx, s.AutoIncrementsBefore)
	if err != nil {
		return err
	}
	if !rowsMatch || !autoIncrementsMatch {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback verification failed after auto-increment recovery")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.ApplyState = "rolled_back"
	return writeSnapshot(path, *s)
}

func lockRollbackTargets(ctx context.Context, tx *sql.Tx, s snapshot) error {
	taskIDs := make([]int64, 0, len(s.Tasks)+len(s.AfterTasks))
	for _, item := range s.Tasks {
		taskIDs = append(taskIDs, item.ID)
	}
	for _, item := range s.AfterTasks {
		taskIDs = append(taskIDs, item.ID)
	}
	userIDs := []int64{}
	for _, item := range append(append([]organizationStateSnapshot{}, s.OrganizationBefore...), s.OrganizationAfter...) {
		if item.SubjectType == "task" {
			taskIDs = append(taskIDs, item.SubjectID)
		} else if item.SubjectType == "user" {
			userIDs = append(userIDs, item.SubjectID)
		}
	}
	for _, item := range append(append([]accessStateSnapshot{}, s.AccessBefore...), s.AccessAfter...) {
		userIDs = append(userIDs, item.UserID)
	}
	for _, userID := range uniqueSortedInt64(userIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM users WHERE id=? FOR UPDATE`, userID); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM auth_user_role_assignments WHERE user_id=? ORDER BY id FOR UPDATE`, userID); err != nil {
			return err
		}
	}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM tasks WHERE id=? FOR UPDATE`, taskID); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_sku_items WHERE task_id=? ORDER BY id FOR UPDATE`, taskID); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_modules WHERE task_id=? ORDER BY id FOR UPDATE`, taskID); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `SELECT group_id FROM task_asset_group_search_documents WHERE task_id=? ORDER BY group_id FOR UPDATE`, taskID); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `SELECT task_id FROM task_search_documents WHERE task_id=? FOR UPDATE`, taskID); err != nil {
			return err
		}
	}
	groupIDs := []int64{}
	for _, item := range s.ResourceGroups {
		groupIDs = append(groupIDs, item.ID)
	}
	for _, item := range s.AfterResourceGroups {
		groupIDs = append(groupIDs, item.ID)
	}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE task_id=? ORDER BY id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, ids...)
	}
	for _, groupID := range uniqueSortedInt64(groupIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE id=? FOR UPDATE`, groupID); err != nil {
			return err
		}
		revisionIDs, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_group_revisions WHERE group_id=? ORDER BY id FOR UPDATE`, groupID)
		if err != nil {
			return err
		}
		for _, revisionID := range revisionIDs {
			if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_group_revision_items WHERE revision_id=? ORDER BY id FOR UPDATE`, revisionID); err != nil {
				return err
			}
			if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_group_revision_references WHERE revision_id=? ORDER BY id FOR UPDATE`, revisionID); err != nil {
				return err
			}
		}
	}
	assetIDs := []int64{}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_assets WHERE task_id=? ORDER BY id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		assetIDs = append(assetIDs, ids...)
	}
	for _, item := range s.AssetBindings {
		assetIDs = append(assetIDs, item.ID)
	}
	for _, item := range s.AfterAssetBindings {
		assetIDs = append(assetIDs, item.ID)
	}
	for _, assetID := range uniqueSortedInt64(assetIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_assets WHERE id=? FOR UPDATE`, assetID); err != nil {
			return err
		}
	}
	storageRefIDs := make([]string, 0, len(s.StorageRefsBefore)+len(s.StorageRefsAfter))
	for _, item := range s.StorageRefsBefore {
		storageRefIDs = append(storageRefIDs, item.RefID)
	}
	for _, item := range s.StorageRefsAfter {
		storageRefIDs = append(storageRefIDs, item.RefID)
	}
	sort.Strings(storageRefIDs)
	previousStorageRefID := ""
	for _, refID := range storageRefIDs {
		if refID == previousStorageRefID {
			continue
		}
		if _, err := queryStringIDs(ctx, tx, `SELECT ref_id FROM asset_storage_refs WHERE ref_id=? FOR UPDATE`, refID); err != nil {
			return err
		}
		previousStorageRefID = refID
	}
	planningTaskIDs := []int64{}
	for _, item := range s.PlanningBefore {
		planningTaskIDs = append(planningTaskIDs, item.TaskID)
	}
	for _, item := range s.PlanningAfter {
		planningTaskIDs = append(planningTaskIDs, item.TaskID)
	}
	for _, taskID := range uniqueSortedInt64(planningTaskIDs) {
		queries := []string{
			`SELECT task_id FROM task_planning_settings WHERE task_id=? FOR UPDATE`,
			`SELECT d.task_sku_item_id FROM task_planning_sku_details d JOIN task_sku_items si ON si.id=d.task_sku_item_id WHERE si.task_id=? ORDER BY d.task_sku_item_id FOR UPDATE`,
			`SELECT r.id FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY r.id FOR UPDATE`,
			`SELECT i.revision_id FROM task_planning_sku_revision_images i JOIN task_planning_sku_revisions r ON r.id=i.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY i.revision_id FOR UPDATE`,
		}
		for _, query := range queries {
			if _, err := lockInt64Rows(ctx, tx, query, taskID); err != nil {
				return err
			}
		}
	}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM task_event_logs WHERE task_id=? ORDER BY sequence,id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `
			SELECT e.id
			FROM task_module_events e
			JOIN task_modules tm ON tm.id=e.task_module_id
			WHERE tm.task_id=?
			ORDER BY e.id FOR UPDATE`, taskID); err != nil {
			return err
		}
	}
	return nil
}

func snapshotStateMatches(ctx context.Context, q snapshotQueryer, s snapshot, after bool) (bool, error) {
	tasks := s.Tasks
	groups := s.ResourceGroups
	assets := s.AssetBindings
	origins := s.SKUOrigins
	planning := s.PlanningBefore
	organizations := s.OrganizationBefore
	access := s.AccessBefore
	storageRefs := s.StorageRefsBefore
	if after {
		tasks = s.AfterTasks
		groups = s.AfterResourceGroups
		assets = s.AfterAssetBindings
		origins = s.AfterSKUOrigins
		planning = s.PlanningAfter
		organizations = s.OrganizationAfter
		access = s.AccessAfter
		storageRefs = s.StorageRefsAfter
	}
	taskIDsForGroups := make([]int64, 0, len(tasks))
	for _, item := range tasks {
		taskIDsForGroups = append(taskIDsForGroups, item.ID)
	}
	actualTasks, err := captureTaskSnapshotsBulk(ctx, q, taskIDsForGroups)
	if errors.Is(err, errBulkSnapshotMissingRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	expectedTasks := append([]taskSnapshot(nil), tasks...)
	for index := range actualTasks {
		if s.Version == legacySnapshotVersion {
			actualTasks[index].ModuleEventIDs = nil
			expectedTasks[index].ModuleEventIDs = nil
		}
		if len(actualTasks[index].ModuleEventIDs) == 0 {
			actualTasks[index].ModuleEventIDs = nil
		}
		if len(expectedTasks[index].ModuleEventIDs) == 0 {
			expectedTasks[index].ModuleEventIDs = nil
		}
	}
	if len(tasks) > 0 && !reflect.DeepEqual(actualTasks, expectedTasks) {
		return false, nil
	}
	actualModules, err := captureTaskModulesForTasks(ctx, q, taskIDsForGroups)
	if err != nil {
		return false, err
	}
	expectedModules := append(
		[]taskModuleSnapshot(nil),
		s.TaskModulesBefore...,
	)
	if after {
		expectedModules = append(
			[]taskModuleSnapshot(nil),
			s.TaskModulesAfter...,
		)
	}
	if err := normalizeTaskModuleSnapshotJSON(actualModules); err != nil {
		return false, err
	}
	if err := normalizeTaskModuleSnapshotJSON(expectedModules); err != nil {
		return false, err
	}
	if len(actualModules) == 0 {
		actualModules = nil
	}
	if len(expectedModules) == 0 {
		expectedModules = nil
	}
	if !reflect.DeepEqual(actualModules, expectedModules) {
		return false, nil
	}
	expectedSearchDocuments := s.SearchDocumentsBefore
	if after {
		expectedSearchDocuments = s.SearchDocumentsAfter
	}
	searchDocumentsMatch, err := searchDocumentsMatchSnapshot(
		ctx,
		q,
		taskIDsForGroups,
		expectedSearchDocuments,
	)
	if err != nil {
		return false, err
	}
	if !searchDocumentsMatch {
		return false, nil
	}
	expectedTaskSearchDocuments := s.TaskSearchDocumentsBefore
	if after {
		expectedTaskSearchDocuments = s.TaskSearchDocumentsAfter
	}
	taskSearchDocumentsMatch, err := taskSearchDocumentsMatchSnapshot(
		ctx,
		q,
		taskIDsForGroups,
		expectedTaskSearchDocuments,
	)
	if err != nil {
		return false, err
	}
	if !taskSearchDocumentsMatch {
		return false, nil
	}
	actualGroups, err := captureResourceGroupsForTasks(ctx, q, taskIDsForGroups)
	if err != nil {
		return false, err
	}
	if s.Version == legacySnapshotVersion {
		for i := range actualGroups {
			actualGroups[i].Revisions = nil
		}
		for i := range groups {
			groups[i].Revisions = nil
		}
	}
	for i := range actualGroups {
		if len(actualGroups[i].Revisions) == 0 {
			actualGroups[i].Revisions = nil
		}
	}
	for i := range groups {
		if len(groups[i].Revisions) == 0 {
			groups[i].Revisions = nil
		}
	}
	if !reflect.DeepEqual(actualGroups, groups) {
		return false, nil
	}
	actualAssets, err := captureAssetBindingsForTasks(ctx, q, taskIDsForGroups)
	if err != nil {
		return false, err
	}
	if s.Version == legacySnapshotVersion {
		clearAssetValidationState(actualAssets)
		clearAssetValidationState(assets)
	}
	if !reflect.DeepEqual(actualAssets, assets) {
		return false, nil
	}
	taskIDs := make([]int64, 0, len(planning))
	for _, item := range planning {
		taskIDs = append(taskIDs, item.TaskID)
	}
	actualOrigins, err := captureSKUOrigins(ctx, q, taskIDs)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(actualOrigins, origins) {
		return false, nil
	}
	actualPlanning, err := capturePlanningStates(ctx, q, taskIDs)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(actualPlanning, planning) {
		return false, nil
	}
	var actualOrganizations []organizationStateSnapshot
	if len(organizations) > 0 {
		actualOrganizations = make([]organizationStateSnapshot, 0, len(organizations))
	}
	for _, expected := range organizations {
		actual, err := loadOrganizationState(ctx, q, expected.SubjectType, expected.SubjectID)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		actualOrganizations = append(actualOrganizations, actual)
	}
	if !reflect.DeepEqual(actualOrganizations, organizations) {
		return false, nil
	}
	var actualAccess []accessStateSnapshot
	if len(access) > 0 {
		actualAccess = make([]accessStateSnapshot, 0, len(access))
	}
	for _, expected := range access {
		assignments, err := loadAccessAssignmentEvidence(ctx, q, expected.UserID)
		if err != nil {
			return false, err
		}
		actualAccess = append(actualAccess, accessStateSnapshot{UserID: expected.UserID, Assignments: assignments})
	}
	if !reflect.DeepEqual(actualAccess, access) {
		return false, nil
	}
	var actualStorageRefs []assetStorageRefStatusSnapshot
	if len(storageRefs) > 0 {
		actualStorageRefs = make([]assetStorageRefStatusSnapshot, 0, len(storageRefs))
	}
	for _, expected := range storageRefs {
		actual, err := loadAssetStorageRefStatusSnapshot(ctx, q, expected.RefID)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		actualStorageRefs = append(actualStorageRefs, actual)
	}
	return reflect.DeepEqual(actualStorageRefs, storageRefs), nil
}

func clearAssetValidationState(items []assetBindingSnapshot) {
	for i := range items {
		items[i].AssetType = ""
		items[i].ScopeSKUCode = nil
		items[i].RetouchRequirementID = nil
		items[i].MimeType = ""
		items[i].WholeHash = ""
		items[i].DeletedAt = nil
		items[i].CleanedAt = nil
	}
}

func nullableInt64Pointer(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullableStringPointer(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTimePointer(value *time.Time) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func mappingDigest(m mappingFile) (string, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func exactFileDigest(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func snapshotDigest(s snapshot) (string, error) {
	s.IntegritySHA256 = ""
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func readSnapshot(path, database string, m mappingFile) (snapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return snapshot{}, err
	}
	var header struct {
		Version       int    `json:"version"`
		ToolVersion   string `json:"tool_version"`
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return snapshot{}, err
	}
	currentSnapshot := header.Version == workflowGroupsSnapshotVersion &&
		header.ToolVersion == workflowGroupsToolVersion &&
		header.SchemaVersion == workflowGroupsSchemaVersion
	knownHistoricalSnapshot := (header.Version == previousSnapshotVersion && header.ToolVersion == previousToolVersion) ||
		(header.Version == olderSnapshotVersion && header.ToolVersion == olderToolVersion) ||
		(header.Version == historicalSnapshotVersion && header.ToolVersion == historicalToolVersion) ||
		(header.Version == legacySnapshotVersion && header.ToolVersion == legacyToolVersion)
	if knownHistoricalSnapshot && header.SchemaVersion == workflowGroupsSchemaVersion {
		return snapshot{}, v1migrate.NewHardAbort(
			v1migrate.ExitCodeHardAbort,
			"snapshot v%d predates lossless v9 rollback; use the matching historical tool or prepare a reviewed recovery",
			header.Version,
		)
	}
	if !currentSnapshot {
		return snapshot{}, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "snapshot tool/schema version mismatch")
	}
	var s snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return snapshot{}, err
	}
	if s.Database != database {
		return snapshot{}, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "snapshot database %q does not match %q", s.Database, database)
	}
	expectedMappingSHA256, err := mappingDigest(m)
	if err != nil {
		return snapshot{}, err
	}
	if s.MappingSHA256 != expectedMappingSHA256 {
		return snapshot{}, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "snapshot mapping digest does not match the reviewed mapping file")
	}
	expectedIntegrity, err := snapshotDigest(s)
	if err != nil {
		return snapshot{}, err
	}
	if s.IntegritySHA256 == "" || s.IntegritySHA256 != expectedIntegrity {
		return snapshot{}, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "snapshot integrity verification failed")
	}
	return s, nil
}

func writeSnapshot(path string, s snapshot) error {
	digest, err := snapshotDigest(s)
	if err != nil {
		return err
	}
	s.IntegritySHA256 = digest
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".workflow-groups-snapshot-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		_ = tmp.Close()
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	dirHandle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirHandle.Close()
	return dirHandle.Sync()
}

func removeSnapshot(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dirHandle, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dirHandle.Close()
	return dirHandle.Sync()
}
func writeJSONReport(v any, path string) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if path != "" {
		return os.WriteFile(path, raw, 0o600)
	}
	_, err = os.Stdout.Write(raw)
	return err
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
